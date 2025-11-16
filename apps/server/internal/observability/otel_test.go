package observability

import (
    "context"
    "testing"
    "servify/apps/server/internal/config"
)

func TestSetupTracing_Disabled_NoOp(t *testing.T) {
    cfg := config.GetDefaultConfig()
    cfg.Monitoring.Tracing.Enabled = false
    shutdown, err := SetupTracing(context.Background(), cfg)
    if err != nil { t.Fatalf("unexpected err: %v", err) }
    if shutdown == nil { t.Fatalf("expected non-nil shutdown function") }
}

func TestEndpointHost_Parse(t *testing.T) {
    if got := endpointHost("http://localhost:4317"); got != "localhost:4317" {
        t.Fatalf("endpointHost parse mismatch: %s", got)
    }
    if got := endpointHost("127.0.0.1:4317"); got != "127.0.0.1:4317" {
        t.Fatalf("endpointHost identity mismatch: %s", got)
    }
}
