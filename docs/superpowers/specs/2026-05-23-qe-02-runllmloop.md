# QE-02: Submit 内部 LLM 推理循环 (`runLLMLoop`)

> 日期: 2026-05-23 | 关联: DESIGN.md §4.2, §4.4, §3.15

## 循环结构

```go
// runLLMLoop — Submit 和 Resume 共用的核心循环。
// P0 修正: 锁仅在构建请求和处理结果时持有，LLM 调用期间释放锁。
func (qe *QueryEngine) runLLMLoop(ctx context.Context) (*SubmitResult, error) {
    var reply string
    var toolCalls []ToolCallRecord
    roundCount := qe.resumeRound // Resume 时从挂起点恢复轮数 (P0: roundCount 持久化)

    for roundCount < qe.config.MaxToolRounds {
        select {
        case <-ctx.Done():
            return &SubmitResult{Status: SubmitError, Error: ctx.Err().Error()}, nil
        default:
        }

        // 1. 构建 LLM 请求 (需要锁)
        qe.mu.Lock()
        req := port.ChatRequest{
            Messages: qe.history,
            Config:   qe.buildLLMConfig(),
        }
        qe.mu.Unlock() // P0: 释放锁 — LLM 调用可能阻塞数十秒

        // 2. 调用 LLM (无锁)
        qe.state = QueryStateAwaitingLLM
        response, err := qe.llmClient.ChatStream(ctx, req)
        if err != nil {
            return &SubmitResult{Status: SubmitError, Error: fmt.Sprintf("llm: %v", err)}, nil
        }

        // 3. 解析响应 + 累加 Token (需要锁)
        qe.mu.Lock()
        parsed := qe.parseStreamResponse(response)
        qe.tokenUsed += parsed.TokenUsed        // P0: TokenUsed 累加, 防止预算超支
        if qe.tokenUsed >= qe.config.TokenBudget {
            qe.mu.Unlock()
            return &SubmitResult{Status: SubmitError, Error: "token budget exceeded"}, nil
        }
        if parsed.Text != "" {
            reply = parsed.Text
        }

        // 4. 无 tool_use → 对话完成
        if len(parsed.ToolCalls) == 0 {
            qe.history = append(qe.history, port.Message{Role: "agent", Content: reply})
            qe.state = QueryStateAwaitingUser
            qe.mu.Unlock()
            break
        }

        // 5. 权限预检 + 工具分组 (P0: 在工具执行前检查 Gate，不在执行后)
        groups := qe.analyzeDependencies(parsed.ToolCalls)

        // 检测是否有需要 Gate 的工具
        var gateTool *ToolCallParsed
        for i := range groups {
            for j, tc := range groups[i].Tools {
                result := qe.executeToolWithPermission(ctx, tc)
                if result.GateRequired {
                    gateTool = &groups[i].Tools[j]
                    break
                }
                // 正常执行结果追加
                toolCalls = append(toolCalls, ToolCallRecord{
                    Name: tc.Name, Args: tc.Args,
                    Output: result.Output, Err: result.Err, ModifiedFiles: result.ModifiedFiles,
                })
                qe.history = append(qe.history, port.Message{
                    Role: "tool", Content: qe.formatToolResult(tc.Name, result),
                })
            }
            if gateTool != nil { break }
        }
        qe.mu.Unlock()

        // 6. Gate 挂起 (锁外处理, 不阻塞其他操作)
        if gateTool != nil {
            return qe.handleGatePause(ctx, *gateTool), nil
        }

        // 7. 无 Gate 工具 → 继续循环
        roundCount++
    }

    return &SubmitResult{
        Status:    SubmitCompleted,
        Reply:     reply,
        ToolCalls: toolCalls,
        TokenUsed: qe.tokenUsed,
    }, nil
}
```

## parseStreamResponse — 流式解析

