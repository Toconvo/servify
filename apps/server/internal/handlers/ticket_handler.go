package handlers

import (
	"net/http"
	"strconv"

	"servify/apps/server/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// TicketHandler 工单处理器
type TicketHandler struct {
	ticketService *services.TicketService
	logger        *logrus.Logger
}

// NewTicketHandler 创建工单处理器
func NewTicketHandler(ticketService *services.TicketService, logger *logrus.Logger) *TicketHandler {
	return &TicketHandler{
		ticketService: ticketService,
		logger:        logger,
	}
}

// CreateTicket 创建工单
// @Summary 创建工单
// @Description 创建新的客服工单
// @Tags 工单
// @Accept json
// @Produce json
// @Param ticket body services.TicketCreateRequest true "工单信息"
// @Success 201 {object} models.Ticket
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/tickets [post]
func (h *TicketHandler) CreateTicket(c *gin.Context) {
	var req services.TicketCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request body",
			Message: err.Error(),
		})
		return
	}

	ticket, err := h.ticketService.CreateTicket(c.Request.Context(), &req)
	if err != nil {
		h.logger.Errorf("Failed to create ticket: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to create ticket",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, ticket)
}

// GetTicket 获取工单详情
// @Summary 获取工单详情
// @Description 根据ID获取工单的详细信息
// @Tags 工单
// @Accept json
// @Produce json
// @Param id path int true "工单ID"
// @Success 200 {object} models.Ticket
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/tickets/{id} [get]
func (h *TicketHandler) GetTicket(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid ticket ID",
			Message: "ID must be a valid number",
		})
		return
	}

	ticket, err := h.ticketService.GetTicketByID(c.Request.Context(), uint(id))
	if err != nil {
		h.logger.Errorf("Failed to get ticket %d: %v", id, err)
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "Ticket not found",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, ticket)
}

// UpdateTicket 更新工单
// @Summary 更新工单
// @Description 更新工单信息
// @Tags 工单
// @Accept json
// @Produce json
// @Param id path int true "工单ID"
// @Param ticket body services.TicketUpdateRequest true "更新信息"
// @Success 200 {object} models.Ticket
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/tickets/{id} [put]
func (h *TicketHandler) UpdateTicket(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid ticket ID",
			Message: "ID must be a valid number",
		})
		return
	}

	var req services.TicketUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request body",
			Message: err.Error(),
		})
		return
	}

	// 从上下文获取用户ID（假设已经通过中间件设置）
	userID, exists := c.Get("user_id")
	if !exists {
		userID = uint(0) // 系统操作
	}

	ticket, err := h.ticketService.UpdateTicket(c.Request.Context(), uint(id), &req, userID.(uint))
	if err != nil {
		h.logger.Errorf("Failed to update ticket %d: %v", id, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to update ticket",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, ticket)
}

// ListTickets 获取工单列表
// @Summary 获取工单列表
// @Description 获取工单列表，支持分页和过滤
// @Tags 工单
// @Accept json
// @Produce json
// @Param page query int false "页码"
// @Param page_size query int false "每页大小"
// @Param status query []string false "状态过滤"
// @Param priority query []string false "优先级过滤"
// @Param category query []string false "分类过滤"
// @Param agent_id query int false "客服ID过滤"
// @Param customer_id query int false "客户ID过滤"
// @Param search query string false "搜索关键词"
// @Param sort_by query string false "排序字段"
// @Param sort_order query string false "排序方向"
// @Success 200 {object} PaginatedResponse{data=[]models.Ticket}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/tickets [get]
func (h *TicketHandler) ListTickets(c *gin.Context) {
	var req services.TicketListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid query parameters",
			Message: err.Error(),
		})
		return
	}

	tickets, total, err := h.ticketService.ListTickets(c.Request.Context(), &req)
	if err != nil {
		h.logger.Errorf("Failed to list tickets: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to list tickets",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, PaginatedResponse{
		Data:     tickets,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	})
}

