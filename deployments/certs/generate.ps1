# 生成自签 TLS 证书（Windows 开发环境）
# 生产环境请使用 Let's Encrypt 或企业 CA 签发的证书
#
# 依赖：OpenSSL（Git for Windows 自带，或 choco install openssl）
# 如果没有 OpenSSL，请在 Git Bash 中运行 generate.sh

$ErrorActionPreference = "Stop"

Write-Host "Generating self-signed TLS certificate..." -ForegroundColor Green

# 生成 RSA 私钥
openssl genrsa -out server.key 2048

# 生成自签证书（有效期 365 天）
openssl req -new -x509 -key server.key -out server.crt -days 365 -subj "/CN=localhost" -addext "subjectAltName=DNS:localhost,IP:127.0.0.1"

Write-Host ""
Write-Host "Certificates generated:" -ForegroundColor Green
Write-Host "  $PWD\server.crt"
Write-Host "  $PWD\server.key"
Write-Host ""
Write-Host "WARNING: Self-signed certs are for development only." -ForegroundColor Yellow
Write-Host "         Browsers will show a security warning." -ForegroundColor Yellow
