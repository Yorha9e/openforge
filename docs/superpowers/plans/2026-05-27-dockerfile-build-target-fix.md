# Dockerfile 构建目标修复计划

> 日期：2026-05-27  
> 前提：[2026-05-27-nginx-fix-plan.md](./2026-05-27-nginx-fix-plan.md) 中的附件问题

## 一、问题诊断

### 症状

docker-compose 启动后，`server` 容器里跑的不是 Web 服务，连接 `server:8080` 无响应。

### 根因

项目有两个 `main` 入口，Dockerfile 构建了**错误的那一个**：

```
cmd/
├── openforge/main.go    ← CLI 工具（stdin/stdout 对话）  ← 当前 Dockerfile 构建这个
└── server/main.go       ← Web 服务器（HTTP + WebSocket） ← docker-compose 需要这个
```

| | CLI (`cmd/openforge`) | Server (`cmd/server`) |
|---|---|---|
| 输入 | stdin 终端 | HTTP / WebSocket |
| 端口 | 无 | `-addr :8030`（默认） |
| 对外服务 | 否 | 是 |
| docker-compose 需要 | 否 | **是** |

### 受影响的文件

```
Dockerfile                          ← 第 21 行：go build ./cmd/openforge
Makefile                            ← 第 14 行：go build ./cmd/openforge
docker-compose.yml                  ← openforge 服务用 Dockerfile
deployments/docker-compose.standard.yaml  ← server 服务用 Dockerfile
```

---

## 二、修复设计

### 2.1 Dockerfile

**改动点：**

```diff
 # Build binary
-RUN CGO_ENABLED=1 GOOS=linux go build -o /openforge ./cmd/openforge
+RUN CGO_ENABLED=1 GOOS=linux go build -o /openforge ./cmd/server

 # Runtime image
 FROM alpine:3.19

-RUN apk add --no-cache ca-certificates bash
+RUN apk add --no-cache ca-certificates bash curl

 ...

-# Default port
+# Default listen port (server binary defaults to :8030, override here)
 EXPOSE 8080
-ENTRYPOINT ["./openforge"]
+ENTRYPOINT ["./openforge", "-addr", ":8080"]
```

| 改动 | 原因 |
|------|------|
| `./cmd/openforge` → `./cmd/server` | 构建 Web 服务而非 CLI |
| 加 `curl` | 容器内 healthcheck 需要 HTTP 客户端 |
| `-addr :8080` | Server 默认监听 `:8030`，容器内外端口需对齐 |

### 2.2 Makefile

`build-go` 当前只构建 CLI。拆分为两个独立目标：

```diff
-build: build-go build-frontend build-nodejs
+build: build-cli build-server build-frontend build-nodejs

-build-go:
-	@echo "==> Building Go backend..."
+build-cli:
+	@echo "==> Building CLI..."
 	go build -o bin/openforge.exe ./cmd/openforge

+build-server:
+	@echo "==> Building Web Server..."
+	go build -o bin/server.exe ./cmd/server
```

| 目标 | 输出 | 用途 |
|------|------|------|
| `make build-cli` | `bin/openforge.exe` | 本地终端使用 |
| `make build-server` | `bin/server.exe` | docker-compose 用 / 本地调试 |
| `make build` | 两者都构建 | CI / 全量构建 |

### 2.3 deployments/docker-compose.standard.yaml

在 `server` 服务添加 healthcheck + 修改 nginx 依赖为条件等待：

```diff
   server:
+    healthcheck:
+      test: ["CMD", "curl", "-f", "http://localhost:8080/api/health"]
+      interval: 5s
+      timeout: 3s
+      retries: 10
+      start_period: 15s
     expose:
       - "8080"
     depends_on:
       - postgres
       - redis
       - nodejs-io

   nginx:
     depends_on:
-      - server
+      server:
+        condition: service_healthy
```

| 改动 | 原因 |
|------|------|
| server healthcheck | 确认 `/api/health` 返回 200 才算就绪 |
| `start_period: 15s` | DB migration + 启动预留时间 |
| nginx `condition: service_healthy` | 不等 server 就绪不启动，避免 502 |

> **注意**：healthcheck 的前提是 Dockerfile 已安装 `curl`（见 2.1），否则 `CMD` 找不到 curl 会持续失败。

