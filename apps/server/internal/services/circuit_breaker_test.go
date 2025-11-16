package services

import (
    "testing"
    "time"
)

func TestCircuitBreaker_Transitions(t *testing.T) {
    cfg := &CircuitBreakerConfig{ MaxFailures: 1, ResetTimeout: 1 * time.Millisecond, HalfOpenMaxReqs: 1 }
    cb := NewCircuitBreakerWithConfig(cfg)

    if !cb.IsClosed() { t.Fatalf("new breaker should be closed") }
    cb.OnFailure()
    if !cb.IsOpen() { t.Fatalf("breaker should open after reaching max failures") }

    // After reset timeout, Allow should move to half-open and allow one request
    time.Sleep(2 * time.Millisecond)
    if !cb.Allow() { t.Fatalf("expected allow in half-open") }
    if !cb.IsHalfOpen() { t.Fatalf("state should be half-open after allow") }

    // success in half-open -> closed
    cb.OnSuccess()
    if !cb.IsClosed() { t.Fatalf("expected closed after success in half-open") }
}
