package services

import (
	"context"
	"servify/apps/server/internal/models"
	"testing"
	"time"
)

// stubAI implements AIServiceInterface for tests
type stubAI struct{ reply string }

func (s stubAI) ProcessQuery(ctx context.Context, query string, sessionID string) (*AIResponse, error) {
	return &AIResponse{Content: s.reply + ":" + query, Confidence: 0.9, Source: "test"}, nil
}
func (s stubAI) ShouldTransferToHuman(query string, _ []models.Message) bool { return false }
func (s stubAI) GetSessionSummary(_ []models.Message) (string, error)        { return "", nil }
func (s stubAI) InitializeKnowledgeBase()                                    {}
func (s stubAI) GetStatus(ctx context.Context) map[string]interface{} {
	return map[string]interface{}{"ok": true}
}

// ensure stub satisfies interface
var _ AIServiceInterface = (*stubAI)(nil)

func TestMessageRouter_HandleWebMessage_PushesAIResponse(t *testing.T) {
	hub := NewWebSocketHub()
	go hub.Run()

	ai := stubAI{reply: "ok"}
	r := NewMessageRouter(ai, hub, nil)

	// register a client for the session to capture broadcast
	client := &WebSocketClient{ID: "c1", SessionID: "s1", Send: make(chan WebSocketMessage, 1), Hub: hub}
	hub.register <- client
	time.Sleep(10 * time.Millisecond)

	msg := UnifiedMessage{UserID: "s1", Content: "hello", Type: MessageTypeText, Timestamp: time.Now()}
	if err := r.handleWebMessage(context.Background(), msg); err != nil {
		t.Fatalf("handleWebMessage error: %v", err)
	}

	select {
	case out := <-client.Send:
		if out.Type != "ai-response" {
			t.Fatalf("expected ai-response, got %s", out.Type)
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("did not receive response on client channel")
	}
}
