package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"servify/apps/server/internal/config"
	"servify/apps/server/internal/handlers"
	"servify/apps/server/internal/middleware"
	"servify/apps/server/internal/models"
	"servify/apps/server/internal/observability"
	"servify/apps/server/internal/services"
	"servify/apps/server/pkg/weknora"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	gormtracing "gorm.io/plugin/opentelemetry/tracing"
)

func main() {
	// 读取配置文件（默认 ./config.yml）并初始化日志
	viper.AddConfigPath(".")
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AutomaticEnv()
	_ = viper.ReadInConfig()

	cfg := config.Load()

	// 允许通过 flags/env 覆盖数据库连接（保持与 migrate 一致的接口）
	var (
		flagDSN   string
		dbHost    string
		dbPortStr string
		dbUser    string
		dbPass    string
		dbName    string
		dbSSLMode string
		dbTZ      string
		srvHost   string
		srvPort   int
	)
	// 延迟导入以避免顶层 import 冲突
	{
		// 标准库 flag 在此作用域使用
		type strptr = *string
		_ = strptr(nil)
	}
	// 使用标准库 flag
	flagSet := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	flagSet.SetOutput(os.Stdout)
	flagSet.StringVar(&flagDSN, "dsn", os.Getenv("DB_DSN"), "Postgres DSN, if set overrides other DB flags")
	flagSet.StringVar(&dbHost, "db-host", getenvDefault("DB_HOST", cfg.Database.Host), "database host")
	flagSet.StringVar(&dbPortStr, "db-port", getenvDefault("DB_PORT", fmt.Sprintf("%d", cfg.Database.Port)), "database port")
	flagSet.StringVar(&dbUser, "db-user", getenvDefault("DB_USER", cfg.Database.User), "database user")
	flagSet.StringVar(&dbPass, "db-pass", getenvDefault("DB_PASSWORD", cfg.Database.Password), "database password")
	flagSet.StringVar(&dbName, "db-name", getenvDefault("DB_NAME", cfg.Database.Name), "database name")
	flagSet.StringVar(&dbSSLMode, "db-sslmode", getenvDefault("DB_SSLMODE", "disable"), "sslmode (disable, require, verify-ca, verify-full)")
	flagSet.StringVar(&dbTZ, "db-timezone", getenvDefault("DB_TIMEZONE", "UTC"), "database timezone")
	flagSet.StringVar(&srvHost, "host", getenvDefault("SERVIFY_HOST", cfg.Server.Host), "server host (listen)")
	flagSet.IntVar(&srvPort, "port", func() int {
		if p := os.Getenv("SERVIFY_PORT"); p != "" {
			if n, err := strconv.Atoi(p); err == nil {
				return n
			}
		}
		return cfg.Server.Port
	}(), "server port (listen)")
	_ = flagSet.Parse(os.Args[1:])

	// 组装 DSN
	dsn := flagDSN
	if dsn == "" {
		host := firstNonEmpty(dbHost, cfg.Database.Host)
		user := firstNonEmpty(dbUser, cfg.Database.User)
		pass := firstNonEmpty(dbPass, cfg.Database.Password)
		name := firstNonEmpty(dbName, cfg.Database.Name)
		port := dbPortStr
		if port == "" && cfg.Database.Port != 0 {
			port = fmt.Sprintf("%d", cfg.Database.Port)
		}
		ssl := dbSSLMode
		tz := dbTZ
		dsn = fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=%s", host, user, pass, name, port, ssl, tz)
	}
	if err := config.InitLogger(cfg); err != nil {
		logrus.Warnf("init logger: %v", err)
	}
	appLogger := logrus.StandardLogger()

	// OpenTelemetry 初始化（可选）
	shutdownOTel, err := observability.SetupTracing(context.Background(), cfg)
	if err != nil {
		appLogger.Warnf("init tracing: %v", err)
	} else {
		defer func() { _ = shutdownOTel(context.Background()) }()
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Info)})
	if err != nil {
		appLogger.Fatalf("Failed to connect to database: %v", err)
	}
	if cfg.Monitoring.Tracing.Enabled {
		_ = db.Use(gormtracing.NewPlugin())
	}

	// 根据需要迁移（此处默认迁移，生产可改为条件控制）
	if err := db.AutoMigrate(
		&models.User{}, &models.Customer{}, &models.Agent{}, &models.Session{}, &models.Message{},
		&models.TransferRecord{}, &models.WaitingRecord{},
		&models.Ticket{}, &models.TicketComment{}, &models.TicketFile{}, &models.TicketStatus{},
		&models.CustomField{}, &models.TicketCustomFieldValue{},
		&models.KnowledgeDoc{}, &models.WebRTCConnection{}, &models.DailyStats{},
		&models.SLAConfig{}, &models.SLAViolation{}, &models.CustomerSatisfaction{}, &models.SatisfactionSurvey{}, &models.AppIntegration{}, &models.ShiftSchedule{},
		&models.AutomationTrigger{}, &models.AutomationRun{}, &models.Macro{},
	); err != nil {
		appLogger.Fatalf("Failed to migrate database: %v", err)
	}

	// 初始化 AI 服务（可选 WeKnora 增强）
	var aiService services.AIServiceInterface
	baseAI := services.NewAIService(cfg.AI.OpenAI.APIKey, cfg.AI.OpenAI.BaseURL)
	baseAI.InitializeKnowledgeBase()

	var weKnoraClient weknora.WeKnoraInterface
	if cfg.WeKnora.Enabled {
		wkCfg := &weknora.Config{
			BaseURL:    cfg.WeKnora.BaseURL,
			APIKey:     cfg.WeKnora.APIKey,
			TenantID:   cfg.WeKnora.TenantID,
			Timeout:    cfg.WeKnora.Timeout,
			MaxRetries: cfg.WeKnora.MaxRetries,
		}
		weKnoraClient = weknora.NewClient(wkCfg, appLogger)
		aiService = services.NewEnhancedAIService(baseAI, weKnoraClient, cfg.WeKnora.KnowledgeBaseID, appLogger)
	} else {
		aiService = baseAI
	}

	// 初始化实时与路由服务（对齐 CLI 端点）
	wsHub := services.NewWebSocketHub()
	wsHub.SetDB(db)
	webrtcService := services.NewWebRTCService(cfg.WebRTC.STUNServer, wsHub)
	messageRouter := services.NewMessageRouter(aiService, wsHub, db)
	go wsHub.Run()
	// 使 WebSocket 文本消息可直接触发 AI 回复
	wsHub.SetAIService(aiService)
	if err := messageRouter.Start(); err != nil {
		appLogger.Fatalf("Failed to start message router: %v", err)
	}

	// 初始化业务服务
	slaService := services.NewSLAService(db, appLogger)
	automationService := services.NewAutomationService(db, appLogger)
	slaService.SetAutomationService(automationService)
	customerService := services.NewCustomerService(db, appLogger)
	agentService := services.NewAgentService(db, appLogger)
	ticketService := services.NewTicketService(db, appLogger, slaService)
	ticketService.SetAutomationService(automationService)
	sessionTransferService := services.NewSessionTransferService(db, appLogger, aiService, agentService, wsHub)
	statisticsService := services.NewStatisticsService(db, appLogger)
	satisfactionService := services.NewSatisfactionService(db, appLogger)
	ticketService.SetSatisfactionService(satisfactionService)
	shiftService := services.NewShiftService(db, appLogger)
	workspaceService := services.NewWorkspaceService(db, agentService)
	macroService := services.NewMacroService(db)
	appIntegrationService := services.NewAppIntegrationService(db, appLogger)
	customFieldService := services.NewCustomFieldService(db)
	knowledgeDocService := services.NewKnowledgeDocService(db)
	suggestionService := services.NewSuggestionService(db)
	gamificationService := services.NewGamificationService(db)

	// 启动统计服务后台任务
	go statisticsService.StartDailyStatsWorker()

	// 启动SLA监控后台服务（每5分钟检查一次）
	ctx, cancel := context.WithCancel(context.Background())
	go slaService.StartSLAMonitor(ctx, 5*time.Minute)
	defer cancel()

	// 初始化 Gin
	if cfg.Log.Level == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(corsMiddlewareWithConfig(cfg))
	// Rate limit: per-path overrides if configured, otherwise global values
	r.Use(middleware.RateLimitMiddlewareFromConfig(cfg))
	// OpenTelemetry Gin 中间件
	if cfg.Monitoring.Tracing.Enabled {
		r.Use(otelgin.Middleware(cfg.Monitoring.Tracing.ServiceName))
	}

	// 健康检查（增强版）
	healthHandler := handlers.NewEnhancedHealthHandler(cfg, aiService)
	r.GET("/health", healthHandler.Health)
	r.GET("/ready", healthHandler.Ready)

	// Prometheus Metrics（若启用）
	if cfg.Monitoring.Enabled {
		r.GET(cfg.Monitoring.MetricsPath, handlers.NewMetricsHandler(wsHub, webrtcService, aiService, messageRouter, db).GetMetrics)
	}

	// API 路由组（管理类）
	api := r.Group("/api")
	// 全部管理接口先做鉴权
	api.Use(middleware.AuthMiddleware(cfg))

	// Fine-grained RBAC by resource
	customersAPI := api.Group("/")
	customersAPI.Use(middleware.RequireResourcePermission("customers"))
	handlers.RegisterCustomerRoutes(customersAPI, customerHandler(customerService, appLogger))

	agentsAPI := api.Group("/")
	agentsAPI.Use(middleware.RequireResourcePermission("agents"))
	handlers.RegisterAgentRoutes(agentsAPI, agentHandler(agentService, appLogger))

	ticketsAPI := api.Group("/")
	ticketsAPI.Use(middleware.RequireResourcePermission("tickets"))
	handlers.RegisterTicketRoutes(ticketsAPI, ticketHandler(ticketService, appLogger))

	sessionTransferAPI := api.Group("/")
	sessionTransferAPI.Use(middleware.RequireResourcePermission("session_transfer"))
	handlers.RegisterSessionTransferRoutes(sessionTransferAPI, transferHandler(sessionTransferService, appLogger))

	satisfactionAPI := api.Group("/")
	satisfactionAPI.Use(middleware.RequireResourcePermission("satisfaction"))
	handlers.RegisterSatisfactionRoutes(satisfactionAPI, satisfactionHandler(satisfactionService, appLogger))

	workspaceAPI := api.Group("/")
	workspaceAPI.Use(middleware.RequireResourcePermission("workspace"))
	handlers.RegisterWorkspaceRoutes(workspaceAPI, workspaceHandler(workspaceService))

	macrosAPI := api.Group("/")
	macrosAPI.Use(middleware.RequireResourcePermission("macros"))
	handlers.RegisterMacroRoutes(macrosAPI, macroHandler(macroService))

	integrationsAPI := api.Group("/")
	integrationsAPI.Use(middleware.RequireResourcePermission("integrations"))
	handlers.RegisterAppIntegrationRoutes(integrationsAPI, appMarketHandler(appIntegrationService))

	customFieldsAPI := api.Group("/")
	customFieldsAPI.Use(middleware.RequireResourcePermission("custom_fields"))
	handlers.RegisterCustomFieldRoutes(customFieldsAPI, handlers.NewCustomFieldHandler(customFieldService))

	statisticsAPI := api.Group("/")
	statisticsAPI.Use(middleware.RequireResourcePermission("statistics"))
	handlers.RegisterStatisticsRoutes(statisticsAPI, statisticsHandler(statisticsService, appLogger))

	slaAPI := api.Group("/")
	slaAPI.Use(middleware.RequireResourcePermission("sla"))
	handlers.RegisterSLARoutes(slaAPI, slaHandler(slaService, ticketService, appLogger))

	shiftAPI := api.Group("/")
	shiftAPI.Use(middleware.RequireResourcePermission("shift"))
	handlers.RegisterShiftRoutes(shiftAPI, shiftHandler(shiftService))

	automationAPI := api.Group("/")
	automationAPI.Use(middleware.RequireResourcePermission("automation"))
	handlers.RegisterAutomationRoutes(automationAPI, automationHandler(automationService))

	knowledgeAPI := api.Group("/")
	knowledgeAPI.Use(middleware.RequireResourcePermission("knowledge"))
	handlers.RegisterKnowledgeDocRoutes(knowledgeAPI, handlers.NewKnowledgeDocHandler(knowledgeDocService))

	assistAPI := api.Group("/")
	assistAPI.Use(middleware.RequireResourcePermission("assist"))
	handlers.RegisterSuggestionRoutes(assistAPI, handlers.NewSuggestionHandler(suggestionService))

	gamificationAPI := api.Group("/")
	gamificationAPI.Use(middleware.RequireResourcePermission("gamification"))
	handlers.RegisterGamificationRoutes(gamificationAPI, handlers.NewGamificationHandler(gamificationService))

	// 公共（无需登录）API
	public := r.Group("/public")
	handlers.RegisterCSATSurveyRoutes(public, csatSurveyHandler(satisfactionService))
	handlers.RegisterPublicKnowledgeBaseRoutes(public, handlers.NewKnowledgeDocHandler(knowledgeDocService))
	public.GET("/portal/config", handlers.NewPortalConfigHandler(cfg).Get)

	// v1 路由组（实时/AI 与静态服务）
	v1 := r.Group("/api/v1")
	{
		// WebSocket
		wsHandler := handlers.NewWebSocketHandler(wsHub)
		v1.GET("/ws", wsHandler.HandleWebSocket)
		v1.GET("/ws/stats", wsHandler.GetStats)

		// WebRTC
		webrtcHandler := handlers.NewWebRTCHandler(webrtcService)
		v1.GET("/webrtc/stats", webrtcHandler.GetStats)
		v1.GET("/webrtc/connections", webrtcHandler.GetConnections)

		// 路由统计
		messageHandler := handlers.NewMessageHandler(messageRouter)
		v1.GET("/messages/platforms", messageHandler.GetPlatformStats)

		// AI API
		aiHandler := handlers.NewAIHandler(aiService)
		aiAPI := v1.Group("/ai")
		aiAPI.POST("/query", aiHandler.ProcessQuery)
		aiAPI.GET("/status", aiHandler.GetStatus)
		aiAPI.GET("/metrics", aiHandler.GetMetrics)
		if cfg.WeKnora.Enabled {
			aiAPI.POST("/knowledge/upload", aiHandler.UploadDocument)
			aiAPI.POST("/knowledge/sync", aiHandler.SyncKnowledgeBase)
			aiAPI.PUT("/weknora/enable", aiHandler.EnableWeKnora)
			aiAPI.PUT("/weknora/disable", aiHandler.DisableWeKnora)
			aiAPI.POST("/circuit-breaker/reset", aiHandler.ResetCircuitBreaker)
		}

		// 轻量指标上报（客户端/前端）
		ingest := handlers.NewMetricsIngestHandler(handlers.NewMetricsAggregator())
		v1.POST("/metrics/ingest", ingest.Ingest)
	}

	// WebSocket 层支持“转人工”触发
	wsHub.SetSessionTransferService(sessionTransferService)

	// 静态资源：尝试多种路径（本地运行/容器内）
	staticRoots := []string{
		"./apps/demo-web",
		"../demo-web",
		"/app/apps/demo-web",
	}
	var staticRoot string
	for _, p := range staticRoots {
		if _, err := os.Stat(p); err == nil {
			staticRoot = p
			break
		}
	}
	if staticRoot == "" {
		staticRoot = "./apps/demo-web"
	}
	r.Static("/", staticRoot)

	// 启动服务器
	// 监听地址优先使用 flags/env 覆盖
	host := firstNonEmpty(srvHost, cfg.Server.Host)
	port := srvPort
	if port == 0 {
		port = cfg.Server.Port
	}
	listenAddr := fmt.Sprintf("%s:%d", host, port)

	srv := &http.Server{Addr: listenAddr, Handler: r}
	go func() {
		appLogger.Infof("Starting server on %s", listenAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			appLogger.Fatalf("Failed to start server: %v", err)
		}
	}()

	// 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	appLogger.Info("Shutting down server...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		appLogger.Fatalf("Server forced to shutdown: %v", err)
	}
	appLogger.Info("Server exited")
}

