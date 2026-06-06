# OpenForge — Design Spec

> Human-readable design narrative — rationale, audience, style, color choices, content outline.

## I. Project Information

| Item | Value |
| ---- | ----- |
| **Project Name** | openforge_roadshow |
| **Canvas Format** | PPT 16:9 (1280×720) |
| **Page Count** | 10 |
| **Design Style** | C) Top Consulting + dark tech minimal |
| **Target Audience** | Technical experts + investors (技术专家 + 投资人) |
| **Use Case** | Agent 辅助全栈挑战赛 · 题一「AI 工程工具」3-5 分钟路演 |
| **Created Date** | 2026-05-30 |

---

## II. Canvas Specification

| Property | Value |
| -------- | ----- |
| **Format** | PPT 16:9 |
| **Dimensions** | 1280×720 |
| **viewBox** | `0 0 1280 720` |
| **Margins** | left/right 60px, top 50px, bottom 40px |
| **Content Area** | 1160×630 (from margins); safe title zone 1160×100, body zone 1160×490 |

---

## III. Visual Theme

### Theme Style

- **Style**: Top Consulting + dark tech minimal
- **Theme**: Dark theme
- **Tone**: Professional, technical, innovative, authoritative — "engineered depth with investor clarity"

### Color Scheme

> User-specified; locked directly.

| Role | HEX | Purpose |
| ---- | --- | ------- |
| **Background** | `#1a1a2e` | Page background — deep navy |
| **Secondary bg** | `#16213e` | Card background, section background |
| **Primary** | `#00d4ff` | Title decorations, key sections, icons, primary emphasis |
| **Accent** | `#7c3aed` | Data highlights, secondary emphasis, gradient partner |
| **Secondary accent** | `#00a8cc` | Gradient transitions, hover-state equivalents |
| **Body text** | `#ffffff` | Main body text on dark background |
| **Secondary text** | `#b0b8d0` | Captions, annotations, secondary descriptions |
| **Tertiary text** | `#6b7280` | Supplementary info, page numbers, footers |
| **Border/divider** | `#2a2a5e` | Card borders, divider lines — faint cyan-tinted dark border |
| **Success** | `#10b981` | Positive indicators, green checkmarks |
| **Warning** | `#ef4444` | Issue markers, red alerts |

> ❌ Forbidden: purple→pink gradient, emoji as icons, hand-drawn logos, Inter/Roboto as title fonts.

### Gradient Scheme

```xml
<!-- Title accent gradient: cyan → purple (user-specified direction) -->
<linearGradient id="titleGradient" x1="0%" y1="0%" x2="100%" y2="0%">
  <stop offset="0%" stop-color="#00d4ff"/>
  <stop offset="100%" stop-color="#7c3aed"/>
</linearGradient>

<!-- Background decorative glow -->
<radialGradient id="bgGlow" cx="85%" cy="15%" r="60%">
  <stop offset="0%" stop-color="#00d4ff" stop-opacity="0.08"/>
  <stop offset="100%" stop-color="#00d4ff" stop-opacity="0"/>
</radialGradient>

<!-- Card subtle top border gradient -->
<linearGradient id="cardTopBorder" x1="0%" y1="0%" x2="100%" y2="0%">
  <stop offset="0%" stop-color="#00d4ff" stop-opacity="0.6"/>
  <stop offset="100%" stop-color="#7c3aed" stop-opacity="0.2"/>
</linearGradient>
```

