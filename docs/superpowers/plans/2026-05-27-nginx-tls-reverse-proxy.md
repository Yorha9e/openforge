# Nginx TLS 反向代理配置计划

> 日期：2026-05-27

## 一、目标架构

```
                    外部请求
               https://of.example.com:443
                        │
                        ▼
              ┌──────────────────┐
              │  Nginx (反代层)   │
              │                  │
              │  • TLS 终止       │
              │  • IP 限流        │
              │  • 安全 Header    │
              │  • WebSocket 代理 │
              │  • 静态文件服务    │
              │  • SPA fallback   │
              └─────┬────────────┘
                    │  http (内网)
          ┌─────────┴─────────┐
          ▼                   ▼
    Go Server :8080    前端 dist (本地文件)
    (/api/* /ws/chat)
```

## 二、路由映射

| Nginx 匹配 | 动作 | 后端 | 说明 |
|------------|------|------|------|
| `/api/*` | proxy_pass | `http://server:8080` | REST API，透传 Host/真实IP |
| `/ws/chat` | proxy_pass + Upgrade | `http://server:8080` | WebSocket，1h 超时不断开 |
| `/metrics` | proxy_pass | `http://server:8080` | Prometheus，限制内网/白名单 |
| `/` (其余) | try_files | 本地 `frontend/dist` | SPA fallback → index.html |

## 三、安全策略

### 3.1 TLS 配置

```
协议: TLSv1.2 / TLSv1.3
证书: 自签（开发） / Let's Encrypt（生产）
80 端口: 强制 301 跳转 443
```

### 3.2 安全 Header

```
Strict-Transport-Security: max-age=31536000; includeSubDomains
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
X-XSS-Protection: 1; mode=block
Referrer-Policy: strict-origin-when-cross-origin
Content-Security-Policy: default-src 'self'
```

### 3.3 限流策略

| 范围 | 速率 | 说明 |
|------|------|------|
| 全站 | 100 req/s per IP | 配合 Go 内置 100/s 双层限流 |
| `/api/auth/login` | 5 req/min per IP | 防暴力破解 |

### 3.4 其他

- 隐藏 `Server` header（`server_tokens off`）
- 限制请求体大小 `client_max_body_size 10m`
- `/metrics` 仅允许内网 IP 或 `X-Forwarded-For` 白名单

## 四、WebSocket 配置要点

```
proxy_http_version 1.1;
proxy_set_header Upgrade $http_upgrade;
proxy_set_header Connection "upgrade";
proxy_read_timeout 3600s;  # 1h 不断开
proxy_send_timeout 3600s;
```

## 五、部署变更

### 5.1 文件清单

| 文件 | 操作 | 说明 |
|------|------|------|
| `deployments/nginx.conf` | **新增** | Nginx 主配置 |
| `deployments/certs/.gitkeep` | **新增** | 证书目录 |
| `deployments/certs/generate.sh` | **新增** | 自签证书脚本 |
| `deployments/docker-compose.standard.yaml` | **修改** | 加 nginx 服务，frontend 改为只对内 |

### 5.2 docker-compose 前后对比

**之前：**
```yaml
frontend:         # nginx 只 serve 静态文件
  ports: ["5173:80"]
server:
  ports: ["8080:8080"]    # 直接暴露到外网 ← 无 TLS
```

**之后：**
```yaml
nginx:            # 唯一对外端口
  ports: ["443:443", "80:80"]
  volumes: [nginx.conf, certs, frontend/dist]

frontend:         # 不再对外暴露
  expose: ["80"]  # 对内，或直接合并到 nginx

server:
  expose: ["8080"]        # 只对内 ← 外网访问不到
```

### 5.3 是否合并 frontend 服务

**方案 A（推荐，简单）**：nginx 直接挂载 `frontend/dist` 目录 serve 静态文件，删除 `frontend` 服务。

**方案 B（保留）**：保留 `frontend` 服务，nginx 反向代理到它。适合 CI/CD 独立构建场景。

本次采用 **方案 A**，减少一个服务。

### 5.4 开发环境

本地开发直接用 `vite dev server`（已有 proxy 配到 `http://localhost:8030`），不需要经过 nginx。nginx 只用于 docker-compose 部署/生产场景。

## 六、证书管理

### 开发环境（自签）

```bash
cd deployments
mkdir -p certs
openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout certs/server.key \
  -out certs/server.crt \
  -subj "/CN=localhost"
```

浏览器会显示"不安全"警告，点"继续访问"即可。`certs/` 目录加入 `.gitignore`。

### 生产环境（K8s + cert-manager）

不在 docker-compose 层面处理，由 K8s Ingress + cert-manager 自动申请 Let's Encrypt 证书。

## 七、实现步骤

1. 创建 `deployments/nginx.conf`
2. 创建 `deployments/certs/.gitkeep` + `generate.sh`
3. 更新 `.gitignore`（排除 `certs/*.key` `certs/*.crt`）
4. 修改 `deployments/docker-compose.standard.yaml`
   - 加 nginx 服务（443+80）
   - server 服务 `ports` 改为 `expose`
   - 删除 frontend 服务（或改为 expose）
5. 生成自签证书
6. `docker compose -f deployments/docker-compose.standard.yaml up` 验证
