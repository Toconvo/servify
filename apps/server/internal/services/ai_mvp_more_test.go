package services

import (
    "testing"
    "servify/apps/server/internal/models"
)

func TestKnowledgeBase_Search_Basic(t *testing.T) {
    kb := &KnowledgeBase{}
    kb.AddDocument(models.KnowledgeDoc{ Title: "A", Content: "hello world" })
    kb.AddDocument(models.KnowledgeDoc{ Title: "B", Content: "foo bar" })
    got := kb.Search("hello", 5)
    if len(got) == 0 { t.Fatalf("expected >=1 result") }
}

func TestAIService_ShouldTransfer_Complaint(t *testing.T) {
    s := NewAIService("", "")
    if !s.ShouldTransferToHuman("我要投诉你们的服务", nil) {
        t.Fatalf("expected complaint to transfer")
    }
}
