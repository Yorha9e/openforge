# QE-02: Submit 内部 LLM 推理循环 (`runLLMLoop`)

> 日期: 2026-05-23 | 关联: DESIGN.md §4.2, §4.4, §3.15

## 循环结构

```go
// runLLMLoop — Submit 和 Resume 共用的核心循环。
// 调用方必须已持有 qe.mu。
func (qe *QueryEngine) runLLMLoop(ctx context.Context) (*SubmitResult, error) {
    var reply string
    var toolCalls []ToolCallRecord
    roundCount := 0

    for roundCount < qe.config.MaxToolRounds {
        select {
        case <-ctx.Done():
            return &SubmitResult{Status: SubmitError, Error: ctx.Err().Error()}, nil
        default:
        }

        // 1. 构建 LLM 请求
        req := port.ChatRequest{
            Messages: qe.history,
            Config:   qe.buildLLMConfig(),
        }

        // 2. 调用 LLM (流式)
        qe.state = QueryStateAwaitingLLM
        response, err := qe.llmClient.ChatStream(ctx, req)
        if err != nil {
            return &SubmitResult{Status: SubmitError, Error: fmt.Sprintf("llm: %v", err)}, nil
        }

        // 3. 解析响应: 累积 text + 提取 tool_use blocks
        parsed := qe.parseStreamResponse(response)
        if parsed.Text != "" {
            reply = parsed.Text
        }

        // 4. 无 tool_use → 对话完成
        if len(parsed.ToolCalls) == 0 {
            qe.history = append(qe.history, port.Message{
                Role: "agent", Content: reply,
            })
            break
        }

        // 5. 有 tool_use → 执行工具
        qe.state = QueryStateAwaitingTools
        groups := qe.analyzeDependencies(parsed.ToolCalls)

        for _, group := range groups {
            results := qe.executeToolGroup(ctx, group)
            for i, result := range results {
                tc := group.Tools[i]
                toolCalls = append(toolCalls, ToolCallRecord{
                    Name: tc.Name, Args: tc.Args,
                    Output: result.Output, Err: result.Err,
                })

                // 追加 tool_result 到 history
                qe.history = append(qe.history, port.Message{
                    Role:    "tool",
                    Content: qe.formatToolResult(tc.Name, result),
                })

                // 6. Gate 挂起检查 (新增)
                if result.GateRequired {
                    return qe.handleGatePause(ctx, tc, result), nil
                }
            }
        }

        roundCount++
    }

    qe.state = QueryStateAwaitingUser
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
