<p align="center">
  <img alt="OpenForge" src="https://img.shields.io/badge/OpenForge-1a1a2e?style=for-the-badge" width="180" height="40"><br>
  <strong>AI 驱动的端到端全栈开发工作台</strong><br>
  <sub>从需求到部署 — 一切通过对话完成</sub>
</p>

<p align="center">
  <a href="https://github.com/Yorha9e/openforge"><img src="https://img.shields.io/badge/状态-活跃-success?style=flat-square" alt="状态"></a>
  <a href="https://go.dev/doc/install"><img src="https://img.shields.io/badge/go-1.22+-00ADD8?style=flat-square&logo=go" alt="Go"></a>
  <a href="https://nodejs.org"><img src="https://img.shields.io/badge/node-20+-339933?style=flat-square&logo=node.js" alt="Node.js"></a>
  <a href="https://react.dev"><img src="https://img.shields.io/badge/react-18+-61DAFB?style=flat-square&logo=react" alt="React"></a>
  <a href="https://www.postgresql.org"><img src="https://img.shields.io/badge/postgres-15+-4169E1?style=flat-square&logo=postgresql" alt="PostgreSQL"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-待定-lightgrey?style=flat-square" alt="许可证"></a>
</p>

---

<p align="center">
  <a href="README.md">English</a>
</p>

---

## 这是什么？

OpenForge 是一个 AI 驱动的全栈开发工作台。你描述想要构建什么，OpenForge 处理整个流程 — 需求分析、架构设计、编码实现、测试、部署和验证。

以 **Conduit（RealWorld 前后端 monorepo）** 为实验田。

```
你描述需求   →   PM Agent   →   Pipeline 引擎   →   已部署的代码
  "我想要一个           需求澄清、           构建、测试、          运行在
   博客系统"            方案拆解、           审查、部署            你的基础设施上
                       架构设计
```

## 架构

```
┌──────────────────────────────────────────────────────────┐
│                  C层 · 工作台                             │
│            React + Dockview 多面板 + WebSocket            │
├──────────────────────────────────────────────────────────┤
│                  A层 · Pipeline 引擎                      │
│          Go · 状态机 · 审批门禁 · 一键部署                │
├──────────────────────────────────────────────────────────┤
│                  B层 · Agent 集群                         │
│       Go + Node.js · CSP 通道通信 · 多Agent协作           │
├──────────────────────────────────────────────────────────┤
│                    基础设施层                             │
│          PostgreSQL · Redis Streams · Docker              │
└──────────────────────────────────────────────────────────┘
```

## 核心能力

| 模块 | 能力描述 |
|------|---------|
| **对话式 PM** | 自然语言 → 结构化需求 → 任务拆解 |
| **Pipeline 引擎** | 9 状态转换机 + 审批门禁 + 回溯机制 |
| **Agent 集群** | 多 Agent 协调器：Spawn / Delegate / Broadcast 模式 |
| **工具注册中心** | 7 个核心工具：文件读写、代码搜索、Shell 执行、数据库查询、Git、Web 抓取 |
| **Pro 模式** | 完整 IDE：聊天 + Diff + 文件树 + 终端 + 拓扑图，Dockview 多面板布局 |
| **LLM 路由** | 多提供商：Anthropic / OpenAI / DeepSeek，含 Token 计量 |
| **沙箱隔离** | Docker 容器隔离 + LRU 热池 + 5 层安全防护 |
| **合规审计** | WORM 审计日志 + 数据生命周期管理 + 月度分区 |

## 快速开始

### 环境要求

- **Go** 1.22+ · **Node.js** 20+ · **PostgreSQL** 15+ · **Docker**（可选）

### 启动步骤

```bash
# 1. 克隆仓库
git clone https://github.com/Yorha9e/openforge.git && cd openforge

# 2. 配置 Profile
cp config/profiles/minimal.yaml.example config/profiles/minimal.yaml
cp config/profiles/docker-dev.yaml.example config/profiles/docker-dev.yaml

# 3. 设置环境变量
export DB_PASSWORD=你的数据库密码
export JWT_SECRET=你的JWT密钥至少32字符
export ANTHROPIC_AUTH_TOKEN=你的Anthropic_API密钥

# 4. 启动数据库
docker compose up postgres -d

# 5. 执行迁移
go run cmd/openforge/main.go migrate up

# 6. 启动后端
go run cmd/server/main.go --addr :8030

# 7. 启动前端
cd frontend && npm install && npm run dev
```

打开 **http://localhost:5173** — 通过对话开始构建。

## 项目结构

```
openforge/
├── cmd/
│   ├── server/               REST + WebSocket API 服务
│   └── openforge/            CLI 入口
├── proto/                     Protocol Buffers 定义（6 服务）
│
├── internal/
│   ├── agent/                AI Agent：协调器、查询引擎、工具
│   ├── pipeline/             Pipeline：状态机、门禁、部署
│   ├── auth/                 认证鉴权：JWT、RBAC、OIDC
│   ├── llm/                  LLM：多提供商路由、翻译器、Token 计量
│   ├── observability/        可观测性：Prometheus、熔断器
│   └── shared/               共享内核、Profile、功能开关
│
├── frontend/                 React 18 + TypeScript + Vite
│   └── src/features/         聊天 / 代码审查 / 管理 / 监控
│
├── nodejs-io/                Node.js I/O 服务（gRPC）
├── migrations/               数据库迁移（PostgreSQL）
├── config/profiles/          能力配置文件（YAML）
├── deployments/              Docker、Nginx、TLS 配置
└── scripts/                  诊断与工具脚本
```

## 三级能力配置

一套代码，三档部署：

| 配置档 | 适用场景 | 关键特征 |
|--------|---------|---------|
| `minimal` | <10 人，单机开发 | SQLite / 本地文件 / 内存 |
| `standard` | 50-200 人，单可用区 K8s | PG / Redis / MinIO / Vault |
| `enterprise` | 合规行业，多可用区 | 高可用 + 灾备 + WORM + 飞书 + OIDC |

```bash
# 切换配置档
export OPENFORGE_PROFILE=standard
go run cmd/server/main.go
```

## 技术栈

| 层级 | 技术选型 |
|------|---------|
| **后端** | Go · gRPC · gorilla/websocket · bcrypt · golang-jwt |
| **前端** | React 18 · TypeScript · Vite · Dockview · Monaco Editor |
| **数据库** | PostgreSQL 15+ · UUID v7 · WORM 分区 · 22 张表 |
| **基础设施** | Docker Compose · Nginx · TLS · Redis Streams（Phase 5+） |
| **通信协议** | gRPC · WebSocket（14 上行 + 14 下行事件） |
| **AI/LLM** | Anthropic Claude · OpenAI · DeepSeek · 多提供商路由器 |

## 开发

```bash
# 运行所有测试
go test ./...

# 前端测试
cd frontend && npm test

# 构建
go build -o bin/server cmd/server/main.go
cd frontend && npm run build

# 代码检查
golangci-lint run
cd frontend && npm run lint
```

## 安全

**绝对不要提交：**
- `.env` 文件 · API 密钥 · 私钥 · 证书
- 含硬编码密码的配置文件
- 密码哈希（使用 `htpasswd -nbBC 10 "" <密码>` 生成）

**所有敏感信息通过环境变量注入。** 参考 `.env` → `.env.example` 模式。

## 许可证

*待定*

---

<p align="center">
  <sub>Agent 辅助全栈挑战赛 · 题一：AI 工程工具</sub>
</p>
