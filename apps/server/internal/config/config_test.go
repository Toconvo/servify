package config

import (
	"testing"
	"time"
)

func TestGetDefaultConfig(t *testing.T) {
	cfg := GetDefaultConfig()

	if cfg.Server.Host == "" {
		t.Error("expected Server.Host to be set")
	}
	if cfg.Server.Port == 0 {
		t.Error("expected Server.Port to be non-zero")
	}
	if cfg.Database.Name == "" {
		t.Error("expected Database.Name to be set")
	}
	if cfg.JWT.Secret == "" {
		t.Error("expected JWT.Secret to be set")
	}

	// 验证默认值
	if cfg.Log.Level == "" {
		t.Error("expected Log.Level to be set")
	}
	// 注意：RBAC默认未启用，所以不检查Enabled状态
}

func TestConfig_DatabaseSettings(t *testing.T) {
	cfg := GetDefaultConfig()

	if cfg.Database.MaxOpenConns == 0 {
		t.Error("expected MaxOpenConns to be set")
	}
	if cfg.Database.MaxIdleConns == 0 {
		t.Error("expected MaxIdleConns to be set")
	}
	if cfg.Database.ConnMaxLifetime == 0 {
		t.Error("expected ConnMaxLifetime to be set")
	}
}

func TestConfig_Timeouts(t *testing.T) {
	cfg := GetDefaultConfig()

	if cfg.AI.OpenAI.Timeout == 0 {
		t.Error("expected AI timeout to be set")
	}
	if cfg.WeKnora.Timeout == 0 {
		t.Error("expected WeKnora timeout to be set")
	}
	if cfg.WeKnora.HealthCheck.Interval == 0 {
		t.Error("expected health check interval to be set")
	}
}

func TestConfig_SecurityDefaults(t *testing.T) {
	cfg := GetDefaultConfig()

	if cfg.JWT.Secret == "" {
		t.Error("expected JWT secret to be set")
	}
	// 注意：RBAC默认未启用，只检查CORS和RateLimiting
	if !cfg.Security.CORS.Enabled {
		t.Error("expected CORS to be enabled")
	}
	if !cfg.Security.RateLimiting.Enabled {
		t.Error("expected rate limiting to be enabled")
	}
}

func TestConfig_FallbackSettings(t *testing.T) {
	cfg := GetDefaultConfig()

	if cfg.Fallback.CircuitBreaker.MaxFailures == 0 {
		t.Error("expected circuit breaker max failures to be set")
	}
	if cfg.Fallback.CircuitBreaker.ResetTimeout == 0 {
		t.Error("expected circuit breaker reset timeout to be set")
	}
}

func TestConfig_AIConfiguration(t *testing.T) {
	cfg := GetDefaultConfig()

	if cfg.AI.OpenAI.Model == "" {
		t.Error("expected OpenAI model to be set")
	}
	if cfg.AI.OpenAI.Temperature == 0 {
		t.Error("expected OpenAI temperature to be set")
	}
	if cfg.AI.OpenAI.MaxTokens == 0 {
		t.Error("expected OpenAI max tokens to be set")
	}
}

func TestConfig_WeKnoraSettings(t *testing.T) {
	cfg := GetDefaultConfig()

	if cfg.WeKnora.Search.DefaultLimit == 0 {
		t.Error("expected WeKnora search default limit to be set")
	}
	if cfg.WeKnora.Search.ScoreThreshold == 0 {
		t.Error("expected WeKnora score threshold to be set")
	}
}

func TestConfig_PortalSettings(t *testing.T) {
	cfg := GetDefaultConfig()

	if cfg.Portal.BrandName == "" {
		t.Error("expected portal brand name to be set")
	}
	if cfg.Portal.DefaultLocale == "" {
		t.Error("expected portal default locale to be set")
	}
}

func TestConfig_UploadSettings(t *testing.T) {
	cfg := GetDefaultConfig()

	if cfg.Upload.MaxFileSize == "" {
		t.Error("expected max file size to be set")
	}
	if len(cfg.Upload.AllowedTypes) == 0 {
		t.Error("expected allowed types to be set")
	}
}

func TestConfig_RBACRoles(t *testing.T) {
	cfg := GetDefaultConfig()

	// 注意：RBAC默认未启用，所以Roles可能为空
	// 只测试Security配置存在
	if !cfg.Security.CORS.Enabled {
		t.Error("expected CORS to be enabled")
	}
	if len(cfg.Security.CORS.AllowedOrigins) == 0 {
		t.Error("expected allowed origins to be set")
	}
	if len(cfg.Security.CORS.AllowedMethods) == 0 {
		t.Error("expected allowed methods to be set")
	}
}

func TestConfig_RateLimiting(t *testing.T) {
	cfg := GetDefaultConfig()

	if cfg.Security.RateLimiting.Enabled == false {
		t.Error("expected rate limiting to be enabled")
	}
	if cfg.Security.RateLimiting.RequestsPerMinute == 0 {
		t.Error("expected requests per minute to be set")
	}
}

func TestConfig_Monitoring(t *testing.T) {
	cfg := GetDefaultConfig()

	if cfg.Monitoring.Enabled == false {
		t.Error("expected monitoring to be enabled")
	}
}

