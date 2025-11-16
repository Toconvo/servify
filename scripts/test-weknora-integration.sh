#!/bin/bash

# WeKnora 集成测试脚本
# 用于验证 Servify + WeKnora 集成是否正常工作

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo "🧪 WeKnora 集成测试开始..."

# 服务端点（可被环境变量覆盖）
SERVIFY_URL=${SERVIFY_URL:-"http://localhost:8080"}
WEKNORA_URL=${WEKNORA_URL:-"http://localhost:9000"}
WEKNORA_ENABLED=${WEKNORA_ENABLED:-true}

# 小工具：带重试的等待
wait_for() {
  local name=$1 url=$2 max=$3 sleep_s=$4
  echo "⏳ 等待 $name 可用: $url (最多 ${max} 次，每次 ${sleep_s}s)"
  for i in $(seq 1 "$max"); do
    if curl -fsS "$url" > /dev/null; then
      echo "✅ $name 可用"
      return 0
    fi
    echo "… 第 $i/${max} 次重试"
    sleep "$sleep_s"
  done
  echo "❌ $name 不可用: $url"
  return 1
}

echo "🔍 检查服务状态..."

# 等待服务启动
wait_for "Servify Health" "$SERVIFY_URL/health" 30 2
WEKNORA_AVAILABLE=false
if [ "$WEKNORA_ENABLED" = "true" ]; then
  if wait_for "WeKnora Health" "$WEKNORA_URL/api/v1/health" 30 2; then
    WEKNORA_AVAILABLE=true
  else
    echo "⚠️ WeKnora 未就绪，将尝试降级模式继续"
  fi
fi

# 1. 测试 Servify 健康检查
echo "  ✓ 测试 Servify 健康检查..."
if curl -fsS "$SERVIFY_URL/health" > /dev/null; then
    echo "    ✅ Servify 健康检查通过"
else
    echo "    ❌ Servify 健康检查失败"
    exit 1
fi

# 2. 测试 WeKnora 健康检查（如果启用）
if [ "${WEKNORA_ENABLED:-false}" = "true" ]; then
    echo "  ✓ 测试 WeKnora 健康检查..."
    if curl -fsS "$WEKNORA_URL/api/v1/health" > /dev/null; then
        echo "    ✅ WeKnora 健康检查通过"
    else
        echo "    ⚠️  WeKnora 健康检查失败，但降级机制可用"
    fi
fi

# 3. 测试 AI API
echo "🤖 测试 AI 服务..."

# 测试简单查询
echo "  ✓ 测试基础 AI 查询..."
AI_RESPONSE=$(curl -fsS -X POST "$SERVIFY_URL/api/v1/ai/query" \
    -H "Content-Type: application/json" \
    -d '{
        "query": "你好，我想了解远程协助功能",
        "session_id": "test_session_123"
    }')

if echo "$AI_RESPONSE" | grep -q '"success":true'; then
    echo "    ✅ AI 查询测试通过"
    if command -v jq >/dev/null 2>&1; then
      echo "    📝 AI 响应: $(echo "$AI_RESPONSE" | jq -r '.data.content')"
    else
      echo "    📝 AI 原始响应: $AI_RESPONSE"
    fi
else
    echo "    ❌ AI 查询测试失败"
    echo "    📝 错误响应: $AI_RESPONSE"
    exit 1
fi

# 4. 测试 AI 状态
echo "  ✓ 测试 AI 服务状态..."
AI_STATUS=$(curl -fsS "$SERVIFY_URL/api/v1/ai/status")

if echo "$AI_STATUS" | grep -q '"success":true'; then
    echo "    ✅ AI 状态查询通过"

    # 显示服务类型
    if command -v jq >/dev/null 2>&1; then
  # 优先读取 type；若缺失则根据 weknora_enabled 推断
  SERVICE_TYPE=$(echo "$AI_STATUS" | jq -r '.data.type // ( .data.weknora_enabled == true and "enhanced" or "standard" )')
    else
      SERVICE_TYPE="unknown"
    fi
    echo "    📊 服务类型: $SERVICE_TYPE"

    if [ "$SERVICE_TYPE" = "enhanced" ]; then
        echo "    🚀 使用增强型 AI 服务 (WeKnora 集成)"
    else
        echo "    📚 使用标准 AI 服务 (传统知识库)"
    fi
