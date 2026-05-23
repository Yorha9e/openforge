# Phase 5b — Multi-Agent Coordination + Tool Registry + Sub-Pipeline

> **状态: ✅ 已完成**

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task.

**Goal:** AgentCoordinator 从单 Agent 直通升级为多 Agent 协作调度器，Tool Registry 注册 5 个基础工具(Bash/Read/Write/Grep/Glob)，支持子 Pipeline 分支。

**Architecture:** AgentCoordinator 增加 `Spawn()`/`Delegate()` multi-agent 调度，CSPChannel 传递 Agent 间消息。ToolRegistry 用关键词+嵌入索引(降级:关键词匹配) 搜索工具。子 Pipeline 从父 Pipeline checkout 上下文，独立状态机运行，合入时 rebase 父 Pipeline。Gate Hooks 注入 pre/post 拦截器。

**Tech Stack:** Go 1.25 + goroutine + channel, `database/sql`, React 19 + TypeScript + shadcn/ui + design-tokens

**关键约束:**
- Multi-agent: 最多 5 个子 Agent (spawn limit), 层级 ≤2
- Tool: 5 个基础 Go 工具 + 嵌入索引注册
- 子 Pipeline: 继承父 Pipeline 的 project_id/config, 独立 status/current_stage
- CSP: 保持 goroutine channel + WAL 当前实现, Redis Streams 延至 Phase 6
- 嵌入索引: MVP 用 keywords 匹配, 嵌入向量搜索延至 Phase 7

---
## File Map

```
openforge/
├── internal/agent/domain/
│   ├── coordinator.go         # MODIFY: Spawn/Delegate/Broadcast multi-agent
│   ├── coordinator_test.go    # NEW: multi-agent tests
│   ├── query_engine.go        # MODIFY: 注入 ToolRegistry, 工具调用循环
│   └── query_engine_test.go   # MODIFY: 加 tool call 测试
├── internal/agent/port/
│   ├── tool_registry.go       # MODIFY: 扩展 Tool + ToolSearcher 定义
│   └── tool.go                # MODIFY: Tool/I,O 泛型接口对齐
├── internal/tool/
│   ├── registry.go            # NEW: ToolRegistry — 注册+搜索
│   ├── registry_test.go       # NEW: registry tests
│   ├── read_tool.go           # NEW: ReadFileTool
│   ├── write_tool.go          # NEW: WriteFileTool
│   ├── grep_tool.go           # NEW: GrepTool
│   ├── glob_tool.go           # NEW: GlobTool
│   └── bash_tool.go           # (已有, 无需改)
├── internal/pipeline/domain/
│   └── pipeline.go            # MODIFY: 子 Pipeline Fork/Join
├── internal/server/
│   └── routes.go              # MODIFY: 加 /api/pipelines/{id}/fork 端点
└── frontend/src/
    ├── features/chat/
    │   ├── ToolCallCard.tsx    # NEW: 工具调用可视化卡片
    │   └── AgentPanel.tsx      # NEW: 多 Agent 状态面板
    └── shared/api.ts           # MODIFY: 加 fork pipeline API
```

---

### Task 1: Tool Registry + 5 Go Tools

> 实现 ToolRegistry 注册表(关键词搜索)，创建 ReadFile/WriteFile/Grep/Glob 4 个基础工具

**Files:**
- Create: `internal/tool/registry.go`
- Create: `internal/tool/registry_test.go`
- Create: `internal/tool/read_tool.go`
- Create: `internal/tool/write_tool.go`
- Create: `internal/tool/grep_tool.go`
- Create: `internal/tool/glob_tool.go`
- Modify: `internal/agent/port/tool_registry.go` — 扩展接口，保留 `SearchTools` 向后兼容

- [ ] **Step 1: 扩展 ToolRegistry port 接口（保持向后兼容）**

Modify `internal/agent/port/tool_registry.go`（在现有内容基础上追加，**保留**已有 `ToolMatch` 和 `ToolRegistryClient.SearchTools`）:

```go
package port

import "context"

// === 已有类型 (保留, 不可删除) ===

// ToolMatch is a search result for a tool query.
type ToolMatch struct {
	Name        string
	Description string
	Score       float64
}

// === 新增类型 ===

// ToolInfo describes a registered tool for discovery.
type ToolInfo struct {
	Name        string
	Description string
	InputSchema map[string]interface{} // JSON Schema
}

// ToolCall represents a request to invoke a tool.
type ToolCall struct {
	ToolName string
	Input    []byte // JSON-encoded input
}

// ToolResult is the result of a tool invocation.
type ToolResult struct {
	Output []byte // JSON-encoded output
	Error  string
}

// ToolSearcher matches queries to tools (keyword + future embedding).
type ToolSearcher interface {
	Search(ctx context.Context, query string, topK int) ([]ToolMatch, error)
	SearchTools(ctx context.Context, query string, topK int) ([]ToolMatch, error) // 向后兼容别名
	Register(ctx context.Context, info ToolInfo) error
	List(ctx context.Context) ([]ToolInfo, error)
}

// ToolRunner executes registered tools.
type ToolRunner interface {
	Run(ctx context.Context, call ToolCall) (ToolResult, error)
}

// ToolRegistryClientFull combines search + execution (what Agent uses).
// 原有 ToolRegistryClient (仅 SearchTools) 保持不变，新增此组合接口。
type ToolRegistryClientFull interface {
	ToolSearcher
	ToolRunner
}
```

