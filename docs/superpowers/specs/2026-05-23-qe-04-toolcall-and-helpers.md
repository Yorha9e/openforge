# QE-04: ToolCallParsed + Helper 方法

> 日期: 2026-05-23 | 关联: DESIGN.md §4.2, §4.3

## ToolCallParsed

```go
// internal/agent/domain/tool_call_parsed.go

// ToolCallParsed — LLM 响应中提取的已解析工具调用
type ToolCallParsed struct {
    ID         string                 // tool_use block ID
    Name       string                 // 工具名: "bash", "read_file", "write_file"
    Args       map[string]interface{} // 工具参数 (JSON decoded)
    IsReadOnly bool                   // 从 ToolRegistry 查询 (用于权限判定)
}
```

## ToolCallGroup — 并行/串行分组

```go
// analyzeDependencies — 分析工具调用依赖关系，生成执行组
// 规则: 无依赖的只读工具 → 并行组；写入工具 → 独立组 (串行执行)
func (qe *QueryEngine) analyzeDependencies(toolCalls []ToolCallParsed) []ToolCallGroup {
    if len(toolCalls) <= 1 {
        return []ToolCallGroup{{Tools: toolCalls, Parallel: false}}
    }

    var readGroup  []ToolCallParsed
    var writeCalls []ToolCallParsed
    for _, tc := range toolCalls {
        tool := qe.toolRegistry.Get(tc.Name)
        tc.IsReadOnly = tool.IsReadOnly()
        if tc.IsReadOnly && tool.IsConcurrencySafe() {
            readGroup = append(readGroup, tc)
        } else {
            writeCalls = append(writeCalls, tc)
        }
    }

    var groups []ToolCallGroup
    if len(readGroup) > 0 {
        groups = append(groups, ToolCallGroup{Tools: readGroup, Parallel: true})
    }
    for _, tc := range writeCalls {
        groups = append(groups, ToolCallGroup{Tools: []ToolCallParsed{tc}, Parallel: false})
    }
    return groups
}

type ToolCallGroup struct {
    Tools    []ToolCallParsed
    Parallel bool
}
```

## Helper 方法

```go
// collectChangedFiles — 从对话历史中收集被修改的文件列表
func (qe *QueryEngine) collectChangedFiles() []string {
    seen := make(map[string]bool)
    var files []string
    for _, msg := range qe.history {
        if msg.Role == "tool" {
            // 解析 tool_result 中的文件路径
            for _, pattern := range []string{"file_path:", "path:", "File:"} {
                if idx := strings.Index(msg.Content, pattern); idx >= 0 {
                    rest := msg.Content[idx+len(pattern):]
                    if end := strings.IndexAny(rest, " \t\n,"); end > 0 {
                        path := strings.TrimSpace(rest[:end])
                        if !seen[path] { seen[path] = true; files = append(files, path) }
                    }
                }
            }
        }
    }
    return files
}

// calcHistoryHash — 计算当前对话历史的 SHA256 (冲突检测)
func (qe *QueryEngine) calcHistoryHash() string {
    h := sha256.New()
    for _, msg := range qe.history {
        h.Write([]byte(msg.Role))
        h.Write([]byte(msg.Content))
    }
    return fmt.Sprintf("%x", h.Sum(nil))
}

// snapshotHistory — 深拷贝当前对话历史 (Gate 挂起快照)
func (qe *QueryEngine) snapshotHistory() []port.Message {
    cp := make([]port.Message, len(qe.history))
    copy(cp, qe.history)
    return cp
}

// formatToolResult — 格式化工具结果为 LLM 可读的 tool_result 消息
func (qe *QueryEngine) formatToolResult(toolName string, result ToolResult) string {
    if result.Err != nil {
        return fmt.Sprintf("[%s] error: %v", toolName, result.Err)
    }
    return fmt.Sprintf("[%s] completed: %v", toolName, result.Output)
}

// CurrentStage — 从 pipeline 上下文获取当前阶段
func (qe *QueryEngine) CurrentStage() string {
    // 由调用方在 Submit 前注入
    return qe.currentStage
}

// buildLLMConfig — 构建当前 LLM 请求配置 (含 Token 预算剩余)
func (qe *QueryEngine) buildLLMConfig() port.LLMConfig {
    return port.LLMConfig{
        Model:     qe.model,
        MaxTokens: qe.config.TokenBudget - qe.tokenUsed,
    }
}

// resumeApproved — Gate 审批通过后的恢复逻辑
func (qe *QueryEngine) resumeApproved(ctx context.Context, gateResult GateResult) (*SubmitResult, error) {
    qe.history = append(qe.history, port.Message{
        Role: "system", Content: "Gate 审批通过，继续执行。",
    })
    qe.state = QueryStateAwaitingLLM
    return qe.runLLMLoop(ctx)
}

// resumeRejected — Gate 审批驳回后的恢复逻辑
func (qe *QueryEngine) resumeRejected(ctx context.Context, gateResult GateResult) (*SubmitResult, error) {
    feedback := qe.formatGateFeedback(gateResult)
    qe.history = append(qe.history, port.Message{Role: "system", Content: feedback})

    if len(gateResult.LineComments) > 0 {
        files := extractFilesFromComments(gateResult.LineComments)
        qe.history = append(qe.history, port.Message{
            Role: "system",
            Content: fmt.Sprintf("[needs_revision] 请仅修改以下文件: %s", strings.Join(files, ", ")),
        })
    }

    qe.state = QueryStateAwaitingLLM
    return qe.runLLMLoop(ctx)
}

// formatGateFeedback — 将 GateResult 转为结构化 Markdown 注入 history
func (qe *QueryEngine) formatGateFeedback(gr GateResult) string {
    var sb strings.Builder
    sb.WriteString("## Gate 审批被拒绝\n\n")
    if gr.SummaryFeedback != "" {
        sb.WriteString("### 总体反馈\n"); sb.WriteString(gr.SummaryFeedback); sb.WriteString("\n\n")
    }
    if len(gr.LineComments) > 0 {
        sb.WriteString("### 行级反馈\n")
        byFile := groupCommentsByFile(gr.LineComments)
        for file, comments := range byFile {
            sb.WriteString(fmt.Sprintf("**%s**\n", file))
            for _, c := range comments {
                sb.WriteString(fmt.Sprintf("- 第%d行: %s\n", c.LineNumber, c.Content))
            }
            sb.WriteString("\n")
        }
    }
    return sb.String()
}

func groupCommentsByFile(comments []LineComment) map[string][]LineComment { /* ... */ }
func extractFilesFromComments(comments []LineComment) []string { /* unique file paths */ }
```