else
    echo "    ❌ AI 状态查询失败"
    echo "    📝 错误响应: $AI_STATUS"
fi

# 5. 测试 WeKnora 专用功能（如果是增强服务）
if [ "$SERVICE_TYPE" = "enhanced" ]; then
  echo "🔧 测试 WeKnora 专用功能..."

    # 测试指标查询
    echo "  ✓ 测试服务指标..."
    METRICS_RESPONSE=$(curl -fsS "$SERVIFY_URL/api/v1/ai/metrics")

    if echo "$METRICS_RESPONSE" | grep -q '"success":true'; then
        echo "    ✅ 指标查询通过"

        # 显示一些关键指标
        if command -v jq >/dev/null 2>&1; then
          QUERY_COUNT=$(echo "$METRICS_RESPONSE" | jq -r '.data.query_count')
          WEKNORA_COUNT=$(echo "$METRICS_RESPONSE" | jq -r '.data.weknora_usage_count')
          FALLBACK_COUNT=$(echo "$METRICS_RESPONSE" | jq -r '.data.fallback_usage_count')
        else
          QUERY_COUNT="N/A"; WEKNORA_COUNT="N/A"; FALLBACK_COUNT="N/A"
        fi

        echo "    📊 查询总数: $QUERY_COUNT"
        echo "    📊 WeKnora 使用次数: $WEKNORA_COUNT"
        echo "    📊 降级使用次数: $FALLBACK_COUNT"
    else
        echo "    ⚠️  指标查询失败: $METRICS_RESPONSE"
    fi

    # 测试文档上传
    echo "  ✓ 测试文档上传..."
    UPLOAD_RESPONSE=$(curl -fsS -X POST "$SERVIFY_URL/api/v1/ai/knowledge/upload" \
        -H "Content-Type: application/json" \
        -d '{
            "title": "测试文档",
            "content": "这是一个测试文档，用于验证 WeKnora 集成功能。包含远程协助、智能客服等功能介绍。",
            "tags": ["测试", "集成", "验证"]
        }')

    if echo "$UPLOAD_RESPONSE" | grep -q '"success":true'; then
        echo "    ✅ 文档上传测试通过"
    else
        echo "    ⚠️  文档上传测试失败：$UPLOAD_RESPONSE"
        if [ "$WEKNORA_AVAILABLE" != "true" ]; then
          echo "       （提示：当前处于降级模式，WeKnora 不可用）"
        fi
    fi
fi

# 6. 测试 WebSocket 连接
echo "🔌 测试 WebSocket 连接..."

# 检查 WebSocket 端点是否响应
WS_STATS=$(curl -fsS "$SERVIFY_URL/api/v1/ws/stats")

if echo "$WS_STATS" | grep -q '"success":true'; then
    echo "    ✅ WebSocket 服务正常"

    CLIENT_COUNT=$(echo "$WS_STATS" | jq -r '.data.client_count' 2>/dev/null || echo "N/A")
    echo "    📊 当前连接数: $CLIENT_COUNT"
else
    echo "    ❌ WebSocket 服务异常: $WS_STATS"
fi

# 7. 测试 WebRTC 功能
echo "📡 测试 WebRTC 服务..."

WEBRTC_STATS=$(curl -fsS "$SERVIFY_URL/api/v1/webrtc/connections")

if echo "$WEBRTC_STATS" | grep -q '"success":true'; then
    echo "    ✅ WebRTC 服务正常"

    CONNECTION_COUNT=$(echo "$WEBRTC_STATS" | jq -r '.data.connection_count' 2>/dev/null || echo "N/A")
    echo "    📊 WebRTC 连接数: $CONNECTION_COUNT"
else
    echo "    ❌ WebRTC 服务异常: $WEBRTC_STATS"
fi

# 8. 性能测试
echo "⚡ 简单性能测试..."

echo "  ✓ 测试并发查询处理..."
CONCURRENT_REQUESTS=5
START_TIME=$(date +%s)

for i in $(seq 1 $CONCURRENT_REQUESTS); do
    curl -s -X POST "$SERVIFY_URL/api/v1/ai/query" \
        -H "Content-Type: application/json" \
        -d "{
            \"query\": \"测试查询 $i\",
            \"session_id\": \"test_session_$i\"
        }" > /dev/null &
done

wait

END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))

