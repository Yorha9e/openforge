# QE-05: MapToolErrorToFailureCode — Query Engine ↔ §3.15 对接

> 日期: 2026-05-23 | 关联: DESIGN.md §3.14, §3.15, §4.2

## 映射表

Query Engine 工具执行失败 → §3.14 失败分类 → §3.15 恢复链。

```go
// internal/agent/domain/error_mapping.go

// MapToolErrorToFailureCode — 工具执行错误 → 结构化 FailureCode。
// 使 QueryEngine.executeToolWithPolicy 能统一走 §3.15 ClassifyAndRecover()。
func MapToolErrorToFailureCode(err error) FailureCode {
    if err == nil { return "" }

    msg := err.Error()

    // Layer 1: TRANSIENT
    if strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline exceeded") {
        return FailAPITimeout
    }
    if strings.Contains(msg, "rate limit") || strings.Contains(msg, "429") {
        return FailRateLimited
    }
    if strings.Contains(msg, "overloaded") || strings.Contains(msg, "503") {
        return FailOverloaded
    }

    // Layer 2: DEGRADABLE
    if strings.Contains(msg, "context length") || strings.Contains(msg, "token limit") {
        return FailContextOverflow
    }
    if strings.Contains(msg, "quota") || strings.Contains(msg, "insufficient") {
        return FailTokenQuotaExceeded
    }

    // Layer 3: RECOVERABLE
    if strings.Contains(msg, "not found") || strings.Contains(msg, "undefined") {
        return FailModelHallucination
    }
    if strings.Contains(msg, "dependency") || strings.Contains(msg, "version conflict") {
        return FailDependencyConflict
    }
    if strings.Contains(msg, "ambiguous") || strings.Contains(msg, "vague") {
        return FailPromptWeakness
    }

    // Layer 4: FATAL
    if strings.Contains(msg, "permission denied") || strings.Contains(msg, "blocked") {
        return FailRepoBug
    }
    if strings.Contains(msg, "sandbox timeout") || strings.Contains(msg, "killed") {
        return FailSandboxTimeout
    }

    return FailUnknown
}
```

## executeToolWithPolicy — 完整版 (统一 §3.15)

```go
func (qe *QueryEngine) executeToolWithPolicy(ctx context.Context, tc ToolCallParsed) ToolResult {
    policy := qe.config.ToolErrorPolicy

    for attempt := 0; attempt <= policy.MaxRetries; attempt++ {
        tool := qe.toolRegistry.Get(tc.Name)
        output, err := tool.Execute(ctx, tc.Args)

        if err == nil {
            return ToolResult{Output: output}
        }

        // 错误 → 分类
        failCode := MapToolErrorToFailureCode(err)

        // 还有重试次数 → 指数退避
        if attempt < policy.MaxRetries {
            delay := time.Duration(float64(policy.RetryDelay) * math.Pow(policy.BackoffFactor, float64(attempt)))
            qe.logger.Warn("tool retry", "tool", tc.Name, "attempt", attempt+1, "delay", delay, "error", err)
            select {
            case <-time.After(delay):
                continue
            case <-ctx.Done():
                return ToolResult{Err: ctx.Err()}
            }
        }

        // 重试耗尽 → 走 §3.15 恢复链
        recovery := ClassifyAndRecover(failCode, attempt)

        switch recovery.Action {
        case ActionEscalate:
            return ToolResult{Err: fmt.Errorf("tool %s FATAL: %w", tc.Name, err)}

        case ActionRetry:
            // 不应该到这里 (retry 在上面的循环中处理了)
            return ToolResult{Err: err}

        case ActionSelfRepair:
            qe.logger.Info("tool self-repair", "tool", tc.Name, "code", failCode)
            return ToolResult{Output: fmt.Sprintf("[self-repair] %v", err), Err: nil}

        case ActionDowngradeModel:
            qe.logger.Info("downgrading model", "tool", tc.Name)
            qe.switchToFallbackModel()
            return qe.executeToolWithPolicy(ctx, tc) // 用便宜模型重试

        case ActionCompress:
            qe.logger.Info("compressing context", "tool", tc.Name)
            qe.compressHistory(ctx)
            return qe.executeToolWithPolicy(ctx, tc)

        default:
            switch policy.OnFailure {
            case "skip":
                return ToolResult{Output: fmt.Sprintf("[skipped] %v", err)}
            case "notify_llm":
                return ToolResult{Err: err} // LLM 自行判断
            default:
                return ToolResult{Err: err}
            }
        }
    }

    return ToolResult{Err: fmt.Errorf("unreachable: max retries exceeded")}
}

// switchToFallbackModel — §3.15 DEGRADABLE 模型降级
func (qe *QueryEngine) switchToFallbackModel() {
    entry := qe.llmRegistry.Lookup(qe.model)
    if len(entry.Fallback) > 0 {
        qe.model = entry.Fallback[0]
        qe.logger.Info("switched model", "from", entry.Alias, "to", qe.model)
    }
}
```

## 测试矩阵

```
TestMapToolErrorToFailureCode:
  ├── "context deadline exceeded"          → FailAPITimeout
  ├── "rate limit exceeded"               → FailRateLimited
  ├── "server overloaded"                  → FailOverloaded
  ├── "context length exceeded"           → FailContextOverflow
  ├── "quota exceeded"                    → FailTokenQuotaExceeded
  ├── "module not found"                  → FailModelHallucination
  ├── "dependency conflict: react@19"     → FailDependencyConflict
  ├── "ambiguous requirement"             → FailPromptWeakness
  ├── "permission denied"                 → FailRepoBug
  ├── "sandbox killed after 120s"         → FailSandboxTimeout
  └── "unknown error XYZ"                 → FailUnknown

TestExecuteToolWithPolicy:
  ├── 正常执行 → ToolResult{Output: ...}
  ├── 第1次重试成功 → 延迟1s后成功
  ├── 第3次重试成功 → 延迟4s后成功
  ├── 3次重试全部失败 + OnFailure=notify_llm → ToolResult{Err: ...}
  ├── 3次重试全部失败 + OnFailure=skip → ToolResult{Output: "[skipped] ..."}
  ├── TRANSIENT 错误 → 自动重试 (ActionRetry)
  ├── CONTEXT_OVERFLOW → 压缩 + 重试 (ActionCompress)
  └── HALLUCINATION → 自动修复 (ActionSelfRepair)
```
