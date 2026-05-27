# 优雅的大模型思考与工具链调用折叠 (Elegant Thinking & Tool Call Fold) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 
在聊天历史记录或当前流中渲染 Agent 消息时，自动提取并隐藏冗余的原始 `tool_use`/`functioncall` 代码块，并将其封装为小一号、浅一号字体的“思考与工具执行痕迹”折叠卡片，避免页面杂乱。

**Architecture:**
- **解析器**：在前端编写一个 robust 的消息拆分工具 `parseAgentContent(content: string)`。它支持解析 Anthropic 的 Multi-part JSON 块，并能利用正则匹配出普通 Markdown 文本中的 ````json ...tool_use... ```` 块。
- **渲染器**：将解析出的普通文本（Text Parts）渲染为 Markdown 主内容；将解析出的工具调用块（Thinking Parts）渲染为极其精致、带有点击展开详情功能的小字浅色折叠卡。
- **技术栈**：React, Lucide React (若存在，或直接用精美 SVG), CSS transition。

---

## File Map

- Modify: `frontend/src/features/chat/MessageList.tsx`

---

## Tasks

### Task 1: 编写核心解析与拆分逻辑

**Files:**
- Modify: `frontend/src/features/chat/MessageList.tsx`

- [ ] **Step 1: 在 `MessageList.tsx` 顶部或独立区域添加 `parseAgentContent` 辅助函数**

由于数据库可能返回 Multi-part JSON 或包含 ```json [...] ``` 标签的 Markdown，我们需要完美清洗、分类这两者。

