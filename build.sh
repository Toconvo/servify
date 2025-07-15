#!/bin/bash

# 构建脚本
echo "Building Servify..."

# 清理旧的构建产物
rm -f servify

# 构建 Go 二进制文件
go build -o servify ./cmd/main.go

if [ $? -eq 0 ]; then
    echo "Build successful! Binary: ./servify"
else
    echo "Build failed!"
    exit 1
fi