
//go:build !weknora
// +build !weknora

package cli

import (
    "context"
    "fmt"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"
    "strings"

    "servify/apps/server/internal/config"
    "servify/apps/server/internal/handlers"
    "servify/apps/server/internal/services"
    "servify/apps/server/internal/observability"
    "servify/apps/server/internal/middleware"
    "go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
    gormtracing "gorm.io/plugin/opentelemetry/tracing"
    "gorm.io/gorm"
    "gorm.io/gorm/logger"
    "gorm.io/driver/postgres"

    "github.com/gin-gonic/gin"
    "github.com/sirupsen/logrus"
    "github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the servify application",
	Long:  `Run the servify application`,
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

    // 初始化数据库
    dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=disable TimeZone=UTC", cfg.Database.Host, cfg.Database.User, cfg.Database.Password, cfg.Database.Name, cfg.Database.Port)
    db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{ Logger: logger.Default.LogMode(logger.Warn) })
    if err != nil {
        logrus.Warnf("DB connect failed, message persistence disabled: %v", err)
    }
    // GORM OTel 插件
    if db != nil && cfg.Monitoring.Tracing.Enabled {
        _ = db.Use(gormtracing.NewPlugin())
    }

    // 初始化服务
    wsHub := services.NewWebSocketHub()
    if db != nil { wsHub.SetDB(db) }
    webrtcService := services.NewWebRTCService(cfg.WebRTC.STUNServer, wsHub)
    // 使用新的配置结构（cfg.AI.OpenAI.*）
    aiService := services.NewAIService(cfg.AI.OpenAI.APIKey, cfg.AI.OpenAI.BaseURL)
    messageRouter := services.NewMessageRouter(aiService, wsHub, db)

    // 初始化知识库
    aiService.InitializeKnowledgeBase()

    // 将AI服务注入到WebSocket以便直接处理文本消息
    wsHub.SetAIService(aiService)

	// 启动服务
	go wsHub.Run()

	// 启动消息路由
	if err := messageRouter.Start(); err != nil {
		logrus.Fatalf("Failed to start message router: %v", err)
	}

	// 设置 Gin 模式
	if cfg.Server.Host != "localhost" {
		gin.SetMode(gin.ReleaseMode)
	}

    // 创建路由
    router := setupRouter(cfg, wsHub, webrtcService, messageRouter)

	// 创建服务器
	server := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler: router,
	}

	// 启动服务器
	go func() {
		logrus.Infof("Starting server on %s:%d", cfg.Server.Host, cfg.Server.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logrus.Fatalf("Server failed to start: %v", err)
		}
	}()

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logrus.Info("Shutting down server...")

	// 优雅关闭
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 停止消息路由
	if err := messageRouter.Stop(); err != nil {
		logrus.Errorf("Failed to stop message router: %v", err)
	}

	// 关闭服务器
	if err := server.Shutdown(ctx); err != nil {
		logrus.Errorf("Server forced to shutdown: %v", err)
	}

	logrus.Info("Server exited")
}

func setupRouter(cfg *config.Config, wsHub *services.WebSocketHub, webrtcService *services.WebRTCService, messageRouter *services.MessageRouter) *gin.Engine {
    router := gin.New()

    // 中间件
    router.Use(gin.Logger())
    router.Use(gin.Recovery())
    router.Use(corsMiddlewareWithConfig(cfg))
    router.Use(middleware.RateLimitMiddlewareFromConfig(cfg))
    // OTel 中间件
    // 注意：标准 CLI 未持有 cfg，此处仅使用默认服务名
    router.Use(otelgin.Middleware("servify"))

	// 健康检查
	healthHandler := handlers.NewHealthHandler()
	router.GET("/health", healthHandler.Health)
	router.GET("/ready", healthHandler.Ready)

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

        // 轻量指标上报（可选）
        ingest := handlers.NewMetricsIngestHandler(handlers.NewMetricsAggregator())
        api.POST("/metrics/ingest", ingest.Ingest)
    }

	// 静态文件服务（尝试多路径）
	staticRoots := []string{
		"./apps/demo-web",
		"../demo-web",
		"/app/apps/demo-web",
	}
	sr := "./apps/demo-web"
	for _, p := range staticRoots {
		if fi, err := os.Stat(p); err == nil && fi.IsDir() {
			sr = p
			break
		}
	}
	router.Static("/", sr)

	return router
}

func corsMiddlewareWithConfig(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		origins := "*"
		methods := "GET, POST, PUT, DELETE, OPTIONS"
		headers := "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With"
		if cfg != nil && cfg.Security.CORS.Enabled {
			if len(cfg.Security.CORS.AllowedOrigins) > 0 {
				origins = strings.Join(cfg.Security.CORS.AllowedOrigins, ", ")
			}
			if len(cfg.Security.CORS.AllowedMethods) > 0 {
				methods = strings.Join(cfg.Security.CORS.AllowedMethods, ", ")
			}
			if len(cfg.Security.CORS.AllowedHeaders) > 0 {
				headers = strings.Join(cfg.Security.CORS.AllowedHeaders, ", ")
			}
		}
		c.Header("Access-Control-Allow-Origin", origins)
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Headers", headers)
		c.Header("Access-Control-Allow-Methods", methods)
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