```go
type ParsedResponse struct {
    Text      string
    ToolCalls []ToolCallParsed
}

type ToolCallParsed struct {
    ID       string
    Name     string
    Args     map[string]interface{}
    IsReadOnly bool      // 从 ToolRegistry 查询
}

func (qe *QueryEngine) parseStreamResponse(stream <-chan string) ParsedResponse {
    var r ParsedResponse
    var buf strings.Builder
    for delta := range stream { buf.WriteString(delta) }
    // 解析 SSE 帧: data: {...} → 累积 text → 提取 tool_use
    // 简化版本: 假设完整响应后解析 (Phase 1.5)
    r.Text = buf.String()
    return r
}
```

## handleGatePause — Gate 挂起

```go
func (qe *QueryEngine) handleGatePause(ctx context.Context, tc ToolCallParsed, result ToolResult) *SubmitResult {
    now := time.Now()
    qe.pendingGate = &PendingGateState{
        PendingID:    fmt.Sprintf("gate-%s-%d", qe.pipelineID, time.Now().UnixNano()),
        GateRequest: GateRequest{
            PipelineID:   qe.pipelineID,
            Stage:        qe.CurrentStage(),
            ToolName:     tc.Name,
            Reason:       fmt.Sprintf("%s requires Gate approval", tc.Name),
            ChangedFiles: qe.collectChangedFiles(),
            ArtifactHash: qe.calcHistoryHash(),
            CreatedAt:    now,
            ExpiresAt:    now.Add(qe.config.GateTimeout),
        },
        ToolCall:     tc,
        History:      qe.snapshotHistory(),
        TokenUsed:    qe.tokenUsed,
        ArtifactHash: qe.calcHistoryHash(),
        CreatedAt:    now,
        ExpiresAt:    now.Add(qe.config.GateTimeout),
        Status:       "pending",
    }

    // 持久化
    if qe.gateRepo != nil {
        qe.gateRepo.Create(context.Background(), qe.pendingGate)
    }

    qe.state = QueryStateAwaitingGate
    return &SubmitResult{
        Status:      SubmitPendingGate,
        GateRequest: &qe.pendingGate.GateRequest,
        TokenUsed:   qe.tokenUsed,
    }
}
```

## executeToolWithPermission — 权限判定 (调用 §4.4)

```go
type ToolResult struct {
    Output       interface{}
    Err          error
    GateRequired bool
}

func (qe *QueryEngine) executeToolWithPermission(ctx context.Context, tc ToolCallParsed) ToolResult {
    tool := qe.toolRegistry.Get(tc.Name)
    mode := auth.SelectMode(qe.pipelineLevel, qe.CurrentStage())

    decision := auth.Classify(auth.PermissionContext{
        Mode:       mode,
        ToolName:   tc.Name,
        IsReadOnly: tool.IsReadOnly(),
    })

    switch decision {
    case auth.DecisionAllow:
        output, err := qe.executeToolWithPolicy(ctx, tc)
        return ToolResult{Output: output, Err: err}

    case auth.DecisionDeny:
        return ToolResult{Err: fmt.Errorf("tool %s denied by permission policy", tc.Name)}

    case auth.DecisionAskGate:
        // 返回 GateRequired=true，由 runLLMLoop 处理挂起
        return ToolResult{GateRequired: true}

    default:
        return ToolResult{Err: fmt.Errorf("unknown permission decision")}
    }
}
```

## executeToolGroup — 并行/串行分发

```go
func (qe *QueryEngine) executeToolGroup(ctx context.Context, group ToolCallGroup) []ToolResult {
    results := make([]ToolResult, len(group.Tools))
    if !group.Parallel || len(group.Tools) <= 1 {
        for i, t := range group.Tools { results[i] = qe.executeToolWithPermission(ctx, t) }
        return results
    }
    var wg sync.WaitGroup
    for i, t := range group.Tools {
        wg.Add(1)
        go func(idx int, tc ToolCallParsed) {
            defer wg.Done()
            results[idx] = qe.executeToolWithPermission(ctx, tc)
        }(i, t)
    }
    wg.Wait()
    return results
}
```
