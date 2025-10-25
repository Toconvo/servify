#!/bin/bash

# WeKnora 知识库初始化脚本
# 使用方法: ./scripts/init-knowledge-base.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo "🧠 初始化 WeKnora 知识库..."

# 配置变量
WEKNORA_URL="http://localhost:9000"
API_KEY="default-api-key"
TENANT_ID="default-tenant"

# 检查 WeKnora 服务是否运行
echo "🔍 检查 WeKnora 服务状态..."
if ! curl -s "$WEKNORA_URL/api/v1/health" > /dev/null; then
    echo "❌ WeKnora 服务未运行，请先启动服务"
    echo "   运行: ./scripts/start-weknora.sh"
    exit 1
fi

echo "✅ WeKnora 服务运行正常"

# 创建租户（如果不存在）
echo "🏢 创建/检查租户..."
curl -s -X POST "$WEKNORA_URL/api/v1/tenants" \
    -H "Content-Type: application/json" \
    -H "X-API-Key: $API_KEY" \
    -d '{
        "name": "Servify",
        "description": "Servify 智能客服系统",
        "config": {
            "max_documents": 10000,
            "max_storage_mb": 1000
        }
    }' > /dev/null 2>&1 || echo "租户可能已存在"

# 创建知识库
echo "📚 创建知识库..."
KB_RESPONSE=$(curl -s -X POST "$WEKNORA_URL/api/v1/knowledge-bases" \
    -H "Content-Type: application/json" \
    -H "X-API-Key: $API_KEY" \
    -H "X-Tenant-ID: $TENANT_ID" \
    -d '{
        "name": "Servify客服知识库",
        "description": "Servify智能客服系统的主要知识库",
        "config": {
            "chunk_size": 512,
            "chunk_overlap": 50,
            "embedding_model": "bge-large-zh-v1.5",
            "retrieval_mode": "hybrid",
            "score_threshold": 0.7
        }
    }')

KB_ID=$(echo "$KB_RESPONSE" | grep -o '"id":"[^"]*"' | cut -d'"' -f4 || echo "default-kb")
echo "✅ 知识库已创建，ID: $KB_ID"

# 准备示例文档
echo "📄 准备示例文档..."

# 创建示例文档目录
mkdir -p "$PROJECT_ROOT/data/sample-docs"

# 产品使用指南
cat > "$PROJECT_ROOT/data/sample-docs/product-guide.md" << 'EOF'
# Servify 产品使用指南

## 产品概述
Servify 是一款智能客服系统，提供文字聊天、AI 问答和远程协助功能。

## 主要功能

### 1. 智能问答
- 基于知识库的自动回答
- 支持自然语言理解
- 多轮对话支持

### 2. 远程协助
- 屏幕共享功能
- 实时协助
- 基于 WebRTC 技术

### 3. 多平台集成
- 支持微信接入
- 支持 QQ 机器人
- 支持 Telegram Bot

## 快速开始

1. 访问客服页面
2. 发送消息开始对话
3. 如需远程协助，点击"远程协助"按钮
4. 按照提示完成屏幕共享设置

## 常见问题

### Q: 如何开启远程协助？
A: 在聊天界面点击"远程协助"按钮，然后允许浏览器屏幕共享权限。

### Q: 支持哪些浏览器？
A: 支持 Chrome、Firefox、Safari 等现代浏览器。

### Q: 远程协助安全吗？
A: 是的，使用端到端加密的 WebRTC 技术，数据不经过服务器。
EOF

# API 文档
cat > "$PROJECT_ROOT/data/sample-docs/api-docs.md" << 'EOF'
# Servify API 文档

## 概述
Servify 提供 RESTful API 和 WebSocket 接口。

## 认证
使用 JWT Token 进行认证：
```
Authorization: Bearer <your-token>
```

## 核心接口

### 健康检查
```
GET /health
```

### WebSocket 连接
```
WebSocket: /api/v1/ws?session_id=<session_id>
```

### 消息格式
```json
{
  "type": "text-message",
  "data": {
    "content": "用户消息"
  },
  "session_id": "session_123",
  "timestamp": "2024-01-01T12:00:00Z"
}
```

## 错误处理
API 使用标准 HTTP 状态码：
- 200: 成功
- 400: 请求错误
- 401: 未授权
- 500: 服务器错误

## 速率限制
默认限制：每分钟 60 次请求
EOF

# 故障排除指南
cat > "$PROJECT_ROOT/data/sample-docs/troubleshooting.md" << 'EOF'
# 故障排除指南

## 常见问题与解决方案

### 连接问题

#### 无法连接到客服
1. 检查网络连接
2. 刷新页面重试
3. 清除浏览器缓存
4. 检查防火墙设置

#### WebSocket 连接失败
1. 确认浏览器支持 WebSocket
2. 检查代理服务器设置
3. 尝试使用不同网络

### 远程协助问题

#### 无法开启屏幕共享
1. 检查浏览器权限设置
2. 确认使用支持的浏览器
3. 重启浏览器重试

#### 屏幕共享质量差
1. 检查网络带宽
2. 关闭其他网络应用
3. 降低屏幕分辨率

### AI 问答问题

#### AI 回答不准确
1. 尝试重新描述问题
2. 提供更多上下文信息
3. 联系人工客服

#### 响应速度慢
1. 检查网络延迟
2. 刷新页面重试
3. 联系技术支持

## 联系支持
如果问题仍未解决，请联系技术支持：
- 邮箱: support@servify.cloud
- 电话: 400-xxx-xxxx
- 在线客服: 点击右下角客服按钮
EOF

