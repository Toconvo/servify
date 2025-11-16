# WeKnora 集成完整指南

## 🎯 项目概述

本指南详细介绍了如何将腾讯开源的 WeKnora 知识库系统集成到 Servify 智能客服项目中，实现从简单的内存知识库到企业级 RAG 系统的升级。

## 📋 集成计划完成情况

### ✅ 已完成任务

1. **项目路线图更新** ✅
   - 更新了 README.md 中的第二阶段计划
   - 将 WeKnora 集成作为 v1.1 的最高优先级
   - 更新了技术架构图和技术栈说明

2. **技术实施方案设计** ✅
   - 完整的 WeKnora 客户端实现 (`apps/server/pkg/weknora/`)
   - 增强的 AI 服务设计 (`apps/server/internal/services/ai_enhanced.go`)
   - 降级策略和熔断器机制
   - 数据结构和接口定义

3. **开发环境配置** ✅
   - Docker Compose 集成配置 (`infra/compose/docker-compose.weknora.yml`)
   - 数据库初始化脚本 (`scripts/init-db.sql`)
   - 环境变量配置模板 (`.env.weknora.example`)
   - 配置文件模板 (`config.weknora.yml`)

4. **部署和管理脚本** ✅
   - 一键启动脚本 (`scripts/start-weknora.sh`)
   - 知识库初始化脚本 (`scripts/init-knowledge-base.sh`)
   - 知识库管理脚本 (`scripts/manage-knowledge-base.sh`)

## 🏗️ 技术架构

### 集成架构
```
Servify 智能客服
       ↓
   HTTP API 调用
       ↓
   WeKnora 知识库服务
       ↓
PostgreSQL (pgvector) + Redis + Elasticsearch
```

### 服务部署图
```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│   Servify   │───▶│   WeKnora   │───▶│ PostgreSQL  │
│  (Port 8080)│    │ (Port 9000) │    │ (pgvector)  │
└─────────────┘    └─────────────┘    └─────────────┘
                           │
                           ▼
                  ┌─────────────┐
                  │    Redis    │
                  │ (缓存+队列) │
                  └─────────────┘
```

## 🚀 快速开始

### 1. 环境准备
```bash
# 检查环境
docker --version
docker-compose --version

# 克隆项目（如果还没有）
git clone <your-servify-repo>
cd servify
```

### 2. 配置环境变量
```bash
# 复制环境变量模板
cp .env.weknora.example .env

# 编辑环境变量，至少需要配置：
# - OPENAI_API_KEY: OpenAI API 密钥
# - WEKNORA_API_KEY: WeKnora API 密钥（可保持默认）
nano .env
```

### 3. 启动服务
```bash
# 一键启动开发环境
./scripts/start-weknora.sh dev

# 等待服务启动完成...
```

### 4. 初始化知识库
```bash
# 创建知识库并上传示例文档
./scripts/init-knowledge-base.sh
```

### 5. 验证集成
```bash
# 检查服务健康状态
curl http://localhost:8080/health
curl http://localhost:9000/api/v1/health

# 测试知识库搜索
./scripts/manage-knowledge-base.sh search "远程协助"
```

## 🔧 开发指南

### 项目结构
```
servify/
├── docs/
│   └── WEKNORA_INTEGRATION.md     # 本文档
├── scripts/
│   ├── start-weknora.sh           # 启动脚本
│   ├── init-knowledge-base.sh     # 知识库初始化
│   ├── manage-knowledge-base.sh   # 知识库管理
│   └── init-db.sql               # 数据库初始化
├── apps/
│   └── server/
│       ├── pkg/weknora/           # WeKnora 客户端
│       ├── client.go
│       └── types.go
│       └── internal/services/
│           └── ai_enhanced.go     # 增强的 AI 服务
├── infra/compose/docker-compose.weknora.yml     # WeKnora 部署配置
├── config.weknora.yml            # 配置文件模板
└── .env.weknora.example          # 环境变量模板
```

### 核心代码示例

#### WeKnora 客户端使用
```go
// 创建客户端
client := weknora.NewClient(config, logger)

// 搜索知识库
response, err := client.SearchKnowledge(ctx, &weknora.SearchRequest{
    Query:           "用户问题",
    KnowledgeBaseID: "kb-id",
    Limit:           5,
    Strategy:        "hybrid",
})

// 上传文档
docInfo, err := client.UploadDocument(ctx, kbID, &weknora.Document{
    Type:    "text",
    Title:   "文档标题",
    Content: "文档内容",
    Tags:    []string{"tag1", "tag2"},
})
```

#### AI 服务集成
```go
// 创建增强的 AI 服务
aiService := NewEnhancedAIService(config, logger)

// 处理用户查询（自动使用 WeKnora 或降级）
response, err := aiService.ProcessQuery(ctx, userQuery, sessionID)
```

## 📊 监控和维护