// AssignTicket 分配工单
// @Summary 分配工单
// @Description 将工单分配给指定客服
// @Tags 工单
// @Accept json
// @Produce json
// @Param id path int true "工单ID"
// @Param assignment body map[string]uint true "分配信息"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/tickets/{id}/assign [post]
func (h *TicketHandler) AssignTicket(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid ticket ID",
			Message: "ID must be a valid number",
		})
		return
	}

	var req struct {
		AgentID uint `json:"agent_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request body",
			Message: err.Error(),
		})
		return
	}

	// 从上下文获取分配者ID
	assignerID, exists := c.Get("user_id")
	if !exists {
		assignerID = uint(0)
	}

	if err := h.ticketService.AssignTicket(c.Request.Context(), uint(id), req.AgentID, assignerID.(uint)); err != nil {
		h.logger.Errorf("Failed to assign ticket %d to agent %d: %v", id, req.AgentID, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to assign ticket",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "Ticket assigned successfully",
		"ticket_id": id,
		"agent_id":  req.AgentID,
	})
}

// AddComment 添加工单评论
// @Summary 添加工单评论
// @Description 为工单添加评论或内部备注
// @Tags 工单
// @Accept json
// @Produce json
// @Param id path int true "工单ID"
// @Param comment body map[string]string true "评论信息"
// @Success 201 {object} models.TicketComment
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/tickets/{id}/comments [post]
func (h *TicketHandler) AddComment(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid ticket ID",
			Message: "ID must be a valid number",
		})
		return
	}

	var req struct {
		Content string `json:"content" binding:"required"`
		Type    string `json:"type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request body",
			Message: err.Error(),
		})
		return
	}

	// 从上下文获取用户ID
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "Unauthorized",
			Message: "User not authenticated",
		})
		return
	}

	comment, err := h.ticketService.AddComment(c.Request.Context(), uint(id), userID.(uint), req.Content, req.Type)
	if err != nil {
		h.logger.Errorf("Failed to add comment to ticket %d: %v", id, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to add comment",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, comment)
}

// CloseTicket 关闭工单
// @Summary 关闭工单
// @Description 关闭指定的工单
// @Tags 工单
// @Accept json
// @Produce json
// @Param id path int true "工单ID"
// @Param close_info body map[string]string true "关闭信息"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/tickets/{id}/close [post]
func (h *TicketHandler) CloseTicket(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid ticket ID",
			Message: "ID must be a valid number",
		})
		return
	}

	var req struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request body",
			Message: err.Error(),
		})
		return
	}

	// 从上下文获取用户ID
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "Unauthorized",
			Message: "User not authenticated",
		})
		return
	}

	if err := h.ticketService.CloseTicket(c.Request.Context(), uint(id), userID.(uint), req.Reason); err != nil {
		h.logger.Errorf("Failed to close ticket %d: %v", id, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to close ticket",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "Ticket closed successfully",
		"ticket_id": id,
	})
}

// GetTicketStats 获取工单统计
// @Summary 获取工单统计
// @Description 获取工单相关的统计数据
// @Tags 工单
// @Accept json
// @Produce json
// @Param agent_id query int false "客服ID，用于获取特定客服的统计"
// @Success 200 {object} services.TicketStats
// @Failure 500 {object} ErrorResponse
// @Router /api/tickets/stats [get]
func (h *TicketHandler) GetTicketStats(c *gin.Context) {
	var agentID *uint
	if agentIDStr := c.Query("agent_id"); agentIDStr != "" {
		if id, err := strconv.ParseUint(agentIDStr, 10, 32); err == nil {
			agentIDValue := uint(id)
			agentID = &agentIDValue
		}
	}

	stats, err := h.ticketService.GetTicketStats(c.Request.Context(), agentID)
	if err != nil {
		h.logger.Errorf("Failed to get ticket stats: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to get ticket statistics",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// RegisterTicketRoutes 注册工单相关路由
func RegisterTicketRoutes(r *gin.RouterGroup, handler *TicketHandler) {
	tickets := r.Group("/tickets")
	{
		tickets.POST("", handler.CreateTicket)
		tickets.GET("", handler.ListTickets)
		tickets.GET("/stats", handler.GetTicketStats)
		tickets.GET("/:id", handler.GetTicket)
		tickets.PUT("/:id", handler.UpdateTicket)
		tickets.POST("/:id/assign", handler.AssignTicket)
		tickets.POST("/:id/comments", handler.AddComment)
		tickets.POST("/:id/close", handler.CloseTicket)
	}
}
