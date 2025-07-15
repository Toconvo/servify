# Servify - 智能客服系统

一个基于 WebRTC 的智能客服系统，支持文字交流、AI 智能问答和远程协助功能。

## 系统概述

### 核心功能
1. **Web 客户端 SDK** - 支持 React、Vue、原生 JS 集成
2. **AI 智能客服** - 基于知识库的智能问答系统
3. **多平台集成** - 支持微信、QQ、飞书、Telegram 等第三方平台
4. **远程协助** - 基于 WebRTC 的屏幕共享和远程控制

## 技术架构

### 技术栈选择
- **后端**: Go + Pion (WebRTC) + Gin (HTTP服务)
- **前端**: TypeScript + 原生 WebRTC API
- **数据库**: PostgreSQL + Redis
- **AI**: OpenAI API / 本地 LLM + 向量数据库 (Weaviate/Qdrant)
- **消息队列**: Redis/RabbitMQ

### 系统架构图
```
┌─────────────────────────────────────────────────────────────┐
│                    客户端 SDK                                │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐        │
│  │   React     │  │     Vue     │  │  Vanilla JS │        │
│  └─────────────┘  └─────────────┘  └─────────────┘        │
└─────────────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────┐
│                    网关服务                                  │
│           (Go + Gin + WebSocket)                            │
└─────────────────────────────────────────────────────────────┘
                           │
        ┌─────────────────┼─────────────────┐
        ▼                 ▼                 ▼
┌─────────────┐  ┌─────────────┐  ┌─────────────┐
│  WebRTC服务 │  │  AI智能客服  │  │  消息路由    │
│ (Go+Pion)   │  │   服务      │  │    服务     │
└─────────────┘  └─────────────┘  └─────────────┘
                           │                 │
                           ▼                 ▼
                  ┌─────────────┐  ┌─────────────┐
                  │  知识库     │  │  第三方集成  │
                  │ (Vector DB) │  │  (微信/QQ等) │
                  └─────────────┘  └─────────────┘
```

## 客户端实现

### 混合架构设计
- **文本聊天**: WebSocket 连接，低延迟消息传输
- **远程协助**: 原生 WebRTC API，屏幕共享和实时通信
- **统一 SDK**: 封装复杂度，提供简洁的开发接口

### 客户端 SDK 接口
```typescript
interface ServifyClient {
  // 初始化连接
  init(config: ServifyConfig): Promise<void>
  
  // 文本消息
  sendMessage(message: string): Promise<void>
  onMessage(callback: (message: Message) => void): void
  
  // 远程协助
  startRemoteAssist(): Promise<void>
  endRemoteAssist(): void
  
  // 状态管理
  getConnectionStatus(): ConnectionStatus
  onStatusChange(callback: (status: ConnectionStatus) => void): void
}
```

### 框架集成示例
```javascript
// React Hook
const useServify = (config) => {
  const [client, setClient] = useState(null)
  const [messages, setMessages] = useState([])
  // 具体实现...
}

// Vue Composition API
const useServify = (config) => {
  const client = ref(null)
  const messages = ref([])
  // 具体实现...
}

// Vanilla JS
const servify = new ServifyClient(config)
```

## AI 智能客服

### 知识库系统
```go
type KnowledgeBase struct {
    VectorDB    *VectorDatabase // 向量数据库
    Embeddings  *EmbeddingModel // 文本向量化
    LLM         *LanguageModel  // 大语言模型
}

// RAG 流程
func (kb *KnowledgeBase) ProcessQuery(query string) (*Response, error) {
    // 1. 向量化查询
    embedding := kb.Embeddings.Embed(query)
    
    // 2. 相似度搜索
    docs := kb.VectorDB.Search(embedding, topK: 5)
    
    // 3. 构建提示词
    prompt := kb.buildPrompt(query, docs)
    
    // 4. 生成回答
    response := kb.LLM.Generate(prompt)
    
    return response, nil
}
```

### 对话管理
- 会话状态管理
- 上下文记忆
- 人工客服转接逻辑
- 智能路由决策

## 多平台集成

### 统一消息路由
```go
type MessageRouter struct {
    Adapters map[string]PlatformAdapter
    Queue    *MessageQueue
}

type PlatformAdapter interface {
    SendMessage(chatID, message string) error
    ReceiveMessage() <-chan Message
    GetPlatformType() PlatformType
}
```

### 支持平台
- 微信公众号/企业微信
- QQ 机器人
- 飞书应用
- Telegram Bot
- 钉钉机器人

## 实施计划

### 第一阶段 (MVP)
- [ ] 基础 WebRTC 文本通信
- [ ] 简单 AI 问答（接入 OpenAI）
- [ ] Web SDK 基础功能
- [ ] 一个第三方平台集成（Telegram）

### 第二阶段
- [ ] 远程协助功能
- [ ] 完整知识库系统
- [ ] 多平台集成
- [ ] 高级 AI 功能（上下文记忆）

### 第三阶段
- [ ] 性能优化
- [ ] 监控和分析
- [ ] 企业级功能
- [ ] 移动端支持

## 技术选择理由

### 为什么选择 Go + Pion 而非 C++ + MediaSoup？

**Go + Pion 优势：**
- 开发效率高，适合快速迭代
- 天然并发支持，适合实时通信场景
- 生态完善，AI 集成便利
- 内存安全，系统稳定性好
- 跨平台编译，部署运维简单

**客户端方案：**
- 使用浏览器原生 WebRTC API
- 通过 WebSocket 处理信令
- 封装统一的 SDK 接口
- 支持多框架集成

## 开发环境

### 前置要求
- Go 1.21+
- Node.js 18+
- PostgreSQL 13+
- Redis 6+

### 快速开始
```bash
# 克隆项目
git clone https://github.com/your-org/servify.git
cd servify

# 启动后端服务
cd server
go mod tidy
go run main.go

# 启动客户端开发环境
cd client
npm install
npm run dev
```

## 贡献指南

1. Fork 本项目
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 创建 Pull Request

## 许可证

本项目采用 MIT 许可证 - 详见 [LICENSE](LICENSE) 文件

## 联系方式

- 项目地址: [https://github.com/your-org/servify](https://github.com/your-org/servify)
- 问题反馈: [Issues](https://github.com/your-org/servify/issues)