### 健康检查端点
- Servify API: `GET http://localhost:8080/health`
- WeKnora API: `GET http://localhost:9000/api/v1/health`
- PostgreSQL: `docker-compose -f infra/compose/docker-compose.yml exec postgres pg_isready`
- Redis: `docker-compose -f infra/compose/docker-compose.yml exec redis redis-cli ping`

### 日志查看
```bash
# 查看所有服务日志
docker-compose -f infra/compose/docker-compose.yml -f infra/compose/docker-compose.weknora.yml logs -f

# 查看特定服务日志
docker-compose -f infra/compose/docker-compose.yml -f infra/compose/docker-compose.weknora.yml logs -f servify
docker-compose -f infra/compose/docker-compose.yml -f infra/compose/docker-compose.weknora.yml logs -f weknora

# 查看应用日志文件
tail -f logs/servify.log
```

### 性能监控
```bash
# 查看服务状态
docker-compose -f infra/compose/docker-compose.yml -f infra/compose/docker-compose.weknora.yml ps

# 查看资源使用情况
docker stats

# 查看知识库统计
./scripts/manage-knowledge-base.sh stats
```

## 🔒 安全配置

### 生产环境配置
1. **修改默认密钥**：
   - 更改 JWT_SECRET
   - 更改 WEKNORA_API_KEY
   - 更改数据库密码

2. **网络安全**：
   - 限制 CORS 允许的域名
   - 配置防火墙规则
   - 使用 HTTPS

3. **数据安全**：
   - 定期备份数据库
   - 加密敏感数据
   - 实施访问控制

### 配置示例
```yaml
# 生产环境配置
security:
  cors:
    allowed_origins: ["https://yourdomain.com"]
  rate_limiting:
    enabled: true
    requests_per_minute: 60
```

## 📈 性能优化

### 推荐配置
1. **数据库优化**：
   - 配置连接池
   - 创建合适的索引
   - 定期 VACUUM

2. **缓存策略**：
   - 启用 Redis 缓存
   - 配置 TTL
   - 实施缓存预热

3. **WeKnora 优化**：
   - 调整 chunk_size
   - 优化 embedding 模型
   - 配置检索策略

## 🐛 故障排除

### 常见问题

#### 1. WeKnora 服务无法启动
```bash
# 检查端口占用
lsof -i :9000

# 检查配置文件
docker-compose config

# 查看详细错误日志
docker-compose -f infra/compose/docker-compose.yml -f infra/compose/docker-compose.weknora.yml logs weknora
```

#### 2. 数据库连接失败
```bash
# 检查数据库状态
docker-compose -f infra/compose/docker-compose.yml exec postgres pg_isready -U postgres

# 检查网络连接
docker network ls
docker network inspect servify_servify_network
```

#### 3. 知识库搜索无结果
```bash
# 检查文档是否上传成功
./scripts/manage-knowledge-base.sh list

# 检查索引状态
curl -H "X-API-Key: default-api-key" \
     http://localhost:9000/api/v1/knowledge/default-kb
```

## 🔄 更新和维护

### 版本更新
```bash
# 停止服务
docker-compose -f infra/compose/docker-compose.yml -f infra/compose/docker-compose.weknora.yml down

# 更新镜像
docker-compose -f infra/compose/docker-compose.yml -f infra/compose/docker-compose.weknora.yml pull

# 重新启动
./scripts/start-weknora.sh
```

### 数据备份
```bash
# 备份 PostgreSQL
docker-compose -f infra/compose/docker-compose.yml exec postgres pg_dump -U postgres servify > backup.sql

# 备份 WeKnora 数据
docker cp servify_weknora:/app/data ./backup/weknora_data
```

## 📚 相关资源

### 官方文档
- [WeKnora GitHub](https://github.com/Tencent/WeKnora)
- [WeKnora API 文档](https://github.com/Tencent/WeKnora/blob/main/docs/API.md)
- [pgvector 文档](https://github.com/pgvector/pgvector)

### 社区资源
- [WeKnora Issues](https://github.com/Tencent/WeKnora/issues)
- [PostgreSQL 中文社区](https://www.postgresql.org/community/)

## 🎯 下一步计划

### v1.1.1 - 基础集成（本次完成）
- [x] WeKnora 基础集成
- [x] HTTP API 客户端
- [x] 降级机制
- [x] 基础监控

### v1.1.2 - 功能增强（下一步）
- [ ] 文件上传接口
- [ ] 批量文档处理
- [ ] 高级搜索策略
- [ ] 缓存优化

### v1.1.3 - 生产就绪（后续）
- [ ] 详细监控指标
- [ ] 自动故障恢复
- [ ] 性能调优
- [ ] 安全加固

## 💬 支持和反馈

如有问题或建议，请：
1. 查看本文档的故障排除部分
2. 检查 [WeKnora 官方文档](https://github.com/Tencent/WeKnora)
3. 在项目 Issues 中提交问题
4. 联系开发团队

---

**🎉 恭喜！WeKnora 集成计划已全部完成，现在可以享受企业级智能知识库的强大功能了！**
