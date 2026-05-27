# Nginx TLS 配置修复计划

> 日期：2026-05-27  
> 基于审核报告 [2026-05-27-nginx-tls-reverse-proxy.md](./2026-05-27-nginx-tls-reverse-proxy.md)

## 修复列表

| # | 级别 | 问题 | 涉及文件 |
|---|:---:|------|----------|
| 1 | 🔴 | postgres/redis `ports` 改 `expose` 导致宿主机无法直连 | `docker-compose.standard.yaml` |
| 2 | 🔴 | certs 目录缺 `.gitkeep`，新 clone 后 nginx 起不来 | `certs/.gitkeep` |
| 3 | 🔴 | `generate.sh` 是 bash，Windows 用不了 | 新增 `.ps1` |
| 4 | 🟡 | nginx 没等 server 就绪，启动窗口期 502 | `docker-compose.standard.yaml` |
| 5 | 🟡 | `X-XSS-Protection` 已废弃且可能引入安全问题 | `nginx.conf` |
| 6 | 🟢 | CSP `unsafe-inline` 可收紧 | `nginx.conf` |
| 7 | 🟢 | 缺 `ssl_session_cache` | `nginx.conf` |
| 8 | 🟢 | ACME 路径无对应 volume | `nginx.conf` |

---

## 修复 1：恢复 postgres/redis 的 `ports`

**文件**：`deployments/docker-compose.standard.yaml`

```yaml
# postgres → 改回 ports
postgres:
    ports:
      - "5432:5432"

# redis → 改回 ports
redis:
    ports:
      - "6379:6379"
```

**原因**：这两个是数据基础设施，不通过 nginx 暴露，本地开发需要直连。

---

## 修复 2：添加 certs 目录占位

**文件**：`deployments/certs/.gitkeep`（新增空文件）

**原因**：确保 git clone 后 `deployments/certs/` 目录存在。`.gitignore` 已排除证书文件，但 `.gitkeep` 是明文文件名，不会被排除。

**同时**：在 `deployments/certs/` 下加一个 `README.md`：

```markdown
# TLS 证书目录

开发环境运行 `./generate.sh`（Linux/Mac）或 `.\generate.ps1`（Windows）生成自签证书。

生产环境将正规 CA 签发的证书放入此目录，文件名为 `server.crt` 和 `server.key`。

⚠️ 证书文件已被 .gitignore 排除，不会提交到仓库。
```

---

## 修复 3：提供 Windows 版证书生成脚本

**文件**：`deployments/certs/generate.ps1`（新增）

```powershell
# 生成自签证书（Windows 开发环境）
# 生产环境请使用 Let's Encrypt 或企业 CA 签发的证书

$ErrorActionPreference = "Stop"

Write-Host "Generating self-signed TLS certificate..."

$cert = New-SelfSignedCertificate `
    -DnsName "localhost" `
    -CertStoreLocation "Cert:\CurrentUser\My" `
    -NotAfter (Get-Date).AddDays(365) `
    -KeyAlgorithm RSA `
    -KeyLength 2048

# 导出公钥
Export-Certificate -Cert $cert -FilePath "server.crt" -Type CERT

# 导出私钥（需要密码保护，这里用空密码）
$certPath = "Cert:\CurrentUser\My\$($cert.Thumbprint)"
$password = ConvertTo-SecureString -String "" -Force -AsPlainText
Export-PfxCertificate -Cert $certPath -FilePath "temp.pfx" -Password $password

# PFX → PEM 私钥（需要 openssl）
openssl pkcs12 -in temp.pfx -nocerts -out server.key -nodes -passin pass:
Remove-Item temp.pfx

# 清理证书存储
Remove-Item $certPath

Write-Host "Certificates generated:"
Write-Host "  server.crt"
Write-Host "  server.key"
Write-Host ""
Write-Host "WARNING: Self-signed certs are for development only."
Write-Host "         Browsers will show a security warning."
```

> **注意**：这个脚本依赖 OpenSSL（Windows 上可通过 `choco install openssl` 或 Git for Windows 自带安装）。如果用户没有 OpenSSL，可改用纯 PowerShell 方案（导出 `.pfx` 供 nginx 使用），但 nginx alpine 镜像默认只支持 PEM 格式。**更简单的方案**：提示 Windows 用户在 Git Bash 中运行 `generate.sh`。

---

## 修复 4：加健康检查 + 条件等待

**文件**：`deployments/docker-compose.standard.yaml`

在 `server` 服务加 healthcheck：

```yaml
server:
    # ... 现有配置 ...
    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:8080/api/health"]
      interval: 5s
      timeout: 3s
      retries: 10
      start_period: 15s
