package handlers

import (
	"net/http"
	"strconv"

	"servify/apps/server/internal/services"

	"github.com/gin-gonic/gin"
)

type SuggestionHandler struct {
	service *services.SuggestionService
}

func NewSuggestionHandler(service *services.SuggestionService) *SuggestionHandler {
	return &SuggestionHandler{service: service}
}

func (h *SuggestionHandler) Suggest(c *gin.Context) {
	query := c.Query("query")
	limit := parseIntDefault(c.Query("limit"), 5)
	docLimit := parseIntDefault(c.Query("doc_limit"), 5)

	resp, err := h.service.Suggest(c.Request.Context(), &services.SuggestionRequest{
		Query:             query,
		TicketLimit:       limit,
		KnowledgeDocLimit: docLimit,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to suggest", Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    resp,
	})
}

func (h *SuggestionHandler) SuggestPost(c *gin.Context) {
	var req services.SuggestionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid request", Message: err.Error()})
		return
	}
	resp, err := h.service.Suggest(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to suggest", Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    resp,
	})
}

func RegisterSuggestionRoutes(r *gin.RouterGroup, handler *SuggestionHandler) {
	assist := r.Group("/assist")
	{
		assist.GET("/suggest", handler.Suggest)
		assist.POST("/suggest", handler.SuggestPost)
	}
}

func parseIntDefault(v string, def int) int {
	if v == "" {
		return def
	}
	if n, err := strconv.Atoi(v); err == nil {
		return n
	}
	return def
}
