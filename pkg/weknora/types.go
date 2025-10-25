package weknora

import "time"

// SearchRequest 搜索请求
type SearchRequest struct {
	Query           string  `json:"query"`
	KnowledgeBaseID string  `json:"kb_id"`
	Limit           int     `json:"limit"`
	Threshold       float64 `json:"threshold"`
	Strategy        string  `json:"strategy"` // "bm25", "vector", "hybrid", "graph"
}

// SearchResponse 搜索响应
type SearchResponse struct {
	Success   bool      `json:"success"`
	Data      SearchData `json:"data"`
	Message   string    `json:"message"`
	RequestID string    `json:"request_id"`
	Duration  int64     `json:"duration_ms"`
}

type SearchData struct {
	Results   []SearchResult `json:"results"`
	Total     int            `json:"total"`
	Strategy  string         `json:"strategy"`
	QueryTime int64          `json:"query_time_ms"`
}

type SearchResult struct {
	DocumentID string                 `json:"document_id"`
	Title      string                 `json:"title"`
	Content    string                 `json:"content"`
	Score      float64                `json:"score"`
	Highlights []string               `json:"highlights"`
	Metadata   map[string]interface{} `json:"metadata"`
	ChunkIndex int                    `json:"chunk_index"`
	Source     string                 `json:"source"`
}

// Document 文档结构
type Document struct {
	Type     string                 `json:"type"`     // "text", "file", "url"
	Title    string                 `json:"title"`
	Content  string                 `json:"content"`
	URL      string                 `json:"url"`
	FilePath string                 `json:"file_path"`
	Tags     []string               `json:"tags"`
	Metadata map[string]interface{} `json:"metadata"`
}

type DocumentInfo struct {
	ID          string                 `json:"id"`
	Title       string                 `json:"title"`
	Status      string                 `json:"status"`
	ProcessedAt time.Time              `json:"processed_at"`
	ChunkCount  int                    `json:"chunk_count"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// KnowledgeBase 知识库信息
type KnowledgeBase struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Config      KBConfig  `json:"config"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Stats       KBStats   `json:"stats"`
}

type KBConfig struct {
	ChunkSize      int     `json:"chunk_size"`
	ChunkOverlap   int     `json:"chunk_overlap"`
	EmbeddingModel string  `json:"embedding_model"`
	RetrievalMode  string  `json:"retrieval_mode"`
	ScoreThreshold float64 `json:"score_threshold"`
}

type KBStats struct {
	DocumentCount int   `json:"document_count"`
	ChunkCount    int   `json:"chunk_count"`
	IndexSize     int64 `json:"index_size_bytes"`
}

// UploadRequest 文档上传请求
type UploadRequest struct {
	KnowledgeBaseID string                 `json:"kb_id"`
	Document        Document               `json:"document"`
	Options         map[string]interface{} `json:"options"`
}

// UploadResponse 文档上传响应
type UploadResponse struct {
	Success    bool      `json:"success"`
	DocumentID string    `json:"document_id"`
	Status     string    `json:"status"`
	Message    string    `json:"message"`
	RequestID  string    `json:"request_id"`
	Timestamp  time.Time `json:"timestamp"`
}

// CreateKBRequest 创建知识库请求
type CreateKBRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Config      KBConfig `json:"config"`
}

// Session 会话相关
type SessionRequest struct {
	UserID      string                 `json:"user_id"`
	SessionType string                 `json:"session_type"`
	Metadata    map[string]interface{} `json:"metadata"`
}

type Session struct {
	ID        string                 `json:"id"`
	UserID    string                 `json:"user_id"`
	Status    string                 `json:"status"`
	CreatedAt time.Time              `json:"created_at"`
	Metadata  map[string]interface{} `json:"metadata"`
}

type ChatRequest struct {
	Message     string                 `json:"message"`
	MessageType string                 `json:"message_type"`
	Metadata    map[string]interface{} `json:"metadata"`
}

type ChatResponse struct {
	Response    string         `json:"response"`
	Sources     []SearchResult `json:"sources"`
	Confidence  float64        `json:"confidence"`
	Duration    int64          `json:"duration_ms"`
	MessageID   string         `json:"message_id"`
}

// API 响应基础结构
type BaseResponse struct {
	Success   bool        `json:"success"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data"`
	RequestID string      `json:"request_id"`
	Timestamp time.Time   `json:"timestamp"`
}

// 错误响应
type ErrorResponse struct {
	Success   bool      `json:"success"`
	Error     string    `json:"error"`
	ErrorCode string    `json:"error_code"`
	RequestID string    `json:"request_id"`
	Timestamp time.Time `json:"timestamp"`
}

// 健康检查响应
type HealthResponse struct {
	Status    string            `json:"status"`
	Version   string            `json:"version"`
	Timestamp time.Time         `json:"timestamp"`
	Services  map[string]string `json:"services"`
}

// 客户端配置
type Config struct {
	BaseURL      string        `yaml:"base_url"`
	APIKey       string        `yaml:"api_key"`
	TenantID     string        `yaml:"tenant_id"`
	Timeout      time.Duration `yaml:"timeout"`
	MaxRetries   int           `yaml:"max_retries"`
	RetryDelay   time.Duration `yaml:"retry_delay"`
}

// 默认配置
func DefaultConfig() *Config {
	return &Config{
		BaseURL:    "http://localhost:9000",
		Timeout:    30 * time.Second,
		MaxRetries: 3,
		RetryDelay: 1 * time.Second,
	}
}