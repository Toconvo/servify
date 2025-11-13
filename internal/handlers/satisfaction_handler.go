package handlers

import (
	"net/http"
	"strconv"
	"time"

	"servify/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// SatisfactionHandler 客户满意度处理器
type SatisfactionHandler struct {
	satisfactionService *services.SatisfactionService
	logger              *logrus.Logger
}

// NewSatisfactionHandler 创建满意度处理器
func NewSatisfactionHandler(satisfactionService *services.SatisfactionService, logger *logrus.Logger) *SatisfactionHandler {
	return &SatisfactionHandler{
		satisfactionService: satisfactionService,
		logger:              logger,
	}
}

// CreateSatisfaction 创建满意度评价
// @Summary 创建满意度评价
// @Description 为已关闭的工单创建客户满意度评价
// @Tags 满意度评价
// @Accept json
// @Produce json
// @Param satisfaction body services.SatisfactionCreateRequest true "满意度评价信息"
// @Success 201 {object} models.CustomerSatisfaction
// @Failure 400 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/satisfactions [post]
func (h *SatisfactionHandler) CreateSatisfaction(c *gin.Context) {
	var req services.SatisfactionCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request body",
			Message: err.Error(),
		})
		return
	}

	satisfaction, err := h.satisfactionService.CreateSatisfaction(c.Request.Context(), &req)
	if err != nil {
		h.logger.Errorf("Failed to create satisfaction: %v", err)

		// 根据错误类型返回不同状态码
		if err.Error() == "satisfaction rating already exists for this ticket" {
			c.JSON(http.StatusConflict, ErrorResponse{
				Error:   "Satisfaction already exists",
				Message: err.Error(),
			})
			return
		}

		if err.Error() == "customer is not the owner of this ticket" {
			c.JSON(http.StatusForbidden, ErrorResponse{
				Error:   "Access denied",
				Message: err.Error(),
			})
			return
		}

		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to create satisfaction",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, satisfaction)
}

// GetSatisfaction 获取满意度评价详情
// @Summary 获取满意度评价详情
// @Description 根据ID获取满意度评价的详细信息
// @Tags 满意度评价
// @Produce json
// @Param id path int true "满意度评价ID"
// @Success 200 {object} models.CustomerSatisfaction
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/satisfactions/{id} [get]
func (h *SatisfactionHandler) GetSatisfaction(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid satisfaction ID",
			Message: "ID must be a positive integer",
		})
		return
	}

	satisfaction, err := h.satisfactionService.GetSatisfaction(c.Request.Context(), uint(id))
	if err != nil {
		if err.Error() == "satisfaction not found" {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "Satisfaction not found",
				Message: err.Error(),
			})
			return
		}

		h.logger.Errorf("Failed to get satisfaction: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to get satisfaction",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, satisfaction)
}

// ListSatisfactions 获取满意度评价列表
// @Summary 获取满意度评价列表
// @Description 获取满意度评价列表，支持分页和筛选
// @Tags 满意度评价
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页大小" default(20)
// @Param ticket_id query int false "工单ID"
// @Param customer_id query int false "客户ID"
// @Param agent_id query int false "客服ID"
// @Param rating query []int false "评分筛选"
// @Param category query []string false "分类筛选"
// @Param date_from query string false "开始日期 (YYYY-MM-DD)"
// @Param date_to query string false "结束日期 (YYYY-MM-DD)"
// @Param sort_by query string false "排序字段" default(created_at)
// @Param sort_order query string false "排序顺序 (asc/desc)" default(desc)
// @Success 200 {object} PaginatedResponse{data=[]models.CustomerSatisfaction}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/satisfactions [get]
func (h *SatisfactionHandler) ListSatisfactions(c *gin.Context) {
	var req services.SatisfactionListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid query parameters",
			Message: err.Error(),
		})
		return
	}

	// 解析日期参数
	if dateFromStr := c.Query("date_from"); dateFromStr != "" {
		if dateFrom, err := time.Parse("2006-01-02", dateFromStr); err == nil {
			req.DateFrom = &dateFrom
		} else {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "Invalid date_from format",
				Message: "Date must be in YYYY-MM-DD format",
			})
			return
		}
	}

	if dateToStr := c.Query("date_to"); dateToStr != "" {
		if dateTo, err := time.Parse("2006-01-02", dateToStr); err == nil {
			req.DateTo = &dateTo
		} else {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "Invalid date_to format",
				Message: "Date must be in YYYY-MM-DD format",
			})
			return
		}
	}

	satisfactions, total, err := h.satisfactionService.ListSatisfactions(c.Request.Context(), &req)
	if err != nil {
		h.logger.Errorf("Failed to list satisfactions: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to list satisfactions",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, PaginatedResponse{
		Data:     satisfactions,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	})
}