echo "📤 上传示例文档到知识库..."

# 上传文档函数
upload_document() {
    local file_path="$1"
    local title="$2"
    local category="$3"

    echo "   上传: $title"

    curl -s -X POST "$WEKNORA_URL/api/v1/knowledge/$KB_ID/documents" \
        -H "Content-Type: application/json" \
        -H "X-API-Key: $API_KEY" \
        -H "X-Tenant-ID: $TENANT_ID" \
        -d "{
            \"title\": \"$title\",
            \"content\": $(cat "$file_path" | jq -Rs .),
            \"category\": \"$category\",
            \"tags\": [\"servify\", \"帮助\", \"指南\"]
        }" > /dev/null

    if [ $? -eq 0 ]; then
        echo "   ✅ $title 上传成功"
    else
        echo "   ❌ $title 上传失败"
    fi
}

# 上传所有文档
upload_document "$PROJECT_ROOT/data/sample-docs/product-guide.md" "产品使用指南" "产品介绍"
upload_document "$PROJECT_ROOT/data/sample-docs/api-docs.md" "API 开发文档" "技术文档"
upload_document "$PROJECT_ROOT/data/sample-docs/troubleshooting.md" "故障排除指南" "技术支持"

# 等待文档处理
echo "⏳ 等待文档处理和索引建立..."
sleep 5

# 测试搜索功能
echo "🔍 测试知识库搜索功能..."

test_search() {
    local query="$1"
    echo "   搜索: $query"

    SEARCH_RESULT=$(curl -s -X POST "$WEKNORA_URL/api/v1/knowledge/search" \
        -H "Content-Type: application/json" \
        -H "X-API-Key: $API_KEY" \
        -H "X-Tenant-ID: $TENANT_ID" \
        -d "{
            \"query\": \"$query\",
            \"kb_id\": \"$KB_ID\",
            \"limit\": 3,
            \"threshold\": 0.5,
            \"strategy\": \"hybrid\"
        }")

    RESULT_COUNT=$(echo "$SEARCH_RESULT" | grep -o '"total":[0-9]*' | cut -d':' -f2 || echo "0")
    echo "   ✅ 找到 $RESULT_COUNT 个相关结果"
}

# 执行测试搜索
test_search "远程协助"
test_search "API 接口"
test_search "连接问题"

# 更新配置文件中的知识库 ID
echo "🔧 更新配置文件..."

if [ -f "$PROJECT_ROOT/.env" ]; then
    if grep -q "WEKNORA_KB_ID=" "$PROJECT_ROOT/.env"; then
        sed -i.bak "s/WEKNORA_KB_ID=.*/WEKNORA_KB_ID=$KB_ID/" "$PROJECT_ROOT/.env"
    else
        echo "WEKNORA_KB_ID=$KB_ID" >> "$PROJECT_ROOT/.env"
    fi
    echo "✅ 已更新 .env 文件中的知识库 ID"
fi

# 创建管理脚本
cat > "$PROJECT_ROOT/scripts/manage-knowledge-base.sh" << EOF
#!/bin/bash

# 知识库管理脚本

WEKNORA_URL="http://localhost:9000"
API_KEY="default-api-key"
TENANT_ID="default-tenant"
KB_ID="$KB_ID"

case "\$1" in
    "search")
        query="\$2"
        if [ -z "\$query" ]; then
            echo "用法: \$0 search <查询内容>"
            exit 1
        fi
        curl -X POST "\$WEKNORA_URL/api/v1/knowledge/search" \\
            -H "Content-Type: application/json" \\
            -H "X-API-Key: \$API_KEY" \\
            -H "X-Tenant-ID: \$TENANT_ID" \\
            -d "{
                \"query\": \"\$query\",
                \"kb_id\": \"\$KB_ID\",
                \"limit\": 5,
                \"strategy\": \"hybrid\"
            }" | jq .
        ;;
    "list")
        curl -X GET "\$WEKNORA_URL/api/v1/knowledge/\$KB_ID/documents" \\
            -H "X-API-Key: \$API_KEY" \\
            -H "X-Tenant-ID: \$TENANT_ID" | jq .
        ;;
    "stats")
        curl -X GET "\$WEKNORA_URL/api/v1/knowledge/\$KB_ID" \\
            -H "X-API-Key: \$API_KEY" \\
            -H "X-Tenant-ID: \$TENANT_ID" | jq .
        ;;
    *)
        echo "用法: \$0 {search|list|stats}"
        echo "  search <query>  - 搜索知识库"
        echo "  list           - 列出所有文档"
        echo "  stats          - 显示知识库统计"
        ;;
esac
EOF

chmod +x "$PROJECT_ROOT/scripts/manage-knowledge-base.sh"

echo ""
echo "🎉 知识库初始化完成！"
echo ""
echo "📊 知识库信息："
echo "   知识库 ID: $KB_ID"
echo "   文档数量: 3 个示例文档"
echo "   配置策略: 混合检索（BM25 + 向量搜索）"
echo ""
echo "🔧 管理命令："
echo "   搜索知识库: ./scripts/manage-knowledge-base.sh search '远程协助'"
echo "   查看文档: ./scripts/manage-knowledge-base.sh list"
echo "   查看统计: ./scripts/manage-knowledge-base.sh stats"
echo ""
echo "🌐 Web 界面："
echo "   WeKnora 管理界面: http://localhost:9001"
echo ""
echo "✨ 现在可以测试智能客服功能了！"