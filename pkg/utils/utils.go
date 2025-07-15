package utils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// 生成随机 ID
func GenerateID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// 生成会话 ID
func GenerateSessionID() string {
	return fmt.Sprintf("session_%s_%d", GenerateID()[:8], time.Now().Unix())
}

// 时间格式化
func FormatTime(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}

// 验证消息内容
func ValidateMessage(content string) bool {
	if len(content) == 0 || len(content) > 4096 {
		return false
	}
	return true
}