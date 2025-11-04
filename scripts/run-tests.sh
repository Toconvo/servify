#!/bin/bash

# Servify Unit Tests Runner
# 运行所有单元测试并生成覆盖率报告

set -e

echo "🧪 Running Servify Unit Tests..."
echo "================================"

# 确保在项目根目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
OUT_DIR="$SCRIPT_DIR/test-results"
GOWORK_CACHE_DIR="$PROJECT_ROOT/.cache"
export GOCACHE="$GOWORK_CACHE_DIR/gocache"
cd "$PROJECT_ROOT"

# 创建测试输出目录
mkdir -p "$OUT_DIR" "$GOCACHE"

# 运行所有测试并生成覆盖率报告
echo "📊 Running tests with coverage (no race)..."
go test -v -coverprofile="$OUT_DIR/coverage.out" ./internal/services/... ./internal/handlers/...

# 生成覆盖率HTML报告
echo "📈 Generating coverage report..."
go tool cover -html="$OUT_DIR/coverage.out" -o "$OUT_DIR/coverage.html"

# 显示覆盖率概要
echo "📋 Coverage Summary:"
go tool cover -func="$OUT_DIR/coverage.out" | tail -1

# 运行基准测试
echo ""
echo "⚡ Running benchmark tests..."
go test -bench=. -benchmem ./internal/services/... ./internal/handlers/... > "$OUT_DIR/benchmark.txt"

echo ""
echo "✅ Test run completed!"
echo "📁 Results saved to $OUT_DIR"
echo "  - coverage.out: Raw coverage data"
echo "  - coverage.html: Coverage report (open in browser)"
echo "  - benchmark.txt: Benchmark results"

# 覆盖率阈值（默认 20%，可通过 TEST_COVERAGE_TARGET 环境变量覆盖）
COVERAGE=$(go tool cover -func="$OUT_DIR/coverage.out" | tail -1 | awk '{print $3}' | sed 's/%//')
TARGET=${TEST_COVERAGE_TARGET:-20.0}

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
