#!/bin/bash
# Phase 4 L1 预热: 装基础工具链
# Usage: warm_dependencies.sh [CACHE_LAYER]
set -e

CACHE_LAYER="${1:-/var/lib/openforge/deps}"

mkdir -p "$CACHE_LAYER"

# Node.js 项目基础依赖 (React 19 + TS 5 + Vitest 2)
if [ ! -d "$CACHE_LAYER/node-react" ]; then
  mkdir -p "$CACHE_LAYER/node-react"
  cd "$CACHE_LAYER/node-react"
  npm init -y > /dev/null
  npm install --save-dev react@19 react-dom@19 typescript@5 vitest@2
fi

# Go 项目基础依赖 (Viper + GORM)
if [ ! -d "$CACHE_LAYER/go-base" ]; then
  mkdir -p "$CACHE_LAYER/go-base"
  cd "$CACHE_LAYER/go-base"
  go mod init cache > /dev/null 2>&1 || true
  go get github.com/spf13/viper@1.18
  go get gorm.io/gorm@1.25
fi

echo "Dependencies warmed to $CACHE_LAYER"
