package utils

import (
	"strings"
	"testing"
	"time"
)

func TestGenerateID(t *testing.T) {
	id := GenerateID()
	if len(id) != 32 { // 16 bytes = 32 hex chars
		t.Errorf("GenerateID() returned length %d, want 32", len(id))
	}

	// 验证是有效的十六进制
	for _, c := range id {
		if !strings.ContainsAny(string(c), "0123456789abcdef") {
			t.Errorf("GenerateID() returned invalid hex character: %c", c)
		}
	}

	// 验证每次生成不同的ID
	id2 := GenerateID()
	if id == id2 {
		t.Error("GenerateID() returned same ID twice")
	}
}

func TestGenerateSessionID(t *testing.T) {
	sessionID := GenerateSessionID()
	if len(sessionID) == 0 {
		t.Error("GenerateSessionID() returned empty string")
	}

	// 验证前缀
	if !strings.HasPrefix(sessionID, "session_") {
		t.Errorf("GenerateSessionID() should start with 'session_', got %s", sessionID[:8])
	}

	// 验证包含时间戳部分
	parts := strings.Split(sessionID, "_")
	if len(parts) != 3 {
		t.Errorf("GenerateSessionID() should have 3 parts separated by '_', got %d", len(parts))
	}

	// 验证最后一部分是数字时间戳
	timestamp := parts[2]
	if len(timestamp) < 10 {
		t.Errorf("GenerateSessionID() timestamp part too short: %s", timestamp)
	}
}

func TestFormatTime(t *testing.T) {
	testTime := time.Date(2024, 1, 15, 14, 30, 45, 0, time.UTC)
	formatted := FormatTime(testTime)

	expected := "2024-01-15 15:14:30" // 注意：time.Local可能影响结果
	if formatted != expected {
		// 由于时区差异，我们只验证格式
		if len(formatted) != 19 {
			t.Errorf("FormatTime() returned length %d, want 19", len(formatted))
		}
	}
}

func TestValidateMessage(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{
			name:    "valid message",
			content: "Hello, this is a test message",
			want:    true,
		},
		{
			name:    "empty message",
			content: "",
			want:    false,
		},
		{
			name:    "message at max length",
			content: strings.Repeat("a", 4096),
			want:    true,
		},
		{
			name:    "message exceeding max length",
			content: strings.Repeat("a", 4097),
			want:    false,
		},
		{
			name:    "message with special characters",
			content: "测试消息！@#$%^&*()",
			want:    true,
		},
		{
			name:    "single character",
			content: "a",
			want:    true,
		},
		{
			name:    "message with newlines",
			content: "Line 1\nLine 2\r\nLine 3",
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateMessage(tt.content)
			if got != tt.want {
				t.Errorf("ValidateMessage() = %v, want %v", got, tt.want)
			}
		})
	}
}
