package domain

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type ToolMeta struct {
	Name              string
	IsConcurrencySafe bool
	Timeout           time.Duration
	Executor          func(ctx context.Context, args map[string]interface{}) (string, error)
}

type ToolRegistry map[string]ToolMeta

type ToolCallGroup struct {
	Tools    []ToolCallParsed
	Parallel bool
}

type ToolCallParsed struct {
	ID   string
	Name string
	Args map[string]interface{}
}

type FormattedToolResult struct {
	Tool          string   `json:"tool"`
	Status        string   `json:"status"`
	Output        string   `json:"output"`
	ModifiedFiles []string `json:"modified_files,omitempty"`
	ErrorCode     string   `json:"error_code,omitempty"`
	Retries       int      `json:"retries"`
	DurationMs    int64    `json:"duration_ms"`
}

const maxParallel = 8

var sem = make(chan struct{}, maxParallel)

func (reg ToolRegistry) partition(toolCalls []ToolCallParsed) []ToolCallGroup {
	var groups []ToolCallGroup
	var parallel []ToolCallParsed

	for _, tc := range toolCalls {
		meta, ok := reg[tc.Name]
		if !ok {
			groups = append(groups, ToolCallGroup{Tools: []ToolCallParsed{tc}, Parallel: false})
			continue
		}
		if meta.IsConcurrencySafe {
			parallel = append(parallel, tc)
		} else {
			if len(parallel) > 0 {
				groups = append(groups, ToolCallGroup{Tools: parallel, Parallel: true})
				parallel = nil
			}
			groups = append(groups, ToolCallGroup{Tools: []ToolCallParsed{tc}, Parallel: false})
		}
	}
	if len(parallel) > 0 {
		groups = append(groups, ToolCallGroup{Tools: parallel, Parallel: true})
	}
	return groups
}

func (reg ToolRegistry) executeTools(ctx context.Context, toolCalls []ToolCallParsed, errorPolicy ToolErrorPolicy) []FormattedToolResult {
	groups := reg.partition(toolCalls)
	var results []FormattedToolResult
	for _, g := range groups {
		if g.Parallel {
			results = append(results, reg.executeParallel(ctx, g.Tools, errorPolicy)...)
		} else {
			for _, tc := range g.Tools {
				results = append(results, reg.executeOne(ctx, tc, errorPolicy))
			}
		}
	}
	return results
}

func (reg ToolRegistry) executeParallel(ctx context.Context, tools []ToolCallParsed, errorPolicy ToolErrorPolicy) []FormattedToolResult {
	var wg sync.WaitGroup
	results := make([]FormattedToolResult, len(tools))
	for i, tc := range tools {
		wg.Add(1)
		go func(idx int, tc ToolCallParsed) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[idx] = reg.executeOne(ctx, tc, errorPolicy)
		}(i, tc)
	}
	wg.Wait()
	return results
}

func (reg ToolRegistry) executeOne(ctx context.Context, tc ToolCallParsed, errorPolicy ToolErrorPolicy) FormattedToolResult {
	start := time.Now()
	meta, ok := reg[tc.Name]
	if !ok {
		return FormattedToolResult{Tool: tc.Name, Status: "error", Output: "tool not registered: " + tc.Name, DurationMs: time.Since(start).Milliseconds()}
	}
	timeout := meta.Timeout
	if timeout == 0 { timeout = 60 * time.Second }
	toolCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for attempt := 0; attempt <= errorPolicy.MaxRetries; attempt++ {
		output, err := meta.Executor(toolCtx, tc.Args)
		if err == nil {
			return FormattedToolResult{Tool: tc.Name, Status: "success", Output: output, Retries: attempt, DurationMs: time.Since(start).Milliseconds()}
		}
		code := MapToolErrorToFailureCode(err)
		recovery := ClassifyAndRecover(code, attempt)
		switch recovery.Action {
		case ActionRetry:
			delay := errorPolicy.RetryDelay
			for i := 0; i < attempt; i++ { delay = time.Duration(float64(delay) * errorPolicy.BackoffFactor) }
			time.Sleep(delay)
			continue
		case ActionCompress, ActionDowngradeModel:
			return FormattedToolResult{Tool: tc.Name, Status: "degraded", Output: recovery.Message, ErrorCode: string(code), Retries: attempt, DurationMs: time.Since(start).Milliseconds()}
		case ActionSelfRepair, ActionClarify:
			return FormattedToolResult{Tool: tc.Name, Status: "error", Output: recovery.Message, ErrorCode: string(code), Retries: attempt, DurationMs: time.Since(start).Milliseconds()}
		default:
			return FormattedToolResult{Tool: tc.Name, Status: "error", Output: err.Error(), ErrorCode: string(code), Retries: attempt, DurationMs: time.Since(start).Milliseconds()}
		}
	}
	return FormattedToolResult{Tool: tc.Name, Status: "error", Output: "max retries exceeded", Retries: errorPolicy.MaxRetries, DurationMs: time.Since(start).Milliseconds()}
}

func formatToolResultXML(r FormattedToolResult) string {
	extra := ""
	if r.ErrorCode != "" { extra += fmt.Sprintf(` error_code="%s"`, r.ErrorCode) }
	if r.Retries > 0 { extra += fmt.Sprintf(` retries="%d"`, r.Retries) }
	return fmt.Sprintf(`<tool_result tool="%s" status="%s" duration_ms="%d"%s>
%s
</tool_result>`, r.Tool, r.Status, r.DurationMs, extra, r.Output)
}
