# Agent Prompt：生成 OpenForge 路演 PPT

> 将此 Prompt 发给任意支持 PPT 生成的 AI Agent（如 Gamma、Gamma AI、Claude with PPT skill）

---

## 任务

请根据以下项目大纲和技术细节，生成一份 **3-5 分钟路演 PPT**。输出格式：**.pptx** 或 .pdf。

---

## 项目名称

**OpenForge** — AI-driven end-to-end full-stack development workbench

---

## 目标评委

技术专家 + 投资人，关注技术创新性和工程完整度。

---

## 设计要求

- **风格：** 现代极简，深色主题（#1a1a2e 底色，白色/青色文字）
- **字体：** 标题用无衬线几何字体（如 Montserrat），正文用系统等宽字体（如 JetBrains Mono）
- **配色：** 主色 #00d4ff（青色），辅色 #7c3aed（紫），不要在渐变色中使用粉色
- **禁止：** Emoji 作为图标、紫色→粉色渐变、手绘 Logo、Inter/Roboto 作为标题字体
- **每页：** 标题 + 3-5 个要点，不要大段文字
- **页数：** 8-10 页

---

## 幻灯片内容（严格按此结构）

### Slide 1 · 封面

- 标题：OpenForge
- 副标题：AI 驱动的端到端全栈开发工作台
- 底部小字：Agent 辅助全栈挑战赛 · 题一「AI 工程工具」
- Logo 占位：[logo-pending]（不要手绘，用文字替代）

### Slide 2 · 痛点

- 标题：全栈开发的三座大山
- 要点：
  - 需求→代码：沟通断层，反复返工
  - 全链路：需求/设计/实现/测试/部署/审计，环节太多
  - AI 现状：现有工具只做"代码补全"，不是"工程管理"

### Slide 3 · 架构

- 标题：三层正交架构
- 图示：三层堆叠图
  - C 层 · 工作台（React + Dockview + WebSocket）
  - A 层 · Pipeline 引擎（Go · 9 状态机 · 审批门禁）
  - B 层 · Agent 集群（Go + Node.js · CSP · 多 Agent)
  - 底座：PG + Redis + Docker + Nginx
- 关键标注：Proto/gRPC/WebSocket 解耦，每层独立演进

### Slide 4 · 工作流程

- 标题：一次对话，完成全部
- 流程箭头图：
  ```
  💬输入需求 → 🔍PM Agent澄清拆解 → 🔧Pipeline调度Agent →
  ✅自动测试+审批 → 🚀一键部署 → 📊验证反馈
  ```
- 底部标注：所有步骤通过对话驱动，无需切换工具

### Slide 5 · 核心能力

- 标题：8 大核心能力
- 要点（每项一行）：
  - 对话式 PM — NL→结构化需求→任务拆解
  - Pipeline 状态机 — 9 状态 + 审批门禁 + 回溯
  - Agent 集群 — 多 Agent 协作 + CSP 通信
  - Pro Mode IDE — Dockview 多面板 + Monaco 编辑器
  - LLM 路由 — 多 Provider + Token 计量
  - 沙箱安全 — Docker 隔离 + 5 层防御
  - 合规审计 — SHA256 哈希链 + WORM
  - 渐进运维 — 3 档配置，同一套代码

### Slide 6 · 协议与通信

- 标题：完整的协议栈
- 三个卡片排列：
  - REST API：40+ 端点（项目管理/Pipeline/审批/成本/管理）
  - WebSocket：12 种事件（实时聊天流/工具代理/阶段变更）
  - gRPC：6 个服务（Agent调度/LLM路由/工具注册/学习引擎）

### Slide 7 · 数据库与安全

- 标题：工程级数据架构
- 要点：
  - 22 张表 · UUID v7 · 乐观锁
  - WORM 审计日志 + SHA256 哈希链
  - 月度分区 + 法定留存
  - 会话分支 / 软删除 / JWT + RBAC

### Slide 8 · Demo 截图（预留）

- 标题：产品展示
- 4 张截图占位（标注 [screenshot-pending]）：
  - 左上：登录 + 项目列表
  - 右上：Pro Mode IDE（Chat+Diff+文件树）
  - 左下：Pipeline 审批页
  - 右下：管理后台（断路器/技能管理）

### Slide 9 · 关键指标

- 标题：技术指标
- 数字卡片（强调视觉冲击）：
  - 196 commits
  - 22 张数据表
  - 40+ API 端点
  - 12 WebSocket 事件
  - 6 Proto 服务
  - 3 档配置（minimal / standard / enterprise）

### Slide 10 · 总结

- 标题：OpenForge = 3 个关键差异
- 三个大号卡片：
  - 1️⃣ 不只是代码补全 — 覆盖需求→部署全链路
  - 2️⃣ 不只是单 Agent — CSP 多 Agent 集群协作
  - 3️⃣ 不只是 Demo — minimal→enterprise 渐进演进
- 底部：GitHub 链接 + 联系方式

---

## 技术约束

- 输出格式：.pptx（优先）或 PDF
- 16:9 宽屏比例
- 所有代码/技术术语使用等宽字体
- 不要使用 Emoji 作为图标（用 SVG icon 或文字）
- 配色不要使用紫色→粉色渐变
- 如果工具不支持自定义字体，使用系统默认即可

---

## 项目背景（供 Agent 理解上下文）

OpenForge 是一个 AI 辅助全栈开发平台，核心创新在于：

1. **全链路覆盖：** 不只是代码生成，而是从需求澄清→方案拆解→实现→测试→部署→验证的全流程
2. **三层正交架构：** C层（工作台UI）/ A层（Pipeline引擎）/ B层（Agent集群），通过 Proto/gRPC/WebSocket 解耦
3. **多Agent协作：** 使用 Go 原生 CSP channel 实现 Agent 间通信（非消息队列）
4. **形式化状态机：** Pipeline 有严格的 9 状态转移矩阵和回溯限制
5. **渐进式运维：** 同一套代码通过 YAML profile 切换 minimal/standard/enterprise 三档部署
6. **工程完整度：** 22 张表、40+ API、6 个 gRPC 服务、12 种 WebSocket 事件、JWT+RBAC、WORM 审计、SHA256 哈希链

技术栈：Go · React 18 + TypeScript · PostgreSQL 15+ · Docker · gRPC · WebSocket · Node.js
