package services

import (
	"context"
	"servify/apps/server/internal/models"
	"testing"
	"time"
)

// stub AI to avoid external calls
type stubAI2 struct{ reply string }

func (s stubAI2) ProcessQuery(ctx context.Context, query string, sessionID string) (*AIResponse, error) {
	return &AIResponse{Content: s.reply, Confidence: 0.9, Source: "test"}, nil
}
func (s stubAI2) ShouldTransferToHuman(query string, _ []models.Message) bool { return false }

// Implement the full AIServiceInterface
func (s stubAI2) GetSessionSummary(_ []models.Message) (string, error) { return "", nil }
func (s stubAI2) InitializeKnowledgeBase()                             {}
func (s stubAI2) GetStatus(ctx context.Context) map[string]interface{} {
	return map[string]interface{}{"ok": true}
}

// compile-time assert
var _ AIServiceInterface = (*stubAI2)(nil)

func TestMessageRouter_RouteMessage_DBNil_FallbackAndContinue(t *testing.T) {
	hub := NewWebSocketHub()
	go hub.Run()
	ai := stubAI2{reply: "ok"}
	r := NewMessageRouter(ai, hub, nil) // db = nil -> persist fallback (log only)

	// register a client to capture broadcast
	client := &WebSocketClient{ID: "c1", SessionID: "s1", Send: make(chan WebSocketMessage, 1), Hub: hub}
	hub.register <- client
	time.Sleep(10 * time.Millisecond)

	msg := UnifiedMessage{UserID: "s1", Content: "hi", Type: MessageTypeText, Timestamp: time.Now()}
	if err := r.routeMessage(string(PlatformWeb), msg); err != nil {
		t.Fatalf("routeMessage error: %v", err)
	}

	select {
	case out := <-client.Send:
		if out.Type != "ai-response" {
			t.Fatalf("expected ai-response, got %s", out.Type)
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("no response delivered to client")
	}
}