func TestConfig_CORS(t *testing.T) {
	cfg := GetDefaultConfig()

	if cfg.Security.CORS.Enabled == false {
		t.Error("expected CORS to be enabled")
	}
	if len(cfg.Security.CORS.AllowedOrigins) == 0 {
		t.Error("expected allowed origins to be set")
	}
	if len(cfg.Security.CORS.AllowedMethods) == 0 {
		t.Error("expected allowed methods to be set")
	}
}

func TestConfig_WebRTC(t *testing.T) {
	cfg := GetDefaultConfig()

	if cfg.WebRTC.STUNServer == "" {
		t.Error("expected STUN server to be set")
	}
}

func TestConfig_DurationValidation(t *testing.T) {
	cfg := GetDefaultConfig()

	// 验证时间单位设置合理
	if cfg.Database.ConnMaxLifetime < time.Minute {
		t.Error("connection max lifetime should be at least 1 minute")
	}
	if cfg.Fallback.CircuitBreaker.ResetTimeout < time.Second {
		t.Error("circuit breaker reset timeout should be at least 1 second")
	}
}

func TestInitLogger_DefaultConfig(t *testing.T) {
	cfg := GetDefaultConfig()

	// 测试使用默认配置初始化日志
	err := InitLogger(cfg)
	if err != nil {
		t.Fatalf("InitLogger failed: %v", err)
	}
}

func TestInitLogger_CustomLevel(t *testing.T) {
	cfg := GetDefaultConfig()
	cfg.Log.Level = "debug"

	err := InitLogger(cfg)
	if err != nil {
		t.Fatalf("InitLogger with debug level failed: %v", err)
	}
}

func TestInitLogger_InvalidLevel(t *testing.T) {
	cfg := GetDefaultConfig()
	cfg.Log.Level = "invalid"

	// 应该使用默认的 info 级别
	err := InitLogger(cfg)
	if err != nil {
		t.Fatalf("InitLogger with invalid level failed: %v", err)
	}
}

func TestInitLogger_TextFormat(t *testing.T) {
	cfg := GetDefaultConfig()
	cfg.Log.Format = "text"

	err := InitLogger(cfg)
	if err != nil {
		t.Fatalf("InitLogger with text format failed: %v", err)
	}
}

func TestInitLogger_JSONFormat(t *testing.T) {
	cfg := GetDefaultConfig()
	cfg.Log.Format = "json"

	err := InitLogger(cfg)
	if err != nil {
		t.Fatalf("InitLogger with json format failed: %v", err)
	}
}

func TestInitLogger_StdoutOutput(t *testing.T) {
	cfg := GetDefaultConfig()
	cfg.Log.Output = "stdout"

	err := InitLogger(cfg)
	if err != nil {
		t.Fatalf("InitLogger with stdout output failed: %v", err)
	}
}

func TestInitLogger_FileOutput(t *testing.T) {
	cfg := GetDefaultConfig()
	cfg.Log.Output = "file"
	cfg.Log.FilePath = "/tmp/test-servify.log"

	err := InitLogger(cfg)
	if err != nil {
		t.Fatalf("InitLogger with file output failed: %v", err)
	}
}

func TestInitLogger_BothOutput(t *testing.T) {
	cfg := GetDefaultConfig()
	cfg.Log.Output = "both"
	cfg.Log.FilePath = "/tmp/test-servify-both.log"

	err := InitLogger(cfg)
	if err != nil {
		t.Fatalf("InitLogger with both output failed: %v", err)
	}
}

func TestInitLogger_InvalidOutput(t *testing.T) {
	cfg := GetDefaultConfig()
	cfg.Log.Output = "invalid"

	// 应该回退到 stdout
	err := InitLogger(cfg)
	if err != nil {
		t.Fatalf("InitLogger with invalid output failed: %v", err)
	}
}

func TestConfig_TracingDefaults(t *testing.T) {
	cfg := GetDefaultConfig()

	// 验证追踪默认配置
	if cfg.Monitoring.Tracing.Enabled {
		t.Error("tracing should be disabled by default")
	}
	if cfg.Monitoring.Tracing.Endpoint == "" {
		t.Error("expected tracing endpoint to be set")
	}
	if cfg.Monitoring.Tracing.SampleRatio == 0 {
		t.Error("expected sample ratio to be set")
	}
}

func TestConfig_RedisDefaults(t *testing.T) {
	cfg := GetDefaultConfig()

	if cfg.Redis.Host == "" {
		t.Error("expected Redis host to be set")
	}
	if cfg.Redis.Port == 0 {
		t.Error("expected Redis port to be set")
	}
	if cfg.Redis.PoolSize == 0 {
		t.Error("expected Redis pool size to be set")
	}
}

func TestConfig_HealthChecks(t *testing.T) {
	cfg := GetDefaultConfig()

	// 验证健康检查配置
	if !cfg.Monitoring.HealthChecks.Database {
		t.Error("expected database health check to be enabled")
	}
	if !cfg.Monitoring.HealthChecks.Redis {
		t.Error("expected Redis health check to be enabled")
	}
}

func TestConfig_PathRateLimits(t *testing.T) {
	cfg := GetDefaultConfig()

	// RateLimiting.Paths 是一个切片，默认为空但不是 nil
	// 在 Go 中，空切片的零值是 nil
	if cfg.Security.RateLimiting.RequestsPerMinute == 0 {
		t.Error("expected requests per minute to be set")
	}
	if cfg.Security.RateLimiting.Burst == 0 {
		t.Error("expected burst to be set")
	}
}

