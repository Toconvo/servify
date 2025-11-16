package services

import (
	"context"
	"servify/apps/server/internal/models"
)

// AIServiceInterface 定义 AI 服务的接口
type AIServiceInterface interface {
	// 基础查询处理
	ProcessQuery(ctx context.Context, query string, sessionID string) (*AIResponse, error)

	// 转人工判断
	ShouldTransferToHuman(query string, sessionHistory []models.Message) bool

	// 会话摘要
	GetSessionSummary(messages []models.Message) (string, error)

	// 知识库初始化
	InitializeKnowledgeBase()

	// 获取服务状态（增强版本专用）
	GetStatus(ctx context.Context) map[string]interface{}
}

// EnhancedAIServiceInterface 增强 AI 服务接口（扩展功能）
type EnhancedAIServiceInterface interface {
	AIServiceInterface

	// 增强查询处理
	ProcessQueryEnhanced(ctx context.Context, query string, sessionID string) (*EnhancedAIResponse, error)

	// WeKnora 文档上传
	UploadDocumentToWeKnora(ctx context.Context, title, content string, tags []string) error

	// 获取服务指标
	GetMetrics() *AIMetrics

	// 控制开关
	SetWeKnoraEnabled(enabled bool)
	SetFallbackEnabled(enabled bool)
	ResetCircuitBreaker()

	// 知识库同步
	SyncKnowledgeBase(ctx context.Context) error
}

// 确保增强 AI 服务实现了接口
var _ EnhancedAIServiceInterface = (*EnhancedAIService)(nil)

// 为原始 AIService 实现基础接口
func (s *AIService) GetStatus(ctx context.Context) map[string]interface{} {
	return map[string]interface{}{
		"type":             "standard",
		"openai_enabled":   s.openAIAPIKey != "",
		"knowledge_base":   "legacy",
		"document_count":   len(s.knowledgeBase.documents),
	}
}

// 确保原始 AI 服务也实现了接口
var _ AIServiceInterface = (*AIService)(nil)
