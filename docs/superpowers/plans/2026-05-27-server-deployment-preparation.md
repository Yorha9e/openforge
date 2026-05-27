# 服务器部署准备计划

> 日期：2026-05-27  
> 前提：nginx-fix-plan、dockerfile-build-target-fix、nginx-tls-reverse-proxy 均已完成

---

## 一、部署架构

```
                    Internet
                       │
              ┌────────┴────────┐
              │   云服务器 (VPS)  │
              │                 │
              │  :443 ┌──────┐  │
              │  ────→│Nginx │  │  TLS 终止
              │       └──┬───┘  │
              │          │      │
              │     ┌────┴────┐ │
              │     ▼         ▼ │
              │  :8080    /dist │
              │  Server   静态   │
              │  │   │    文件   │
              │  │   └──────┘   │
              │  │              │
              │  ▼  ▼           │
              │ PG Redis  容器内 │
              └─────────────────┘
```

---

## 二、部署前检查清单

### 2.1 代码层面

| # | 检查项 | 说明 |
|---|--------|------|
| 1 | `Dockerfile` 构建 `./cmd/server` | ✅ 已修复 |
| 2 | `make build-frontend` 输出到 `frontend/dist/` | 执行确认 |
| 3 | `go.mod` / `go.sum` 无未提交依赖 | `git diff` 确认 |
| 4 | 所有 migration 文件就绪 | `migrations/` 目录 |
| 5 | 无 hardcoded 密钥/密码 | 搜索 `TODO` / `FIXME` |
| 6 | `deployments/certs/` 缺证书文件不阻塞 | `.gitkeep` 已就位 |

### 2.2 服务器环境

| # | 检查项 | 最低要求 |
|---|--------|----------|
| 1 | 操作系统 | Ubuntu 22.04 / Debian 12 |
| 2 | Docker | 24.0+ |
| 3 | Docker Compose | v2 (plugin) |
| 4 | 可用磁盘 | ≥ 20GB |
| 5 | 可用内存 | ≥ 4GB |
| 6 | 防火墙开放端口 | 80, 443 (22 管理口) |

### 2.3 安全配置

| # | 检查项 | 说明 |
|---|--------|------|
| 1 | `JWT_SECRET` | 生成强随机密钥，不允许 dev 默认值 |
| 2 | `POSTGRES_PASSWORD` | 生成强随机密码 |
| 3 | HTTPS 证书 | 生产环境用 Let's Encrypt / 企业 CA |
| 4 | 防火墙规则 | 仅开放 80/443，5432/6379 仅本地 |
| 5 | SSH 安全 | 禁用密码登录，仅密钥认证 |

---

## 三、部署步骤

### Step 1: 服务器初始化

```bash
# SSH 到服务器
ssh user@your-server-ip

# 安装 Docker（如未安装）
curl -fsSL https://get.docker.com | bash
sudo usermod -aG docker $USER
# 重新登录使权限生效

# 确认版本
docker --version        # ≥ 24.0
docker compose version  # ≥ 2.0
```

### Step 2: 拉取代码 & 构建

```bash
# 在服务器上
git clone <repo-url> /opt/openforge
cd /opt/openforge

# 构建前端
cd frontend && npm install && npm run build && cd ..

# 确保 dist 存在
ls frontend/dist/index.html
```

### Step 3: 生产环境配置

创建 `.env` 文件（在 `/opt/openforge/deployments/`）：

```bash
# deployments/.env
JWT_SECRET=<openssl rand -base64 48 的输出>
POSTGRES_PASSWORD=<openssl rand -base64 24 的输出>
ANTHROPIC_API_KEY=sk-ant-...
ANTHROPIC_BASE_URL=https://api.anthropic.com
ANTHROPIC_MODEL=claude-sonnet-4-7-20250514
```

修改 `docker-compose.standard.yaml` 引用 `.env`：

```yaml
services:
  postgres:
    environment:
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}  # 从 .env 读取
  
  openforge:
    environment:
      JWT_SECRET: ${JWT_SECRET}
      # ... LLM 相关也改用 .env
```

**生产配置文件** (`config/profiles/docker-production.yaml`)：

沿用 `docker-standard.yaml` 但修改：
```yaml
security_tier: production
secret_store: envfile
jwt:
  secret: "${JWT_SECRET}"  # 从环境变量读取
```

### Step 4: 生成 TLS 证书

```bash
cd /opt/openforge/deployments/certs

# 方案 A：Let's Encrypt（推荐生产）
sudo apt install certbot
sudo certbot certonly --standalone -d your-domain.com
sudo cp /etc/letsencrypt/live/your-domain.com/fullchain.pem server.crt
sudo cp /etc/letsencrypt/live/your-domain.com/privkey.pem server.key

# 方案 B：自签证书（仅测试）
bash generate.sh
```

### Step 5: 启动服务

```bash
cd /opt/openforge/deployments

# 构建并启动
docker compose -f docker-compose.standard.yaml up -d --build

# 查看日志
docker compose -f docker-compose.standard.yaml logs -f

# 确认所有容器运行
docker compose -f docker-compose.standard.yaml ps
# 预期: of-nginx, of-server, of-postgres, of-redis, of-nodejs-io 全部 Up
```

### Step 6: 验证部署