// GetSatisfactionByTicket 根据工单获取满意度评价
// @Summary 根据工单获取满意度评价
// @Description 根据工单ID获取对应的满意度评价
// @Tags 满意度评价
// @Produce json
// @Param ticket_id path int true "工单ID"
// @Success 200 {object} models.CustomerSatisfaction
// @Success 204 "未找到满意度评价"
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/tickets/{ticket_id}/satisfaction [get]
func (h *SatisfactionHandler) GetSatisfactionByTicket(c *gin.Context) {
	ticketIDStr := c.Param("ticket_id")
	ticketID, err := strconv.ParseUint(ticketIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid ticket ID",
			Message: "Ticket ID must be a positive integer",
		})
		return
	}

	satisfaction, err := h.satisfactionService.GetSatisfactionByTicket(c.Request.Context(), uint(ticketID))
	if err != nil {
		h.logger.Errorf("Failed to get satisfaction by ticket: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to get satisfaction",
			Message: err.Error(),
		})
		return
	}

	if satisfaction == nil {
		c.Status(http.StatusNoContent)
		return
	}

	c.JSON(http.StatusOK, satisfaction)
}

// GetSatisfactionStats 获取满意度统计
// @Summary 获取满意度统计
// @Description 获取客户满意度的统计数据，包括平均评分、分布情况和趋势
// @Tags 满意度评价
// @Produce json
// @Param date_from query string false "开始日期 (YYYY-MM-DD)"
// @Param date_to query string false "结束日期 (YYYY-MM-DD)"
// @Success 200 {object} services.SatisfactionStatsResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/satisfactions/stats [get]
func (h *SatisfactionHandler) GetSatisfactionStats(c *gin.Context) {
	var dateFrom, dateTo *time.Time

	// 解析日期参数
	if dateFromStr := c.Query("date_from"); dateFromStr != "" {
		if parsed, err := time.Parse("2006-01-02", dateFromStr); err == nil {
			dateFrom = &parsed
		} else {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "Invalid date_from format",
				Message: "Date must be in YYYY-MM-DD format",
			})
			return
		}
	}

	if dateToStr := c.Query("date_to"); dateToStr != "" {
		if parsed, err := time.Parse("2006-01-02", dateToStr); err == nil {
			dateTo = &parsed
		} else {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "Invalid date_to format",
				Message: "Date must be in YYYY-MM-DD format",
			})
			return
		}
	}

	stats, err := h.satisfactionService.GetSatisfactionStats(c.Request.Context(), dateFrom, dateTo)
	if err != nil {
		h.logger.Errorf("Failed to get satisfaction stats: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to get satisfaction statistics",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// UpdateSatisfaction 更新满意度评价
// @Summary 更新满意度评价
// @Description 更新满意度评价的评论（仅允许更新评论内容）
// @Tags 满意度评价
// @Accept json
// @Produce json
// @Param id path int true "满意度评价ID"
// @Param request body object{comment=string} true "更新请求"
// @Success 200 {object} models.CustomerSatisfaction
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/satisfactions/{id} [put]
func (h *SatisfactionHandler) UpdateSatisfaction(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid satisfaction ID",
			Message: "ID must be a positive integer",
		})
		return
	}

	var req struct {
		Comment string `json:"comment"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request body",
			Message: err.Error(),
		})
		return
	}

	satisfaction, err := h.satisfactionService.UpdateSatisfaction(c.Request.Context(), uint(id), req.Comment)
	if err != nil {
		if err.Error() == "satisfaction not found" {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "Satisfaction not found",
				Message: err.Error(),
			})
			return
		}

		h.logger.Errorf("Failed to update satisfaction: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to update satisfaction",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, satisfaction)
}

// DeleteSatisfaction 删除满意度评价
// @Summary 删除满意度评价
// @Description 删除指定的满意度评价（管理员功能）
// @Tags 满意度评价
// @Param id path int true "满意度评价ID"
// @Success 204 "删除成功"
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/satisfactions/{id} [delete]
func (h *SatisfactionHandler) DeleteSatisfaction(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid satisfaction ID",
			Message: "ID must be a positive integer",
		})
		return
	}

	err = h.satisfactionService.DeleteSatisfaction(c.Request.Context(), uint(id))
	if err != nil {
		if err.Error() == "satisfaction not found" {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "Satisfaction not found",
				Message: err.Error(),
			})
			return
		}

		h.logger.Errorf("Failed to delete satisfaction: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to delete satisfaction",
			Message: err.Error(),
		})
		return
	}

	c.Status(http.StatusNoContent)
}

// RegisterSatisfactionRoutes 注册满意度评价相关路由
func RegisterSatisfactionRoutes(r *gin.RouterGroup, handler *SatisfactionHandler) {
	satisfactions := r.Group("/satisfactions")
	{
		satisfactions.POST("", handler.CreateSatisfaction)
		satisfactions.GET("", handler.ListSatisfactions)
		satisfactions.GET("/stats", handler.GetSatisfactionStats)
		satisfactions.GET("/:id", handler.GetSatisfaction)
		satisfactions.PUT("/:id", handler.UpdateSatisfaction)
		satisfactions.DELETE("/:id", handler.DeleteSatisfaction)
	}

	// 工单相关的满意度路由
	tickets := r.Group("/tickets")
	{
		tickets.GET("/:ticket_id/satisfaction", handler.GetSatisfactionByTicket)
	}
}