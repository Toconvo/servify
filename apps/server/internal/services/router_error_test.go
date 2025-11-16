package services

import (
    "context"
    "testing"
)

func TestMessageRouter_External_NoAdapter(t *testing.T) {
    r := NewMessageRouter(NewAIService("", ""), NewWebSocketHub(), nil)
    msg := UnifiedMessage{ UserID:"u", Content:"hi", Type: MessageTypeText }
    if err := r.handleExternalPlatformMessage(context.Background(), string(PlatformTelegram), msg); err == nil {
        t.Fatalf("expected error when adapter missing")
    }
}
