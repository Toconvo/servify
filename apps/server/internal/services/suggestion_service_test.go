package services

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"servify/apps/server/internal/models"
)

func newSuggestionServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	name := t.Name()
	dsn := "file:suggestion_" + name + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Ticket{},
		&models.KnowledgeDoc{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func TestSuggestionService_Suggest_EmptyQuery(t *testing.T) {
	db := newSuggestionServiceTestDB(t)
	svc := NewSuggestionService(db)

	req := &SuggestionRequest{
		Query: "",
	}

	resp, err := svc.Suggest(context.Background(), req)
	if err != nil {
		t.Fatalf("Suggest() error = %v", err)
	}
	if resp == nil {
		t.Fatal("expected response, got nil")
	}
	if resp.Intent.Label != "general" {
		t.Errorf("expected intent 'general', got '%s'", resp.Intent.Label)
	}
}

func TestSuggestionService_Suggest_WithTickets(t *testing.T) {
	db := newSuggestionServiceTestDB(t)
	svc := NewSuggestionService(db)

	// Create test tickets
	ticket := &models.Ticket{
		Title:       "Login error",
		Description: "User cannot login to the system",
		Status:      "open",
		Category:    "technical",
		Priority:    "high",
	}
	db.Create(ticket)

	req := &SuggestionRequest{
		Query: "login",
	}

	resp, err := svc.Suggest(context.Background(), req)
	if err != nil {
		t.Fatalf("Suggest() error = %v", err)
	}
	if resp == nil {
		t.Fatal("expected response, got nil")
	}
}

func TestSuggestionService_Suggest_WithKnowledgeDocs(t *testing.T) {
	db := newSuggestionServiceTestDB(t)
	svc := NewSuggestionService(db)

	// Create test doc
	doc := &models.KnowledgeDoc{
		Title:    "API Guide",
		Content:  "How to use the API",
		Category: "Technical",
		Tags:     "api,guide",
	}
	db.Create(doc)

	req := &SuggestionRequest{
		Query: "api guide",
	}

	resp, err := svc.Suggest(context.Background(), req)
	if err != nil {
		t.Fatalf("Suggest() error = %v", err)
	}
	if resp == nil {
		t.Fatal("expected response, got nil")
	}
}

func TestSuggestionService_Suggest_CustomLimits(t *testing.T) {
	db := newSuggestionServiceTestDB(t)
	svc := NewSuggestionService(db)

	req := &SuggestionRequest{
		Query:             "test",
		TicketLimit:       10,
		KnowledgeDocLimit: 15,
		CandidateTicketMax: 500,
	}

	resp, err := svc.Suggest(context.Background(), req)
	if err != nil {
		t.Fatalf("Suggest() error = %v", err)
	}
	if resp == nil {
		t.Fatal("expected response, got nil")
	}
}

func TestExtractTokens(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected []string
	}{
		{
			name:     "empty query",
			query:    "",
			expected: nil,
		},
		{
			name:     "whitespace query",
			query:    "   ",
			expected: nil,
		},
		{
			name:     "simple words",
			query:    "hello world",
			expected: []string{"hello", "world"},
		},
		{
			name:     "chinese characters - single char tokens",
			query:    "测试查询",
			expected: []string{"测", "试", "查", "询"},
		},
		{
			name:     "mixed",
			query:    "test 测试 api",
			expected: []string{"test", "测", "试", "api"},
		},
		{
			name:     "duplicates",
			query:    "test test api",
			expected: []string{"test", "api"},
		},
		{
			name:     "long token filtered",
			query:    "short " + string(make([]byte, 35)) + " another",
			expected: []string{"short", "another"}, // long token filtered out
		},
		{
			name:     "max tokens",
			query:    "one two three four five six seven eight nine ten eleven twelve thirteen fourteen fifteen sixteen seventeen eighteen nineteen twenty twentyone",
			expected: []string{"one", "two", "three", "four", "five", "six", "seven", "eight", "nine", "ten", "eleven", "twelve", "thirteen", "fourteen", "fifteen", "sixteen", "seventeen", "eighteen", "nineteen", "twenty"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractTokens(tt.query)
			if tt.expected == nil {
				if result != nil {
					t.Errorf("extractTokens(%q) = %v, want nil", tt.query, result)
				}
				return
			}
			if len(result) != len(tt.expected) {
				t.Errorf("extractTokens(%q) length = %d, want %d", tt.query, len(result), len(tt.expected))
				return
			}
			for i, exp := range tt.expected {
				if result[i] != exp {
					t.Errorf("extractTokens(%q)[%d] = %q, want %q", tt.query, i, result[i], exp)
				}
			}
		})
	}
}