// 轻量包装以减少重复（仅为保持现有 Register*Routes 签名方便调用）
func customerHandler(s *services.CustomerService, l *logrus.Logger) *handlers.CustomerHandler {
	return handlers.NewCustomerHandler(s, l)
}
func agentHandler(s *services.AgentService, l *logrus.Logger) *handlers.AgentHandler {
	return handlers.NewAgentHandler(s, l)
}
func ticketHandler(s *services.TicketService, l *logrus.Logger) *handlers.TicketHandler {
	return handlers.NewTicketHandler(s, l)
}
func transferHandler(s *services.SessionTransferService, l *logrus.Logger) *handlers.SessionTransferHandler {
	return handlers.NewSessionTransferHandler(s, l)
}
func statisticsHandler(s *services.StatisticsService, l *logrus.Logger) *handlers.StatisticsHandler {
	return handlers.NewStatisticsHandler(s, l)
}
func satisfactionHandler(s *services.SatisfactionService, l *logrus.Logger) *handlers.SatisfactionHandler {
	return handlers.NewSatisfactionHandler(s, l)
}
func slaHandler(s *services.SLAService, t *services.TicketService, l *logrus.Logger) *handlers.SLAHandler {
	return handlers.NewSLAHandler(s, t)
}
func shiftHandler(s *services.ShiftService) *handlers.ShiftHandler {
	return handlers.NewShiftHandler(s)
}
func automationHandler(s *services.AutomationService) *handlers.AutomationHandler {
	return handlers.NewAutomationHandler(s)
}
func macroHandler(s *services.MacroService) *handlers.MacroHandler {
	return handlers.NewMacroHandler(s)
}
func workspaceHandler(s *services.WorkspaceService) *handlers.WorkspaceHandler {
	return handlers.NewWorkspaceHandler(s)
}
func appMarketHandler(s *services.AppIntegrationService) *handlers.AppMarketHandler {
	return handlers.NewAppMarketHandler(s)
}
func csatSurveyHandler(s *services.SatisfactionService) *handlers.CSATSurveyHandler {
	return handlers.NewCSATSurveyHandler(s)
}

// helpers (copied from migrate for consistency)
func getenvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// corsMiddleware CORS 中间件
func corsMiddlewareWithConfig(cfg *config.Config) gin.HandlerFunc {
	allowedOrigins := "*"
	allowedMethods := "GET, POST, PUT, DELETE, OPTIONS"
	allowedHeaders := "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization"
	if cfg != nil && cfg.Security.CORS.Enabled {
		if len(cfg.Security.CORS.AllowedOrigins) > 0 {
			allowedOrigins = strings.Join(cfg.Security.CORS.AllowedOrigins, ", ")
		}
		if len(cfg.Security.CORS.AllowedMethods) > 0 {
			allowedMethods = strings.Join(cfg.Security.CORS.AllowedMethods, ", ")
		}
		if len(cfg.Security.CORS.AllowedHeaders) > 0 {
			allowedHeaders = strings.Join(cfg.Security.CORS.AllowedHeaders, ", ")
		}
	}
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", allowedOrigins)
		c.Header("Access-Control-Allow-Methods", allowedMethods)
		c.Header("Access-Control-Allow-Headers", allowedHeaders)
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
