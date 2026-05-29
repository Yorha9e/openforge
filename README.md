<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://img.shields.io/badge/OpenForge-ffffff?style=for-the-badge&logo=data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCAyNCAyNCIgZmlsbD0ibm9uZSIgc3Ryb2tlPSJ3aGl0ZSIgc3Ryb2tlLXdpZHRoPSIyIiBzdHJva2UtbGluZWNhcD0icm91bmQiIHN0cm9rZS1saW5lam9pbj0icm91bmQiPjxwYXRoIGQ9Ik0xMiAyMHYtNiIvPjxwYXRoIGQ9Ik02IDIwdi00Ii8+PHBhdGggZD0iTTE4IDIwVjYiLz48cGF0aCBkPSJNOSAxNi44IDEyIDE0bDMgMi44TDEyIDE4WiIvPjxwYXRoIGQ9Ik0xMiAyVjE0Ii8+PHBhdGggZD0ibTggOCA0LTQgNCA0Ii8+PC9zdmc+">
    <source media="(prefers-color-scheme: light)" srcset="https://img.shields.io/badge/OpenForge-1a1a2e?style=for-the-badge&logo=data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCAyNCAyNCIgZmlsbD0ibm9uZSIgc3Ryb2tlPSJ3aGl0ZSIgc3Ryb2tlLXdpZHRoPSIyIiBzdHJva2UtbGluZWNhcD0icm91bmQiIHN0cm9rZS1saW5lam9pbj0icm91bmQiPjxwYXRoIGQ9Ik0xMiAyMHYtNiIvPjxwYXRoIGQ9Ik02IDIwdi00Ii8+PHBhdGggZD0iTTE4IDIwVjYiLz48cGF0aCBkPSJNOSAxNi44IDEyIDE0bDMgMi44TDEyIDE4WiIvPjxwYXRoIGQ9Ik0xMiAyVjE0Ii8+PHBhdGggZD0ibTggOCA0LTQgNCA0Ii8+PC9zdmc+">
    <img alt="OpenForge" src="https://img.shields.io/badge/OpenForge-1a1a2e?style=for-the-badge">
  </picture>
</p>

<p align="center">
  <strong>AI-driven end-to-end full-stack development workbench</strong><br>
  <sub>From requirement clarification to deployment verification — all through conversation</sub>
</p>

<p align="center">
  <a href="https://github.com/Yorha9e/openforge"><img src="https://img.shields.io/badge/status-active-success?style=flat-square" alt="Status"></a>
  <a href="https://go.dev/doc/install"><img src="https://img.shields.io/badge/go-1.22+-00ADD8?style=flat-square&logo=go" alt="Go"></a>
  <a href="https://nodejs.org"><img src="https://img.shields.io/badge/node-20+-339933?style=flat-square&logo=node.js" alt="Node.js"></a>
  <a href="https://react.dev"><img src="https://img.shields.io/badge/react-18+-61DAFB?style=flat-square&logo=react" alt="React"></a>
  <a href="https://www.postgresql.org"><img src="https://img.shields.io/badge/postgres-15+-4169E1?style=flat-square&logo=postgresql" alt="PostgreSQL"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-TBD-lightgrey?style=flat-square" alt="License"></a>
</p>

---

<p align="center">
  <a href="README_CN.md">中文文档</a>
</p>

---

## What is OpenForge?

OpenForge is an AI-powered workbench that turns conversation into production code. Describe what you want to build, and OpenForge handles the entire pipeline — requirements analysis, architecture design, implementation, testing, deployment, and verification.

Built with **Conduit (RealWorld monorepo)** as the experimental field.

```
You describe   →   PM Agent   →   Pipeline Engine   →   Deployed Code
    "I want a              clarifies,        builds, tests,        running on
     blog app"             decomposes,       reviews,              your infra
                           designs           deploys
```

## Architecture

```
┌──────────────────────────────────────────────────────────┐
│                    C-Layer · Workbench                     │
│              React + Dockview + WebSocket                 │
├──────────────────────────────────────────────────────────┤
│                   A-Layer · Pipeline Engine               │
│           Go · State Machine · Gate · Deploy              │
├──────────────────────────────────────────────────────────┤
│                  B-Layer · Agent Swarm                    │
│        Go + Node.js · CSP Channels · Multi-Agent         │
├──────────────────────────────────────────────────────────┤
│                     Infrastructure                        │
│          PostgreSQL · Redis Streams · Docker              │
└──────────────────────────────────────────────────────────┘
```

## Key Features

