package services

import (
    "context"
    "testing"
    "strings"
    "servify/internal/models"
)

func newEnhancedForUnit() *EnhancedAIService {
    base := NewAIService("", "")
    base.InitializeKnowledgeBase()
    // weKnoraClient nil -> disabled
    return NewEnhancedAIService(base, nil, "kb", nil)
}

func TestEnhancedAI_CalculateConfidence(t *testing.T) {
    s := newEnhancedForUnit()
    // no docs
    if v := s.calculateConfidence(nil, "none"); v < 0.1 || v > 0.95 {
        t.Fatalf("confidence out of range: %v", v)
    }
    // weknora with docs should be higher than fallback with no docs
    docs := []models.KnowledgeDoc{{Title:"A", Content:"x"}, {Title:"B", Content:"y"}}
    c1 := s.calculateConfidence(docs, "weknora")
    c2 := s.calculateConfidence(nil, "fallback")
    if c1 <= c2 { t.Fatalf("expected weknora+docs confidence higher, got %v <= %v", c1, c2) }
}

func TestEnhancedAI_BuildEnhancedPrompt(t *testing.T) {
    s := newEnhancedForUnit()
    docs := []models.KnowledgeDoc{{Title:"Doc1", Content:"Content1"}}
    p := s.buildEnhancedPrompt("你好？", docs)
    if !strings.Contains(p, "Doc1") || !strings.Contains(p, "Content1") || !strings.Contains(p, "你好") {
        t.Fatalf("prompt should contain docs and query, got: %s", p)
    }
}

func TestEnhancedAI_ConvertDocsToSources(t *testing.T) {
    s := newEnhancedForUnit()
    docs := []models.KnowledgeDoc{{ID:1, Title:"D", Content:"C"}}
    ss := s.convertDocsToSources(docs)
    if len(ss) != 1 || ss[0].Title != "D" { t.Fatalf("unexpected sources: %+v", ss) }
}

func TestEnhancedAI_StatusAndCircuitBreaker(t *testing.T) {
    s := newEnhancedForUnit()
    // induce failures then reset
    s.circuitBreaker.OnFailure()
    s.circuitBreaker.OnFailure()

    st := s.GetStatus(context.Background())
    cb := st["circuit_breaker"].(map[string]interface{})
    if cb["failure_count"].(int) < 2 { t.Fatalf("expected failure_count >=2, got %v", cb["failure_count"]) }

    s.ResetCircuitBreaker()
    st2 := s.GetStatus(context.Background())
    cb2 := st2["circuit_breaker"].(map[string]interface{})
    if cb2["failure_count"].(int) != 0 || cb2["state"].(CircuitBreakerState) != StateClosedCB {
        t.Fatalf("expected reset to closed with 0 failures, got %+v", cb2)
    }
}

// no-op
