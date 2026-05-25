# Phase 7.5 — Agent 核心能力完善 设计规范

> 日期: 2026-05-25 | 版本: v1.0 | 状态: 设计完成

## 概要

补完 OpenForge Agent 的两个核心短板：多轮 Function Call 循环和 LLM 驱动的对话摘要压缩。两者互为依赖——多轮调用会产生大量 tool_use/tool_result 消息需压缩，而摘要依赖理解哪些 tool 调用结果关键。

---

## §1 多轮 Function Call 循环

### 1.1 循环流程

```
runLLMLoop(ctx, messages):
  while true:  (硬上限 50 轮)
    ├─ 1. systemPrompt = PromptBuilder.Build(ctx, buildReq)
    ├─ 2. response = LLM.ChatStream(ctx, ChatRequest{
    │        SystemPrompt: systemPrompt,
    │        Messages:     messages,
    │        Config:       qe.config,
    │    })
    │    └─ tokenUsed += response.Usage  (API 精确值; 降级: len/4)
    │
    ├─ 3. 解析 stop_reason:
    │   ├─ "end_turn"  → 模型自然结束, break
    │   └─ "tool_use"  →
    │       ├─ 并行组: ToolRegistry[tc.Name].IsConcurrencySafe==true → goroutine 并发
    │       ├─ 串行组: ToolRegistry[tc.Name].IsConcurrencySafe==false → 顺序执行
    │       ├─ 输出: 结构化 FormattedToolResult
    │       ├─ 失败: MapToolErrorToFailureCode → ClassifyAndRecover
    │       └─ messages += tool_results
    │
    ├─ 4. 每轮 LLM 完成后检查 token:
    │   ├─ > 80% context → compress(messages)
    │   │   ├─ 仍 > 80% → compress(messages, aggressive)
    │   │   └─ 仍超 → saveCheckpoint("context_overflow") + return error
    │   └─ ≤ 80% → 继续
    │
    └─ 5. messages += assistant_response
```

### 1.2 Query State 扩展

在现有 `QueryStateIdle` / `QueryStateAwaitingUser` 基础上增加:

```
IDLE → AWAITING_LLM → AWAITING_TOOLS → IDLE
                                    ↘ AWAITING_USER (循环结束)
```

### 1.3 退出条件

优先级从高到低:
1. LLM 返回 `stop_reason=end_turn` (自然结束)
2. 本轮无 tool_use (LLM 用文本回复)
3. 达到 50 轮硬上限 (安全兜底, WARN 日志)
4. context_overflow 且二次压缩仍超

### 1.4 Round 定义

一轮 = user message + assistant response + N 个 tool_result (0 或多个):

```go
type Round struct {
    User      Message   // 用户消息
    Assistant Message   // LLM 回复 (可能包含 tool_use)
    Tools     []Message // 0 或多个 tool_result
}
```

压缩和 context window 管理以 Round 为单位, 保留最近 N 个 Round。

### 1.5 与现有 SubmitMessage 的兼容

`SubmitMessage` 外部接口不变。内部改为调用 `runLLMLoop`。返回的 `StreamEvent` 增加 `"tool_start"` 和 `"tool_done"` 类型,前端可实时展示。

---

## §2 并行工具执行

### 2.1 ToolRegistry + ToolMeta

在 `tool_executor.go` 中定义工具注册表。每个已注册工具通过 name lookup:

```go
type ToolMeta struct {
    Name              string
    IsConcurrencySafe bool
    Timeout           time.Duration
    Executor          func(ctx context.Context, args map[string]interface{}) (string, error)
}

type ToolRegistry map[string]ToolMeta
```

### 2.2 分组逻辑

```go
func (reg ToolRegistry) partition(toolCalls []ToolCallParsed) []ToolCallGroup {
    // reg[tc.Name].IsConcurrencySafe == true → parallel group
    // reg[tc.Name].IsConcurrencySafe == false → individual serial groups
}
```

