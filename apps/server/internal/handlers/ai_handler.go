package handlers

import (
    "context"
    "database/sql"
    "fmt"
    "net/http"
    "os"
    "path/filepath"
    "strconv"
    "strings"
    "time"
    "runtime"

    "servify/apps/server/internal/config"
    "servify/apps/server/internal/services"
    svrmetrics "servify/apps/server/internal/metrics"
    "servify/apps/server/internal/version"
    "gorm.io/gorm"

    "github.com/gin-gonic/gin"
    "github.com/sirupsen/logrus"
)

// AIHandler AI 服务处理器
type AIHandler struct {
	aiService services.AIServiceInterface
	logger    *logrus.Logger
}

// NewAIHandler 创建 AI 处理器
func NewAIHandler(aiService services.AIServiceInterface) *AIHandler {
	return &AIHandler{
		aiService: aiService,
		logger:    logrus.StandardLogger(),
	}
}

// QueryRequest 查询请求
type QueryRequest struct {
	Query     string `json:"query" binding:"required"`
	SessionID string `json:"session_id"`
}

// QueryResponse 查询响应
type QueryResponse struct {
	Success   bool        `json:"success"`
	Data      interface{} `json:"data,omitempty"`
	Error     string      `json:"error,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
	Duration  string      `json:"duration"`
}

// ProcessQuery 处理 AI 查询
func (h *AIHandler) ProcessQuery(c *gin.Context) {
	start := time.Now()

	var req QueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, QueryResponse{
			Success:   false,
			Error:     "Invalid request format: " + err.Error(),
			Timestamp: time.Now(),
			Duration:  time.Since(start).String(),
		})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// 检查是否是增强服务，优先使用增强查询
	if enhancedService, ok := h.aiService.(services.EnhancedAIServiceInterface); ok {
		response, err := enhancedService.ProcessQueryEnhanced(ctx, req.Query, req.SessionID)
		if err != nil {
			h.logger.Errorf("Enhanced AI query failed: %v", err)
			c.JSON(http.StatusInternalServerError, QueryResponse{
				Success:   false,
				Error:     "AI processing failed: " + err.Error(),
				Timestamp: time.Now(),
				Duration:  time.Since(start).String(),
			})
			return
		}

		c.JSON(http.StatusOK, QueryResponse{
			Success:   true,
			Data:      response,
			Timestamp: time.Now(),
			Duration:  time.Since(start).String(),
		})
	} else {
		// 使用标准查询
		response, err := h.aiService.ProcessQuery(ctx, req.Query, req.SessionID)
		if err != nil {
			h.logger.Errorf("AI query failed: %v", err)
			c.JSON(http.StatusInternalServerError, QueryResponse{
				Success:   false,
				Error:     "AI processing failed: " + err.Error(),
				Timestamp: time.Now(),
				Duration:  time.Since(start).String(),
			})
			return
		}

		c.JSON(http.StatusOK, QueryResponse{
			Success:   true,
			Data:      response,
			Timestamp: time.Now(),
			Duration:  time.Since(start).String(),
		})
	}
}

// GetStatus 获取 AI 服务状态
func (h *AIHandler) GetStatus(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	status := h.aiService.GetStatus(ctx)

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"data":      status,
		"timestamp": time.Now(),
	})
}

// GetMetrics 获取 AI 服务指标（仅增强服务支持）
func (h *AIHandler) GetMetrics(c *gin.Context) {
	enhancedService, ok := h.aiService.(services.EnhancedAIServiceInterface)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Metrics not available for standard AI service",
		})
		return
	}

	metrics := enhancedService.GetMetrics()

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"data":      metrics,
		"timestamp": time.Now(),
	})
}

// UploadDocumentRequest 文档上传请求
type UploadDocumentRequest struct {
	Title   string   `json:"title" binding:"required"`
	Content string   `json:"content" binding:"required"`
	Tags    []string `json:"tags"`
}

// UploadDocument 上传文档到知识库
func (h *AIHandler) UploadDocument(c *gin.Context) {
	enhancedService, ok := h.aiService.(services.EnhancedAIServiceInterface)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Document upload not available for standard AI service",
		})
		return
	}

	var req UploadDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request format: " + err.Error(),
		})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	err := enhancedService.UploadDocumentToWeKnora(ctx, req.Title, req.Content, req.Tags)
	if err != nil {
		h.logger.Errorf("Document upload failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Document upload failed: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Document uploaded successfully",
		"data": gin.H{
			"title": req.Title,
			"tags":  req.Tags,
		},
	})
}

// SyncKnowledgeBase 同步知识库
func (h *AIHandler) SyncKnowledgeBase(c *gin.Context) {
	enhancedService, ok := h.aiService.(services.EnhancedAIServiceInterface)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Knowledge base sync not available for standard AI service",
		})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	err := enhancedService.SyncKnowledgeBase(ctx)
	if err != nil {
		h.logger.Errorf("Knowledge base sync failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Knowledge base sync failed: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Knowledge base synchronized successfully",
	})
}

// EnableWeKnora 启用 WeKnora
func (h *AIHandler) EnableWeKnora(c *gin.Context) {
	enhancedService, ok := h.aiService.(services.EnhancedAIServiceInterface)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "WeKnora control not available for standard AI service",
		})
		return
	}

	enhancedService.SetWeKnoraEnabled(true)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "WeKnora enabled",
	})
}

// DisableWeKnora 禁用 WeKnora
func (h *AIHandler) DisableWeKnora(c *gin.Context) {
	enhancedService, ok := h.aiService.(services.EnhancedAIServiceInterface)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "WeKnora control not available for standard AI service",
		})
		return
	}

	enhancedService.SetWeKnoraEnabled(false)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "WeKnora disabled",
	})
}

// ResetCircuitBreaker 重置熔断器
func (h *AIHandler) ResetCircuitBreaker(c *gin.Context) {
	enhancedService, ok := h.aiService.(services.EnhancedAIServiceInterface)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Circuit breaker control not available for standard AI service",
		})
		return
	}

	enhancedService.ResetCircuitBreaker()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Circuit breaker reset",
	})
}

// MetricsHandler 指标处理器
type MetricsHandler struct{
    wsHub          *services.WebSocketHub
    webrtcService  *services.WebRTCService
    aiService      services.AIServiceInterface
    messageRouter  *services.MessageRouter
    startedAt      time.Time
    db             *gorm.DB
}

// NewMetricsHandler 创建指标处理器
func NewMetricsHandler(wsHub *services.WebSocketHub, webrtc *services.WebRTCService, ai services.AIServiceInterface, router *services.MessageRouter, db *gorm.DB) *MetricsHandler {
    return &MetricsHandler{ wsHub: wsHub, webrtcService: webrtc, aiService: ai, messageRouter: router, startedAt: time.Now(), db: db }
}

// GetMetrics 获取系统指标（Prometheus 格式）
func (h *MetricsHandler) GetMetrics(c *gin.Context) {
    c.Header("Content-Type", "text/plain")

    // 采样运行态
    uptime := time.Since(h.startedAt).Seconds()
    wsClients := 0
    webrtcConns := 0
    if h.wsHub != nil { wsClients = h.wsHub.GetClientCount() }
    if h.webrtcService != nil { webrtcConns = h.webrtcService.GetConnectionCount() }

    var aiQueries, aiWeKnora, aiFallback int64
    var aiAvgLatency float64
    if enh, ok := h.aiService.(services.EnhancedAIServiceInterface); ok && enh.GetMetrics() != nil {
        m := enh.GetMetrics()
        aiQueries = m.QueryCount
        aiWeKnora = m.WeKnoraUsageCount
        aiFallback = m.FallbackUsageCount
        aiAvgLatency = m.AverageLatency.Seconds()
    }

    // Prometheus exposition format
    b := &strings.Builder{}
    fmt.Fprintf(b, "# HELP servify_info Information about the Servify instance\n")
    fmt.Fprintf(b, "# TYPE servify_info gauge\n")
    // include labels for version/commit/build_time
    v := strings.ReplaceAll(version.Version, "\"", "\\\"")
    cmt := strings.ReplaceAll(version.Commit, "\"", "\\\"")
    bt := strings.ReplaceAll(version.BuildTime, "\"", "\\\"")
    fmt.Fprintf(b, "servify_info{version=\"%s\",commit=\"%s\",build_time=\"%s\"} 1\n\n", v, cmt, bt)

    fmt.Fprintf(b, "# HELP servify_uptime_seconds Total uptime of the Servify instance in seconds\n")
    fmt.Fprintf(b, "# TYPE servify_uptime_seconds counter\n")
    fmt.Fprintf(b, "servify_uptime_seconds %.0f\n\n", uptime)

    fmt.Fprintf(b, "# HELP servify_websocket_active_connections Active WebSocket connections\n")
    fmt.Fprintf(b, "# TYPE servify_websocket_active_connections gauge\n")
    fmt.Fprintf(b, "servify_websocket_active_connections %d\n\n", wsClients)

    fmt.Fprintf(b, "# HELP servify_webrtc_connections Active WebRTC peer connections\n")
    fmt.Fprintf(b, "# TYPE servify_webrtc_connections gauge\n")
    fmt.Fprintf(b, "servify_webrtc_connections %d\n\n", webrtcConns)

    fmt.Fprintf(b, "# HELP servify_ai_requests_total Total AI queries processed\n")
    fmt.Fprintf(b, "# TYPE servify_ai_requests_total counter\n")
    fmt.Fprintf(b, "servify_ai_requests_total %d\n\n", aiQueries)

    fmt.Fprintf(b, "# HELP servify_ai_weknora_usage_total Total AI queries served via WeKnora\n")
    fmt.Fprintf(b, "# TYPE servify_ai_weknora_usage_total counter\n")
    fmt.Fprintf(b, "servify_ai_weknora_usage_total %d\n\n", aiWeKnora)

    fmt.Fprintf(b, "# HELP servify_ai_fallback_usage_total Total AI queries served via fallback KB\n")
    fmt.Fprintf(b, "# TYPE servify_ai_fallback_usage_total counter\n")
    fmt.Fprintf(b, "servify_ai_fallback_usage_total %d\n\n", aiFallback)

    fmt.Fprintf(b, "# HELP servify_ai_avg_latency_seconds Average AI processing latency seconds\n")
    fmt.Fprintf(b, "# TYPE servify_ai_avg_latency_seconds gauge\n")
    fmt.Fprintf(b, "servify_ai_avg_latency_seconds %.3f\n\n", aiAvgLatency)

    // Go runtime minimal metrics
    fmt.Fprintf(b, "# HELP servify_go_goroutines Number of goroutines\n")
    fmt.Fprintf(b, "# TYPE servify_go_goroutines gauge\n")
    fmt.Fprintf(b, "servify_go_goroutines %d\n\n", runtime.NumGoroutine())

    var ms runtime.MemStats
    runtime.ReadMemStats(&ms)
    fmt.Fprintf(b, "# HELP servify_go_mem_alloc_bytes Bytes of allocated heap objects\n")
    fmt.Fprintf(b, "# TYPE servify_go_mem_alloc_bytes gauge\n")
    fmt.Fprintf(b, "servify_go_mem_alloc_bytes %d\n", ms.Alloc)

    // Database/sql stats (if available)
    if h.db != nil {
        var sqlDB *sql.DB
        if s, err := h.db.DB(); err == nil {
            sqlDB = s
        }
        if sqlDB != nil {
            ds := sqlDB.Stats()
            fmt.Fprintf(b, "\n# HELP servify_db_max_open_connections Maximum number of open connections to the database\n")
            fmt.Fprintf(b, "# TYPE servify_db_max_open_connections gauge\n")
            fmt.Fprintf(b, "servify_db_max_open_connections %d\n", ds.MaxOpenConnections)

            fmt.Fprintf(b, "# HELP servify_db_open_connections The number of established connections both in use and idle\n")
            fmt.Fprintf(b, "# TYPE servify_db_open_connections gauge\n")
            fmt.Fprintf(b, "servify_db_open_connections %d\n", ds.OpenConnections)

            fmt.Fprintf(b, "# HELP servify_db_inuse_connections The number of connections currently in use\n")
            fmt.Fprintf(b, "# TYPE servify_db_inuse_connections gauge\n")
            fmt.Fprintf(b, "servify_db_inuse_connections %d\n", ds.InUse)

            fmt.Fprintf(b, "# HELP servify_db_idle_connections The number of idle connections\n")
            fmt.Fprintf(b, "# TYPE servify_db_idle_connections gauge\n")
            fmt.Fprintf(b, "servify_db_idle_connections %d\n", ds.Idle)

            fmt.Fprintf(b, "# HELP servify_db_wait_count The total number of connections waited for\n")
            fmt.Fprintf(b, "# TYPE servify_db_wait_count counter\n")
            fmt.Fprintf(b, "servify_db_wait_count %d\n", ds.WaitCount)

            fmt.Fprintf(b, "# HELP servify_db_wait_duration_seconds The total time blocked waiting for a new connection\n")
            fmt.Fprintf(b, "# TYPE servify_db_wait_duration_seconds counter\n")
            fmt.Fprintf(b, "servify_db_wait_duration_seconds %.6f\n", ds.WaitDuration.Seconds())

            fmt.Fprintf(b, "# HELP servify_db_max_idle_closed_total The total number of connections closed due to SetMaxIdleConns\n")
            fmt.Fprintf(b, "# TYPE servify_db_max_idle_closed_total counter\n")
            fmt.Fprintf(b, "servify_db_max_idle_closed_total %d\n", ds.MaxIdleClosed)

            fmt.Fprintf(b, "# HELP servify_db_max_lifetime_closed_total The total number of connections closed due to SetConnMaxLifetime\n")
            fmt.Fprintf(b, "# TYPE servify_db_max_lifetime_closed_total counter\n")
            fmt.Fprintf(b, "servify_db_max_lifetime_closed_total %d\n", ds.MaxLifetimeClosed)
        }
    }

    // Rate limit drops (by prefix)
    totalDrops, byPrefix := svrmetrics.RateLimitSnapshot()
    fmt.Fprintf(b, "\n# HELP servify_ratelimit_dropped_total Total HTTP 429 responses due to rate limiting\n")
    fmt.Fprintf(b, "# TYPE servify_ratelimit_dropped_total counter\n")
    if len(byPrefix) == 0 {
        fmt.Fprintf(b, "servify_ratelimit_dropped_total{prefix=\"global\"} %d\n", 0)
    } else {
        for p, v := range byPrefix {
            label := strings.ReplaceAll(p, "\"", "\\\"")
            fmt.Fprintf(b, "servify_ratelimit_dropped_total{prefix=\"%s\"} %d\n", label, v)
        }
    }
    // Optional sum without labels
    fmt.Fprintf(b, "servify_ratelimit_dropped_sum %d\n", totalDrops)

    c.String(http.StatusOK, b.String())
}

// UploadHandler 文件上传处理器
type UploadHandler struct {
    config    *config.Config
    aiService services.AIServiceInterface
    logger    *logrus.Logger
}

// NewUploadHandler 创建文件上传处理器
func NewUploadHandler(cfg *config.Config, aiService services.AIServiceInterface) *UploadHandler {
    return &UploadHandler{
        config:    cfg,
        aiService: aiService,
        logger:    logrus.StandardLogger(),
    }
}

// UploadFile 处理文件上传
func (h *UploadHandler) UploadFile(c *gin.Context) {
    // 实现文件上传逻辑

    // 1. 验证文件类型和大小
    file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "No file provided: " + err.Error(),
		})
		return
	}
	defer file.Close()

    // 解析并验证文件大小（配置为字符串，如 "10MB"）
    maxSizeBytes, parseErr := parseSizeToBytes(h.config.Upload.MaxFileSize)
    if parseErr != nil {
        // 配置错误不应导致请求失败，记录警告并跳过大小校验
        h.logger.Warnf("Invalid max file size config '%s': %v", h.config.Upload.MaxFileSize, parseErr)
    }

    if maxSizeBytes > 0 && header.Size > maxSizeBytes {
        c.JSON(http.StatusBadRequest, gin.H{
            "success": false,
            "error":   fmt.Sprintf("File too large: %d bytes (max: %d)", header.Size, maxSizeBytes),
        })
        return
    }

    // 验证文件类型（同时支持 MIME 与扩展名配置）
    if len(h.config.Upload.AllowedTypes) > 0 {
        mimeType := header.Header.Get("Content-Type")
        filename := header.Filename
        if !isAllowedType(filename, mimeType, h.config.Upload.AllowedTypes) {
            c.JSON(http.StatusBadRequest, gin.H{
                "success": false,
                "error":   fmt.Sprintf("File type not allowed: %s (%s)", mimeType, filename),
            })
            return
        }
    }

	// 2. 保存文件到指定目录
	filename := fmt.Sprintf("%d_%s", time.Now().Unix(), header.Filename)
    dstPath := fmt.Sprintf("%s/%s", h.config.Upload.StoragePath, filename)

    if err := c.SaveUploadedFile(header, dstPath); err != nil {
        h.logger.Errorf("Failed to save file: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{
            "success": false,
            "error":   "Failed to save file: " + err.Error(),
        })
        return
    }

    // 3. 如果启用自动处理，提取文本内容（对纯文本/Markdown进行快速提取，其他格式占位）
    var extractedText string
    if h.config.Upload.AutoProcess {
        if err := func() error {
            // 仅对文本类文件尝试读取，避免引入额外依赖
            switch {
            case isExt(filename, ".txt", ".md", ".log") || strings.HasPrefix(header.Header.Get("Content-Type"), "text/"):
                b, readErr := os.ReadFile(filepath.Clean(dstPath))
                if readErr != nil {
                    return readErr
                }
                // 限制读取长度，避免大文件撑爆响应
                const maxPreview = 100_000 // 100KB 预览
                if len(b) > maxPreview {
                    b = b[:maxPreview]
                }
                extractedText = string(b)
            default:
                extractedText = "(extraction not implemented for this file type)"
            }
            return nil
        }(); err != nil {
            h.logger.Warnf("Failed to extract text from '%s': %v", header.Filename, err)
            extractedText = "(failed to extract text)"
        }
    }

	// 4. 如果启用自动索引，上传到 WeKnora
	if h.config.Upload.AutoIndex {
		if enhancedService, ok := h.aiService.(services.EnhancedAIServiceInterface); ok {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			err := enhancedService.UploadDocumentToWeKnora(ctx, header.Filename, extractedText, []string{"uploaded_file"})
			if err != nil {
				h.logger.Warnf("Failed to index file in WeKnora: %v", err)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "File uploaded successfully",
		"data": gin.H{
			"filename":        filename,
			"original_name":   header.Filename,
			"size":           header.Size,
			"extracted_text": extractedText,
			"auto_indexed":   h.config.Upload.AutoIndex,
		},
	})
}

// parseSizeToBytes 将形如 "10MB", "512KB", "1048576" 的配置解析为字节数
func parseSizeToBytes(s string) (int64, error) {
    s = strings.TrimSpace(strings.ToUpper(s))
    if s == "" {
        return 0, nil
    }
    // 纯数字直接解析为字节
    if n, err := strconv.ParseInt(s, 10, 64); err == nil {
        return n, nil
    }
    // 含单位
    units := []struct {
        suffix string
        mul    int64
    }{
        {"KB", 1024},
        {"MB", 1024 * 1024},
        {"GB", 1024 * 1024 * 1024},
    }
    for _, u := range units {
        if strings.HasSuffix(s, u.suffix) {
            val := strings.TrimSuffix(s, u.suffix)
            n, err := strconv.ParseFloat(strings.TrimSpace(val), 64)
            if err != nil {
                return 0, err
            }
            return int64(n * float64(u.mul)), nil
        }
    }
    return 0, fmt.Errorf("unknown size format: %s", s)
}

// isAllowedType 判断给定文件是否满足允许类型（支持 MIME 与扩展名）
func isAllowedType(filename, mimeType string, allowed []string) bool {
    // 允许类型若包含通配符 "*" 或 "*/*"，直接通过
    for _, t := range allowed {
        if t == "*" || t == "*/*" {
            return true
        }
    }
    ext := strings.ToLower(filepath.Ext(filename))
    for _, t := range allowed {
        t = strings.ToLower(strings.TrimSpace(t))
        if strings.HasPrefix(t, ".") {
            if ext == t {
                return true
            }
        } else {
            // 作为 MIME 类型匹配（支持前缀，如 "image/"）
            if mimeType == t || (strings.HasSuffix(t, "/*") && strings.HasPrefix(mimeType, strings.TrimSuffix(t, "*"))) {
                return true
            }
        }
    }
    return false
}

// isExt 判断文件扩展名是否在给定列表中
func isExt(filename string, exts ...string) bool {
    e := strings.ToLower(filepath.Ext(filename))
    for _, x := range exts {
        if e == strings.ToLower(x) {
            return true
        }
    }
    return false
}

// GetUploadStatus 获取上传状态
func (h *UploadHandler) GetUploadStatus(c *gin.Context) {
	uploadID := c.Param("id")

	// 实现上传状态查询
	// 这里应该查询数据库或缓存来获取上传状态

	// 当前简化实现：返回模拟状态
	if uploadID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Upload ID is required",
		})
		return
	}

	// 模拟状态查询
	status := map[string]interface{}{
		"upload_id": uploadID,
		"status":    "completed", // pending, processing, completed, failed
		"progress":  100,
		"message":   "File uploaded and processed successfully",
		"created_at": time.Now().Add(-5 * time.Minute),
		"completed_at": time.Now(),
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    status,
	})
}
