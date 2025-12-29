package handlers

import (
	"net/http"
	"strconv"

	"servify/apps/server/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// AgentHandler 客服管理处理器
type AgentHandler struct {
	agentService *services.AgentService
	logger       *logrus.Logger
}

// NewAgentHandler 创建客服处理器
func NewAgentHandler(agentService *services.AgentService, logger *logrus.Logger) *AgentHandler {
	return &AgentHandler{
		agentService: agentService,
		logger:       logger,
	}
}

// CreateAgent 创建客服
// @Summary 创建客服
// @Description 创建新的客服代理
// @Tags 客服管理
// @Accept json
// @Produce json
// @Param agent body services.AgentCreateRequest true "客服信息"
// @Success 201 {object} models.Agent
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/agents [post]
func (h *AgentHandler) CreateAgent(c *gin.Context) {
	var req services.AgentCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request body",
			Message: err.Error(),
		})
		return
	}

	agent, err := h.agentService.CreateAgent(c.Request.Context(), &req)
	if err != nil {
		h.logger.Errorf("Failed to create agent: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to create agent",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, agent)
}

// GetAgent 获取客服详情
// @Summary 获取客服详情
// @Description 根据用户ID获取客服的详细信息
// @Tags 客服管理
// @Accept json
// @Produce json
// @Param id path int true "用户ID"
// @Success 200 {object} models.Agent
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/agents/{id} [get]
func (h *AgentHandler) GetAgent(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid agent ID",
			Message: "ID must be a valid number",
		})
		return
	}

	agent, err := h.agentService.GetAgentByUserID(c.Request.Context(), uint(id))
	if err != nil {
		h.logger.Errorf("Failed to get agent %d: %v", id, err)
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "Agent not found",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, agent)
}

// UpdateAgentStatus 更新客服状态
// @Summary 更新客服状态
// @Description 更新客服的在线状态
// @Tags 客服管理
// @Accept json
// @Produce json
// @Param id path int true "用户ID"
// @Param status body map[string]string true "状态信息"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/agents/{id}/status [put]
func (h *AgentHandler) UpdateAgentStatus(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid agent ID",
			Message: "ID must be a valid number",
		})
		return
	}

	var req struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request body",
			Message: err.Error(),
		})
		return
	}

	if err := h.agentService.UpdateAgentStatus(c.Request.Context(), uint(id), req.Status); err != nil {
		h.logger.Errorf("Failed to update agent %d status: %v", id, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to update agent status",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Agent status updated successfully",
		"agent_id": id,
		"status":   req.Status,
	})
}

// AgentGoOnline 客服上线
// @Summary 客服上线
// @Description 将客服状态设置为在线
// @Tags 客服管理
// @Accept json
// @Produce json
// @Param id path int true "用户ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/agents/{id}/online [post]
func (h *AgentHandler) AgentGoOnline(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid agent ID",
			Message: "ID must be a valid number",
		})
		return
	}

	if err := h.agentService.AgentGoOnline(c.Request.Context(), uint(id)); err != nil {
		h.logger.Errorf("Failed to set agent %d online: %v", id, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to set agent online",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Agent is now online",
		"agent_id": id,
	})
}

// AgentGoOffline 客服下线
// @Summary 客服下线
// @Description 将客服状态设置为离线
// @Tags 客服管理
// @Accept json
// @Produce json
// @Param id path int true "用户ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/agents/{id}/offline [post]
func (h *AgentHandler) AgentGoOffline(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid agent ID",
			Message: "ID must be a valid number",
		})
		return
	}

	if err := h.agentService.AgentGoOffline(c.Request.Context(), uint(id)); err != nil {
		h.logger.Errorf("Failed to set agent %d offline: %v", id, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to set agent offline",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Agent is now offline",
		"agent_id": id,
	})
}

// GetOnlineAgents 获取在线客服列表
// @Summary 获取在线客服列表
// @Description 获取当前在线的所有客服
// @Tags 客服管理
// @Accept json
// @Produce json
// @Success 200 {array} services.AgentInfo
// @Failure 500 {object} ErrorResponse
// @Router /api/agents/online [get]
func (h *AgentHandler) GetOnlineAgents(c *gin.Context) {
	agents := h.agentService.GetOnlineAgents(c.Request.Context())
	c.JSON(http.StatusOK, agents)
}