> ⚠️ The cyan→purple gradient is a user-approved accent for title decorations and card borders. It is NOT a pink gradient (purple #7c3aed, not pink). The user explicitly forbad purple→pink.

### AI Image Strategy

- **Image Rendering**: editorial
- **Image Palette**: dark-cinematic

> Only 1 AI row (cover background). `editorial × dark-cinematic` pairs deep-toned magazine-layout aesthetic with the deck's existing dark navy palette. No conflict with user colors — dark-cinematic's temperament can carry #1a1a2e / #00d4ff / #7c3aed naturally.

---

## IV. Typography System

### Font Plan

**Typography direction**: geometric sans title × monospace body — tech-forward contrast

| Role | Chinese | English | Fallback tail |
| ---- | ------- | ------- | ------------- |
| **Title** | `"Microsoft YaHei", "PingFang SC"` | `Montserrat, Arial` | `sans-serif` |
| **Body** | — | `Consolas, "Courier New"` | `monospace` |
| **Emphasis** | — | `Arial, "Segoe UI"` | `sans-serif` |
| **Code** | — | `Consolas, "Courier New"` | `monospace` |

**Per-role font stacks**:

- Title: `Montserrat, Arial, "Microsoft YaHei", "PingFang SC", sans-serif`
- Body: `Consolas, "Courier New", "Microsoft YaHei", monospace`
- Emphasis: `Arial, "Segoe UI", "Microsoft YaHei", sans-serif`
- Code: `Consolas, "Courier New", monospace`

> ⚠️ Montserrat requires install or PPTX embed; Arial is the PPT-safe geometric-sans fallback. JetBrains Mono was the user preference; Consolas is the closest PPT-safe monospace alternative (pre-installed on all Windows systems).

### Font Size Hierarchy

**Baseline**: Body font size = **18px** (dense roadshow deck, 3-5 points per page)

| Purpose | Ratio to body | Range @ body=18 | Weight |
| ------- | ------------- | --------------- | ------ |
| Cover title (hero headline) | 2.5-5x | 45-90px | Bold |
| Page title | 1.5-2x | 27-36px | Bold |
| Hero number (KPIs) | 1.5-2x | 27-36px | Bold |
| Subtitle | 1.2-1.5x | 22-27px | SemiBold |
| **Body content** | **1x** | **18px** | Regular |
| Annotation / caption | 0.7-0.85x | 13-15px | Regular |
| Page number / footnote | 0.5-0.65x | 9-12px | Regular |

---

## V. Layout Principles

### Page Structure

- **Header area**: Top 100px — page title with cyan left-edge accent bar (4px × 40px) + gradient underline
- **Content area**: 100px–680px — flexible layout per page rhythm
- **Footer area**: Bottom 40px — page number (right-aligned), subtle divider line

### Layout Pattern Library

| Pattern | Suitable Scenarios |
| ------- | ----------------- |
| **Single column centered** | Covers (P01), summary (P10) |
| **Three/four column cards** | Feature lists (P05), protocol cards (P06) |
| **Top-bottom split** | Architecture diagram (P03), workflow (P04) |
| **Matrix grid (2×2)** | Screenshot placeholders (P08) |
| **Asymmetric split (2:8)** | Pain points (P02) — icon left, text right |
| **Full-bleed + floating text** | Cover (P01) with AI background |
| **Center-radiating** | Metrics overview (P09) — 6 KPI cards around center |

### Spacing Specification

**Universal**:

| Element | Value |
| ------- | ----- |
| Safe margin from canvas edge | 60px |
| Content block gap | 32px |
| Icon-text gap | 12px |

**Card-based layouts**:

| Element | Value |
| ------- | ----- |
| Card gap | 24px |
| Card padding | 24px |
| Card border radius | 12px |
| Single-row card height | 540px |
| Three-column card width | 360px |

**Non-card containers**:

- Line-height: 1.5× body font size
- Full-bleed text placement: inset from focal points, gradient overlay for legibility

---

## VI. Icon Usage Specification

### Source

- **Built-in icon library**: `phosphor-duotone` — duotone style, main shape + 20% opacity backplate, contemporary depth on dark backgrounds
- **Usage method**: `<use data-icon="phosphor-duotone/<icon>" ... fill="#00d4ff"/>`

### Recommended Icon List

| Purpose | Icon Path | Page |
| ------- | --------- | ---- |
| Pain point warning | `phosphor-duotone/warning-circle` | P02 |
| Architecture layers | `phosphor-duotone/stack-simple` | P03 |
| Data storage | `phosphor-duotone/database` | P03, P07 |
| Compute/runtime | `phosphor-duotone/cpu` | P03 |
| Process flow | `phosphor-duotone/flow-arrow` | P04 |
| Conversational AI | `phosphor-duotone/chat-centered-dots` | P05 |
| Code/development | `phosphor-duotone/code-simple` | P05 |
| Agent collaboration | `phosphor-duotone/users-three` | P05 |
| Security | `phosphor-duotone/shield-check` | P05, P07 |
| Deployment speed | `phosphor-duotone/rocket` | P05 |
| AI/brain | `phosphor-duotone/brain` | P05 |
| IDE/monitor | `phosphor-duotone/monitor` | P08 |
| Settings/engine | `phosphor-duotone/gear-six` | P05 |
| Cloud/deploy | `phosphor-duotone/cloud-arrow-up` | P05 |
| Metrics | `phosphor-duotone/graph` | P09 |
| Global/web | `phosphor-duotone/globe-hemisphere-east` | P06 |
| Real-time speed | `phosphor-duotone/lightning` | P06 |
| Audit/fingerprint | `phosphor-duotone/fingerprint` | P07 |
| Encryption/key | `phosphor-duotone/key` | P07 |
| Achievement | `phosphor-duotone/star-four` | P10 |
| Excellence | `phosphor-duotone/trophy` | P10 |
| Target/metric | `phosphor-duotone/target` | P09 |

---

## VII. Visualization Reference List

Catalog read: 71 templates

| Page | Template | Path | Summary-quote (verbatim from `charts_index.json`) | Usage |
| ---- | -------- | ---- | ------------------------------------------------- | ----- |
| P02 | vertical_list | `templates/charts/vertical_list.svg` | "Pick for 3-6 numbered key points each with a short description — design principles, core tenets, action items, key takeaways, recommendations, executive summary points." | 3 pain points with descriptions — 沟通断层 / 全链路复杂 / AI 现状不足 |
| P03 | layered_architecture | `templates/charts/layered_architecture.svg` | "Pick for 3-4 horizontal architecture layers (presentation/service/data), 2-4 module cards per layer, each card = title + 1-line description (description required, even if source brief)." | 3-layer architecture (C/A/B) + base, each layer with component cards |
| P04 | numbered_steps | `templates/charts/numbered_steps.svg` | "Pick for 3-6 horizontal sequential steps with numeric emphasis — how-it-works section, getting-started guide, methodology overview, implementation phases." | 5-step workflow: 需求输入→PM澄清→Pipeline调度→测试审批→部署验证 |
| P05 | icon_grid | `templates/charts/icon_grid.svg` | "Pick for 4-9 parallel features/capabilities/services as icon cards — feature grid, service lineup, benefits matrix, brand values, product highlights." | 8 core capabilities as icon+title+description cards |
| P06 | labeled_card | `templates/charts/labeled_card.svg` | "Pick for 3-4 parallel aspects of one subject with per-aspect titles + short body (self-introduction, four-pillar overview, capability quadrant)." | 3 protocol cards — REST API / WebSocket / gRPC |
| P07 | vertical_list | `templates/charts/vertical_list.svg` | "Pick for 3-6 numbered key points each with a short description — design principles, core tenets, action items, key takeaways, recommendations, executive summary points." | 4 DB/security highlights — 22 tables / WORM / partitions / RBAC |
| P09 | kpi_cards | `templates/charts/kpi_cards.svg` | "Pick for 4-8 standalone numeric metrics shown as overview cards (2x2 or 1x4) — exec summary opener, dashboard headline, quarterly recap, results-at-a-glance." | 6 technical metrics — 196 commits / 22 tables / 40+ APIs / 12 events / 6 protos / 3 configs |
| P10 | labeled_card | `templates/charts/labeled_card.svg` | "Pick for 3-4 parallel aspects of one subject with per-aspect titles + short body (self-introduction, four-pillar overview, capability quadrant)." | 3 key differentiators as large numbered cards |

**Runners-up considered**:

- `process_flow` | rejected for P04: source explicitly lists numbered sequential steps with emphasis on order, not connected-arrow pipeline flow
- `vertical_pillars` | rejected for P05: icon_grid is more appropriate for 8 equally-weighted parallel capabilities vs pillar-style column layout
- `comparison_columns` | rejected for P06: labeled_card fits "parallel aspects of one subject" with body text better than pricing-tier column layout

---

## VIII. Image Resource List

| Filename | Dimensions | Ratio | Purpose | Layout pattern | Acquire Via | Status | Reference | text_policy | page_role |
| -------- | --------- | ----- | ------- | -------------- | ----------- | ------ | --------- | ----------- | --------- |
| cover_bg.png | 1280×720 | 1.78 | Cover background — dark abstract tech atmosphere | #1 full-bleed background with floating title + #29 two-stop scrim | ai | Pending | Dark abstract digital landscape suggesting code infrastructure and AI orchestration; vast central area for centered title overlay; subtle geometric grid lines fading into deep navy void; no human figures, no text | none | |
| screenshot_login.png | 960×540 | 1.78 | Login + project list screenshot | #50 tiled grid 2×2 | placeholder | Placeholder | [screenshot-pending] — login page and project list UI | | |
| screenshot_ide.png | 960×540 | 1.78 | Pro Mode IDE (Chat+Diff+File tree) | #50 tiled grid 2×2 | placeholder | Placeholder | [screenshot-pending] — Pro Mode IDE with Chat, Diff, and File Tree panels | | |
| screenshot_pipeline.png | 960×540 | 1.78 | Pipeline approval page | #50 tiled grid 2×2 | placeholder | Placeholder | [screenshot-pending] — Pipeline approval interface | | |
| screenshot_admin.png | 960×540 | 1.78 | Admin dashboard (circuit breakers, skill management) | #50 tiled grid 2×2 | placeholder | Placeholder | [screenshot-pending] — Admin dashboard with circuit breaker and skill management | | |

> P08 uses a single #50 tiled grid (2×2) with 4 equal cells. Cover bg is the single AI row — `editorial × dark-cinematic` applied deck-wide per §III.

---

## IX. Content Outline

### Part 1: OpenForge Roadshow

#### Slide 01 — Cover

- **Layout**: #1 full-bleed background with floating title + #29 two-stop scrim (dark gradient overlay fading upward from bottom-center)
- **Rhythm**: anchor
- **Title**: OpenForge
- **Subtitle**: AI 驱动的端到端全栈开发工作台
- **Info**: Agent 辅助全栈挑战赛 · 题一「AI 工程工具」
- **Annotation**: [logo-pending] — text-only placeholder, no hand-drawn logo

#### Slide 02 — Pain Points

- **Layout**: Asymmetric split (icon column left 120px + text body right); 3 numbered items
- **Rhythm**: dense
- **Title**: 全栈开发的三座大山
- **Visualization**: vertical_list
- **Content**:
  - 01 · 需求→代码：沟通断层，反复返工 — 产品需求到技术实现存在天然鸿沟，传统流程中 PM 和开发者反复对齐、来回修改
  - 02 · 全链路环节太多 — 需求澄清→方案拆解→代码实现→自动化测试→部署上线→验证反馈，六个环节工具割裂
  - 03 · 现有 AI 工具只做"代码补全" — Copilot/Cursor 解决的是编码片段，不是工程管理全流程

#### Slide 03 — Architecture

- **Layout**: Top-bottom split — architecture diagram filling upper 70%, annotation text below
- **Rhythm**: breathing
- **Title**: 三层正交架构
- **Visualization**: layered_architecture
- **Content**:
  - C 层 · 工作台 — React + Dockview 多面板 + WebSocket 实时通信
  - A 层 · Pipeline 引擎 — Go · 9 状态机 · 审批门禁 · 回溯
  - B 层 · Agent 集群 — Go + Node.js · CSP channel 通信 · 多 Agent 协作
  - 底座 — PostgreSQL + Redis + Docker + Nginx
- **Annotation**: Proto/gRPC/WebSocket 解耦 — 每层独立演进，互不阻塞

#### Slide 04 — Workflow

- **Layout**: Top-bottom — title + 5 horizontal numbered steps
- **Rhythm**: dense
- **Title**: 一次对话，完成全部
- **Visualization**: numbered_steps
- **Content** (5 sequential steps):
  - 01 · 输入需求 — NL 对话，无需学习工具
  - 02 · PM Agent 澄清拆解 — 结构化需求→任务拆分
  - 03 · Pipeline 调度 Agent — 自动分配、并行执行
  - 04 · 自动测试 + 审批 — 门禁检查、人工审批节点
  - 05 · 一键部署 + 验证反馈 — Docker 沙箱、实时监控
- **Annotation**: 所有步骤通过对话驱动，无需切换工具

#### Slide 05 — Core Capabilities

- **Layout**: 2×4 icon card grid (4 rows × 2 columns)
- **Rhythm**: dense
- **Title**: 8 大核心能力
- **Visualization**: icon_grid
- **Content** (8 icon cards, icon + title + one-line description each):
  - 💬 对话式 PM — NL→结构化需求→任务拆解
  - ⚙️ Pipeline 状态机 — 9 状态 + 审批门禁 + 回溯
  - 🧠 Agent 集群 — 多 Agent 协作 + CSP 通信
  - 🖥️ Pro Mode IDE — Dockview 多面板 + Monaco 编辑器
  - 📊 LLM 路由 — 多 Provider + Token 计量
  - 🛡️ 沙箱安全 — Docker 隔离 + 5 层防御
  - 🔐 合规审计 — SHA256 哈希链 + WORM
  - 🚀 渐进运维 — 3 档配置，同一套代码

#### Slide 06 — Protocols & Communication

- **Layout**: 3-column cards (equal width, side-by-side)
- **Rhythm**: dense
- **Title**: 完整的协议栈
- **Visualization**: labeled_card
- **Content** (3 cards):
  - REST API — 40+ 端点，覆盖项目管理、Pipeline、审批、成本、管理
  - WebSocket — 12 种事件类型，实时聊天流、工具代理、阶段变更
  - gRPC — 6 个服务，Agent 调度、LLM 路由、工具注册、学习引擎

#### Slide 07 — Database & Security

- **Layout**: Asymmetric split — icon column left 120px + numbered list right
- **Rhythm**: dense
- **Title**: 工程级数据架构
- **Visualization**: vertical_list
- **Content** (4 key points):
  - 01 · 22 张表 · UUID v7 · 乐观锁 — 完整的数据模型覆盖
  - 02 · WORM 审计日志 + SHA256 哈希链 — 不可篡改的合规审计
  - 03 · 月度分区 + 法定留存 — 性能与合规兼顾
  - 04 · 会话分支 / 软删除 / JWT + RBAC — 灵活且安全的权限体系

#### Slide 08 — Product Showcase

- **Layout**: 2×2 grid of screenshot placeholders with labels
- **Rhythm**: breathing
- **Title**: 产品展示
- **Visualization**: none (placeholder grid)
- **Content** (4 placeholder cells):
  - 左上：登录 + 项目列表
  - 右上：Pro Mode IDE（Chat + Diff + 文件树）
  - 左下：Pipeline 审批页
  - 右下：管理后台（断路器 / 技能管理）
- **Annotation**: 每个占位标注 [screenshot-pending]，用户后续替换

#### Slide 09 — Key Metrics

- **Layout**: 2×3 KPI card grid
- **Rhythm**: anchor
- **Title**: 技术指标
- **Visualization**: kpi_cards
- **Content** (6 KPI cards, large number + label):
  - 196 · Commits
  - 22 · 数据表
  - 40+ · API 端点
  - 12 · WebSocket 事件
  - 6 · Proto 服务
  - 3 · 档配置（minimal / standard / enterprise）

#### Slide 10 — Summary

- **Layout**: 3 large horizontal cards, stacked vertically with generous spacing
- **Rhythm**: anchor
- **Title**: OpenForge = 3 个关键差异
- **Visualization**: labeled_card
- **Content** (3 large cards, icon + number + title + description):
  - 1️⃣ 不只是代码补全 — 覆盖需求→部署全链路，从 PM 对话到一键上线
  - 2️⃣ 不只是单 Agent — CSP 多 Agent 集群协作，Go 原生 channel 通信
  - 3️⃣ 不只是 Demo — minimal→enterprise 渐进演进，同一套代码、三档部署
- **Footer**: GitHub 链接 + 联系方式

---

## X. Speaker Notes Requirements

One speaker note file per page, saved to `notes/`:

- **Filename**: match SVG name (e.g., `01_cover.md`)
- **Total duration**: 3-5 minutes (~20-30 seconds per page)
- **Style**: Formal with technical precision — concise, conclusion-first, investor-ready
- **Purpose**: Persuade — convince technical experts and investors of OpenForge's innovation and completeness
- **Content**: script key points, timing cues, transition phrases

---

## XI. Technical Constraints Reminder

### SVG Generation Must Follow:

1. viewBox: `0 0 1280 720`
2. Background uses `<rect>` elements
3. Text wrapping uses `<tspan>` (`<foreignObject>` FORBIDDEN)
4. Transparency uses `fill-opacity` / `stroke-opacity`; `rgba()` FORBIDDEN
5. FORBIDDEN: `mask`, `<style>`, `class`, `foreignObject`
6. FORBIDDEN: `textPath`, `animate*`, `script`
7. Text characters: write typography & symbols as raw Unicode (em dash `—`, en dash `–`, `©`, `®`, `→`, NBSP, etc.); HTML named entities (`&nbsp;`, `&mdash;`, `&copy;`, `&reg;` …) are FORBIDDEN. XML reserved chars in text MUST be escaped as `&amp;` `&lt;` `&gt;` `&quot;` `&apos;`
8. `clipPath` conditionally allowed **only on `<image>` elements**
9. `<g opacity="...">` FORBIDDEN (group opacity); set on each child element individually

### PPT Compatibility Rules:

- `<g opacity="...">` FORBIDDEN
- Image transparency uses overlay mask layer (`<rect fill="bg-color" opacity="0.x"/>`)
- Inline styles only; external CSS and `@font-face` FORBIDDEN
