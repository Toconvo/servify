package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
	WebRTC   WebRTCConfig
	AI       AIConfig
	JWT      JWTConfig
	Log      LogConfig
}

type ServerConfig struct {
	Host string
	Port int
}

type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Name     string
}

type RedisConfig struct {
	Host     string
	Port     int
	Password string
}

type WebRTCConfig struct {
	STUNServer string
}

type AIConfig struct {
	OpenAIAPIKey string
	OpenAIBaseURL string
}

type JWTConfig struct {
	Secret string
}

type LogConfig struct {
	Level      string
	Format     string // json, text
	Output     string // stdout, file
	FilePath   string
	MaxSize    int  // MB
	MaxAge     int  // days
	MaxBackups int  // number of backup files
	Compress   bool // compress backup files
}

func Load() *Config {
	// 加载 .env 文件
	if err := godotenv.Load(); err != nil {
		logrus.Warn("No .env file found, using environment variables")
	}

	return &Config{
		Server: ServerConfig{
			Host: getEnv("HOST", "0.0.0.0"),
			Port: getEnvInt("PORT", 8080),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnvInt("DB_PORT", 5432),
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", "password"),
			Name:     getEnv("DB_NAME", "servify"),
		},
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     getEnvInt("REDIS_PORT", 6379),
			Password: getEnv("REDIS_PASSWORD", ""),
		},
		WebRTC: WebRTCConfig{
			STUNServer: getEnv("STUN_SERVER", "stun:stun.l.google.com:19302"),
		},
		AI: AIConfig{
			OpenAIAPIKey:  getEnv("OPENAI_API_KEY", ""),
			OpenAIBaseURL: getEnv("OPENAI_BASE_URL", "https://api.openai.com/v1"),
		},
		JWT: JWTConfig{
			Secret: getEnv("JWT_SECRET", "default-secret-key"),
		},
		Log: LogConfig{
			Level:      getEnv("LOG_LEVEL", "info"),
			Format:     getEnv("LOG_FORMAT", "json"),
			Output:     getEnv("LOG_OUTPUT", "stdout"),
			FilePath:   getEnv("LOG_FILE_PATH", "./logs/servify.log"),
			MaxSize:    getEnvInt("LOG_MAX_SIZE", 100),
			MaxAge:     getEnvInt("LOG_MAX_AGE", 7),
			MaxBackups: getEnvInt("LOG_MAX_BACKUPS", 3),
			Compress:   getEnvBool("LOG_COMPRESS", true),
		},
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}