- [ ] **Step 2: 写 Registry 测试**

Create `internal/tool/registry_test.go`:

```go
package tool

import (
	"context"
	"testing"

	"openforge/internal/agent/port"
)

func TestRegistry_RegisterAndSearch(t *testing.T) {
	reg := NewRegistry()
	ctx := context.Background()

	reg.Register(ctx, port.ToolInfo{
		Name: "read_file", Description: "Read contents of a file by path",
	})
	reg.Register(ctx, port.ToolInfo{
		Name: "write_file", Description: "Write content to a file",
	})
	reg.Register(ctx, port.ToolInfo{
		Name: "bash", Description: "Execute a shell command",
	})

	matches, err := reg.Search(ctx, "read a file", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatal("expected at least 1 match")
	}
	// "read_file" should have highest score for "read a file" query
	if matches[0].Name != "read_file" {
		t.Errorf("top match = %q, want read_file", matches[0].Name)
	}
	if matches[0].Score <= 0 {
		t.Error("score should be > 0")
	}
}

func TestRegistry_RunTool(t *testing.T) {
	reg := NewRegistry()
	// 用一个轻量的 echo 工具做测试（实现在 registry.go 尾部）
	reg.RegisterTool(&EchoTool{})

	result, err := reg.Run(context.Background(), port.ToolCall{
		ToolName: "echo",
		Input:    []byte(`{"message":"hello"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Error != "" {
		t.Errorf("unexpected error: %s", result.Error)
	}
	expected := `{"message":"hello"}`
	if string(result.Output) != expected {
		t.Errorf("output = %s, want %s", string(result.Output), expected)
	}
}
```

- [ ] **Step 3: 运行测试 — FAIL**

```bash
go test ./internal/tool/ -v -run TestRegistry -count=1
```

- [ ] **Step 4: 实现 Registry**

Create `internal/tool/registry.go`:

```go
package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"openforge/internal/agent/port"
)

// Tool is a simple tool with a Run method.
type Tool interface {
	Name() string
	Description() string
	InputSchema() map[string]interface{}
	Run(ctx context.Context, input []byte) ([]byte, error)
}

// Registry implements port.ToolSearcher + port.ToolRunner with keyword-based search.
type Registry struct {
	mu      sync.RWMutex
	tools   map[string]Tool       // name → Tool (executable)
	infos   map[string]port.ToolInfo // name → ToolInfo (search-only, for Register)
}

func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
		infos: make(map[string]port.ToolInfo),
	}
}

func (r *Registry) Register(ctx context.Context, info port.ToolInfo) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.infos[info.Name] = info
	return nil
}

func (r *Registry) RegisterTool(t Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[t.Name()] = t
	r.infos[t.Name()] = port.ToolInfo{
		Name:        t.Name(),
		Description: t.Description(),
		InputSchema: t.InputSchema(),
	}
}

// Search performs keyword-based tool matching.
// Each word in the query is matched against tool name + description.
func (r *Registry) Search(ctx context.Context, query string, topK int) ([]port.ToolMatch, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	keywords := strings.Fields(strings.ToLower(query))
	var results []port.ToolMatch

	for name, info := range r.infos {
		text := strings.ToLower(name + " " + info.Description)
		score := r.matchScore(keywords, text)
		if score > 0 {
			results = append(results, port.ToolMatch{
				Name:        name,
				Description: info.Description,
				Score:       score,
			})
		}
	}

	// Sort by score descending
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Score > results[i].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	if topK > 0 && len(results) > topK {
		results = results[:topK]
	}
	return results, nil
}

// SearchTools is the backward-compatible alias.
func (r *Registry) SearchTools(ctx context.Context, query string, topK int) ([]port.ToolMatch, error) {
	return r.Search(ctx, query, topK)
}

func (r *Registry) matchScore(keywords []string, text string) float64 {
	hits := 0
	for _, kw := range keywords {
		if strings.Contains(text, kw) {
			hits++
		}
	}
	if hits == 0 {
		return 0
	}
	return float64(hits) / float64(len(keywords))
}

