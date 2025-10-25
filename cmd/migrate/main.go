package main

import (
	"log"
	"os"
	"time"

	"servify/internal/config"
	"servify/internal/models"

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

	// 连接数据库
	db, err := gorm.Open(postgres.Open(cfg.Database.URL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	log.Println("Starting database migration...")

	// 自动迁移所有模型
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
		log.Fatalf("Failed to migrate database: %v", err)
	}

	log.Println("Database migration completed successfully!")

	// 创建索引
	log.Println("Creating additional indexes...")

	// 为消息表创建复合索引
	db.Exec("CREATE INDEX IF NOT EXISTS idx_messages_session_created ON messages(session_id, created_at)")

	// 为工单表创建复合索引
	db.Exec("CREATE INDEX IF NOT EXISTS idx_tickets_status_created ON tickets(status, created_at)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_tickets_agent_status ON tickets(agent_id, status)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_tickets_customer_created ON tickets(customer_id, created_at)")

	// 为会话表创建索引
	db.Exec("CREATE INDEX IF NOT EXISTS idx_sessions_user_created ON sessions(user_id, created_at)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_sessions_agent_status ON sessions(agent_id, status)")

	// 为客户表创建索引
	db.Exec("CREATE INDEX IF NOT EXISTS idx_customers_priority ON customers(priority)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_customers_source ON customers(source)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_customers_industry ON customers(industry)")

	// 为客服表创建索引
	db.Exec("CREATE INDEX IF NOT EXISTS idx_agents_status ON agents(status)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_agents_department ON agents(department)")

	// 为统计表创建索引
	db.Exec("CREATE INDEX IF NOT EXISTS idx_daily_stats_date ON daily_stats(date)")

	log.Println("Additional indexes created successfully!")

	// 插入默认数据
	if len(os.Args) > 1 && os.Args[1] == "--seed" {
		log.Println("Seeding default data...")
		seedDefaultData(db)
		log.Println("Default data seeded successfully!")
	}

	log.Println("Migration process completed!")
}

func seedDefaultData(db *gorm.DB) {
	// 创建默认管理员用户
	var adminUser models.User
	if err := db.Where("username = ?", "admin").First(&adminUser).Error; err != nil {
		adminUser = models.User{
			Username: "admin",
			Email:    "admin@servify.com",
			Name:     "系统管理员",
			Role:     "admin",
			Status:   "active",
		}
		db.Create(&adminUser)
		log.Println("Created default admin user")
	}

	// 创建测试客户
	var testCustomer models.User
	if err := db.Where("username = ?", "test_customer").First(&testCustomer).Error; err != nil {
		testCustomer = models.User{
			Username: "test_customer",
			Email:    "customer@test.com",
			Name:     "测试客户",
			Role:     "customer",
			Status:   "active",
		}
		db.Create(&testCustomer)

		// 创建客户扩展信息
		customer := models.Customer{
			UserID:   testCustomer.ID,
			Company:  "测试公司",
			Industry: "technology",
			Source:   "web",
			Priority: "normal",
			Tags:     "测试,新客户",
			Notes:    "这是一个测试客户账户",
		}
		db.Create(&customer)
		log.Println("Created test customer")
	}

	// 创建测试客服
	var testAgent models.User
	if err := db.Where("username = ?", "test_agent").First(&testAgent).Error; err != nil {
		testAgent = models.User{
			Username: "test_agent",
			Email:    "agent@test.com",
			Name:     "测试客服",
			Role:     "agent",
			Status:   "active",
		}
		db.Create(&testAgent)

		// 创建客服扩展信息
		agent := models.Agent{
			UserID:          testAgent.ID,
			Department:      "客户服务部",
			Skills:          "技术支持,产品咨询,投诉处理",
			Status:          "offline",
			MaxConcurrent:   5,
			CurrentLoad:     0,
			Rating:          5.0,
			AvgResponseTime: 30,
		}
		db.Create(&agent)
		log.Println("Created test agent")
	}

	// 创建示例知识库文档
	var existingDoc models.KnowledgeDoc
	if err := db.Where("title = ?", "欢迎使用 Servify").First(&existingDoc).Error; err != nil {
		doc := models.KnowledgeDoc{
			Title:    "欢迎使用 Servify",
			Content:  "Servify 是一个智能客服系统，集成了 AI 对话和人工客服功能。系统支持自动回复、工单管理、客户管理等功能。",
			Category: "getting-started",
			Tags:     "welcome,guide,introduction",
		}
		db.Create(&doc)
		log.Println("Created sample knowledge document")
	}

	// 创建示例统计数据
	var todayStats models.DailyStats
	today := time.Now().Truncate(24 * time.Hour)
	if err := db.Where("date = ?", today).First(&todayStats).Error; err != nil {
		stats := models.DailyStats{
			Date:                 today,
			TotalSessions:        10,
			TotalMessages:        50,
			TotalTickets:         5,
			ResolvedTickets:      3,
			AvgResponseTime:      45,
			AvgResolutionTime:    3600,
			CustomerSatisfaction: 4.2,
			AIUsageCount:         25,
			WeKnoraUsageCount:    15,
		}
		db.Create(&stats)
		log.Println("Created sample daily statistics")
	}
}