### 2.3 执行策略

| 工具类型 | 执行方式 | 示例 |
|---------|---------|------|
| IsConcurrencySafe=true | goroutine 并发 | read_file, grep, glob, lsp_* |
| IsConcurrencySafe=false | 顺序执行 | write_file, edit_file, bash |

并发组内任一工具报错 → 兄弟工具继续,不级联中止。串行组按调用顺序执行,前一个失败不阻断后一个(所有结果都反馈给 LLM)。

### 2.4 并发数量限制

限制并发 goroutine 数量 (默认 8), 超过排队等待:

```go
const maxParallel = 8
var sem = make(chan struct{}, maxParallel)

func (reg ToolRegistry) executeParallel(tools []ToolCallParsed) []FormattedToolResult {
    var wg sync.WaitGroup
    results := make([]FormattedToolResult, len(tools))
    for i, tc := range tools {
        wg.Add(1)
        go func(idx int, tc ToolCallParsed) {
            defer wg.Done()
            sem <- struct{}{}        // acquire
            defer func() { <-sem }() // release
            results[idx] = reg.executeOne(tc)
        }(i, tc)
    }
    wg.Wait()
    return results
}
```

**不是**限制单次 tool_use 调用的数量 — 一次可以请求任意数量的只读工具, 但 goroutine 并发数受 semaphore 限制。

### 2.5 超时控制

每个 tool 使用 `ToolMeta.Timeout` (独立配置, 默认 60s)。超时 → ctx.Cancel → 返回 timeout error 给 LLM。

---

## §3 工具输出格式化

### 3.1 统一格式

```go
type FormattedToolResult struct {
    Tool          string   `json:"tool"`           // e.g. "bash", "write_file"
    Status        string   `json:"status"`         // "success" | "error" | "degraded"
    Output        string   `json:"output"`         // human-readable result
    ModifiedFiles []string `json:"modified_files,omitempty"`
    ErrorCode     string   `json:"error_code,omitempty"`  // FailureCode
    Retries       int      `json:"retries"`
    DurationMs    int64    `json:"duration_ms"`
}
```

### 3.2 注入 messages 的方式

```json
{
  "role": "tool",
  "content": "<tool_result tool=\"bash\" status=\"success\" duration_ms=\"1234\">\n 实际输出内容\n</tool_result>"
}
```

---

## §4 错误恢复集成

### 4.1 复用 error_recovery.go

现有 `error_recovery.go` 已有完整的 `FailureCode` 映射和 `ClassifyAndRecover` 四层分类。tool 执行失败时:

```go
func (qe *QueryEngine) executeOne(tc ToolCallParsed) FormattedToolResult {
    start := time.Now()
    for attempt := 0; attempt <= qe.config.ToolErrorPolicy.MaxRetries; attempt++ {
        output, err := tc.Execute(ctx)
        if err == nil {
            return FormattedToolResult{Tool: tc.Name, Status: "success", Output: format(output), DurationMs: ms(start)}
        }
        code := MapToolErrorToFailureCode(err)
        recovery := ClassifyAndRecover(code, attempt)

        switch recovery.Action {
        case ActionRetry:     // Layer 1 TRANSIENT
            time.Sleep(qe.retryDelay(attempt))
            continue
        case ActionCompress, ActionDowngradeModel: // Layer 2 DEGRADABLE
            return FormattedToolResult{Tool: tc.Name, Status: "degraded", Output: recovery.Message, ErrorCode: string(code), Retries: attempt}
        case ActionSelfRepair, ActionClarify: // Layer 3 RECOVERABLE
            return FormattedToolResult{Tool: tc.Name, Status: "error", Output: recovery.Message, ErrorCode: string(code), Retries: attempt}
        default: // Layer 4 FATAL
            return FormattedToolResult{Tool: tc.Name, Status: "error", Output: err.Error(), ErrorCode: string(code), Retries: attempt}
        }
    }
}
```

---

