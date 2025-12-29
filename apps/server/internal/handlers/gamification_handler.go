package handlers

import (
	"net/http"
	"strconv"
	"time"

	"servify/apps/server/internal/services"

	"github.com/gin-gonic/gin"
)

type GamificationHandler struct {
	service *services.GamificationService
}

func NewGamificationHandler(service *services.GamificationService) *GamificationHandler {
	return &GamificationHandler{service: service}
}

// GetLeaderboard returns leaderboard based on resolved tickets + CSAT + response time.
// Query:
// - start_date/end_date: YYYY-MM-DD (optional if days provided)
// - days: int (default 7) when start/end omitted
// - limit: int (default 10)
// - department: string
func (h *GamificationHandler) GetLeaderboard(c *gin.Context) {
	limit := parseIntQuery(c, "limit", 10)
	days := parseIntQuery(c, "days", 7)
	dept := c.Query("department")

	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")

	var (
		start time.Time
		end   time.Time
		err   error
	)
	if startDateStr != "" && endDateStr != "" {
		start, err = time.Parse("2006-01-02", startDateStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid start_date format", Message: "Use YYYY-MM-DD format"})
			return
		}
		end, err = time.Parse("2006-01-02", endDateStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid end_date format", Message: "Use YYYY-MM-DD format"})
			return
		}
		// make end inclusive by adding almost a day
		end = end.Add(24*time.Hour - time.Nanosecond)
	} else {
		if days <= 0 {
			days = 7
		}
		if days > 365 {
			days = 365
		}
		end = time.Now()
		start = end.AddDate(0, 0, -days)
	}

	resp, err := h.service.GetLeaderboard(c.Request.Context(), &services.LeaderboardRequest{
		StartDate:  start,
		EndDate:    end,
		Limit:      limit,
		Department: dept,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Failed to get leaderboard", Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func RegisterGamificationRoutes(r *gin.RouterGroup, handler *GamificationHandler) {
	g := r.Group("/gamification")
	{
		g.GET("/leaderboard", handler.GetLeaderboard)
	}
}

func parseIntQuery(c *gin.Context, key string, def int) int {
	v := c.Query(key)
	if v == "" {
		return def
	}
	if n, err := strconv.Atoi(v); err == nil {
		return n
	}
	return def
}
