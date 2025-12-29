//go:build weknora
// +build weknora

package cli

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	gormtracing "gorm.io/plugin/opentelemetry/tracing"
	"servify/apps/server/internal/config"
	"servify/apps/server/internal/handlers"
	"servify/apps/server/internal/observability"
	"servify/apps/server/internal/services"
	"servify/apps/server/pkg/weknora"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the servify application with WeKnora integration",
	Long:  `Run the servify application with enhanced AI capabilities powered by WeKnora`,
	Run:   run,
}

func init() {
	rootCmd.AddCommand(runCmd)
}

func run(cmd *cobra.Command, args []string) {
	// 加载配置
	cfg := config.Load()

	// 初始化日志系统
	if err := config.InitLogger(cfg); err != nil {
		logrus.Fatalf("Failed to initialize logger: %v", err)
	}

	// OpenTelemetry 初始化（可选）
	if shutdown, err := observability.SetupTracing(context.Background(), cfg); err == nil {
		defer func() { _ = shutdown(context.Background()) }()
	} else {
		logrus.Warnf("init tracing: %v", err)
	}

	logrus.Info("🚀 Starting Servify with WeKnora integration...")

	// 初始化数据库
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=disable TimeZone=UTC", cfg.Database.Host, cfg.Database.User, cfg.Database.Password, cfg.Database.Name, cfg.Database.Port)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Warn)})
	if err != nil {
		logrus.Warnf("DB connect failed, message persistence disabled: %v", err)
	}
	if db != nil && cfg.Monitoring.Tracing.Enabled {
		_ = db.Use(gormtracing.NewPlugin())
	}

	// 初始化基础服务
	wsHub := services.NewWebSocketHub()
	if db != nil {
		wsHub.SetDB(db)
	}
	webrtcService := services.NewWebRTCService(cfg.WebRTC.STUNServer, wsHub)

	// 初始化 WeKnora 客户端
	var weKnoraClient weknora.WeKnoraInterface
	if cfg.WeKnora.Enabled {
		logrus.Info("📚 Initializing WeKnora client...")
		weKnoraConfig := &weknora.Config{
			BaseURL:    cfg.WeKnora.BaseURL,
			APIKey:     cfg.WeKnora.APIKey,
			TenantID:   cfg.WeKnora.TenantID,
			Timeout:    cfg.WeKnora.Timeout,
			MaxRetries: cfg.WeKnora.MaxRetries,
		}
		weKnoraClient = weknora.NewClient(weKnoraConfig, logrus.StandardLogger())

		// 测试 WeKnora 连接
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := weKnoraClient.HealthCheck(ctx); err != nil {
			logrus.Warnf("⚠️  WeKnora health check failed: %v", err)
			if !cfg.Fallback.Enabled {
				logrus.Fatalf("❌ WeKnora is required but unavailable, and fallback is disabled")
			}
			logrus.Warn("🔄 WeKnora unavailable, will use fallback mode")
		} else {
			logrus.Info("✅ WeKnora client initialized successfully")
		}
	} else {
		logrus.Info("📚 WeKnora integration disabled, using legacy knowledge base")
	}

	// 初始化 AI 服务
	logrus.Info("🤖 Initializing AI services...")
	originalAIService := services.NewAIService(cfg.AI.OpenAI.APIKey, cfg.AI.OpenAI.BaseURL)
	originalAIService.InitializeKnowledgeBase()

	// 创建增强的 AI 服务
	var aiService services.AIServiceInterface
	if cfg.WeKnora.Enabled && weKnoraClient != nil {
		enhancedAIService := services.NewEnhancedAIService(
			originalAIService,
			weKnoraClient,
			cfg.WeKnora.KnowledgeBaseID,
			logrus.StandardLogger(),
		)

		// 同步知识库（如果配置了自动同步）
		if cfg.Upload.AutoIndex {
			logrus.Info("🔄 Syncing knowledge base to WeKnora...")
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			if err := enhancedAIService.SyncKnowledgeBase(ctx); err != nil {
				logrus.Warnf("⚠️  Knowledge base sync failed: %v", err)
			} else {
				logrus.Info("✅ Knowledge base synced successfully")
			}
		}

		aiService = enhancedAIService
		logrus.Info("✅ Enhanced AI service with WeKnora initialized")
	} else {
		aiService = originalAIService
		logrus.Info("✅ Standard AI service initialized")
	}

	// 初始化消息路由
	messageRouter := services.NewMessageRouter(aiService, wsHub, db)

	// 启动后台服务
	logrus.Info("🔌 Starting background services...")
	go wsHub.Run()

	// 将 AI 服务注入 WebSocketHub 以便直接处理文本消息
	wsHub.SetAIService(aiService)

	// 启动消息路由
	if err := messageRouter.Start(); err != nil {
		logrus.Fatalf("❌ Failed to start message router: %v", err)
	}

	// 设置 Gin 模式
	if cfg.Log.Level == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// 创建路由
	router := setupEnhancedRouter(cfg, wsHub, webrtcService, messageRouter, aiService)

	// 创建服务器
	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// 启动服务器
	go func() {
		logrus.Infof("🌐 Server starting on %s:%d", cfg.Server.Host, cfg.Server.Port)
		logrus.Infof("📍 Web UI: http://%s:%d", cfg.Server.Host, cfg.Server.Port)
		logrus.Infof("🔗 API Base: http://%s:%d/api/v1", cfg.Server.Host, cfg.Server.Port)
		logrus.Infof("🔌 WebSocket: ws://%s:%d/api/v1/ws", cfg.Server.Host, cfg.Server.Port)

		if cfg.WeKnora.Enabled {
			logrus.Infof("📚 WeKnora: %s", cfg.WeKnora.BaseURL)
		}

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logrus.Fatalf("❌ Server failed to start: %v", err)
		}
	}()

	// 启动健康检查（如果启用）
	if cfg.Monitoring.Enabled {
		go startHealthMonitoring(cfg, weKnoraClient)
	}

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logrus.Info("🛑 Shutting down server...")

	// 优雅关闭
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 停止消息路由
	if err := messageRouter.Stop(); err != nil {
		logrus.Errorf("❌ Failed to stop message router: %v", err)
	}

	// 关闭服务器
	if err := server.Shutdown(ctx); err != nil {
		logrus.Errorf("❌ Server forced to shutdown: %v", err)
	}

	logrus.Info("✅ Server shutdown complete")
}

