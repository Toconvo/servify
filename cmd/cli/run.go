
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

	"servify/internal/config"
	"servify/internal/handlers"
	"servify/internal/services"

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

	// 初始化服务
	wsHub := services.NewWebSocketHub()
	webrtcService := services.NewWebRTCService(cfg.WebRTC.STUNServer, wsHub)
    // 使用新的配置结构（cfg.AI.OpenAI.*）
    aiService := services.NewAIService(cfg.AI.OpenAI.APIKey, cfg.AI.OpenAI.BaseURL)
    messageRouter := services.NewMessageRouter(aiService, wsHub)

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
	router := setupRouter(wsHub, webrtcService, messageRouter)

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

func setupRouter(wsHub *services.WebSocketHub, webrtcService *services.WebRTCService, messageRouter *services.MessageRouter) *gin.Engine {
	router := gin.New()

	// 中间件
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.Use(corsMiddleware())

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
	}

	// 静态文件服务
	router.Static("/static", "./static")
	router.Static("/", "./web") // 服务官网静态文件

	return router
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Header("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
