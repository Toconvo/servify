#!/bin/bash

# WeKnora 集成启动脚本
# 使用方法: ./scripts/start-weknora.sh [dev|prod]

set -e

# 获取脚本所在目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# 环境类型
ENV=${1:-dev}

echo "🚀 启动 Servify + WeKnora 集成环境 (${ENV})"

# 检查必要文件
if [ ! -f "$PROJECT_ROOT/.env" ]; then
    echo "📝 未找到 .env 文件，从示例文件创建..."

    if [ "$ENV" = "dev" ]; then
        cp "$PROJECT_ROOT/.env.weknora.example" "$PROJECT_ROOT/.env"
        echo "✅ 已创建开发环境配置文件"
        echo "⚠️  请编辑 .env 文件，填入实际的 API 密钥"
    else
        echo "❌ 生产环境需要手动配置 .env 文件"
        exit 1
    fi
fi

# 检查 Docker 和 Docker Compose
if ! command -v docker &> /dev/null; then
    echo "❌ Docker 未安装，请先安装 Docker"
    exit 1
fi

if ! command -v docker-compose &> /dev/null; then
    echo "❌ Docker Compose 未安装，请先安装 Docker Compose"
    exit 1
fi

# 切换到项目根目录
cd "$PROJECT_ROOT"

# 创建必要的目录
echo "📁 创建必要的目录..."
mkdir -p logs uploads data/postgres data/redis data/weknora

# 设置权限
chmod 755 logs uploads data

echo "🔧 准备启动服务..."

# 根据环境选择启动方式
if [ "$ENV" = "dev" ]; then
    echo "🛠️  启动开发环境..."

    # 启动基础服务（数据库、Redis）
    echo "📊 启动数据库和缓存服务..."
    docker-compose up -d postgres redis

    # 等待数据库启动
    echo "⏳ 等待数据库启动..."
    timeout 60 bash -c 'until docker-compose exec postgres pg_isready -U postgres; do sleep 2; done'

    if [ $? -eq 0 ]; then
        echo "✅ 数据库已就绪"
    else
        echo "❌ 数据库启动超时"
        exit 1
    fi

    # 启动 WeKnora 服务
    echo "🧠 启动 WeKnora 知识库服务..."
    docker-compose -f docker-compose.yml -f docker-compose.weknora.yml up -d weknora

    # 等待 WeKnora 启动
    echo "⏳ 等待 WeKnora 服务启动..."
    timeout 120 bash -c 'until curl -s http://localhost:9000/api/v1/health > /dev/null; do sleep 5; done'

    if [ $? -eq 0 ]; then
        echo "✅ WeKnora 服务已就绪"
    else
        echo "⚠️  WeKnora 服务启动可能需要更多时间，继续启动主服务..."
    fi

    # 启动主服务
    echo "🚀 启动 Servify 主服务..."
    docker-compose -f docker-compose.yml -f docker-compose.weknora.yml up -d servify

    # 可选服务提示
    echo ""
    echo "🔧 可选服务启动命令："
    echo "   本地 Embedding 服务: docker-compose --profile local-embedding up -d"
    echo "   Elasticsearch 服务:  docker-compose --profile with-elasticsearch up -d"

else
    echo "🏭 启动生产环境..."

    # 生产环境启动所有服务
    docker-compose -f docker-compose.yml -f docker-compose.weknora.yml up -d
fi

# 健康检查
echo ""
echo "🔍 正在进行健康检查..."

services=(
    "http://localhost:8080/health:Servify API"
    "http://localhost:9000/api/v1/health:WeKnora API"
    "http://localhost:5432:PostgreSQL"
    "http://localhost:6379:Redis"
)

for service in "${services[@]}"; do
    IFS=':' read -r url name <<< "$service"

    if [[ "$url" == *"5432"* ]]; then
        # PostgreSQL 检查
        if docker-compose exec postgres pg_isready -U postgres > /dev/null 2>&1; then
            echo "✅ $name: 健康"
        else
            echo "❌ $name: 不健康"
        fi
    elif [[ "$url" == *"6379"* ]]; then
        # Redis 检查
        if docker-compose exec redis redis-cli ping > /dev/null 2>&1; then
            echo "✅ $name: 健康"
        else
            echo "❌ $name: 不健康"
        fi
    else
        # HTTP 服务检查
        if curl -s "$url" > /dev/null 2>&1; then
            echo "✅ $name: 健康"
        else
            echo "❌ $name: 不健康"
        fi
    fi
done

echo ""
echo "🎉 启动完成！"
echo ""
echo "📍 服务地址："
echo "   Servify Web:    http://localhost:8080"
echo "   Servify API:    http://localhost:8080/api/v1"
echo "   WeKnora API:    http://localhost:9000/api/v1"
echo "   WeKnora Web:    http://localhost:9001"
echo "   PostgreSQL:     localhost:5432"
echo "   Redis:          localhost:6379"
echo ""
echo "📚 快速测试："
echo "   健康检查:       curl http://localhost:8080/health"
echo "   WebSocket:      wscat -c ws://localhost:8080/api/v1/ws"
echo "   WeKnora 健康:   curl http://localhost:9000/api/v1/health"
echo ""
echo "📋 管理命令："
echo "   查看日志:       docker-compose logs -f"
echo "   停止服务:       docker-compose down"
echo "   重启服务:       docker-compose restart"
echo "   查看状态:       docker-compose ps"
echo ""

# 如果是开发环境，提供额外的开发提示
if [ "$ENV" = "dev" ]; then
    echo "🛠️  开发环境提示："
    echo "   配置文件:       config.weknora.yml"
    echo "   环境变量:       .env"
    echo "   日志目录:       ./logs/"
    echo "   上传目录:       ./uploads/"
    echo "   数据目录:       ./data/"
    echo ""
    echo "🧪 初始化知识库："
    echo "   ./scripts/init-knowledge-base.sh"
    echo ""
fi

echo "✨ 享受使用 Servify + WeKnora！"