```bash
# 1. 健康检查
curl -k https://localhost/api/health
# 预期: {"status":"ok"}

# 2. 从外部访问
curl -k https://your-server-ip/api/health

# 3. 浏览器访问
# https://your-server-ip
# 接受自签证书警告（如使用自签证书）

# 4. WebSocket 测试
# 使用 wscat 或浏览器 devtools 连接 wss://your-server-ip/ws/chat

# 5. 登录测试
curl -k -X POST https://your-server-ip/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}'
# 预期: 返回 JWT token
```

### Step 7: 数据库初始化

```bash
# 确认 migrations 已执行
docker exec of-server ls /app/migrations/

# 查看数据库表
docker exec of-postgres psql -U openforge -d openforge -c "\dt"

# 预期能看到 users, projects, conversations 等表
```

---

## 四、持续部署（可选）

### 4.1 简单更新脚本

`deployments/update.sh`：

```bash
#!/bin/bash
set -e

cd /opt/openforge

echo "==> Pulling latest code..."
git pull origin main

echo "==> Building frontend..."
cd frontend && npm install && npm run build && cd ..

echo "==> Rebuilding and restarting services..."
cd deployments
docker compose -f docker-compose.standard.yaml up -d --build

echo "==> Cleaning old images..."
docker image prune -f

echo "✅ Deployment complete"
```

### 4.2 systemd 自启动（可选）

```ini
# /etc/systemd/system/openforge.service
[Unit]
Description=OpenForge Docker Compose
Requires=docker.service
After=docker.service

[Service]
Type=oneshot
RemainAfterExit=yes
WorkingDirectory=/opt/openforge/deployments
ExecStart=/usr/bin/docker compose -f docker-compose.standard.yaml up -d
ExecStop=/usr/bin/docker compose -f docker-compose.standard.yaml down
ExecReload=/usr/bin/docker compose -f docker-compose.standard.yaml restart

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl enable openforge
sudo systemctl start openforge
```

---

## 五、监控与日志

| 方面 | 方案 | 说明 |
|------|------|------|
| 容器状态 | `docker compose ps` / `docker stats` | 基础监控 |
| 应用日志 | `docker compose logs -f server` | 实时日志 |
| Nginx 日志 | `docker exec of-nginx cat /var/log/nginx/access.log` | 访问日志 |
| 健康检查 | `curl https://domain/api/health` | 配合 cron 或 uptime monitor |
| 磁盘空间 | `df -h` | 定期检查 pg-data volume |
| 数据库备份 | `pg_dump` + cron | 每日备份到外部存储 |

### 数据库备份脚本

`deployments/backup-db.sh`：

```bash
#!/bin/bash
BACKUP_DIR=/opt/backups/openforge
mkdir -p $BACKUP_DIR

docker exec of-postgres pg_dump -U openforge openforge \
  > $BACKUP_DIR/openforge_$(date +%Y%m%d_%H%M%S).sql

# 保留最近 7 天
find $BACKUP_DIR -name "*.sql" -mtime +7 -delete
```

---

## 六、回滚方案

```bash
# 如果部署出错
cd /opt/openforge

# 回滚代码
git checkout <previous-commit-hash>

# 重新构建前端
cd frontend && npm run build && cd ..

# 重启服务
cd deployments
docker compose -f docker-compose.standard.yaml up -d --build
```

---

## 七、安全检查清单（生产环境必做）

- [ ] JWT_SECRET 已更换为强随机值，非 dev 默认值
- [ ] POSTGRES_PASSWORD 已更换为强随机值
- [ ] TLS 证书使用 Let's Encrypt 或正规 CA
- [ ] 防火墙仅开放 22, 80, 443
- [ ] SSH 仅允许密钥登录
- [ ] `.env` 文件已加入 `.gitignore`（或放在 `/opt/openforge/deployments/` 而非代码仓库内）
- [ ] 定期数据库备份已配置
- [ ] `docker-compose.standard.yaml` 中无硬编码密码（统一用 `${VAR}`）

---

## 八、执行顺序

```
Step 1: 服务器初始化（一次性）
    ↓
Step 2: 拉取代码 & 构建前端
    ↓
Step 3: 生产环境配置（.env + 配置文件）
    ↓
Step 4: 生成 TLS 证书（Let's Encrypt 或自签）
    ↓
Step 5: docker compose up -d
    ↓
Step 6: 验证部署（health / 登录 / WebSocket）
    ↓
Step 7: 确认数据库初始化
    ↓
（可选）部署 systemd 自启动 + 备份脚本
```

---

## 九、当前待办

- [ ] **前置：执行本地 Docker Compose 测试**，确认所有服务在容器内正常运行
- [ ] **前置：确认前端构建无报错** (`cd frontend && npm run build`)
- [ ] 准备/确认目标服务器（VPS IP、SSH 密钥）
- [ ] 生成生产环境密钥（JWT_SECRET、POSTGRES_PASSWORD）
- [ ] 决定 TLS 证书方案（自签 vs Let's Encrypt）
- [ ] 确认域名解析（如使用域名）
- [ ] 配置防火墙规则
- [ ] 执行部署步骤 1-7
- [ ] 配置 systemd 自启动
- [ ] 配置数据库备份 cron
- [ ] 配置健康检查监控
