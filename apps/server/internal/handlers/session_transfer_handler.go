package handlers

import (
	"net/http"

	"servify/apps/server/internal/models"
	"servify/apps/server/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// SessionTransferHandler 会话转接处理器
type SessionTransferHandler struct {
	transferService *services.SessionTransferService
	logger          *logrus.Logger
}

// NewSessionTransferHandler 创建会话转接处理器
func NewSessionTransferHandler(transferService *services.SessionTransferService, logger *logrus.Logger) *SessionTransferHandler {
	return &SessionTransferHandler{
		transferService: transferService,
		logger:          logger,
	}
}

// TransferToHuman 转接到人工客服
// @Summary 转接到人工客服
// @Description 将AI会话转接到人工客服
// @Tags 会话转接
// @Accept json
// @Produce json
// @Param transfer body services.TransferRequest true "转接请求"
// @Success 200 {object} services.TransferResult
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/session-transfer/to-human [post]
func (h *SessionTransferHandler) TransferToHuman(c *gin.Context) {
	var req services.TransferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request body",
			Message: err.Error(),
		})
		return
	}

	result, err := h.transferService.TransferToHuman(c.Request.Context(), &req)
	if err != nil {
		h.logger.Errorf("Failed to transfer session %s to human: %v", req.SessionID, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to transfer to human",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

// TransferToAgent 转接到指定客服
// @Summary 转接到指定客服
// @Description 将会话转接到指定的客服代理
// @Tags 会话转接
// @Accept json
// @Produce json
// @Param transfer body map[string]interface{} true "转接信息"
// @Success 200 {object} services.TransferResult
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/session-transfer/to-agent [post]
func (h *SessionTransferHandler) TransferToAgent(c *gin.Context) {
	var req struct {
		SessionID     string `json:"session_id" binding:"required"`
		TargetAgentID uint   `json:"target_agent_id" binding:"required"`
		Reason        string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request body",
			Message: err.Error(),
		})
		return
	}

	result, err := h.transferService.TransferToAgent(c.Request.Context(), req.SessionID, req.TargetAgentID, req.Reason)
	if err != nil {
		h.logger.Errorf("Failed to transfer session %s to agent %d: %v", req.SessionID, req.TargetAgentID, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to transfer to agent",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetTransferHistory 获取转接历史
// @Summary 获取转接历史
// @Description 获取会话的转接历史记录
// @Tags 会话转接
// @Accept json
// @Produce json
// @Param session_id path string true "会话ID"
// @Success 200 {array} services.TransferRecord
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/session-transfer/history/{session_id} [get]
func (h *SessionTransferHandler) GetTransferHistory(c *gin.Context) {
	sessionID := c.Param("session_id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Missing session ID",
			Message: "Session ID is required",
		})
		return
	}

	history, err := h.transferService.GetTransferHistory(c.Request.Context(), sessionID)
	if err != nil {
		h.logger.Errorf("Failed to get transfer history for session %s: %v", sessionID, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to get transfer history",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, history)
}

// ProcessWaitingQueue 处理等待队列
// @Summary 处理等待队列
// @Description 手动触发等待队列处理，分配等待中的会话给可用客服
// @Tags 会话转接
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} ErrorResponse
// @Router /api/session-transfer/process-queue [post]
func (h *SessionTransferHandler) ProcessWaitingQueue(c *gin.Context) {
	if err := h.transferService.ProcessWaitingQueue(c.Request.Context()); err != nil {
		h.logger.Errorf("Failed to process waiting queue: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to process waiting queue",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Waiting queue processed successfully",
	})
}

// CheckAutoTransfer 检查自动转接
// @Summary 检查自动转接
// @Description 检查会话是否需要自动转接到人工客服
// @Tags 会话转接
// @Accept json
// @Produce json
// @Param check body map[string]interface{} true "检查信息"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/session-transfer/check-auto [post]
func (h *SessionTransferHandler) CheckAutoTransfer(c *gin.Context) {
	var req struct {
		SessionID string `json:"session_id" binding:"required"`
		Messages  []struct {
			Content string `json:"content"`
			Sender  string `json:"sender"`
		} `json:"messages" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request body",
			Message: err.Error(),
		})
		return
	}

	// 转换消息格式
	var messages []models.Message
	for _, msg := range req.Messages {
		messages = append(messages, models.Message{
			Content: msg.Content,
			Sender:  msg.Sender,
		})
	}

	shouldTransfer := h.transferService.AutoTransferCheck(c.Request.Context(), req.SessionID, messages)

	c.JSON(http.StatusOK, gin.H{
		"session_id":      req.SessionID,
		"should_transfer": shouldTransfer,
		"recommendation":  "Based on conversation analysis",
	})
}

// RegisterSessionTransferRoutes 注册会话转接相关路由
func RegisterSessionTransferRoutes(r *gin.RouterGroup, handler *SessionTransferHandler) {
	transfer := r.Group("/session-transfer")
	{
		transfer.POST("/to-human", handler.TransferToHuman)
		transfer.POST("/to-agent", handler.TransferToAgent)
		transfer.GET("/history/:session_id", handler.GetTransferHistory)
		transfer.POST("/process-queue", handler.ProcessWaitingQueue)
		transfer.POST("/check-auto", handler.CheckAutoTransfer)
	}
}