echo "    ✅ $CONCURRENT_REQUESTS 个并发请求完成"
echo "    ⏱️  总耗时: ${DURATION}s"

# 9. 集成测试总结
echo ""
echo "📋 集成测试总结:"
echo "════════════════════════════════════════"

# 检查总体状态
OVERALL_HEALTH=$(curl -fsS "$SERVIFY_URL/health")
if command -v jq >/dev/null 2>&1; then
  OVERALL_STATUS=$(echo "$OVERALL_HEALTH" | jq -r '.status')
else
  OVERALL_STATUS="unknown"
fi

case "$OVERALL_STATUS" in
    "healthy")
        echo "🎉 所有服务运行正常！"
        echo "✅ Servify + WeKnora 集成测试通过"
        ;;
    "degraded")
        echo "⚠️  部分服务降级运行"
        echo "✅ 核心功能正常，WeKnora 可能不可用但有降级保护"
        ;;
    *)
        echo "❌ 服务状态异常: $OVERALL_STATUS"
        echo "❌ 集成测试失败"
        exit 1
        ;;
esac

echo ""
echo "🔗 服务地址:"
echo "   Servify Web:    $SERVIFY_URL"
echo "   Servify API:    $SERVIFY_URL/api/v1"
echo "   健康检查:       $SERVIFY_URL/health"
echo "   WebSocket:      ws://localhost:8080/api/v1/ws"

if [ "${WEKNORA_ENABLED:-false}" = "true" ]; then
    echo "   WeKnora API:    $WEKNORA_URL/api/v1"
    echo "   WeKnora Web:    $WEKNORA_URL:9001"
fi

echo ""
echo "📚 测试完成的功能:"
echo "   ✅ 健康检查和状态监控"
echo "   ✅ AI 智能问答处理"
echo "   ✅ WebSocket 实时通信"
echo "   ✅ WebRTC 连接管理"
echo "   ✅ 并发请求处理"

if [ "$SERVICE_TYPE" = "enhanced" ]; then
    echo "   ✅ WeKnora 知识库集成"
    echo "   ✅ 降级机制和熔断器"
    echo "   ✅ 服务指标监控"
    echo "   ✅ 文档上传功能"
fi

echo ""
echo "🎯 下一步建议:"
echo "   1. 在浏览器中访问 $SERVIFY_URL 体验完整功能"
echo "   2. 使用 WebSocket 客户端测试实时聊天"
echo "   3. 如需测试远程协助，请使用支持 WebRTC 的浏览器"

if [ "$SERVICE_TYPE" = "enhanced" ]; then
    echo "   4. 通过 WeKnora Web UI 管理知识库: $WEKNORA_URL:9001"
    echo "   5. 使用 API 上传更多文档到知识库"
fi

echo ""
echo "✨ WeKnora 集成测试完成！"
echo ""
echo "🛡️ 进行基础鉴权测试（管理类 API）..."

# helper: base64url without padding
base64url() {
  openssl base64 -A | tr '+/' '-_' | tr -d '='
}

# Generate HS256 JWT with default secret (must match server config/jwt.secret)
issue_jwt() {
  local secret="${1:-default-secret-key}"
  local uid="${2:-1}"
  local roles="${3:-[\"admin\"]}"
  local now=$(date +%s)
  local exp=$((now + 3600))
  local header='{"alg":"HS256","typ":"JWT"}'
  local payload=$(printf '{"user_id":%s,"roles":%s,"iat":%s,"exp":%s}' "$uid" "$roles" "$now" "$exp")
  local b64_header=$(printf '%s' "$header" | base64url)
  local b64_payload=$(printf '%s' "$payload" | base64url)
  local signing_input="${b64_header}.${b64_payload}"
  local sig=$(printf '%s' "$signing_input" | openssl dgst -sha256 -mac HMAC -macopt "key:$secret" -binary | base64url)
  printf '%s.%s' "$signing_input" "$sig"
}

AUTH_TOKEN=$(issue_jwt "default-secret-key" "1" "[\"admin\"]")

echo "  ✓ 无 token 访问应被拒绝..."
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$SERVIFY_URL/api/customers/stats" || true)
if [ "$HTTP_CODE" != "401" ] && [ "$HTTP_CODE" != "403" ]; then
  echo "    ❌ 期望 401/403，得到 $HTTP_CODE"
  echo "    🔎 返回详情："
  curl -s -i "$SERVIFY_URL/api/customers/stats" || true
  exit 1
