package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"servify/internal/config"
	"servify/internal/handlers"
	"servify/internal/models"
	"servify/internal/services"
	"servify/pkg/weknora"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	// 加载配置
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 初始化日志
	logLevel, err := logrus.ParseLevel(cfg.Log.Level)
	if err != nil {
		logLevel = logrus.InfoLevel
	}

	appLogger := logrus.New()
	appLogger.SetLevel(logLevel)
	appLogger.SetFormatter(&logrus.JSONFormatter{})

	// 连接数据库
	db, err := gorm.Open(postgres.Open(cfg.Database.URL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		appLogger.Fatalf("Failed to connect to database: %v", err)
	}

	// 自动迁移（开发环境）
	if cfg.Environment == "development" {
		appLogger.Info("Running auto-migration for development environment")
		err = db.AutoMigrate(
			&models.User{},
			&models.Customer{},
			&models.Agent{},
			&models.Session{},
			&models.Message{},
			&models.Ticket{},
			&models.TicketComment{},
			&models.TicketFile{},
			&models.TicketStatus{},
			&models.KnowledgeDoc{},
			&models.WebRTCConnection{},
			&models.DailyStats{},
		)
		if err != nil {
			appLogger.Fatalf("Failed to migrate database: %v", err)
		}
	}

	// 初始化 WeKnora 客户端
	weKnoraClient := weknora.NewClient(cfg.WeKnora.BaseURL, cfg.WeKnora.APIKey, cfg.WeKnora.Timeout)

	// 初始化熔断器
	circuitBreaker := services.NewCircuitBreaker(
		cfg.WeKnora.CircuitBreaker.MaxRequests,
		time.Duration(cfg.WeKnora.CircuitBreaker.Interval)*time.Second,
		time.Duration(cfg.WeKnora.CircuitBreaker.Timeout)*time.Second,
	)

	// 初始化 AI 服务
	var aiService services.AIServiceInterface
	if cfg.AI.Provider == "enhanced" {
		enhancedAI := services.NewEnhancedAIService(
			cfg.AI.APIKey,
			cfg.AI.Model,
			weKnoraClient,
			circuitBreaker,
			appLogger,
		)
		aiService = enhancedAI
	} else {
		// 使用原始的AI服务作为后备
		aiService = &services.SimpleAIService{} // 需要实现这个简单版本
	}

	// 初始化所有服务
	customerService := services.NewCustomerService(db, appLogger)
	agentService := services.NewAgentService(db, appLogger)
	ticketService := services.NewTicketService(db, appLogger)
	sessionTransferService := services.NewSessionTransferService(db, appLogger, aiService, agentService, nil) // WebSocket hub 需要单独初始化
	statisticsService := services.NewStatisticsService(db, appLogger)

	// 启动统计服务后台任务
	go statisticsService.StartDailyStatsWorker()

	// 初始化处理器
	customerHandler := handlers.NewCustomerHandler(customerService, appLogger)
	agentHandler := handlers.NewAgentHandler(agentService, appLogger)
	ticketHandler := handlers.NewTicketHandler(ticketService, appLogger)
	transferHandler := handlers.NewSessionTransferHandler(sessionTransferService, appLogger)
	statisticsHandler := handlers.NewStatisticsHandler(statisticsService, appLogger)

	// 初始化 Gin 路由
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()

	// 中间件
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(corsMiddleware())

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "ok",
			"timestamp": time.Now().UTC(),
			"version":   "v1.1.0",
		})
	})

	// API 路由组
	api := r.Group("/api")

	// 注册各个模块的路由
	handlers.RegisterCustomerRoutes(api, customerHandler)
	handlers.RegisterAgentRoutes(api, agentHandler)
	handlers.RegisterTicketRoutes(api, ticketHandler)
	handlers.RegisterSessionTransferRoutes(api, transferHandler)
	handlers.RegisterStatisticsRoutes(api, statisticsHandler)

	// 启动服务器
	srv := &http.Server{
		Addr:    ":" + cfg.Server.Port,
		Handler: r,
	}

	// 优雅关闭
	go func() {
		appLogger.Infof("Starting server on port %s", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			appLogger.Fatalf("Failed to start server: %v", err)
		}
	}()

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	appLogger.Info("Shutting down server...")

	// 优雅关闭，超时时间30秒
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		appLogger.Fatalf("Server forced to shutdown: %v", err)
	}

	appLogger.Info("Server exited")
}

// corsMiddleware CORS 中间件
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}