package services

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"servify/apps/server/internal/models"

	"gorm.io/gorm"
)

type GamificationService struct {
	db *gorm.DB
}

func NewGamificationService(db *gorm.DB) *GamificationService {
	return &GamificationService{db: db}
}

type Badge struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type LeaderboardEntry struct {
	Rank            int     `json:"rank"`
	AgentID         uint    `json:"agent_id"`
	AgentName       string  `json:"agent_name"`
	Username        string  `json:"username"`
	Department      string  `json:"department"`
	ResolvedTickets int64   `json:"resolved_tickets"`
	CSATAvg         float64 `json:"csat_avg"`
	CSATCount       int64   `json:"csat_count"`
	AvgResponseTime int64   `json:"avg_response_time"` // seconds
	Score           float64 `json:"score"`
	Badges          []Badge `json:"badges,omitempty"`
}

type LeaderboardRequest struct {
	StartDate  time.Time
	EndDate    time.Time
	Limit      int
	Department string
}

type LeaderboardResponse struct {
	StartDate string             `json:"start_date"`
	EndDate   string             `json:"end_date"`
	Limit     int                `json:"limit"`
	Entries   []LeaderboardEntry `json:"entries"`
}

type agentProfileRow struct {
	UserID          uint
	Username        string
	Name            string
	Department      string
	AvgResponseTime int64
}

type agentAggRow struct {
	AgentID uint
	Value   float64
	Count   int64
}

func (s *GamificationService) GetLeaderboard(ctx context.Context, req *LeaderboardRequest) (*LeaderboardResponse, error) {
	if s.db == nil {
		return nil, errors.New("db not configured")
	}
	if req == nil {
		return nil, errors.New("request required")
	}

	start := req.StartDate
	end := req.EndDate
	if start.IsZero() || end.IsZero() {
		return nil, errors.New("start_date and end_date required")
	}
	if end.Before(start) {
		return nil, errors.New("end_date must be after start_date")
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	agentsQ := s.db.WithContext(ctx).
		Model(&models.Agent{}).
		Select("agents.user_id as user_id, users.username as username, users.name as name, agents.department as department, agents.avg_response_time as avg_response_time").
		Joins("LEFT JOIN users ON users.id = agents.user_id")
	if dept := strings.TrimSpace(req.Department); dept != "" {
		agentsQ = agentsQ.Where("agents.department = ?", dept)
	}
	var profiles []agentProfileRow
	if err := agentsQ.Find(&profiles).Error; err != nil {
		return nil, err
	}

	// Aggregate resolved tickets in range.
	var resolved []agentAggRow
	if err := s.db.WithContext(ctx).
		Model(&models.Ticket{}).
		Select("agent_id as agent_id, COUNT(*) as count").
		Where("agent_id IS NOT NULL").
		Where("resolved_at IS NOT NULL").
		Where("resolved_at >= ? AND resolved_at <= ?", start, end).
		Where("status IN ?", []string{"resolved", "closed"}).
		Group("agent_id").
		Scan(&resolved).Error; err != nil {
		return nil, err
	}
	resolvedByAgent := make(map[uint]int64, len(resolved))
	for _, r := range resolved {
		resolvedByAgent[r.AgentID] = r.Count
	}

	// Aggregate CSAT in range.
	type csatRow struct {
		AgentID uint
		Avg     float64
		Count   int64
	}
	var csats []csatRow
	if err := s.db.WithContext(ctx).
		Model(&models.CustomerSatisfaction{}).
		Select("agent_id as agent_id, AVG(rating) as avg, COUNT(*) as count").
		Where("agent_id IS NOT NULL").
		Where("created_at >= ? AND created_at <= ?", start, end).
		Group("agent_id").
		Scan(&csats).Error; err != nil {
		return nil, err
	}
	csatByAgent := make(map[uint]csatRow, len(csats))
	for _, c := range csats {
		csatByAgent[c.AgentID] = c
	}

	entries := make([]LeaderboardEntry, 0, len(profiles))
	for _, p := range profiles {
		rc := resolvedByAgent[p.UserID]
		csat := csatByAgent[p.UserID]
		if rc == 0 && csat.Count == 0 {
			continue
		}
		score := computeGamificationScore(rc, csat.Avg, csat.Count, p.AvgResponseTime)
		entries = append(entries, LeaderboardEntry{
			AgentID:         p.UserID,
			AgentName:       p.Name,
			Username:        p.Username,
			Department:      p.Department,
			ResolvedTickets: rc,
			CSATAvg:         csat.Avg,
			CSATCount:       csat.Count,
			AvgResponseTime: p.AvgResponseTime,
			Score:           score,
		})
	}

	applyBadges(entries)

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Score == entries[j].Score {
			if entries[i].ResolvedTickets == entries[j].ResolvedTickets {
				return entries[i].AgentID < entries[j].AgentID
			}
			return entries[i].ResolvedTickets > entries[j].ResolvedTickets
		}
		return entries[i].Score > entries[j].Score
	})

	if len(entries) > limit {
		entries = entries[:limit]
	}
	for i := range entries {
		entries[i].Rank = i + 1
	}

	return &LeaderboardResponse{
		StartDate: start.Format("2006-01-02"),
		EndDate:   end.Format("2006-01-02"),
		Limit:     limit,
		Entries:   entries,
	}, nil
}

