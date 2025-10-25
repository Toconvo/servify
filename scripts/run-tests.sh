#!/bin/bash

# Servify Unit Tests Runner
# 运行所有单元测试并生成覆盖率报告

set -e

echo "🧪 Running Servify Unit Tests..."
echo "================================"

# 确保在项目根目录
cd "$(dirname "$0")"

# 创建测试输出目录
mkdir -p test-results

# 运行所有测试并生成覆盖率报告
echo "📊 Running tests with coverage..."
go test -v -race -coverprofile=test-results/coverage.out ./internal/services/... ./internal/handlers/...

# 生成覆盖率HTML报告
echo "📈 Generating coverage report..."
go tool cover -html=test-results/coverage.out -o test-results/coverage.html

# 显示覆盖率概要
echo "📋 Coverage Summary:"
go tool cover -func=test-results/coverage.out | tail -1

# 运行基准测试
echo ""
echo "⚡ Running benchmark tests..."
go test -bench=. -benchmem ./internal/services/... ./internal/handlers/... > test-results/benchmark.txt

echo ""
echo "✅ Test run completed!"
echo "📁 Results saved to test-results/"
echo "  - coverage.out: Raw coverage data"
echo "  - coverage.html: Coverage report (open in browser)"
echo "  - benchmark.txt: Benchmark results"

# 检查覆盖率是否达到目标（20%）
COVERAGE=$(go tool cover -func=test-results/coverage.out | tail -1 | awk '{print $3}' | sed 's/%//')
TARGET=20.0

echo ""
echo "🎯 Coverage Target: ${TARGET}%"
echo "📊 Actual Coverage: ${COVERAGE}%"

if (( $(echo "$COVERAGE >= $TARGET" | bc -l) )); then
    echo "✅ Coverage target achieved!"
    exit 0
else
    echo "❌ Coverage below target. Need to add more tests."
    exit 1
fi