func TestClassifyIntent(t *testing.T) {
	tests := []struct {
		name             string
		query            string
		expectedLabel    string
		minConfidence    float64
	}{
		{
			name:          "empty query",
			query:         "",
			expectedLabel: "general",
			minConfidence: 0.2,
		},
		{
			name:          "complaint chinese",
			query:         "我要投诉",
			expectedLabel: "complaint",
			minConfidence: 0.6,
		},
		{
			name:          "complaint english",
			query:         "I want to complaint",
			expectedLabel: "complaint",
			minConfidence: 0.6,
		},
		{
			name:          "billing chinese",
			query:         "怎么开发票",
			expectedLabel: "billing",
			minConfidence: 0.6,
		},
		{
			name:          "billing english",
			query:         "Need invoice",
			expectedLabel: "billing",
			minConfidence: 0.6,
		},
		{
			name:          "technical chinese",
			query:         "系统报错",
			expectedLabel: "technical",
			minConfidence: 0.6,
		},
		{
			name:          "technical english",
			query:         "System error",
			expectedLabel: "technical",
			minConfidence: 0.6,
		},
		{
			name:          "general query",
			query:         "Hello world",
			expectedLabel: "general",
			minConfidence: 0.3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyIntent(tt.query)
			if result.Label != tt.expectedLabel {
				t.Errorf("classifyIntent(%q) label = %q, want %q", tt.query, result.Label, tt.expectedLabel)
			}
			if result.Confidence < tt.minConfidence {
				t.Errorf("classifyIntent(%q) confidence = %f, want >= %f", tt.query, result.Confidence, tt.minConfidence)
			}
		})
	}
}

func TestScoreText(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		text     string
		expected float64
	}{
		{
			name:     "exact match",
			query:    "hello world",
			text:     "hello world",
			expected: 1.0,
		},
		{
			name:     "partial match",
			query:    "hello world",
			text:     "hello there",
			expected: 0.5,
		},
		{
			name:     "no match",
			query:    "hello",
			text:     "world",
			expected: 0.0,
		},
		{
			name:     "empty query",
			query:    "",
			text:     "hello",
			expected: 0.0,
		},
		{
			name:     "empty text",
			query:    "hello",
			text:     "",
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scoreText(tt.query, tt.text)
			if result != tt.expected {
				t.Errorf("scoreText(%q, %q) = %f, want %f", tt.query, tt.text, result, tt.expected)
			}
		})
	}
}

func TestBuildLikeWhereTokens(t *testing.T) {
	tests := []struct {
		name      string
		fields    []string
		tokens    []string
		maxTokens int
		wantEmpty bool
	}{
		{
			name:      "empty fields",
			fields:    []string{},
			tokens:    []string{"test"},
			maxTokens: 3,
			wantEmpty: true,
		},
		{
			name:      "empty tokens",
			fields:    []string{"title"},
			tokens:    []string{},
			maxTokens: 3,
			wantEmpty: true,
		},
		{
			name:      "zero max tokens",
			fields:    []string{"title"},
			tokens:    []string{"test"},
			maxTokens: 0,
			wantEmpty: true,
		},
		{
			name:      "valid",
			fields:    []string{"title", "content"},
			tokens:    []string{"test", "api"},
			maxTokens: 3,
			wantEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			where, args := buildLikeWhereTokens(tt.fields, tt.tokens, tt.maxTokens)
			isEmpty := where == "" || len(args) == 0
			if isEmpty != tt.wantEmpty {
				t.Errorf("buildLikeWhereTokens() isEmpty = %v, want %v", isEmpty, tt.wantEmpty)
			}
		})
	}
}