```

nginx 改为条件依赖：

```yaml
nginx:
    depends_on:
      server:
        condition: service_healthy
```

> **注意**：如果 Dockerfile 构建的是 CLI 而非 server（见"附加问题"一节），此 healthcheck 需根据实际服务端口调整。

---

## 修复 5：删除 `X-XSS-Protection`

**文件**：`deployments/nginx.conf` 第 60 行

```nginx
# 删除这一行
-    add_header X-XSS-Protection "1; mode=block" always;
```

**原因**：该 header 已被 Chrome/Edge/Firefox 废弃。`mode=block` 在某些旧版浏览器中反而可能引入 XSS 风险（详见 Chrome CVE-2018 相关报告）。现代防护由 CSP 完成。

---

## 修复 6：收紧 CSP（可选，标注 TODO）

**文件**：`deployments/nginx.conf` 第 62 行

当前：
```nginx
add_header Content-Security-Policy "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; connect-src 'self' ws: wss:;" always;
```

Vite 生产构建不使用内联脚本，可以收紧。但需要先验证前端确实没有内联代码，否则会炸页面。

**方案**：暂不改，加注释标注 TODO。

```nginx
# TODO: 生产环境收紧 CSP，去掉 'unsafe-inline'（需确认 Vite 构建产物无内联代码）
add_header Content-Security-Policy "...";
```

---

## 修复 7：添加 TLS Session Cache

**文件**：`deployments/nginx.conf` 第 48 行后追加：

```nginx
    ssl_session_cache shared:SSL:10m;
    ssl_session_timeout 1h;
    ssl_session_tickets off;
```

**效果**：TLS 握手复用，减少 1 次 RTT，对 API 密集场景有明显提升。

---

## 修复 8：ACME 路径加注释

**文件**：`deployments/nginx.conf` 第 27 行

```nginx
    # Let's Encrypt ACME 验证（需要挂载 /var/www/certbot volume 才能使用）
    # 开发环境用自签证书，此路径不生效
    location /.well-known/acme-challenge/ {
        root /var/www/certbot;
    }
```

---

## 附加问题：Dockerfile 构建目标

**现状**：Dockerfile 和 Makefile 都构建 `cmd/openforge`（CLI 工具），不是 `cmd/server`（Web 服务）。docker-compose 的 `server` 服务实际不监听 HTTP 端口。

**影响**：修复 4 的 healthcheck 会失败，nginx 永远无法启动。

**修复方案**（另起一个 issue/plan，不在本次修复范围内）：

```dockerfile
# Dockerfile 改为构建 server
-RUN CGO_ENABLED=1 GOOS=linux go build -o /openforge ./cmd/openforge
+RUN CGO_ENABLED=1 GOOS=linux go build -o /openforge ./cmd/server
```

```makefile
# Makefile 构建目标区分 CLI 和 Server
-build-go:
-	go build -o bin/openforge.exe ./cmd/openforge
+build-cli:
+	go build -o bin/of3.exe ./cmd/openforge
+build-server:
+	go build -o bin/openforge.exe ./cmd/server
+build-go: build-cli build-server
```

---

## 执行顺序

```
修复 1 → 修复 2 → 修复 3
    ↓
修复 5 → 修复 7 → 修复 8（同一文件 nginx.conf，可合并）
    ↓
修复 4（依赖于 server 的实际端口和 health endpoint）
    ↓
修复 6（标注 TODO，暂不改）
    ↓
附加问题（另开计划）
```

---

## 修复后验证

```bash
# 1. 生成证书
cd deployments/certs
bash generate.sh    # 或 .\generate.ps1

# 2. 构建前端
cd ../../frontend && npm run build

# 3. 启动
cd ../deployments
docker compose -f docker-compose.standard.yaml up -d

# 4. 验证
curl -k https://localhost/api/health
# 预期: {"status":"ok"}
```
