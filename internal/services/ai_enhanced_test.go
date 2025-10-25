package services

import (
    "context"
    "testing"
)

func TestEnhancedAI_FallbackFlow_NoWeKnora(t *testing.T) {
    base := NewAIService("", "")
    base.InitializeKnowledgeBase()
    enh := NewEnhancedAIService(base, nil, "", nil)

    ctx := context.Background()
    resp, err := enh.ProcessQueryEnhanced(ctx, "介绍一下Servify", "s1")
    if err != nil {
        t.Fatalf("ProcessQueryEnhanced error: %v", err)
    }
    if resp == nil || resp.AIResponse == nil || resp.Content == "" {
        t.Fatalf("expected non-empty content")
    }
    if resp.Strategy == "" {
        t.Fatalf("expected non-empty strategy")
    }
}

