# TLS 证书目录

开发环境运行 `./generate.sh`（Linux/Mac）或 `.\generate.ps1`（Windows）生成自签证书。

生产环境将正规 CA 签发的证书放入此目录，文件名为 `server.crt` 和 `server.key`。

⚠️ 证书文件已被 .gitignore 排除，不会提交到仓库。
