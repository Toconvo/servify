#!/bin/bash

set -e

echo "🧹 清理旧构建文件..."
npm run clean

echo "📦 开始构建 SDK 包..."

# 构建核心包
echo "📦 构建 @servify/core..."
cd packages/core
npm install
npm run build
cd ../..

# 构建 vanilla 包
echo "📦 构建 @servify/vanilla..."
cd packages/vanilla
npm install
npm run build
cd ../..

# 构建 React 包
echo "📦 构建 @servify/react..."
cd packages/react
npm install
npm run build
cd ../..

# 构建 Vue 包
echo "📦 构建 @servify/vue..."
cd packages/vue
npm install
npm run build
cd ../..

echo "✅ 所有包构建完成！"

# 显示构建结果
echo ""
echo "📊 构建统计:"
for pkg in core vanilla react vue; do
    if [ -d "packages/$pkg/dist" ]; then
        size=$(du -sh packages/$pkg/dist | cut -f1)
        echo "  @servify/$pkg: $size"
    fi
done