#!/bin/bash
# 生成自签证书（开发环境用）
# 生产环境请使用 Let's Encrypt 或企业 CA 签发的证书

set -e

cd "$(dirname "$0")"

echo "🔐 正在生成自签 TLS 证书..."

openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout server.key \
  -out server.crt \
  -subj "/C=CN/ST=Guangdong/L=Shenzhen/O=OpenForge/CN=localhost"

chmod 600 server.key

echo "✅ 证书已生成:"
echo "   私钥: $(pwd)/server.key"
echo "   证书: $(pwd)/server.crt"
echo ""
echo "⚠️  自签证书仅用于开发环境，浏览器会显示不安全警告。"
echo "   生产环境请使用正规 CA 签发的证书。"
