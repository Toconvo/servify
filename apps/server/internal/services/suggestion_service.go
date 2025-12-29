package services

import (
	"context"
	"regexp"
	"sort"
	"strings"
	"time"

	"servify/apps/server/internal/models"

	"gorm.io/gorm"
)

type SuggestionService struct {
	db *gorm.DB
}

func NewSuggestionService(db *gorm.DB) *SuggestionService {
	return &SuggestionService{db: db}
}

type IntentSuggestion struct {
	Label      string   `json:"label"`
	Confidence float64  `json:"confidence"`
	Matches    []string `json:"matches,omitempty"`
}

type TicketSuggestion struct {
	ID        uint      `json:"id"`
	Title     string    `json:"title"`
	Status    string    `json:"status"`
	Category  string    `json:"category"`
	Priority  string    `json:"priority"`
	CreatedAt time.Time `json:"created_at"`
	Score     float64   `json:"score"`
}

type KnowledgeDocSuggestion struct {
	ID       uint    `json:"id"`
	Title    string  `json:"title"`
	Category string  `json:"category"`
	Tags     string  `json:"tags"`
	Score    float64 `json:"score"`
}

type SuggestionResponse struct {
	Query          string                   `json:"query"`
	Intent         IntentSuggestion         `json:"intent"`
	SimilarTickets []TicketSuggestion       `json:"similar_tickets"`
	KnowledgeDocs  []KnowledgeDocSuggestion `json:"knowledge_docs"`
	Meta           map[string]interface{}   `json:"meta,omitempty"`
}

type SuggestionRequest struct {
	Query              string `json:"query" binding:"required"`
	TicketLimit        int    `json:"ticket_limit"`
	KnowledgeDocLimit  int    `json:"knowledge_doc_limit"`
	CandidateTicketMax int    `json:"candidate_ticket_max"`
}

var tokenRe = regexp.MustCompile(`[\p{Han}]|[A-Za-z0-9_]+`)

func (s *SuggestionService) Suggest(ctx context.Context, req *SuggestionRequest) (*SuggestionResponse, error) {
	query := ""
	ticketLimit := 5
	docLimit := 5
	candidateMax := 200
	if req != nil {
		query = strings.TrimSpace(req.Query)
		if req.TicketLimit > 0 {
			ticketLimit = req.TicketLimit
		}
		if req.KnowledgeDocLimit > 0 {
			docLimit = req.KnowledgeDocLimit
		}
		if req.CandidateTicketMax > 0 {
			candidateMax = req.CandidateTicketMax
		}
	}
	if ticketLimit > 20 {
		ticketLimit = 20
	}
	if docLimit > 20 {
		docLimit = 20
	}
	if candidateMax > 1000 {
		candidateMax = 1000
	}

	intent := classifyIntent(query)
	tokens := extractTokens(query)

	tickets, ticketMeta, err := s.suggestTickets(ctx, query, tokens, ticketLimit, candidateMax)
	if err != nil {
		return nil, err
	}
	docs, docMeta, err := s.suggestKnowledgeDocs(ctx, query, tokens, docLimit)
	if err != nil {
		return nil, err
	}

	meta := map[string]interface{}{
		"tokens":            tokens,
		"ticket_candidates": ticketMeta["candidates"],
		"doc_candidates":    docMeta["candidates"],
	}

	return &SuggestionResponse{
		Query:          query,
		Intent:         intent,
		SimilarTickets: tickets,
		KnowledgeDocs:  docs,
		Meta:           meta,
	}, nil
}

type ticketCandidateRow struct {
	ID          uint
	Title       string
	Description string
	Status      string
	Category    string
	Priority    string
	CreatedAt   time.Time
}

func (s *SuggestionService) suggestTickets(ctx context.Context, query string, tokens []string, limit int, candidateMax int) ([]TicketSuggestion, map[string]interface{}, error) {
	q := s.db.WithContext(ctx).Model(&models.Ticket{}).
		Select("id, title, description, status, category, priority, created_at").
		Order("created_at DESC")

	where, args := buildLikeWhereTokens([]string{"title", "description"}, tokens, 3)
	if where != "" {
		q = q.Where(where, args...)
	}

	var rows []ticketCandidateRow
	if err := q.Limit(candidateMax).Scan(&rows).Error; err != nil {
		return nil, nil, err
	}

	sugs := make([]TicketSuggestion, 0, len(rows))
	for _, r := range rows {
		score := scoreText(query, r.Title+" "+r.Description)
		if score <= 0 {
			continue
		}
		sugs = append(sugs, TicketSuggestion{
			ID:        r.ID,
			Title:     r.Title,
			Status:    r.Status,
			Category:  r.Category,
			Priority:  r.Priority,
			CreatedAt: r.CreatedAt,
			Score:     score,
		})
	}

	sort.Slice(sugs, func(i, j int) bool {
		if sugs[i].Score == sugs[j].Score {
			return sugs[i].CreatedAt.After(sugs[j].CreatedAt)
		}
		return sugs[i].Score > sugs[j].Score
	})
	if len(sugs) > limit {
		sugs = sugs[:limit]
	}
	return sugs, map[string]interface{}{"candidates": len(rows)}, nil
}