| Module | Capability |
|--------|-----------|
| **Conversational PM** | Natural language → structured requirements → task decomposition |
| **Pipeline Engine** | 9-state transition machine with approval gates and backtracking |
| **Agent Swarm** | Multi-agent coordinator with Spawn / Delegate / Broadcast patterns |
| **Tool Registry** | 7 core tools — File I/O, Code Search, Shell Exec, DB Query, Git, Web Fetch |
| **Pro Mode** | Full IDE: Chat + Diff + File Tree + Terminal + Topology in Dockview panels |
| **LLM Router** | Multi-provider: Anthropic / OpenAI / DeepSeek, with token metering |
| **Sandbox** | Docker container isolation with LRU warm pool and 5-layer defense |
| **Compliance** | WORM audit logs, data lifecycle, monthly partitioning |

## Quick Start

### Prerequisites

- **Go** 1.22+ · **Node.js** 20+ · **PostgreSQL** 15+ · **Docker** (optional)

### Setup

```bash
# 1. Clone
git clone https://github.com/Yorha9e/openforge.git && cd openforge

# 2. Configure
cp config/profiles/minimal.yaml.example config/profiles/minimal.yaml
cp config/profiles/docker-dev.yaml.example config/profiles/docker-dev.yaml

# 3. Environment
export DB_PASSWORD=your_db_password
export JWT_SECRET=your_jwt_secret_at_least_32_chars
export ANTHROPIC_AUTH_TOKEN=your_anthropic_api_key

# 4. Start database
docker compose up postgres -d

# 5. Run migrations
go run cmd/openforge/main.go migrate up

# 6. Start server
go run cmd/server/main.go --addr :8030

# 7. Start frontend
cd frontend && npm install && npm run dev
```

Open **http://localhost:5173** — start building through conversation.

## Project Structure

```
openforge/
├── cmd/
│   ├── server/               REST + WebSocket API server
│   └── openforge/            CLI entry point
├── proto/                     Protocol Buffers (6 services)
│
├── internal/
│   ├── agent/                AI agent: coordinator, query engine, tools
│   ├── pipeline/             Pipeline: state machine, gate, deploy
│   ├── auth/                 Authentication: JWT, RBAC, OIDC
│   ├── llm/                  LLM: provider router, translator, token meter
│   ├── observability/        Monitoring: Prometheus, circuit breaker
│   └── shared/               Shared kernel, profiles, feature flags
│
├── frontend/                 React 18 + TypeScript + Vite
│   └── src/features/         chat / code-review / admin / monitoring
│
├── nodejs-io/                Node.js I/O service (gRPC)
├── migrations/               Database migrations (PostgreSQL)
├── config/profiles/          Capability profiles (YAML)
├── deployments/              Docker, Nginx, TLS configs
└── scripts/                  Diagnostic & utility scripts
```

## Configuration Profiles

Three tiers, one codebase:

| Profile | Target | Key Traits |
|---------|--------|------------|
| `minimal` | <10 people, single machine | SQLite / local FS / in-memory |
| `standard` | 50-200 people, single-AZ K8s | PG / Redis / MinIO / Vault |
| `enterprise` | Regulated, multi-AZ | HA + DR + WORM + Feishu + OIDC |

```bash
# Switch profile
export OPENFORGE_PROFILE=standard
go run cmd/server/main.go
```

## Tech Stack

| Layer | Technology |
|-------|-----------|
| **Backend** | Go · gRPC · gorilla/websocket · bcrypt · golang-jwt |
| **Frontend** | React 18 · TypeScript · Vite · Dockview · Monaco Editor |
| **Database** | PostgreSQL 15+ · UUID v7 · WORM partitioning · 22 tables |
| **Infrastructure** | Docker Compose · Nginx · TLS · Redis Streams (Phase 5+) |
| **Protocol** | gRPC · WebSocket (14 upstream + 14 downstream events) |
| **AI/LLM** | Anthropic Claude · OpenAI · DeepSeek · Multi-provider router |

## Development

```bash
# Run all tests
go test ./...

# Frontend tests
cd frontend && npm test

# Build
go build -o bin/server cmd/server/main.go
cd frontend && npm run build

# Lint
golangci-lint run
cd frontend && npm run lint
```

## Security

**Never commit:**
- `.env` files · API keys · Private keys · Certificates
- Configuration files with hardcoded credentials
- Password hashes (use `htpasswd -nbBC 10 "" <password>`)

**Use environment variables for all secrets.** See `.env` → `.env.example` pattern.

## License

*To be determined*

---

<p align="center">
  <sub>Built for the Agent-Assisted Full-Stack Challenge · Topic 1: AI Engineering Tools</sub>
</p>
