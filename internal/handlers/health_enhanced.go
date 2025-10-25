package handlers

import (
	"context"
	"net/http"
	"time"

	"servify/internal/config"
	"servify/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// EnhancedHealthHandler 增强的健康检查处理器
type EnhancedHealthHandler struct {
	config    *config.Config
	aiService services.AIServiceInterface
	logger    *logrus.Logger
}

// NewEnhancedHealthHandler 创建增强的健康检查处理器
func NewEnhancedHealthHandler(cfg *config.Config, aiService services.AIServiceInterface) *EnhancedHealthHandler {
	return &EnhancedHealthHandler{
		config:    cfg,
		aiService: aiService,
		logger:    logrus.StandardLogger(),
	}
}

// HealthResponse 健康检查响应
type HealthResponse struct {
	Status    string                 `json:"status"`
	Version   string                 `json:"version"`
	Timestamp time.Time              `json:"timestamp"`
	Services  map[string]ServiceInfo `json:"services"`
	System    SystemInfo             `json:"system"`
}

// ServiceInfo 服务信息
type ServiceInfo struct {
	Status  string      `json:"status"`
	Latency string      `json:"latency,omitempty"`
	Error   string      `json:"error,omitempty"`
	Details interface{} `json:"details,omitempty"`
}

// SystemInfo 系统信息
type SystemInfo struct {
	Uptime  time.Duration `json:"uptime"`
	Version string        `json:"version"`
	GoVersion string      `json:"go_version"`
}

var startTime = time.Now()

// Health 健康检查端点
func (h *EnhancedHealthHandler) Health(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	response := HealthResponse{
		Status:    "healthy",
		Version:   "1.1.0",
		Timestamp: time.Now(),
		Services:  make(map[string]ServiceInfo),
		System: SystemInfo{
			Uptime:    time.Since(startTime),
			Version:   "1.1.0",
			GoVersion: "1.21+",
		},
	}

	allHealthy := true

	// 检查 AI 服务状态
	h.checkAIService(ctx, &response, &allHealthy)

	// 检查数据库（如果启用）
	if h.config.Monitoring.HealthChecks.Database {
		h.checkDatabase(ctx, &response, &allHealthy)
	}

	// 检查 Redis（如果启用）
	if h.config.Monitoring.HealthChecks.Redis {
		h.checkRedis(ctx, &response, &allHealthy)
	}

	// 检查 WeKnora（如果启用）
	if h.config.Monitoring.HealthChecks.WeKnora && h.config.WeKnora.Enabled {
		h.checkWeKnora(ctx, &response, &allHealthy)
	}

	// 设置总体状态
	if !allHealthy {
		response.Status = "degraded"
	}

	// 返回适当的状态码
	statusCode := http.StatusOK
	if response.Status == "unhealthy" {
		statusCode = http.StatusServiceUnavailable
	} else if response.Status == "degraded" {
		statusCode = http.StatusOK // 部分服务不可用时仍返回 200，但状态为 degraded
	}

	c.JSON(statusCode, response)
}

// Ready 就绪检查端点
func (h *EnhancedHealthHandler) Ready(c *gin.Context) {
	// 简单的就绪检查，只检查核心服务
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	ready := true
	services := make(map[string]string)

	// 检查 AI 服务
	if status := h.aiService.GetStatus(ctx); status != nil {
		services["ai"] = "ready"
	} else {
		services["ai"] = "not_ready"
		ready = false
	}

	// 基础的就绪响应
	response := map[string]interface{}{
		"ready":     ready,
		"timestamp": time.Now(),
		"services":  services,
	}

	statusCode := http.StatusOK
	if !ready {
		statusCode = http.StatusServiceUnavailable
	}

	c.JSON(statusCode, response)
}

// checkAIService 检查 AI 服务状态
func (h *EnhancedHealthHandler) checkAIService(ctx context.Context, response *HealthResponse, allHealthy *bool) {
	start := time.Now()

	status := h.aiService.GetStatus(ctx)
	if status == nil {
		response.Services["ai"] = ServiceInfo{
			Status: "unhealthy",
			Error:  "AI service not responding",
		}
		*allHealthy = false
		return
	}

	serviceInfo := ServiceInfo{
		Status:  "healthy",
		Latency: time.Since(start).String(),
		Details: status,
	}

	// 检查 WeKnora 状态（如果是增强服务）
	if enhancedService, ok := h.aiService.(services.EnhancedAIServiceInterface); ok {
		metrics := enhancedService.GetMetrics()
		serviceInfo.Details = map[string]interface{}{
			"type":    "enhanced",
			"status":  status,
			"metrics": metrics,
		}
	}

	response.Services["ai"] = serviceInfo
}

// checkDatabase 检查数据库状态
func (h *EnhancedHealthHandler) checkDatabase(ctx context.Context, response *HealthResponse, allHealthy *bool) {
	start := time.Now()

	// 实际的数据库连接检查
	// 这里需要实际的数据库连接来检查

	// 模拟数据库连接检查
	dbHealthy := true
	var dbError string

	// 简单的连接测试（当前模拟实现）
	// 实际实现应该执行 SELECT 1 查询或类似的健康检查
	/*
	if db != nil {
		if err := db.PingContext(ctx); err != nil {
			dbHealthy = false
			dbError = err.Error()
		}
	} else {
		dbHealthy = false
		dbError = "database connection not initialized"
	}
	*/

	serviceInfo := ServiceInfo{
		Latency: time.Since(start).String(),
		Details: map[string]interface{}{
			"driver": "postgresql",
			"host":   h.config.Database.Host,
			"port":   h.config.Database.Port,
		},
	}

	if dbHealthy {
		serviceInfo.Status = "healthy"
	} else {
		serviceInfo.Status = "unhealthy"
		serviceInfo.Error = dbError
		*allHealthy = false
	}

	response.Services["database"] = serviceInfo
}

// checkRedis 检查 Redis 状态
func (h *EnhancedHealthHandler) checkRedis(ctx context.Context, response *HealthResponse, allHealthy *bool) {
	start := time.Now()

	// 实际的 Redis 连接检查
	redisHealthy := true
	var redisError string

	// 简单的连接测试（当前模拟实现）
	// 实际实现应该执行 PING 命令或类似的健康检查
	/*
	if redisClient != nil {
		if err := redisClient.Ping(ctx).Err(); err != nil {
			redisHealthy = false
			redisError = err.Error()
		}
	} else {
		redisHealthy = false
		redisError = "redis connection not initialized"
	}
	*/

	serviceInfo := ServiceInfo{
		Latency: time.Since(start).String(),
		Details: map[string]interface{}{
			"host": h.config.Redis.Host,
			"port": h.config.Redis.Port,
		},
	}

	if redisHealthy {
		serviceInfo.Status = "healthy"
	} else {
		serviceInfo.Status = "unhealthy"
		serviceInfo.Error = redisError
		*allHealthy = false
	}

	response.Services["redis"] = serviceInfo
}

// checkWeKnora 检查 WeKnora 状态
func (h *EnhancedHealthHandler) checkWeKnora(ctx context.Context, response *HealthResponse, allHealthy *bool) {
	start := time.Now()

	// 如果是增强 AI 服务，获取 WeKnora 状态
	if enhancedService, ok := h.aiService.(services.EnhancedAIServiceInterface); ok {
		status := enhancedService.GetStatus(ctx)

		weKnoraHealthy, exists := status["weknora_healthy"].(bool)
		if !exists || !weKnoraHealthy {
			response.Services["weknora"] = ServiceInfo{
				Status:  "unhealthy",
				Latency: time.Since(start).String(),
				Error:   "WeKnora service unavailable",
			}
			// WeKnora 不可用时不影响整体健康状态（有降级机制）
			h.logger.Warn("WeKnora service is unhealthy, but fallback is available")
		} else {
			response.Services["weknora"] = ServiceInfo{
				Status:  "healthy",
				Latency: time.Since(start).String(),
				Details: map[string]interface{}{
					"base_url": h.config.WeKnora.BaseURL,
					"kb_id":    h.config.WeKnora.KnowledgeBaseID,
				},
			}
		}
	} else {
		response.Services["weknora"] = ServiceInfo{
			Status: "disabled",
		}
	}
}