## §5 对话摘要压缩

### 5.1 触发时机

**仅在每轮 LLM 响应完成后检测**,不在每个 tool_use 后检测。原因: 一轮 LLM 调用内的多个 tool_use 是连贯的,中间插入压缩会破坏上下文。

### 5.2 压缩算法

```
compress(messages, mode):
  ├─ normal mode:
  │   ├─ keepLast = 最近 10 轮 (user+assistant+tool 对)
  │   └─ toCompress = messages[:len-keepLast]
  │       └─ Haiku 生成摘要 → <conversation_summary>
  │       └─ messages = [summary] + keepLast
  │
  ├─ aggressive mode (normal 后仍 > 80%):
  │   ├─ keepLast = 最近 5 轮
  │   └─ toCompress = messages[:len-keepLast]
  │       └─ Haiku 再一次摘要
  │       └─ messages = [summary] + keepLast
  │
  └─ 二次压缩后仍 > 80%:
      └─ saveCheckpoint("context_overflow")
      └─ return error("context_overflow: unable to compress")
```

### 5.3 摘要 Prompt

```
Summarize the following conversation and tool interactions.

Preserve:
1. Key decisions and their rationale
2. Error messages and their resolutions
3. Code changes made (file paths, function names)
4. Current task progress and next steps
5. Any pending issues or blockers

Format as structured bullet points.
```

### 5.4 摘要注入格式

```
<conversation_summary compressed="true" rounds="25" original_tokens="120000">
  ... 摘要内容 ...
</conversation_summary>
```

---

## §6 Token 追踪

### 6.1 数据来源

```go
// 优先: API 返回的 usage
if response.Usage != nil {
    qe.tokenUsed += response.Usage.InputTokens + response.Usage.OutputTokens
    // 注: 累加而非替代, 非首次调用需 +=
}

// 降级: 字符估算
else {
    qe.tokenUsed += len(response.Content) / 4
}
```

### 6.2 Context Window 参考值

| 模型 | Context Window |
|------|---------------|
| Opus | 200,000 |
| Sonnet | 200,000 |
| Haiku | 200,000 |
| DeepSeek | 131,072 |

从模型注册表 `ModelEntry.Features.ContextWindow` 读取。80% 阈值 = `contextWindow * 0.8`。

---

## §7 溢出处理

### 7.1 QueryEngine 新增字段

```go
type QueryEngine struct {
    // ... 现有字段 (llmClient, config, messages, tokenCount, state, promptBuilder, pipelineCtx, forceSkill, mu)

    toolRegistry    ToolRegistry        // 工具注册表
    roundCount      int32               // 当前轮数
    checkpointSeq   int32               // 检查点序号
    checkpointCache []*Checkpoint       // 内存最近 3 个检查点
    checkpointRepo  CheckpointRepository // PG 异步持久化
}
```

### 7.2 与检查点系统集成

```go
func (qe *QueryEngine) saveCheckpoint(reason string) {
    cp := &Checkpoint{
        PipelineID:  qe.pipelineCtx.PipelineID,
        Stage:       qe.pipelineCtx.Stage,
        Seq:         atomic.AddInt32(&qe.checkpointSeq, 1),
        Trigger:     "auto", // 区别于用户触发的 "manual"
        Data: CheckpointData{
            Messages:   qe.snapshotMessages(),
            TokenUsed:  qe.tokenUsed,
            RoundCount: qe.roundCount,
            Reason:     reason,
            Timestamp:  time.Now(),
        },
    }
    // 先写内存缓存 (最近 3 个)
    qe.checkpointCache.push(cp)
    if len(qe.checkpointCache) > 3 {
        qe.checkpointCache = qe.checkpointCache[1:]
    }
    // 异步 flush PG
    go func() {
        if err := qe.checkpointRepo.Save(context.Background(), cp); err != nil {
            log.Printf("[WARN] checkpoint save failed: %v", err)
        }
    }()
}
```

### 7.3 恢复路径

