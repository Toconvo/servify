package observability

import (
    "context"
    "fmt"

    "servify/apps/server/internal/config"

    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
    "go.opentelemetry.io/otel/sdk/resource"
    sdktrace "go.opentelemetry.io/otel/sdk/trace"
    "go.opentelemetry.io/otel/attribute"
)

// SetupTracing 初始化 OpenTelemetry TracerProvider，返回关闭函数
func SetupTracing(ctx context.Context, cfg *config.Config) (func(context.Context) error, error) {
    tc := cfg.Monitoring.Tracing
    if !tc.Enabled {
        return func(context.Context) error { return nil }, nil
    }

    endpoint := tc.Endpoint
    if endpoint == "" {
        endpoint = "http://localhost:4317"
    }

    // 构建 OTLP gRPC 导出器
    var opts []otlptracegrpc.Option
    opts = append(opts, otlptracegrpc.WithEndpoint(endpointHost(endpoint)))
    if tc.Insecure {
        opts = append(opts, otlptracegrpc.WithInsecure())
    }

    exp, err := otlptracegrpc.New(ctx, opts...)
    if err != nil {
        return nil, fmt.Errorf("otlp exporter: %w", err)
    }

    // 资源属性
    svcName := tc.ServiceName
    if svcName == "" { svcName = "servify" }
    res, err := resource.New(ctx,
        resource.WithFromEnv(),
        resource.WithTelemetrySDK(),
        resource.WithAttributes(
            attribute.String("service.name", svcName),
        ),
    )
    if err != nil {
        return nil, fmt.Errorf("resource: %w", err)
    }

    // 采样器
    ratio := tc.SampleRatio
    if ratio <= 0 || ratio > 1 {
        ratio = 0.1
    }
    sampler := sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))

    tp := sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(exp),
        sdktrace.WithSampler(sampler),
        sdktrace.WithResource(res),
    )
    otel.SetTracerProvider(tp)

    return tp.Shutdown, nil
}

// endpointHost 从 http://host:port 或 host:port 提取 host:port 供 gRPC 使用
func endpointHost(s string) string {
    // 简化处理
    if len(s) > 7 && (s[:7] == "http://" || s[:8] == "https://") {
        // 去掉协议
        for i := 0; i < len(s); i++ {
            if s[i] == '/' && i+2 < len(s) && s[i+1] == '/' {
                return s[i+2:]
            }
        }
    }
    return s
}
