package handlers

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
)

// IngestedMetric 表示客户端上报的一个指标事件（受限白名单）
type IngestedMetric struct {
	Name   string            `json:"name" binding:"required"`
	Value  float64           `json:"value"`
	Labels map[string]string `json:"labels"`
}

// MetricsIngestRequest 客户端上报载荷
type MetricsIngestRequest struct {
	Source    string           `json:"source" binding:"required"` // sdk|admin|agent
	Tenant    string           `json:"tenant"`
	SessionID string           `json:"session_id"`
	Metrics   []IngestedMetric `json:"metrics" binding:"required"`
}

// MetricsAggregator 简易聚合器（内存）
type MetricsAggregator struct {
	mu      sync.RWMutex
	counter map[string]map[string]float64 // key: series(signature) -> labels-json -> value
}

func NewMetricsAggregator() *MetricsAggregator {
	return &MetricsAggregator{counter: make(map[string]map[string]float64)}
}

func (a *MetricsAggregator) Add(name string, labels map[string]string, v float64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, ok := a.counter[name]; !ok {
		a.counter[name] = make(map[string]float64)
	}
	key := labelsKey(labels)
	a.counter[name][key] += v
}

func (a *MetricsAggregator) Snapshot() map[string]map[string]float64 {
	a.mu.RLock()
	defer a.mu.RUnlock()
	out := make(map[string]map[string]float64, len(a.counter))
	for n, series := range a.counter {
		m := make(map[string]float64, len(series))
		for k, v := range series {
			m[k] = v
		}
		out[n] = m
	}
	return out
}

// MetricsIngestHandler 处理上报
type MetricsIngestHandler struct {
	agg *MetricsAggregator
}

func NewMetricsIngestHandler(agg *MetricsAggregator) *MetricsIngestHandler {
	return &MetricsIngestHandler{agg: agg}
}

var allowedMetrics = map[string]bool{
	"sdk_ws_reconnects_total":   true,
	"sdk_messages_sent_total":   true,
	"sdk_messages_recv_total":   true,
	"sdk_webrtc_sessions_total": true,
	"admin_actions_total":       true,
	"agent_online_gauge":        true, // 将按 counter 处理（增减由客户端上报 +/-1）
	"agent_takeover_total":      true,
}

func (h *MetricsIngestHandler) Ingest(c *gin.Context) {
	var req MetricsIngestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	// 基本校验
	for _, m := range req.Metrics {
		if !allowedMetrics[m.Name] {
			continue
		}
		labels := map[string]string{}
		// 基础标签
		labels["source"] = req.Source
		if req.Tenant != "" {
			labels["tenant"] = req.Tenant
		}
		if req.SessionID != "" {
			labels["session"] = req.SessionID
		}
		for k, v := range m.Labels {
			labels[k] = v
		}
		val := m.Value
		if val == 0 {
			val = 1
		}
		h.agg.Add(m.Name, labels, val)
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// labelsKey 生成稳定的 labels 序列化（简单字典序拼接）
func labelsKey(m map[string]string) string {
	if len(m) == 0 {
		return ""
	}
	// 简化：不引入额外依赖，做稳定排序
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// 朴素冒泡排序（规模极小）
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[j] < keys[i] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	s := ""
	for idx, k := range keys {
		if idx > 0 {
			s += ","
		}
		s += k + "=\"" + m[k] + "\""
	}
	return s
}
