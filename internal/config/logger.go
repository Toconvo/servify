package config

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

// InitLogger 初始化日志系统
func InitLogger(cfg *Config) error {
	// 设置日志级别
	level, err := logrus.ParseLevel(cfg.Log.Level)
	if err != nil {
		logrus.Warnf("Invalid log level '%s', using 'info'", cfg.Log.Level)
		level = logrus.InfoLevel
	}
	logrus.SetLevel(level)

	// 设置日志格式
	switch strings.ToLower(cfg.Log.Format) {
	case "json":
		logrus.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: "2006-01-02 15:04:05",
		})
	case "text":
		logrus.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02 15:04:05",
		})
	default:
		logrus.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: "2006-01-02 15:04:05",
		})
	}

	// 设置日志输出
	switch strings.ToLower(cfg.Log.Output) {
	case "stdout":
		logrus.SetOutput(os.Stdout)
	case "file":
		// 创建日志目录
		logDir := filepath.Dir(cfg.Log.FilePath)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return err
		}

		// 配置日志轮转
		rotateLogger := &lumberjack.Logger{
			Filename:   cfg.Log.FilePath,
			MaxSize:    cfg.Log.MaxSize,    // MB
			MaxBackups: cfg.Log.MaxBackups, // 保留文件数
			MaxAge:     cfg.Log.MaxAge,     // 保留天数
			Compress:   cfg.Log.Compress,   // 压缩
			LocalTime:  true,               // 使用本地时间
		}

		logrus.SetOutput(rotateLogger)
	case "both":
		// 同时输出到控制台和文件
		logDir := filepath.Dir(cfg.Log.FilePath)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return err
		}

		rotateLogger := &lumberjack.Logger{
			Filename:   cfg.Log.FilePath,
			MaxSize:    cfg.Log.MaxSize,
			MaxBackups: cfg.Log.MaxBackups,
			MaxAge:     cfg.Log.MaxAge,
			Compress:   cfg.Log.Compress,
			LocalTime:  true,
		}

		multiWriter := io.MultiWriter(os.Stdout, rotateLogger)
		logrus.SetOutput(multiWriter)
	default:
		logrus.SetOutput(os.Stdout)
	}

	// 添加调用者信息（可选）
	logrus.SetReportCaller(true)

	logrus.Infof("Logger initialized - Level: %s, Format: %s, Output: %s", 
		cfg.Log.Level, cfg.Log.Format, cfg.Log.Output)

	return nil
}