// ListAgents 获取全部客服列表（包含 User 信息）
// @Summary 获取客服列表
// @Description 获取客服列表（用于批量指派下拉/后台管理）
// @Tags 客服管理
// @Accept json
// @Produce json
// @Param limit query int false "返回条数（默认 200，最大 500）"
// @Success 200 {array} models.Agent
// @Failure 500 {object} ErrorResponse
// @Router /api/agents [get]
func (h *AgentHandler) ListAgents(c *gin.Context) {
	limit := 200
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	agents, err := h.agentService.ListAgents(c.Request.Context(), limit)
	if err != nil {
		h.logger.Errorf("Failed to list agents: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to list agents",
			Message: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": agents})
}

// AssignSession 分配会话给客服
// @Summary 分配会话给客服
// @Description 将会话分配给指定的客服
// @Tags 客服管理
// @Accept json
// @Produce json
// @Param id path int true "客服用户ID"
// @Param assignment body map[string]string true "分配信息"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/agents/{id}/assign-session [post]
func (h *AgentHandler) AssignSession(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid agent ID",
			Message: "ID must be a valid number",
		})
		return
	}

	var req struct {
		SessionID string `json:"session_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request body",
			Message: err.Error(),
		})
		return
	}

	if err := h.agentService.AssignSessionToAgent(c.Request.Context(), req.SessionID, uint(id)); err != nil {
		h.logger.Errorf("Failed to assign session %s to agent %d: %v", req.SessionID, id, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to assign session",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Session assigned successfully",
		"agent_id":   id,
		"session_id": req.SessionID,
	})
}

// ReleaseSession 释放客服的会话
// @Summary 释放客服的会话
// @Description 从客服释放指定的会话
// @Tags 客服管理
// @Accept json
// @Produce json
// @Param id path int true "客服用户ID"
// @Param release body map[string]string true "释放信息"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/agents/{id}/release-session [post]
func (h *AgentHandler) ReleaseSession(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid agent ID",
			Message: "ID must be a valid number",
		})
		return
	}

	var req struct {
		SessionID string `json:"session_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request body",
			Message: err.Error(),
		})
		return
	}

	if err := h.agentService.ReleaseSessionFromAgent(c.Request.Context(), req.SessionID, uint(id)); err != nil {
		h.logger.Errorf("Failed to release session %s from agent %d: %v", req.SessionID, id, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to release session",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Session released successfully",
		"agent_id":   id,
		"session_id": req.SessionID,
	})
}

// GetAgentStats 获取客服统计
// @Summary 获取客服统计
// @Description 获取客服相关的统计数据
// @Tags 客服管理
// @Accept json
// @Produce json
// @Param agent_id query int false "特定客服ID，用于获取单个客服的统计"
// @Success 200 {object} services.AgentStats
// @Failure 500 {object} ErrorResponse
// @Router /api/agents/stats [get]
func (h *AgentHandler) GetAgentStats(c *gin.Context) {
	var agentID *uint
	if agentIDStr := c.Query("agent_id"); agentIDStr != "" {
		if id, err := strconv.ParseUint(agentIDStr, 10, 32); err == nil {
			agentIDValue := uint(id)
			agentID = &agentIDValue
		}
	}

	stats, err := h.agentService.GetAgentStats(c.Request.Context(), agentID)
	if err != nil {
		h.logger.Errorf("Failed to get agent stats: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to get agent statistics",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// FindAvailableAgent 查找可用客服
// @Summary 查找可用客服
// @Description 根据技能和优先级查找可用的客服
// @Tags 客服管理
// @Accept json
// @Produce json
// @Param skills query []string false "所需技能"
// @Param priority query string false "优先级"
// @Success 200 {object} services.AgentInfo
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/agents/find-available [get]
func (h *AgentHandler) FindAvailableAgent(c *gin.Context) {
	skills := c.QueryArray("skills")
	priority := c.DefaultQuery("priority", "normal")

	agent, err := h.agentService.FindAvailableAgent(c.Request.Context(), skills, priority)
	if err != nil {
		h.logger.Errorf("Failed to find available agent: %v", err)
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "No available agent found",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, agent)
}

// RegisterAgentRoutes 注册客服管理相关路由
func RegisterAgentRoutes(r *gin.RouterGroup, handler *AgentHandler) {
	agents := r.Group("/agents")
	{
		agents.POST("", handler.CreateAgent)
		agents.GET("", handler.ListAgents)
		agents.GET("/online", handler.GetOnlineAgents)
		agents.GET("/stats", handler.GetAgentStats)
		agents.GET("/find-available", handler.FindAvailableAgent)
		agents.GET("/:id", handler.GetAgent)
		agents.PUT("/:id/status", handler.UpdateAgentStatus)
		agents.POST("/:id/online", handler.AgentGoOnline)
		agents.POST("/:id/offline", handler.AgentGoOffline)
		agents.POST("/:id/assign-session", handler.AssignSession)
		agents.POST("/:id/release-session", handler.ReleaseSession)
	}
}
