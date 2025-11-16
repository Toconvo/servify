package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"net/http"
	"servify/apps/server/internal/services"
)

type WebSocketHandler struct {
	wsHub *services.WebSocketHub
}

func NewWebSocketHandler(wsHub *services.WebSocketHub) *WebSocketHandler {
	return &WebSocketHandler{
		wsHub: wsHub,
	}
}

func (h *WebSocketHandler) HandleWebSocket(c *gin.Context) {
	h.wsHub.HandleWebSocket(c)
}

func (h *WebSocketHandler) GetStats(c *gin.Context) {
	stats := map[string]interface{}{
		"connected_clients": h.wsHub.GetClientCount(),
		"status":            "running",
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
	})
}

type WebRTCHandler struct {
	webrtcService *services.WebRTCService
}

func NewWebRTCHandler(webrtcService *services.WebRTCService) *WebRTCHandler {
	return &WebRTCHandler{
		webrtcService: webrtcService,
	}
}

func (h *WebRTCHandler) GetStats(c *gin.Context) {
	sessionID := c.Query("session_id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "session_id is required",
		})
		return
	}

	stats, err := h.webrtcService.GetConnectionStats(sessionID)
	if err != nil {
		logrus.Errorf("Failed to get WebRTC stats: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to get connection stats",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
	})
}

func (h *WebRTCHandler) GetConnections(c *gin.Context) {
	count := h.webrtcService.GetConnectionCount()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": map[string]interface{}{
			"connection_count": count,
		},
	})
}

type MessageHandler struct {
	messageRouter *services.MessageRouter
}

func NewMessageHandler(messageRouter *services.MessageRouter) *MessageHandler {
	return &MessageHandler{
		messageRouter: messageRouter,
	}
}

func (h *MessageHandler) GetPlatformStats(c *gin.Context) {
	stats := h.messageRouter.GetPlatformStats()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
	})
}

type HealthHandler struct{}

func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

func (h *HealthHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": c.GetHeader("X-Request-Time"),
		"version":   "1.0.0",
	})
}

func (h *HealthHandler) Ready(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ready",
	})
}
