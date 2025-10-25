package config

import (
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server     ServerConfig     `yaml:"server"`
	Database   DatabaseConfig   `yaml:"database"`
	Redis      RedisConfig      `yaml:"redis"`
	WebRTC     WebRTCConfig     `yaml:"webrtc"`
	AI         AIConfig         `yaml:"ai"`
	WeKnora    WeKnoraConfig    `yaml:"weknora"`
	Fallback   FallbackConfig   `yaml:"fallback"`
	JWT        JWTConfig        `yaml:"jwt"`
	Log        LogConfig        `yaml:"log"`
	Monitoring MonitoringConfig `yaml:"monitoring"`
	Security   SecurityConfig   `yaml:"security"`
	Upload     UploadConfig     `yaml:"upload"`
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type DatabaseConfig struct {
	Host            string        `yaml:"host"`
	Port            int           `yaml:"port"`
	User            string        `yaml:"user"`
	Password        string        `yaml:"password"`
	Name            string        `yaml:"name"`
	MaxOpenConns    int           `yaml:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
}

type RedisConfig struct {
	Host         string `yaml:"host"`
	Port         int    `yaml:"port"`
	Password     string `yaml:"password"`
	DB           int    `yaml:"db"`
	PoolSize     int    `yaml:"pool_size"`
	MinIdleConns int    `yaml:"min_idle_conns"`
}

type WebRTCConfig struct {
	STUNServer string `yaml:"stun_server"`
}

type AIConfig struct {
	OpenAI OpenAIConfig `yaml:"openai"`
}

type OpenAIConfig struct {
	APIKey      string        `yaml:"api_key"`
	BaseURL     string        `yaml:"base_url"`
	Model       string        `yaml:"model"`
	Temperature float64       `yaml:"temperature"`
	MaxTokens   int           `yaml:"max_tokens"`
	Timeout     time.Duration `yaml:"timeout"`
}

type WeKnoraConfig struct {
	Enabled         bool                    `yaml:"enabled"`
	BaseURL         string                  `yaml:"base_url"`
	APIKey          string                  `yaml:"api_key"`
	TenantID        string                  `yaml:"tenant_id"`
	KnowledgeBaseID string                  `yaml:"knowledge_base_id"`
	Timeout         time.Duration           `yaml:"timeout"`
	MaxRetries      int                     `yaml:"max_retries"`
	Search          WeKnoraSearchConfig     `yaml:"search"`
	HealthCheck     WeKnoraHealthConfig     `yaml:"health_check"`
}

type WeKnoraSearchConfig struct {
	DefaultLimit   int     `yaml:"default_limit"`
	ScoreThreshold float64 `yaml:"score_threshold"`
	Strategy       string  `yaml:"strategy"`
}

type WeKnoraHealthConfig struct {
	Interval time.Duration `yaml:"interval"`
	Timeout  time.Duration `yaml:"timeout"`
}

type FallbackConfig struct {
	Enabled         bool                    `yaml:"enabled"`
	LegacyKBEnabled bool                    `yaml:"legacy_kb_enabled"`
	CircuitBreaker  CircuitBreakerConfig    `yaml:"circuit_breaker"`
}

type CircuitBreakerConfig struct {
	Enabled         bool          `yaml:"enabled"`
	MaxFailures     int           `yaml:"max_failures"`
	ResetTimeout    time.Duration `yaml:"reset_timeout"`
	HalfOpenMaxReqs int           `yaml:"half_open_max_requests"`
}

type JWTConfig struct {
	Secret    string        `yaml:"secret"`
	ExpiresIn time.Duration `yaml:"expires_in"`
}

type LogConfig struct {
	Level      string `yaml:"level"`
	Format     string `yaml:"format"`     // json, text
	Output     string `yaml:"output"`     // stdout, file, both
	FilePath   string `yaml:"file_path"`
	MaxSize    int    `yaml:"max_size"`    // MB
	MaxAge     int    `yaml:"max_age"`     // days
	MaxBackups int    `yaml:"max_backups"` // number of backup files
	Compress   bool   `yaml:"compress"`    // compress backup files
}

type MonitoringConfig struct {
    Enabled     bool                      `yaml:"enabled"`
    MetricsPath string                    `yaml:"metrics_path"`
    Performance PerformanceMonitorConfig  `yaml:"performance"`
    HealthChecks HealthChecksConfig       `yaml:"health_checks"`
    Tracing     TracingConfig             `yaml:"tracing"`
}

type PerformanceMonitorConfig struct {
	SlowQueryThreshold    time.Duration `yaml:"slow_query_threshold"`
	EnableRequestLogging  bool          `yaml:"enable_request_logging"`
}

type HealthChecksConfig struct {
    Database bool `yaml:"database"`
    Redis    bool `yaml:"redis"`
    WeKnora  bool `yaml:"weknora"`
    OpenAI   bool `yaml:"openai"`
}

// TracingConfig OpenTelemetry 追踪配置
type TracingConfig struct {
    Enabled     bool    `yaml:"enabled"`
    Endpoint    string  `yaml:"endpoint"`     // OTLP gRPC 端点，例如 http://otel-collector:4317 或 0.0.0.0:4317
    Insecure    bool    `yaml:"insecure"`     // 是否使用明文（本地/开发）
    SampleRatio float64 `yaml:"sample_ratio"` // 采样率 0.0~1.0
    ServiceName string  `yaml:"service_name"` // 自定义服务名，缺省使用 "servify"
}

type SecurityConfig struct {
	CORS         CORSConfig         `yaml:"cors"`
	RateLimiting RateLimitingConfig `yaml:"rate_limiting"`
}

type CORSConfig struct {
	Enabled        bool     `yaml:"enabled"`
	AllowedOrigins []string `yaml:"allowed_origins"`
	AllowedMethods []string `yaml:"allowed_methods"`
	AllowedHeaders []string `yaml:"allowed_headers"`
}

type RateLimitingConfig struct {
	Enabled            bool `yaml:"enabled"`
	RequestsPerMinute  int  `yaml:"requests_per_minute"`
	Burst              int  `yaml:"burst"`
}

type UploadConfig struct {
	Enabled       bool     `yaml:"enabled"`
	MaxFileSize   string   `yaml:"max_file_size"`
	AllowedTypes  []string `yaml:"allowed_types"`
	StoragePath   string   `yaml:"storage_path"`
	AutoProcess   bool     `yaml:"auto_process"`
	AutoIndex     bool     `yaml:"auto_index"`
}

func Load() *Config {
	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		panic(err)
	}
	return &config
}

// GetDefaultConfig 返回默认配置
func GetDefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host: "0.0.0.0",
			Port: 8080,
		},
		Database: DatabaseConfig{
			Host:            "localhost",
			Port:            5432,
			User:            "postgres",
			Password:        "password",
			Name:            "servify",
			MaxOpenConns:    100,
			MaxIdleConns:    10,
			ConnMaxLifetime: 3600 * time.Second,
		},
		Redis: RedisConfig{
			Host:         "localhost",
			Port:         6379,
			Password:     "",
			DB:           0,
			PoolSize:     10,
			MinIdleConns: 5,
		},
		WebRTC: WebRTCConfig{
			STUNServer: "stun:stun.l.google.com:19302",
		},
		AI: AIConfig{
			OpenAI: OpenAIConfig{
				BaseURL:     "https://api.openai.com/v1",
				Model:       "gpt-3.5-turbo",
				Temperature: 0.7,
				MaxTokens:   1000,
				Timeout:     30 * time.Second,
			},
		},
		WeKnora: WeKnoraConfig{
			Enabled:         false,
			BaseURL:         "http://localhost:9000",
			APIKey:          "default-api-key",
			TenantID:        "default-tenant",
			KnowledgeBaseID: "default-kb",
			Timeout:         30 * time.Second,
			MaxRetries:      3,
			Search: WeKnoraSearchConfig{
				DefaultLimit:   5,
				ScoreThreshold: 0.7,
				Strategy:       "hybrid",
			},
			HealthCheck: WeKnoraHealthConfig{
				Interval: 30 * time.Second,
				Timeout:  10 * time.Second,
			},
		},
		Fallback: FallbackConfig{
			Enabled:         true,
			LegacyKBEnabled: true,
			CircuitBreaker: CircuitBreakerConfig{
				Enabled:         true,
				MaxFailures:     5,
				ResetTimeout:    60 * time.Second,
				HalfOpenMaxReqs: 3,
			},
		},
		JWT: JWTConfig{
			Secret:    "default-secret-key",
			ExpiresIn: 24 * time.Hour,
		},
		Log: LogConfig{
			Level:      "info",
			Format:     "json",
			Output:     "both",
			FilePath:   "./logs/servify.log",
			MaxSize:    100,
			MaxAge:     7,
			MaxBackups: 3,
			Compress:   true,
		},
        Monitoring: MonitoringConfig{
            Enabled:     true,
            MetricsPath: "/metrics",
            Performance: PerformanceMonitorConfig{
                SlowQueryThreshold:   1 * time.Second,
                EnableRequestLogging: true,
            },
            HealthChecks: HealthChecksConfig{
                Database: true,
                Redis:    true,
                WeKnora:  true,
                OpenAI:   false,
            },
            Tracing: TracingConfig{
                Enabled:     false,
                Endpoint:    "http://localhost:4317",
                Insecure:    true,
                SampleRatio: 0.1,
                ServiceName: "servify",
            },
        },
		Security: SecurityConfig{
			CORS: CORSConfig{
				Enabled:        true,
				AllowedOrigins: []string{"*"},
				AllowedMethods: []string{"GET", "POST", "PUT", "DELETE"},
				AllowedHeaders: []string{"*"},
			},
			RateLimiting: RateLimitingConfig{
				Enabled:           true,
				RequestsPerMinute: 60,
				Burst:             10,
			},
		},
		Upload: UploadConfig{
			Enabled:      true,
			MaxFileSize:  "10MB",
			AllowedTypes: []string{".pdf", ".docx", ".txt", ".md", ".png", ".jpg", ".jpeg"},
			StoragePath:  "./uploads",
			AutoProcess:  true,
			AutoIndex:    true,
		},
	}
}