func setupEnhancedRouter(
	cfg *config.Config,
	wsHub *services.WebSocketHub,
	webrtcService *services.WebRTCService,
	messageRouter *services.MessageRouter,
	aiService services.AIServiceInterface,
) *gin.Engine {
	router := gin.New()

	// 中间件
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.Use(enhancedCorsMiddleware(cfg))
	if cfg.Monitoring.Tracing.Enabled {
		router.Use(otelgin.Middleware(cfg.Monitoring.Tracing.ServiceName))
	}

	// 速率限制中间件（如果启用）
	if cfg.Security.RateLimiting.Enabled {
		router.Use(rateLimitMiddleware(cfg))
		logrus.Info("🔒 Rate limiting enabled")
	}

	// 根路径重定向到静态文件
	router.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/index.html")
	})

	// 健康检查
	healthHandler := handlers.NewEnhancedHealthHandler(cfg, aiService)
	router.GET("/health", healthHandler.Health)
	router.GET("/ready", healthHandler.Ready)

	// 监控端点
	if cfg.Monitoring.Enabled {
		router.GET(cfg.Monitoring.MetricsPath, handlers.NewMetricsHandler(wsHub, webrtcService, aiService, messageRouter, db).GetMetrics)
	}

	// API 路由组
	api := router.Group("/api/v1")
	{
		// WebSocket 连接
		wsHandler := handlers.NewWebSocketHandler(wsHub)
		api.GET("/ws", wsHandler.HandleWebSocket)
		api.GET("/ws/stats", wsHandler.GetStats)

		// WebRTC 相关
		webrtcHandler := handlers.NewWebRTCHandler(webrtcService)
		api.GET("/webrtc/stats", webrtcHandler.GetStats)
		api.GET("/webrtc/connections", webrtcHandler.GetConnections)

		// 消息路由
		messageHandler := handlers.NewMessageHandler(messageRouter)
		api.GET("/messages/platforms", messageHandler.GetPlatformStats)

		// AI 相关 API
		aiHandler := handlers.NewAIHandler(aiService)
		aiAPI := api.Group("/ai")
		{
			aiAPI.POST("/query", aiHandler.ProcessQuery)
			aiAPI.GET("/status", aiHandler.GetStatus)
			aiAPI.GET("/metrics", aiHandler.GetMetrics)

			// WeKnora 特定功能
			if cfg.WeKnora.Enabled {
				aiAPI.POST("/knowledge/upload", aiHandler.UploadDocument)
				aiAPI.POST("/knowledge/sync", aiHandler.SyncKnowledgeBase)
				aiAPI.PUT("/weknora/enable", aiHandler.EnableWeKnora)
				aiAPI.PUT("/weknora/disable", aiHandler.DisableWeKnora)
				aiAPI.POST("/circuit-breaker/reset", aiHandler.ResetCircuitBreaker)
			}
		}

		// 轻量指标上报（客户端/前端）
		ingest := handlers.NewMetricsIngestHandler(handlers.NewMetricsAggregator())
		api.POST("/metrics/ingest", ingest.Ingest)

		// 文件上传 API（如果启用）必须放在相同作用域下，复用 api 组
		if cfg.Upload.Enabled {
			uploadHandler := handlers.NewUploadHandler(cfg, aiService)
			api.POST("/upload", uploadHandler.UploadFile)
			api.GET("/upload/status/:id", uploadHandler.GetUploadStatus)
		}
	}

	// 静态文件服务
	router.Static("/static", "./static")
	router.Static("/uploads", cfg.Upload.StoragePath)
	router.Static("/", "./web") // 服务官网静态文件

	return router
}