func (r *Registry) List(ctx context.Context) ([]port.ToolInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []port.ToolInfo
	for _, info := range r.infos {
		result = append(result, info)
	}
	return result, nil
}

func (r *Registry) Run(ctx context.Context, call port.ToolCall) (port.ToolResult, error) {
	r.mu.RLock()
	t, ok := r.tools[call.ToolName]
	r.mu.RUnlock()
	if !ok {
		return port.ToolResult{}, fmt.Errorf("tool %q not found", call.ToolName)
	}

	output, err := t.Run(ctx, call.Input)
	if err != nil {
		return port.ToolResult{Error: err.Error()}, nil
	}
	return port.ToolResult{Output: output}, nil
}

// EchoTool is a simple test tool that returns the input as output.
// Used in tests to verify Registry.Run.
type EchoTool struct{}

func (t *EchoTool) Name() string        { return "echo" }
func (t *EchoTool) Description() string { return "Echo back the input as output (for testing)" }
func (t *EchoTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{"message": map[string]string{"type": "string"}},
	}
}
func (t *EchoTool) Run(ctx context.Context, input []byte) ([]byte, error) {
	// Validate JSON, return as-is
	var v interface{}
	if err := json.Unmarshal(input, &v); err != nil {
		return nil, err
	}
	return json.Marshal(v)
}
```

- [ ] **Step 5: 运行测试 — PASS**

```bash
go test ./internal/tool/ -v -run TestRegistry -count=1
```

- [ ] **Step 6: 实现 4 个文件工具**

Create `internal/tool/read_tool.go`:

```go
package tool

import (
	"context"
	"encoding/json"
	"os"
)

type ReadFileTool struct{}

func (t *ReadFileTool) Name() string        { return "read_file" }
func (t *ReadFileTool) Description() string { return "Read contents of a file" }
func (t *ReadFileTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]string{"type": "string", "description": "File path to read"},
		},
		"required": []string{"path"},
	}
}
func (t *ReadFileTool) Run(ctx context.Context, input []byte) ([]byte, error) {
	var req struct{ Path string `json:"path"` }
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, err
	}
	content, err := os.ReadFile(req.Path)
	if err != nil {
		return nil, err
	}
	result, _ := json.Marshal(map[string]string{"content": string(content)})
	return result, nil
}
```

Create `internal/tool/write_tool.go`:

```go
package tool

import (
	"context"
	"encoding/json"
	"os"
)

type WriteFileTool struct{}

func (t *WriteFileTool) Name() string        { return "write_file" }
func (t *WriteFileTool) Description() string { return "Write content to a file" }
func (t *WriteFileTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path":    map[string]string{"type": "string", "description": "File path to write"},
			"content": map[string]string{"type": "string", "description": "Content to write"},
		},
		"required": []string{"path", "content"},
	}
}
func (t *WriteFileTool) Run(ctx context.Context, input []byte) ([]byte, error) {
	var req struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, err
	}
	if err := os.WriteFile(req.Path, []byte(req.Content), 0644); err != nil {
		return nil, err
	}
	return json.Marshal(map[string]string{"status": "ok"})
}
```

Create `internal/tool/grep_tool.go`:

```go
package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type GrepTool struct{}