服务重启 → `recoverFlyingPipelines()` 扫描 PG `WHERE status='running'` → 从 Checkpoint 恢复 `Messages + TokenUsed + RoundCount` → `QueryEngine.Resume()`。

---

## §8 前端事件

### 8.1 新增 WebSocket 下行事件

| 事件 | Payload | 说明 |
|------|---------|------|
| `tool.start` | `{tool_name, args_summary}` | 工具开始执行 |
| `tool.done` | `{tool_name, status, duration_ms}` | 工具执行完成 |
| `tool.error` | `{tool_name, error_code, message, retries}` | 工具执行失败 |
| `context.compress` | `{before_tokens, after_tokens, rounds_compressed}` | 压缩触发通知 |

### 8.2 前端展示

- `MessageList` 新增 `ToolCallCard` 组件 — 在 Agent 消息下方展开显示 tool 调用详情
- 流式输出时, cursor 闪烁不中断

---

## §9 降级策略

| 故障 | 行为 |
|------|------|
| Haiku API 不可用 | 退化为纯文本截断 (Phase 6.5 现有行为) |
| 工具执行超时 | 返回 error 结果给 LLM, 不终止循环 |
| 50 轮硬上限触发 | 退出循环 + WARN 日志 + 前端通知 |
| context_overflow | saveCheckpoint + 前端通知 "上下文溢出, 已保存状态" |
| API 不返回 usage | 降级为 len/4 估算 |

---

## §10 文件清单

```
internal/agent/domain/
  ├── query_engine.go            # MODIFY: SubmitMessage → runLLMLoop
  ├── tool_executor.go           # NEW: executeTools + executeOne + 格式化
  ├── context_compressor.go      # NEW: compress() + Haiku 摘要
  ├── tool_executor_test.go      # NEW
  ├── context_compressor_test.go # NEW
  └── query_engine_test.go       # MODIFY: 新增 runLLMLoop 测试

internal/agent/port/
  └── llm_client.go              # MODIFY: ChatRequest +Usage 字段

internal/llm/
  └── anthropic_provider.go      # MODIFY: 解析 stop_reason + usage

frontend/src/features/chat/
  ├── ChatProvider.tsx           # MODIFY: 订阅 tool.* / context.compress 事件
  └── ToolCallCard.tsx           # MODIFY: 展开详情
```

---

## §11 配置

```yaml
# config/agent_engine.yaml — Phase 7.5 新增
agent:
  loop:
    max_rounds: 50
    context_threshold: 0.8       # 80% 触发压缩
    keep_rounds_normal: 10
    keep_rounds_aggressive: 5

  tool:
    timeout_ms: 60000
    parallel_max: 8              # 最大并发 goroutine 数 (超过排队)

  compression:
    summary_model: haiku          # 摘要专用模型
    max_summary_tokens: 2000
```

---

## §12 验收标准

| # | Criterion |
|---|-----------|
| 1 | LLM 返回 tool_use → 自动执行 → tool_result 反馈 → 继续推理 (最⻓ 50 轮) |
| 2 | 并行安全工具并发执行, 非安全工具串行执行 |
| 3 | 工具失败走 error_recovery.go 四层恢复链, 结果反馈 LLM |
| 4 | 工具输出统一格式 (tool/status/output/error_code/duration_ms) |
| 5 | 每轮 LLM 后检测 token > 80% → Haiku 摘要压缩 |
| 6 | 压缩后仍超 → aggressive 二次压缩 → 仍超 → saveCheckpoint + error |
| 7 | Token 优先用 API usage, 降级 len/4 |
| 8 | context_overflow → 内存缓存 + 异步 PG, 与现有 checkpoint.go 集成 |
| 9 | 前端显示 tool 执行状态 (tool.start/tool.done/tool.error 事件) |
| 10 | Haiku 不可用 → 退化为纯文本截断 |
| 11 | go test ./... + go vet + frontend tsc 通过 |