func enhancedCorsMiddleware(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 动态配置 CORS
		if cfg.Security.CORS.Enabled {
			origins := "*"
			if len(cfg.Security.CORS.AllowedOrigins) > 0 && cfg.Security.CORS.AllowedOrigins[0] != "*" {
				// 在生产环境中应该验证 Origin
				origins = cfg.Security.CORS.AllowedOrigins[0]
			}

			c.Header("Access-Control-Allow-Origin", origins)
			c.Header("Access-Control-Allow-Credentials", "true")

			allowedHeaders := "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With"
			if len(cfg.Security.CORS.AllowedHeaders) > 0 {
				allowedHeaders = cfg.Security.CORS.AllowedHeaders[0]
			}
			c.Header("Access-Control-Allow-Headers", allowedHeaders)

			allowedMethods := "POST, OPTIONS, GET, PUT, DELETE"
			if len(cfg.Security.CORS.AllowedMethods) > 0 {
				allowedMethods = cfg.Security.CORS.AllowedMethods[0]
			}
			c.Header("Access-Control-Allow-Methods", allowedMethods)
		}

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// startHealthMonitoring 启动健康监控
func startHealthMonitoring(cfg *config.Config, weKnoraClient weknora.WeKnoraInterface) {
	ticker := time.NewTicker(cfg.WeKnora.HealthCheck.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// 检查 WeKnora 健康状态
			if cfg.WeKnora.Enabled && weKnoraClient != nil {
				ctx, cancel := context.WithTimeout(context.Background(), cfg.WeKnora.HealthCheck.Timeout)
				err := weKnoraClient.HealthCheck(ctx)
				cancel()

				if err != nil {
					logrus.Warnf("⚠️  WeKnora health check failed: %v", err)
				} else {
					logrus.Debug("✅ WeKnora health check passed")
				}
			}
		}
	}
}

// rateLimitMiddleware 速率限制中间件
func rateLimitMiddleware(cfg *config.Config) gin.HandlerFunc {
	// 令牌桶实现：
	// - 速率：RequestsPerMinute / 60 tokens/sec
	// - 桶容量：Burst（若 Burst 未配置则退化为 RequestsPerMinute）

	type bucket struct {
		tokens     float64
		lastRefill time.Time
		mutex      sync.Mutex
	}

	ratePerSec := float64(cfg.Security.RateLimiting.RequestsPerMinute) / 60.0
	capacity := cfg.Security.RateLimiting.Burst
	if capacity <= 0 {
		capacity = cfg.Security.RateLimiting.RequestsPerMinute
		if capacity <= 0 {
			capacity = 60
		}
	}

	buckets := make(map[string]*bucket)
	var bucketsMu sync.RWMutex

	return func(c *gin.Context) {
		clientIP := c.ClientIP()

		bucketsMu.RLock()
		b, ok := buckets[clientIP]
		bucketsMu.RUnlock()
		if !ok {
			bucketsMu.Lock()
			if b, ok = buckets[clientIP]; !ok {
				b = &bucket{tokens: float64(capacity), lastRefill: time.Now()}
				buckets[clientIP] = b
			}
			bucketsMu.Unlock()
		}

		b.mutex.Lock()
		now := time.Now()
		elapsed := now.Sub(b.lastRefill).Seconds()
		// refill
		b.tokens += elapsed * ratePerSec
		if b.tokens > float64(capacity) {
			b.tokens = float64(capacity)
		}
		b.lastRefill = now

		if b.tokens >= 1.0 {
			b.tokens -= 1.0
			b.mutex.Unlock()
			c.Next()
			return
		}

		// 计算重试时间
		need := 1.0 - b.tokens
		retryAfter := 1
		if ratePerSec > 0 {
			secs := int(need/ratePerSec + 0.9999) // ceil
			if secs > 0 {
				retryAfter = secs
			}
		}
		b.mutex.Unlock()

		c.Header("Retry-After", fmt.Sprintf("%d", retryAfter))
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error":       "Rate limit exceeded",
			"message":     fmt.Sprintf("Too many requests. Limit: %d req/min (burst %d)", cfg.Security.RateLimiting.RequestsPerMinute, capacity),
			"retry_after": retryAfter,
		})
		c.Abort()
		return
	}
}