```typescript
interface ContentPart {
  type: 'text' | 'thinking';
  content: string;
  toolName?: string;
  toolInput?: any;
}

function parseAgentContent(content: string): ContentPart[] {
  const parts: ContentPart[] = [];
  if (!content) return parts;

  // 1. 尝试解析 Anthropic 原始 JSON blocks
  try {
    if (content.trim().startsWith('[') && content.trim().endsWith(']')) {
      const blocks = JSON.parse(content);
      if (Array.isArray(blocks)) {
        blocks.forEach(b => {
          if (b.type === 'text' && b.text) {
            parts.push({ type: 'text', content: b.text });
          } else if (b.type === 'tool_use') {
            parts.push({
              type: 'thinking',
              content: `Calling tool: ${b.name}`,
              toolName: b.name,
              toolInput: b.input,
            });
          }
        });
        if (parts.length > 0) return parts;
      }
    }
  } catch {
    // 忽略解析错误，降级到 Markdown 正则解析
  }

  // 2. 正则匹配：提取 ```json\n[{"type":"tool_use"...}]\n``` 类似的围栏代码块
  // 匹配：(?s)```(?:json)?\s*(\[.*?\"tool_use\".*?\])\s*```
  let remaining = content;
  const fencedRe = /```(?:json)?\s*(\[\s*\{\s*"type"\s*:\s*"tool_use"[\s\S]*?\])\s*```/g;
  let match;
  let lastIndex = 0;

  while ((match = fencedRe.exec(content)) !== null) {
    const textBefore = content.substring(lastIndex, match.index);
    if (textBefore.trim()) {
      parts.push({ type: 'text', content: textBefore });
    }

    // 尝试解析 tool_use JSON
    try {
      const toolUseList = JSON.parse(match[1]);
      if (Array.isArray(toolUseList)) {
        toolUseList.forEach(tu => {
          parts.push({
            type: 'thinking',
            content: `Calling tool: ${tu.name}`,
            toolName: tu.name,
            toolInput: tu.input,
          });
        });
      } else {
        parts.push({ type: 'thinking', content: 'Tool execution logs' });
      }
    } catch {
      parts.push({ type: 'thinking', content: match[0] });
    }
    lastIndex = fencedRe.lastIndex;
  }

  const textAfter = content.substring(lastIndex);
  if (textAfter.trim() || parts.length === 0) {
    parts.push({ type: 'text', content: textAfter || content });
  }

  return parts;
}
```

---

### Task 2: 创建精致的 "Thinking / Tool Use" 折叠渲染组件

为了展现 UI/UX Pro Max 级的设计感，我们将设计一个名为 `AgentThinkingBlock` 的微型状态折叠组件。

**Files:**
- Modify: `frontend/src/features/chat/MessageList.tsx`

- [ ] **Step 1: 新建 `AgentThinkingBlock` 内部组件**

该组件采用 `Fira Code` 等宽字体、浅灰色 `tokens.muted`，并带有精致的圆角、1px 细边框。支持点击展开/折叠输入参数：

```typescript
import { useState } from 'react';

function AgentThinkingBlock({ part }: { part: ContentPart }) {
  const [expanded, setExpanded] = useState(false);
  const hasInput = part.toolInput && Object.keys(part.toolInput).length > 0;

  return (
    <div style={{
      margin: '6px 0',
      background: 'rgba(30, 41, 59, 0.4)',
      border: `1px solid ${tokens.border}`,
      borderRadius: 6,
      overflow: 'hidden',
      fontSize: 12,
      fontFamily: tokens.fontHeading,
      transition: tokens.transition,
    }}>
      <div 
        onClick={() => hasInput && setExpanded(!expanded)}
        style={{
          padding: '6px 12px',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          cursor: hasInput ? 'pointer' : 'default',
          userSelect: 'none',
          color: tokens.muted,
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
          {/* 小号优雅指示灯/图标 */}
          <span style={{
            display: 'inline-block',
            width: 6,
            height: 6,
            borderRadius: '50%',
            background: tokens.cta,
            boxShadow: `0 0 4px ${tokens.cta}`,
          }} />
          <span>
            Thinking: Executing <strong>{part.toolName || 'tool'}</strong>
          </span>
        </div>
        {hasInput && (
          <span style={{ fontSize: 10, color: tokens.muted, opacity: 0.7 }}>
            {expanded ? 'Collapse ▴' : 'Expand ▾'}
          </span>
        )}
      </div>
      {expanded && hasInput && (
        <div style={{
          padding: '8px 12px',
          borderTop: `1px solid ${tokens.border}`,
          background: 'rgba(15, 23, 42, 0.3)',
          maxHeight: 200,
          overflowY: 'auto',
          whiteSpace: 'pre-wrap',
          color: tokens.muted,
          opacity: 0.85,
        }}>
          <code>{JSON.stringify(part.toolInput, null, 2)}</code>
        </div>
      )}
    </div>
  );
}
```

---

### Task 3: 在 `MessageList` 中应用多部分流式与渲染重构

我们需要重组消息主体渲染，由单纯的 `dangerouslySetInnerHTML` 改为：
1. 提取所有 parts。
2. 依次映射渲染 `text`（原样交给 marked）与 `thinking`（交给我们精心设计的折叠框）。

**Files:**
- Modify: `frontend/src/features/chat/MessageList.tsx`

- [ ] **Step 1: 修改 Agent 渲染区块，使用 parts 映射渲染**

在 `MessageList.tsx` 的 `messages.map` 循环中，定位到 `msg.role !== 'user'` 且 `msg.role !== 'tool'` 的分支，即原先的 markdown-body 容器：

```tsx
            <div style={{
              maxWidth: '80%', borderRadius: 8, padding: '10px 16px',
              background: msg.role === 'user' ? tokens.userBubble : msg.role === 'system' ? 'rgba(185,28,28,0.3)' : tokens.surface,
              color: msg.role === 'system' ? tokens.error : tokens.text,
              fontSize: 14, lineHeight: 1.6,
              overflowWrap: 'break-word',
              overflowX: 'auto',
              minWidth: 0,
            }}>
              {msg.role === 'user' ? (
                <p style={{ whiteSpace: 'pre-wrap', margin: 0 }}>{msg.content}</p>
              ) : (
                // 替换为按 parts 展开渲染
                <div>
                  {parseAgentContent(msg.content).map((part, pIdx) => {
                    if (part.type === 'thinking') {
                      return <AgentThinkingBlock key={pIdx} part={part} />;
                    }
                    return (
                      <div
                        key={pIdx}
                        className="markdown-body"
                        dangerouslySetInnerHTML={{ __html: renderMarkdown(part.content) }}
                        style={{
                          wordBreak: 'break-word',
                        }}
                      />
                    );
                  })}
                </div>
              )}
            </div>
```

---

### Task 4: 编译和语法验证

**Files:**
- Build Check

- [ ] **Step 1: 运行 linter 和编译确认无类型及冲突问题**
  Run: `cd frontend && npx tsc --noEmit`
  Expected: PASS with 0 errors
