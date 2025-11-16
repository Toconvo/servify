package services

import (
    "context"
    "testing"
)

func TestEnhancedAI_RetrieveKnowledge_Fallback(t *testing.T) {
    base := NewAIService("", "")
    base.InitializeKnowledgeBase()
    s := NewEnhancedAIService(base, nil, "kb", nil)
    docs, strategy, err := s.retrieveKnowledge(context.Background(), "产品")
    if err != nil { t.Fatalf("unexpected err: %v", err) }
    if strategy == "weknora" { t.Fatalf("expected fallback strategy when weknora disabled") }
    if len(docs) == 0 { t.Fatalf("expected some docs from legacy kb") }
}

func TestEnhancedAI_Toggles(t *testing.T) {
    base := NewAIService("", "")
    s := NewEnhancedAIService(base, nil, "kb", nil)
    s.SetWeKnoraEnabled(true)
    s.SetFallbackEnabled(false)
    // simple smoke: status map should reflect toggles
    st := s.GetStatus(context.Background())
    if st["weknora_enabled"].(bool) != true { t.Fatalf("weknora_enabled not set") }
    if st["fallback_enabled"].(bool) != false { t.Fatalf("fallback_enabled not set") }
}