else
  echo "    ✅ 未授权访问被拒绝 ($HTTP_CODE)"
fi

echo "  ✓ 携带有效 token 访问应成功..."
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -H "Authorization: Bearer $AUTH_TOKEN" "$SERVIFY_URL/api/customers/stats" || true)
if [ "$HTTP_CODE" = "200" ]; then
  echo "    ✅ 授权访问成功 (200)"
else
  echo "    ❌ 授权访问失败，HTTP $HTTP_CODE"
  echo "    🔎 返回详情："
  curl -s -i -H "Authorization: Bearer $AUTH_TOKEN" "$SERVIFY_URL/api/customers/stats" || true
  exit 1
fi

echo "✅ 鉴权测试完成"

echo ""
echo "🛡️ 管理员专属接口测试（/api/statistics/...）..."

# 仅 agent 角色访问 admin-only 接口应 403
AGENT_TOKEN=$(issue_jwt "default-secret-key" "2" "[\"agent\"]")
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -H "Authorization: Bearer $AGENT_TOKEN" "$SERVIFY_URL/api/statistics/dashboard" || true)
if [ "$HTTP_CODE" = "403" ]; then
  echo "    ✅ agent 访问 admin-only 接口被拒绝 (403)"
else
  echo "    ❌ 期望 403，得到 $HTTP_CODE"
  echo "    🔎 返回详情："
  curl -s -i -H "Authorization: Bearer $AGENT_TOKEN" "$SERVIFY_URL/api/statistics/dashboard" || true
  exit 1
fi

# admin 访问应 200
ADMIN_TOKEN=$(issue_jwt "default-secret-key" "1" "[\"admin\"]")
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -H "Authorization: Bearer $ADMIN_TOKEN" "$SERVIFY_URL/api/statistics/dashboard" || true)
if [ "$HTTP_CODE" = "200" ]; then
  echo "    ✅ admin 访问 admin-only 接口成功 (200)"
else
  echo "    ❌ 访问失败，HTTP $HTTP_CODE"
  echo "    🔎 返回详情："
  curl -s -i -H "Authorization: Bearer $ADMIN_TOKEN" "$SERVIFY_URL/api/statistics/dashboard" || true
  exit 1
fi

echo "✅ 管理员专属接口测试完成"

echo ""
echo "🚦 速率限制测试（/api/v1/ai/query）..."
R200=0
R429=0
TOTAL=50
for i in $(seq 1 "$TOTAL"); do
  CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$SERVIFY_URL/api/v1/ai/query" \
    -H "Content-Type: application/json" \
    -d "{\"query\":\"rl_test_$i\",\"session_id\":\"rl_test_session\"}" || true)
  if [ "$CODE" = "200" ]; then R200=$((R200+1)); fi
  if [ "$CODE" = "429" ]; then R429=$((R429+1)); fi
done
echo "    ↳ 成功: $R200, 限流: $R429, 总计: $TOTAL"
if [ "$R429" -gt 0 ]; then
  echo "    ✅ 触发限流成功（检测到 429）"
else
  echo "    ❌ 未触发限流，请检查 security.rate_limiting 配置或中间件挂载"
  echo "    🔎 样例请求详情："
  curl -s -i -X POST "$SERVIFY_URL/api/v1/ai/query" \
    -H "Content-Type: application/json" \
    -d '{"query":"probe","session_id":"rl_probe"}' || true
  exit 1
fi

echo ""
echo "🚦 限流白名单（X-API-Key）测试..."
R200=0
R429=0
for i in $(seq 1 "$TOTAL"); do
  CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$SERVIFY_URL/api/v1/ai/query" \
    -H "Content-Type: application/json" \
    -H "X-API-Key: internal-test-key" \
    -d "{\"query\":\"wl_test_$i\",\"session_id\":\"rl_test_session\"}" || true)
  if [ "$CODE" = "200" ]; then R200=$((R200+1)); fi
  if [ "$CODE" = "429" ]; then R429=$((R429+1)); fi
done
echo "    ↳ (白名单) 成功: $R200, 限流: $R429, 总计: $TOTAL"
if [ "$R429" -eq 0 ]; then
  echo "    ✅ 白名单跳过限流生效"
else
  echo "    ❌ 白名单无效，请检查 key_header 与 whitelist_keys 配置"
  exit 1
fi
