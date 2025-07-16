package config

import (
	"github.com/spf13/viper"
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
	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		panic(err)
	}
	return &config
}