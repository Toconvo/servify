package main

import (
    "context"
    "flag"
    "fmt"
    "net/http"
    "os"
    "os/signal"
    "strconv"
    "syscall"
    "time"

    "servify/internal/config"
    "servify/internal/handlers"
    "servify/internal/models"
    "servify/internal/services"
    "servify/pkg/weknora"

    "github.com/gin-gonic/gin"
    "github.com/sirupsen/logrus"
    "github.com/spf13/viper"
    "gorm.io/driver/postgres"
    "gorm.io/gorm"
    "gorm.io/gorm/logger"
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
    flagSet.IntVar(&srvPort, "port", func() int { if p := os.Getenv("SERVIFY_PORT"); p != "" { if n, err := strconv.Atoi(p); err == nil { return n } }; return cfg.Server.Port }(), "server port (listen)")
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

    db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{ Logger: logger.Default.LogMode(logger.Info) })
    if err != nil {
        appLogger.Fatalf("Failed to connect to database: %v", err)
    }

    // 根据需要迁移（此处默认迁移，生产可改为条件控制）
    if err := db.AutoMigrate(
        &models.User{}, &models.Customer{}, &models.Agent{}, &models.Session{}, &models.Message{},
        &models.Ticket{}, &models.TicketComment{}, &models.TicketFile{}, &models.TicketStatus{},
        &models.KnowledgeDoc{}, &models.WebRTCConnection{}, &models.DailyStats{},
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
    webrtcService := services.NewWebRTCService(cfg.WebRTC.STUNServer, wsHub)
    messageRouter := services.NewMessageRouter(aiService, wsHub, db)
    go wsHub.Run()
    if err := messageRouter.Start(); err != nil {
        appLogger.Fatalf("Failed to start message router: %v", err)
    }

    // 初始化业务服务
    customerService := services.NewCustomerService(db, appLogger)
    agentService := services.NewAgentService(db, appLogger)
    ticketService := services.NewTicketService(db, appLogger)
    sessionTransferService := services.NewSessionTransferService(db, appLogger, aiService, agentService, nil)
    statisticsService := services.NewStatisticsService(db, appLogger)

    // 启动统计服务后台任务
    go statisticsService.StartDailyStatsWorker()

    // 初始化 Gin
    if cfg.Log.Level == "debug" {
        gin.SetMode(gin.DebugMode)
    } else {
        gin.SetMode(gin.ReleaseMode)
    }
    r := gin.New()
    r.Use(gin.Logger())
    r.Use(gin.Recovery())
    r.Use(corsMiddleware())

    // 健康检查（增强版）
    healthHandler := handlers.NewEnhancedHealthHandler(cfg, aiService)
    r.GET("/health", healthHandler.Health)
    r.GET("/ready", healthHandler.Ready)

    // API 路由组（管理类）
    api := r.Group("/api")
    handlers.RegisterCustomerRoutes(api, customerHandler(customerService, appLogger))
    handlers.RegisterAgentRoutes(api, agentHandler(agentService, appLogger))
    handlers.RegisterTicketRoutes(api, ticketHandler(ticketService, appLogger))
    handlers.RegisterSessionTransferRoutes(api, transferHandler(sessionTransferService, appLogger))
    handlers.RegisterStatisticsRoutes(api, statisticsHandler(statisticsService, appLogger))

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
    }

    // 静态资源（与 CLI 对齐）
    r.Static("/static", "./static")
    r.Static("/", "./web")

    // 启动服务器
    // 监听地址优先使用 flags/env 覆盖
    host := firstNonEmpty(srvHost, cfg.Server.Host)
    port := srvPort
    if port == 0 { port = cfg.Server.Port }
    listenAddr := fmt.Sprintf("%s:%d", host, port)

    srv := &http.Server{ Addr: listenAddr, Handler: r }
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
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    if err := srv.Shutdown(ctx); err != nil {
        appLogger.Fatalf("Server forced to shutdown: %v", err)
    }
    appLogger.Info("Server exited")
}

// 轻量包装以减少重复（仅为保持现有 Register*Routes 签名方便调用）
func customerHandler(s *services.CustomerService, l *logrus.Logger) *handlers.CustomerHandler { return handlers.NewCustomerHandler(s, l) }
func agentHandler(s *services.AgentService, l *logrus.Logger) *handlers.AgentHandler { return handlers.NewAgentHandler(s, l) }
func ticketHandler(s *services.TicketService, l *logrus.Logger) *handlers.TicketHandler { return handlers.NewTicketHandler(s, l) }
func transferHandler(s *services.SessionTransferService, l *logrus.Logger) *handlers.SessionTransferHandler { return handlers.NewSessionTransferHandler(s, l) }
func statisticsHandler(s *services.StatisticsService, l *logrus.Logger) *handlers.StatisticsHandler { return handlers.NewStatisticsHandler(s, l) }

// helpers (copied from migrate for consistency)
func getenvDefault(key, def string) string {
    if v := os.Getenv(key); v != "" { return v }
    return def
}
func firstNonEmpty(vals ...string) string {
    for _, v := range vals { if v != "" { return v } }
    return ""
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
