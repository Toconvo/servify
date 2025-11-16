#!/bin/bash

# 构建脚本
echo "Building Servify..."

# 清理旧的构建产物
rm -f servify

# 构建 Go 二进制文件（apps/server 模块）附带版本信息
VERSION=${VERSION:-dev}
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo none)
BUILD_TIME=$(date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS="-X 'servify/apps/server/internal/version.Version=${VERSION}' -X 'servify/apps/server/internal/version.Commit=${GIT_COMMIT}' -X 'servify/apps/server/internal/version.BuildTime=${BUILD_TIME}'"
go -C apps/server build -ldflags "${LDFLAGS}" -o ../../servify ./cmd/main.go
if [ $? -eq 0 ]; then
    echo "Build successful! Binary: ./servify"
else
    echo "Build failed!"
    exit 1
fi