func (t *GrepTool) Name() string        { return "grep" }
func (t *GrepTool) Description() string { return "Search for a pattern in file contents" }
func (t *GrepTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"pattern": map[string]string{"type": "string", "description": "Pattern to search for (substring match)"},
			"path":    map[string]string{"type": "string", "description": "File or directory to search in"},
		},
		"required": []string{"pattern", "path"},
	}
}
func (t *GrepTool) Run(ctx context.Context, input []byte) ([]byte, error) {
	var req struct {
		Pattern string `json:"pattern"`
		Path    string `json:"path"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(req.Path)
	if err != nil {
		return nil, err
	}
	var matches []string
	for i, line := range strings.Split(string(data), "\n") {
		if strings.Contains(line, req.Pattern) {
			matches = append(matches, string(append([]byte(fmt.Sprintf("%d:", i+1)), []byte(line)...)))
		}
	}
	result, _ := json.Marshal(map[string]interface{}{
		"matches": matches, "count": len(matches),
	})
	return result, nil
}
```

Create `internal/tool/glob_tool.go`:

```go
package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
)

type GlobTool struct{}

func (t *GlobTool) Name() string        { return "glob" }
func (t *GlobTool) Description() string { return "Find files matching a glob pattern" }
func (t *GlobTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"pattern": map[string]string{"type": "string", "description": "Glob pattern (e.g., **/*.go)"},
		},
		"required": []string{"pattern"},
	}
}
func (t *GlobTool) Run(ctx context.Context, input []byte) ([]byte, error) {
	var req struct{ Pattern string `json:"pattern"` }
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, err
	}
	matches, err := filepath.Glob(req.Pattern)
	if err != nil {
		return nil, err
	}
	result, _ := json.Marshal(map[string]interface{}{
		"files": matches, "count": len(matches),
	})
	return result, nil
}
```

- [ ] **Step 7: 编译 + 测试 + Commit**

```bash
go build ./cmd/server/
go test ./internal/tool/... -count=1
```

```bash
git add internal/tool/ internal/agent/port/tool_registry.go
git commit -m "feat(tool): add ToolRegistry with keyword search + Read/Write/Grep/Glob tools"
```

> ⚠️ **设计注意**: 现有 `BashTool` 实现泛型 `port.Tool[BashInput, kernel.ExecOutput]`，而新增的 4 个工具实现简化的 `tool.Tool`（`Run(ctx, []byte) ([]byte, error)`）。`Registry.RegisterTool` 接受后者。后续若需统一，可在 Registry 内部加适配器将泛型 `port.Tool[I,O]` 桥接到 `tool.Tool`，Phase 5b MVP 期间两套并存无冲突。

---

### Task 2: Multi-Agent Coordinator (Spawn + Delegate + Broadcast)

> AgentCoordinator 升级: Spawn 子 Agent、Delegate 任务、Broadcast 事件

**Files:**
- Modify: `internal/agent/domain/coordinator.go`
- Create: `internal/agent/domain/coordinator_test.go`
- Modify: `internal/shared/profile/bootstrap.go` — 注入 ToolRegistry

- [ ] **Step 1: 写 Multi-Agent 测试**

Create `internal/agent/domain/coordinator_test.go`:

```go
package domain

import (
	"context"
	"fmt"
	"testing"
	"time"

	"openforge/internal/agent/port"
	"openforge/internal/agent/service"
)

func TestCoordinator_SpawnAgent(t *testing.T) {
	coord := NewCoordinator(nil, nil)
	ctx := context.Background()

	agent, err := coord.Spawn(ctx, "agent-1", "proj-1", "dev", "") // parentID="" 表示根 Agent
	if err != nil {
		t.Fatal(err)
	}
	if agent.ID != "agent-1" {
		t.Errorf("agent ID = %q, want agent-1", agent.ID)
	}
	if coord.AgentCount() != 1 {
		t.Errorf("agent count = %d, want 1", coord.AgentCount())
	}
}

func TestCoordinator_Delegate(t *testing.T) {
	coord := NewCoordinator(nil, nil)
	ctx := context.Background()

	coord.Spawn(ctx, "pm", "proj-1", "pm", "")
	coord.Spawn(ctx, "dev-1", "proj-1", "dev", "pm")

	err := coord.Delegate(ctx, "pm", "dev-1", service.Message{
		From: "pm",
		To:   "dev-1",
		Body: []byte(`{"task":"review code"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCoordinator_Broadcast(t *testing.T) {
	coord := NewCoordinator(nil, nil)
	ctx := context.Background()

	coord.Spawn(ctx, "a1", "p1", "dev", "")
	coord.Spawn(ctx, "a2", "p1", "dev", "")

	err := coord.Broadcast(ctx, "pm", service.Message{
		From: "pm",
		Body: []byte(`{"event":"status_update"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCoordinator_SpawnLimit(t *testing.T) {
	coord := NewCoordinator(nil, nil)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		_, err := coord.Spawn(ctx, fmt.Sprintf("a%d", i), "p1", "dev", "")
		if err != nil {
			t.Fatalf("unexpected error at %d: %v", i, err)
		}
	}
	// 6th should fail (max 5)
	_, err := coord.Spawn(ctx, "a6", "p1", "dev", "")
	if err == nil {
		t.Fatal("expected spawn limit error")
	}
}
```

- [ ] **Step 2: 运行测试 — FAIL**

```bash
go test ./internal/agent/domain/ -v -run TestCoordinator -count=1
```

- [ ] **Step 3: 升级 AgentCoordinator**

Modify `internal/agent/domain/coordinator.go`:

```go
package domain

import (
	"context"
	"fmt"
	"sync"
	"time"

	"openforge/internal/agent/port"
	"openforge/internal/agent/service"
)

const maxAgents = 5

type AgentCoordinator struct {
	mu        sync.Mutex
	agents    map[string]*AgentInstance
	llmClient port.LLMRouterClient
	toolReg   port.ToolRegistryClientFull // 使用组合接口 (ToolSearcher + ToolRunner)
	channels  map[string]*service.CSPChannel
}

type AgentInstance struct {
	ID         string
	PipelineID string
	Role       string
	ParentID   string // empty for root agent
	Channel    *service.CSPChannel
	CreatedAt  time.Time
}

func NewCoordinator(llmClient port.LLMRouterClient, toolReg port.ToolRegistryClientFull) *AgentCoordinator {
	if toolReg == nil {
		// 生产代码中始终传入 toolReg；nil 仅在测试中用
	}
	return &AgentCoordinator{
		agents:    make(map[string]*AgentInstance),
		llmClient: llmClient,
		toolReg:   toolReg,
		channels:  make(map[string]*service.CSPChannel),
	}
}

// Spawn creates a new agent and its CSP channel.
func (c *AgentCoordinator) Spawn(ctx context.Context, id, pipelineID, role, parentID string) (*AgentInstance, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.agents) >= maxAgents {
		return nil, fmt.Errorf("max agents (%d) reached", maxAgents)
	}

	ch := service.NewCSPChannel(id, 64)
	agent := &AgentInstance{
		ID:         id,
		PipelineID: pipelineID,
		Role:       role,
		ParentID:   parentID,
		Channel:    ch,
		CreatedAt:  time.Now(),
	}
	c.agents[id] = agent
	c.channels[id] = ch
	return agent, nil
}

// Delegate sends a message from one agent to another.
func (c *AgentCoordinator) Delegate(ctx context.Context, fromID, toID string, msg service.Message) error {
	c.mu.Lock()
	target, ok := c.agents[toID]
	c.mu.Unlock()
	if !ok {
		return fmt.Errorf("agent %q not found", toID)
	}
	return target.Channel.Send(ctx, msg)
}

// Broadcast sends a message to all agents except the sender.
func (c *AgentCoordinator) Broadcast(ctx context.Context, fromID string, msg service.Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for id, agent := range c.agents {
		if id == fromID {
			continue
		}
		if err := agent.Channel.Send(ctx, msg); err != nil {
			return fmt.Errorf("broadcast to %q: %w", id, err)
		}
	}
	return nil
}

// AgentCount returns the number of spawned agents.
func (c *AgentCoordinator) AgentCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.agents)
}

// ListAgents returns all agent instances.
func (c *AgentCoordinator) ListAgents() []AgentInstance {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := make([]AgentInstance, 0, len(c.agents))
	for _, a := range c.agents {
		result = append(result, *a)
	}
	return result
}

// Terminate removes an agent and closes its channel.
func (c *AgentCoordinator) Terminate(ctx context.Context, id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	agent, ok := c.agents[id]
	if !ok {
		return fmt.Errorf("agent %q not found", id)
	}
	delete(c.agents, id)
	delete(c.channels, id)
	// Channel GC'd when no references remain
	_ = agent
	return nil
}

func (c *AgentCoordinator) Chat(ctx context.Context, messages []port.Message, config port.LLMConfig) (string, error) {
	resp, err := c.llmClient.Chat(ctx, port.ChatRequest{Messages: messages, Config: config})
	if err != nil {
		return "", fmt.Errorf("llm chat: %w", err)
	}
	return resp.Content, nil
}

func (c *AgentCoordinator) ChatStream(ctx context.Context, messages []port.Message, config port.LLMConfig) (<-chan string, error) {
	return c.llmClient.ChatStream(ctx, port.ChatRequest{Messages: messages, Config: config})
}

// SearchTools delegates to ToolRegistry for tool discovery.
func (c *AgentCoordinator) SearchTools(ctx context.Context, query string, topK int) ([]port.ToolMatch, error) {
	if c.toolReg == nil {
		return nil, fmt.Errorf("tool registry not available")
	}
	return c.toolReg.Search(ctx, query, topK)
}

// RunTool executes a tool by name.
func (c *AgentCoordinator) RunTool(ctx context.Context, call port.ToolCall) (port.ToolResult, error) {
	if c.toolReg == nil {
		return port.ToolResult{}, fmt.Errorf("tool registry not available")
	}
	return c.toolReg.Run(ctx, call)
}
```

- [ ] **Step 4: 运行测试 — PASS**

```bash
go test ./internal/agent/domain/ -v -run TestCoordinator -count=1
```

- [ ] **Step 5: Bootstrap 注入 ToolRegistry**

```go
toolReg := tool.NewRegistry()
toolReg.RegisterTool(&tool.BashTool{Exec: of.CommandExec})
toolReg.RegisterTool(&tool.ReadFileTool{})
toolReg.RegisterTool(&tool.WriteFileTool{})
toolReg.RegisterTool(&tool.GrepTool{})
toolReg.RegisterTool(&tool.GlobTool{})

coordinator := domain.NewCoordinator(nil, toolReg) // or wire llmClient
```

- [ ] **Step 6: 编译 + 测试 + Commit**

```bash
go build ./cmd/server/
go test ./internal/agent/... ./internal/tool/... -count=1
```

```bash
git add internal/agent/domain/coordinator.go internal/agent/domain/coordinator_test.go internal/shared/profile/bootstrap.go
git commit -m "feat(agent): add multi-agent coordinator with Spawn/Delegate/Broadcast/ToolRegistry"
```

---

### Task 3: Sub-Pipeline Fork + Join

> Agent 可从当前 Pipeline 创建子分支，独立状态机，合入时继承上下文

**Files:**
- Modify: `internal/pipeline/domain/pipeline.go` — Fork/Join 方法
- Modify: `internal/pipeline/service/pipeline_service.go` — ForkPipeline API
- Modify: `internal/server/routes.go` — POST /api/pipelines/{id}/fork
- Modify: `internal/pipeline/port/repository.go` — 加 Fork 接口方法

- [ ] **Step 1: Pipeline Fork 域逻辑 + 新增字段**

**前置修改** — 在 `internal/pipeline/domain/pipeline.go` 的 `Pipeline` struct 中增加 3 个字段：

```go
// 新增字段（追加到现有 struct 尾部）
ParentPipelineID *string  `json:"parent_pipeline_id,omitempty"` // nil = root pipeline
Region           string   `json:"region"`                        // 部署区域, fork 时继承
Config           PipelineConfig `json:"config"`                  // 可克隆的配置快照
```

**新增 `PipelineConfig` 类型**（追加到 pipeline.go 尾部）:

```go
// PipelineConfig holds the configuration snapshot for a pipeline.
type PipelineConfig struct {
	Language   string `json:"language"`
	Framework  string `json:"framework"`
	MaxAgents  int    `json:"max_agents"`
}

func (c PipelineConfig) Clone() PipelineConfig { return c }
```

**新增 `Fork` 方法 + `IsSubPipeline`:**

```go
// Fork creates a sub-pipeline inheriting parent context.
func (p *Pipeline) Fork(childID, title, createdBy string) *Pipeline {
	childLevel := p.Level
	if p.Level == "L1" {
		childLevel = "L2"
	} else if p.Level == "L2" {
		childLevel = "L3"
	} else {
		childLevel = "L3" // max sub-pipeline level
	}
	parentID := p.ID
	child := &Pipeline{
		ID:               childID,
		ProjectID:        p.ProjectID,
		ParentPipelineID: &parentID,
		Title:            title,
		Level:            childLevel,
		Status:           "pending",
		CreatedBy:        createdBy,
		Region:           p.Region,
		Config:           p.Config.Clone(),
		BacktrackCount:   0,
		Version:          1,
	}
	return child
}

// IsSubPipeline returns true if this is a sub-pipeline (has parent).
func (p *Pipeline) IsSubPipeline() bool {
	return p.ParentPipelineID != nil && *p.ParentPipelineID != ""
}
```

- [ ] **Step 2: ForkPipeline Service**

In `internal/pipeline/service/pipeline_service.go`, add:

```go
func (s *PipelineService) Fork(ctx context.Context, parentID, title, createdBy string) (*domain.Pipeline, error) {
	parent, err := s.repo.GetByID(ctx, parentID)
	if err != nil {
		return nil, err
	}
	childID := "pipe-" + fmt.Sprintf("%d", time.Now().UnixNano())
	child := parent.Fork(childID, title, createdBy)
	if err := s.repo.Create(ctx, child); err != nil {
		return nil, err
	}
	// Re-read to get DB defaults (created_at, etc.)
	return s.repo.GetByID(ctx, childID)
}
```

- [ ] **Step 3: REST 端点**

```go
mux.HandleFunc("POST /api/pipelines/{id}/fork", authMw(handleForkPipeline(of)))

func handleForkPipeline(of *profile.OpenForge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct{ Title string `json:"title"` }
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, 400, "invalid body")
			return
		}
		child, err := of.PipelineSvc.Fork(r.Context(), r.PathValue("id"), req.Title, UserIDFromContext(r.Context()))
		if err != nil {
			writeError(w, 500, sanitizeError(err))
			return
		}
		writeJSON(w, 201, child)
	}
}
```

- [ ] **Step 4: 编译 + 测试 + Commit**

```bash
go build ./cmd/server/
go test ./internal/pipeline/... -count=1
```

```bash
git add internal/pipeline/ internal/server/routes.go
git commit -m "feat(pipeline): add sub-pipeline Fork with parent context inheritance"
```

---

### Task 4: Gate Hooks (Pre/Post 拦截器)

> Gate 节点支持 pre/post 拦截器，复用现有 Gate 值对象

**Files:**
- Create: `internal/pipeline/domain/gate_hook.go`
- Modify: `internal/pipeline/service/gate_service.go` — 注入 Hooks

- [ ] **Step 1: GateHook 类型定义**

Create `internal/pipeline/domain/gate_hook.go`:

```go
package domain

import "context"

// GateHook is a pre/post interceptor on gate approval/rejection.
type GateHook interface {
	// PreApprove runs before approval. Return error to block.
	PreApprove(ctx context.Context, event *GateEvent) error
	// PostApprove runs after successful approval.
	PostApprove(ctx context.Context, event *GateEvent)
	// PreReject runs before rejection.
	PreReject(ctx context.Context, event *GateEvent) error
	// PostReject runs after rejection.
	PostReject(ctx context.Context, event *GateEvent)
}

// HookChain executes hooks in order, stopping on first error.
type HookChain []GateHook

func (hc HookChain) RunPreApprove(ctx context.Context, ev *GateEvent) error {
	for _, h := range hc {
		if err := h.PreApprove(ctx, ev); err != nil {
			return err
		}
	}
	return nil
}

func (hc HookChain) RunPostApprove(ctx context.Context, ev *GateEvent) {
	for _, h := range hc {
		h.PostApprove(ctx, ev)
	}
}

func (hc HookChain) RunPreReject(ctx context.Context, ev *GateEvent) error {
	for _, h := range hc {
		if err := h.PreReject(ctx, ev); err != nil {
			return err
		}
	}
	return nil
}

func (hc HookChain) RunPostReject(ctx context.Context, ev *GateEvent) {
	for _, h := range hc {
		h.PostReject(ctx, ev)
	}
}
```

- [ ] **Step 2: 注入 GateService**

Modify `gate_service.go`:

```go
type GateService struct {
	repo  port.GateRepository
	pipeRepo port.PipelineRepository
	hooks domain.HookChain
}

func NewGateService(repo port.GateRepository, pipeRepo port.PipelineRepository, hooks ...domain.GateHook) *GateService {
	return &GateService{repo: repo, pipeRepo: pipeRepo, hooks: hooks}
}

func (s *GateService) Approve(ctx context.Context, ...) error {
	ev := domain.NewGateEvent(...)
	if err := s.hooks.RunPreApprove(ctx, ev); err != nil {
		return err
	}
	if err := s.repo.CreateEvent(ctx, ev); err != nil {
		return err
	}
	s.hooks.RunPostApprove(ctx, ev)
	return nil
}
```

- [ ] **Step 3: 编译 + 测试 + Commit**

```bash
go build ./cmd/server/
go test ./internal/pipeline/... -count=1
git commit -m "feat(gate): add GateHook pre/post interceptor chain"
```

---

### Task 5: Frontend — Multi-Agent Panel + Tool Call Visualization

> 聊天面板增加 Agent 状态面板和工具调用可视化卡片

**Files:**
- Create: `frontend/src/features/chat/AgentPanel.tsx`
- Create: `frontend/src/features/chat/ToolCallCard.tsx`
- Modify: `frontend/src/features/chat/ChatPanel.tsx` — 集成 AgentPanel

- [ ] **Step 1: 创建 AgentPanel**

Create `frontend/src/features/chat/AgentPanel.tsx`:

```tsx
import { useState } from 'react';
import { tokens } from '../../shared/design-tokens';

interface AgentInfo {
  id: string; role: string; pipeline_id: string; parent_id: string;
}

export function AgentPanel({ agents }: { agents: AgentInfo[] }) {
  const [collapsed, setCollapsed] = useState(false);

  return (
    <div style={{ borderBottom: `1px solid ${tokens.border}`, background: tokens.surface }}>
      <button
        onClick={() => setCollapsed(!collapsed)}
        aria-expanded={!collapsed}
        style={{
          width: '100%', padding: '8px 12px', background: 'none', border: 'none',
          color: tokens.muted, fontFamily: tokens.fontBody, fontSize: 12,
          cursor: 'pointer', textAlign: 'left', display: 'flex', justifyContent: 'space-between', alignItems: 'center',
        }}
      >
        Agents ({agents.length})
        <svg width="12" height="12" viewBox="0 0 12 12" fill="none" aria-hidden="true"
          style={{ transform: collapsed ? undefined : 'rotate(180deg)', transition: 'transform 200ms' }}>
          <path d="M3 5l3 3 3-3" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
      </button>
      {!collapsed && (
        <div style={{ padding: '0 12px 8px' }}>
          {agents.map(a => (
            <div key={a.id} style={{ display: 'flex', gap: 8, alignItems: 'center', padding: '4px 0', fontSize: 12, color: tokens.muted }}>
              <span style={{ width: 8, height: 8, borderRadius: '50%', background: a.role === 'pm' ? tokens.cta : tokens.cta, display: 'inline-block', opacity: a.role === 'pm' ? 1 : 0.5 }} />
              <span style={{ fontFamily: tokens.fontHeading, color: tokens.text }}>{a.id}</span>
              <span>{a.role}</span>
              {a.parent_id && <span style={{ color: tokens.muted }}>← {a.parent_id}</span>}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 2: 创建 ToolCallCard**

Create `frontend/src/features/chat/ToolCallCard.tsx`:

```tsx
import { tokens } from '../../shared/design-tokens';
import { useState } from 'react';

export function ToolCallCard({ tool, input, output, error }: {
  tool: string; input: string; output?: string; error?: string;
}) {
  const [expanded, setExpanded] = useState(false);
  return (
    <div style={{
      margin: '8px 0', padding: 12, borderRadius: 8,
      background: tokens.surface, border: `1px solid ${tokens.border}`,
      fontFamily: tokens.fontBody, fontSize: 13,
    }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', cursor: 'pointer' }}
        onClick={() => setExpanded(!expanded)}>
        <span style={{ fontFamily: tokens.fontHeading, color: tokens.cta, fontWeight: 600, display: 'flex', alignItems: 'center', gap: 6 }}>
          <svg width="14" height="14" viewBox="0 0 14 14" fill="none" aria-hidden="true">
            <path d="M2.5 3.5l3 3-3 3M7.5 3.5l3 3-3 3" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
          </svg>
          {tool}
        </span>
        <svg width="12" height="12" viewBox="0 0 12 12" fill="none" aria-hidden="true"
          style={{ transform: expanded ? 'rotate(180deg)' : undefined, transition: 'transform 200ms' }}>
          <path d="M3 5l3 3 3-3" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
      </div>
      {expanded && (
        <div style={{ marginTop: 8 }}>
          <div style={{ color: tokens.muted, marginBottom: 4 }}>Input:</div>
          <pre style={{ background: tokens.bg, padding: 8, borderRadius: 4, color: tokens.text, fontSize: 12, overflow: 'auto', maxHeight: 120 }}>
            {input}
          </pre>
          {output && <><div style={{ color: tokens.muted, marginBottom: 4, marginTop: 8 }}>Output:</div>
            <pre style={{ background: tokens.bg, padding: 8, borderRadius: 4, color: tokens.text, fontSize: 12, overflow: 'auto', maxHeight: 120 }}>
              {output}
            </pre></>
          }
          {error && <div style={{ color: tokens.error, marginTop: 4 }}>Error: {error}</div>}
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 3: 集成到 ChatPanel**

```tsx
import { AgentPanel } from './AgentPanel';
// In ChatPanel: <AgentPanel agents={agents} /> above MessageList
```

- [ ] **Step 4: 编译 + Commit**

```bash
cd frontend && npx tsc --noEmit
git add frontend/src/features/chat/
git commit -m "feat(frontend): add Agent panel and ToolCall visualization cards"
```

---

### Task 6: E2E 验证

- [ ] **Step 1: Go 全量测试**

```bash
go test ./internal/agent/... ./internal/tool/... ./internal/pipeline/... ./internal/server/... -count=1
```

- [ ] **Step 2: 前端编译 + 构建**

```bash
cd frontend && npx tsc --noEmit && npm run build
```

- [ ] **Step 3: Go build**

```bash
go build ./cmd/server/
```

- [ ] **Step 4: 全栈启动**

```bash
go run ./cmd/server/ --addr :8030 &
cd frontend && npm run dev
```

验证: Chat 面板显示 Agent 状态面板, 工具调用显示可展开卡片

- [ ] **Step 5: Commit**

```bash
git commit -m "chore(phase5b): final verification — all tests pass, builds clean"
```

---

## Phase 5b Acceptance Criteria

| # | Criterion | Verification |
|---|-----------|-------------|
| 1 | Tool Registry 注册 5 个工具 + 关键词搜索 | automated (test) |
| 2 | AgentCoordinator Spawn ≤5, Delegate, Broadcast | automated (test) |
| 3 | Sub-Pipeline Fork 继承 parent context | automated |
| 4 | Gate Hooks pre/post 拦截器链 | automated |
| 5 | AgentPanel 显示多 Agent 状态 | visual |
| 6 | ToolCallCard 可展开显示 input/output | visual |
| 7 | `go build ./cmd/server/` | automated |
| 8 | `npm run build` 零错误 | automated |

---

## Phase 5 两阶段总览

| 阶段 | 内容 | 任务数 |
|------|------|--------|
| **5a** (已计划) | LLM Provider 抽象 + Translator + YAML 模型配置 + 回退链 + 模型选择器 UI | 6 |
| **5b** (本文档) | Multi-Agent Coordinator + Tool Registry + 子 Pipeline Fork + Gate Hooks + Agent 面板 UI | 6 |

**延后至 Phase 6+:** Redis Streams CSP 通道、嵌入索引(all-MiniLM)、MCP 动态工具发现、Learning Engine、Zustand 状态管理