func computeGamificationScore(resolved int64, csatAvg float64, csatCount int64, avgResponseTimeSec int64) float64 {
	// MVP scoring:
	// - ticket throughput dominates
	// - CSAT provides strong boost (but discounted if too few samples)
	// - response time gives a small penalty
	csatWeight := 20.0
	if csatCount < 3 {
		csatWeight = 10.0
	}
	respPenalty := 0.0
	if avgResponseTimeSec > 0 {
		respPenalty = float64(avgResponseTimeSec) * 0.03
	}
	return float64(resolved)*10.0 + csatAvg*csatWeight - respPenalty
}

func applyBadges(entries []LeaderboardEntry) {
	if len(entries) == 0 {
		return
	}
	// Resolve badges based on the full candidate set (before limit cut).
	var (
		bestResolvedID uint
		bestResolved   int64

		bestCSATID uint
		bestCSAT   float64
		bestCSATN  int64

		bestSpeedID uint
		bestSpeed   int64
	)
	for _, e := range entries {
		if e.ResolvedTickets > bestResolved {
			bestResolved = e.ResolvedTickets
			bestResolvedID = e.AgentID
		}
		if e.CSATCount >= 3 {
			if e.CSATAvg > bestCSAT || (e.CSATAvg == bestCSAT && e.CSATCount > bestCSATN) {
				bestCSAT = e.CSATAvg
				bestCSATN = e.CSATCount
				bestCSATID = e.AgentID
			}
		}
		if e.AvgResponseTime > 0 {
			if bestSpeed == 0 || e.AvgResponseTime < bestSpeed {
				bestSpeed = e.AvgResponseTime
				bestSpeedID = e.AgentID
			}
		}
	}

	for i := range entries {
		var badges []Badge
		if entries[i].AgentID == bestResolvedID && bestResolved > 0 {
			badges = append(badges, Badge{
				ID:          "top_resolver",
				Name:        "解决王",
				Description: "本周期解决工单数最高",
			})
		}
		if entries[i].AgentID == bestCSATID && bestCSATID != 0 {
			badges = append(badges, Badge{
				ID:          "customer_hero",
				Name:        "满意之星",
				Description: "本周期 CSAT 均分最高（至少 3 条评价）",
			})
		}
		if entries[i].AgentID == bestSpeedID && bestSpeedID != 0 {
			badges = append(badges, Badge{
				ID:          "speedster",
				Name:        "极速响应",
				Description: "本周期平均响应时长最低（基于 Agent.avg_response_time）",
			})
		}
		entries[i].Badges = badges
	}
}