### 2.4 docker-compose.yml（根目录）

同样需要适配 server 二进制：

```diff
   openforge:
     build: .
     ports:
       - "8080:8080"
+    command: ["./openforge", "-addr", ":8080"]
     environment:
       DATABASE_URL: postgres://openforge:openforge_dev@postgres:5432/openforge?sslmode=disable
       LLM_ROUTER_ADDR: nodejs-io:50051
       JWT_SECRET: ${JWT_SECRET:-dev-secret-change-in-production-32b!}
       PROFILE: ${PROFILE:-standard}
+    healthcheck:
+      test: ["CMD", "curl", "-f", "http://localhost:8080/api/health"]
+      interval: 5s
+      timeout: 3s
+      retries: 10
+      start_period: 15s
     depends_on:
       postgres:
         condition: service_healthy
```

> **注意**：根目录 `docker-compose.yml` 用 `DATABASE_URL` 环境变量传数据库连接，而非 yaml profile。这要求 `cmd/server/main.go` 中的 `profile.Bootstrap()` 能够识别该环境变量。如果当前 server 代码不支持 `DATABASE_URL` 环境变量，需额外适配。

---

## 三、文件变更清单

| 文件 | 操作 | 变更行 |
|------|:--:|--------|
| `Dockerfile` | 修改 | L21（构建目标）、L26（加 curl）、L38（加 -addr） |
| `Makefile` | 修改 | L10、L12-14（拆分为 cli + server） |
| `deployments/docker-compose.standard.yaml` | 修改 | server 加 healthcheck + nginx 改条件依赖 |
| `docker-compose.yml` | 修改 | openforge 加 command + healthcheck |

---

## 四、潜在风险

### 4.1 根 docker-compose 的 `DATABASE_URL` 兼容性

`cmd/server/main.go` 通过 `profile.Load()` + `profile.Bootstrap()` 获取 DB 连接，它从 yaml 配置读取 `database.host/port/user/password/dbname`。根 `docker-compose.yml` 传入 `DATABASE_URL` 环境变量，但 server 代码可能不解析它。

**缓解方案**（执行时验证）：
- 如果 server 代码不认 `DATABASE_URL`，改为在 docker-compose 中传 profile 配置字段的环境变量（`DB_HOST`、`DB_PASSWORD` 等），与 yaml 中的 `${DB_PASSWORD}` 占位符匹配。
- 或直接挂载 `config/profiles/standard.yaml` 并配好数据库连接。

### 4.2 CLI 仍需要独立构建

`make build-go` 拆分为 `build-cli` + `build-server` 后，旧的 `build-go` 引用（如有 CI 脚本）会失效。需全局搜索替换。

### 4.3 两个 docker-compose 版本差异

`docker-compose.yml`（Phase 1 MVP）与 `deployments/docker-compose.standard.yaml`（Phase 8 HA）可能存在不一致（如环境变量名、profile 路径）。修复时建议以 `deployments/docker-compose.standard.yaml` 为基准对齐。

---

## 五、执行顺序

```
Step 1: 修改 Dockerfile（构建目标 + curl + -addr）
            │
Step 2: 修改 Makefile（拆 build-cli / build-server）
            │
Step 3: 修改 deployments/docker-compose.standard.yaml （healthcheck + condition）
            │
Step 4: 修改 docker-compose.yml（command + healthcheck）
            │
Step 5: 本地验证
          cd frontend && npm run build
          cd ../deployments && bash certs/generate.sh
          docker compose -f docker-compose.standard.yaml build server
          docker compose -f docker-compose.standard.yaml up -d
          curl -k https://localhost/api/health
            │
Step 6: 全局搜索 "build-go" 确保无遗漏引用
```

---

## 六、验证标准

```bash
# 1. 构建通过
make build-server   # Go build 成功，无编译错误

# 2. Docker 镜像构建通过
docker compose -f deployments/docker-compose.standard.yaml build server

# 3. 服务健康
docker compose -f deployments/docker-compose.standard.yaml up -d
docker compose -f deployments/docker-compose.standard.yaml ps
# server 状态为 healthy

# 4. API 可达
curl -k https://localhost/api/health
# → {"status":"ok"}

# 5. nginx 未因 server 未就绪而报错
docker logs of-nginx | grep -i "502\|error"
# → 无 502 错误
```
