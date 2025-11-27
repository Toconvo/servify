package handlers

import (
	"net/http"
	"strconv"

	"servify/apps/server/internal/services"

	"github.com/gin-gonic/gin"
)

// AutomationHandler 管理自动化触发器
// 说明：当前版本提供最小 CRUD，动作/条件由前端传递 JSON。
type AutomationHandler struct {
	service *services.AutomationService
}

func NewAutomationHandler(service *services.AutomationService) *AutomationHandler {
	return &AutomationHandler{service: service}
}

// ListTriggers 获取触发器列表
func (h *AutomationHandler) ListTriggers(c *gin.Context) {
	triggers, err := h.service.ListTriggers(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to list triggers", Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, triggers)
}

// CreateTrigger 创建触发器
func (h *AutomationHandler) CreateTrigger(c *gin.Context) {
	var req services.AutomationTriggerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid request", Message: err.Error()})
		return
	}

	trigger, err := h.service.CreateTrigger(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Failed to create trigger", Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, trigger)
}

// DeleteTrigger 删除触发器
func (h *AutomationHandler) DeleteTrigger(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid id", Message: err.Error()})
		return
	}

	if err := h.service.DeleteTrigger(c.Request.Context(), uint(id)); err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "trigger not found" {
			status = http.StatusNotFound
		}
		c.JSON(status, ErrorResponse{Error: "Failed to delete trigger", Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{Message: "deleted"})
}

// RegisterAutomationRoutes 注册路由
func RegisterAutomationRoutes(r *gin.RouterGroup, handler *AutomationHandler) {
	auto := r.Group("/automations")
	{
		auto.GET("", handler.ListTriggers)
		auto.POST("", handler.CreateTrigger)
		auto.DELETE(":id", handler.DeleteTrigger)
	}
}
