package handlers

import (
	"net/http"
	"strconv"

	"servify/apps/server/internal/services"

	"github.com/gin-gonic/gin"
)

type KnowledgeDocHandler struct {
	service *services.KnowledgeDocService
}

func NewKnowledgeDocHandler(service *services.KnowledgeDocService) *KnowledgeDocHandler {
	return &KnowledgeDocHandler{service: service}
}

func (h *KnowledgeDocHandler) List(c *gin.Context) {
	var req services.KnowledgeDocListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid query parameters", Message: err.Error()})
		return
	}
	docs, total, err := h.service.List(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to list knowledge docs", Message: err.Error()})
		return
	}
	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	c.JSON(http.StatusOK, PaginatedResponse{
		Data:     docs,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

func (h *KnowledgeDocHandler) Get(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid id", Message: err.Error()})
		return
	}
	doc, err := h.service.Get(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "Knowledge doc not found", Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, doc)
}

func (h *KnowledgeDocHandler) Create(c *gin.Context) {
	var req services.KnowledgeDocCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid request", Message: err.Error()})
		return
	}
	doc, err := h.service.Create(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Failed to create knowledge doc", Message: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, doc)
}

func (h *KnowledgeDocHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid id", Message: err.Error()})
		return
	}
	var req services.KnowledgeDocUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid request", Message: err.Error()})
		return
	}
	doc, err := h.service.Update(c.Request.Context(), uint(id), &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Failed to update knowledge doc", Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, doc)
}

func (h *KnowledgeDocHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid id", Message: err.Error()})
		return
	}
	if err := h.service.Delete(c.Request.Context(), uint(id)); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Failed to delete knowledge doc", Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, SuccessResponse{Message: "deleted"})
}

func RegisterKnowledgeDocRoutes(r *gin.RouterGroup, handler *KnowledgeDocHandler) {
	docs := r.Group("/knowledge-docs")
	{
		docs.GET("", handler.List)
		docs.GET("/:id", handler.Get)
		docs.POST("", handler.Create)
		docs.PUT("/:id", handler.Update)
		docs.DELETE("/:id", handler.Delete)
	}
}

func RegisterPublicKnowledgeBaseRoutes(r *gin.RouterGroup, handler *KnowledgeDocHandler) {
	kb := r.Group("/kb")
	{
		kb.GET("/docs", handler.List)
		kb.GET("/docs/:id", handler.Get)
	}
}