type docCandidateRow struct {
	ID       uint
	Title    string
	Content  string
	Category string
	Tags     string
}

func (s *SuggestionService) suggestKnowledgeDocs(ctx context.Context, query string, tokens []string, limit int) ([]KnowledgeDocSuggestion, map[string]interface{}, error) {
	q := s.db.WithContext(ctx).Model(&models.KnowledgeDoc{}).
		Select("id, title, content, category, tags").
		Order("created_at DESC")

	where, args := buildLikeWhereTokens([]string{"title", "content", "tags"}, tokens, 3)
	if where != "" {
		q = q.Where(where, args...)
	}

	var rows []docCandidateRow
	if err := q.Limit(300).Scan(&rows).Error; err != nil {
		return nil, nil, err
	}

	sugs := make([]KnowledgeDocSuggestion, 0, len(rows))
	for _, r := range rows {
		score := scoreText(query, r.Title+" "+r.Content+" "+r.Tags)
		if score <= 0 {
			continue
		}
		sugs = append(sugs, KnowledgeDocSuggestion{
			ID:       r.ID,
			Title:    r.Title,
			Category: r.Category,
			Tags:     r.Tags,
			Score:    score,
		})
	}

	sort.Slice(sugs, func(i, j int) bool {
		if sugs[i].Score == sugs[j].Score {
			return sugs[i].ID < sugs[j].ID
		}
		return sugs[i].Score > sugs[j].Score
	})
	if len(sugs) > limit {
		sugs = sugs[:limit]
	}
	return sugs, map[string]interface{}{"candidates": len(rows)}, nil
}

func extractTokens(query string) []string {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil
	}
	matches := tokenRe.FindAllString(query, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(matches))
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		m = strings.ToLower(strings.TrimSpace(m))
		if m == "" {
			continue
		}
		if len([]rune(m)) >= 32 {
			continue
		}
		if _, ok := seen[m]; ok {
			continue
		}
		seen[m] = struct{}{}
		out = append(out, m)
		if len(out) >= 20 {
			break
		}
	}
	return out
}

func buildLikeWhereTokens(fields []string, tokens []string, maxTokens int) (string, []interface{}) {
	if len(fields) == 0 || len(tokens) == 0 || maxTokens <= 0 {
		return "", nil
	}
	var parts []string
	var args []interface{}
	used := 0
	for _, t := range tokens {
		if used >= maxTokens {
			break
		}
		if t == "" {
			continue
		}
		like := "%" + t + "%"
		var sub []string
		for _, f := range fields {
			sub = append(sub, f+" LIKE ?")
			args = append(args, like)
		}
		parts = append(parts, "("+strings.Join(sub, " OR ")+")")
		used++
	}
	if len(parts) == 0 {
		return "", nil
	}
	return strings.Join(parts, " OR "), args
}

func scoreText(query string, text string) float64 {
	qTokens := extractTokens(query)
	if len(qTokens) == 0 {
		return 0
	}
	tTokens := extractTokens(text)
	if len(tTokens) == 0 {
		return 0
	}
	set := make(map[string]struct{}, len(tTokens))
	for _, t := range tTokens {
		set[t] = struct{}{}
	}
	hits := 0
	for _, qt := range qTokens {
		if _, ok := set[qt]; ok {
			hits++
		}
	}
	if hits == 0 {
		return 0
	}
	return float64(hits) / float64(len(qTokens))
}

func classifyIntent(query string) IntentSuggestion {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return IntentSuggestion{Label: "general", Confidence: 0.2}
	}
	type bucket struct {
		label    string
		keywords []string
	}
	buckets := []bucket{
		{label: "complaint", keywords: []string{"投诉", "差评", "不满", "生气", "愤怒", "骗子", "垃圾", "坑", "投诉", "complaint", "angry"}},
		{label: "billing", keywords: []string{"发票", "付款", "支付", "收费", "价格", "账单", "退款", "billing", "invoice", "payment", "refund"}},
		{label: "technical", keywords: []string{"报错", "错误", "bug", "无法", "失败", "异常", "error", "crash", "连接", "登录", "超时", "timeout"}},
	}
	bestLabel := "general"
	bestHits := 0
	var matches []string
	for _, b := range buckets {
		hits := 0
		var m []string
		for _, kw := range b.keywords {
			if kw == "" {
				continue
			}
			if strings.Contains(q, strings.ToLower(kw)) {
				hits++
				m = append(m, kw)
			}
		}
		if hits > bestHits {
			bestHits = hits
			bestLabel = b.label
			matches = m
		}
	}
	conf := 0.35
	if bestLabel == "general" {
		conf = 0.35
	} else {
		conf = 0.6 + 0.1*float64(bestHits-1)
		if conf > 0.95 {
			conf = 0.95
		}
	}
	return IntentSuggestion{Label: bestLabel, Confidence: conf, Matches: matches}
}
