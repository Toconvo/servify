package services

import (
	"context"
	"testing"
	"time"

	"servify/apps/server/internal/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newGamificationTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Agent{},
		&models.User{},
		&models.Ticket{},
		&models.CustomerSatisfaction{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func createTestAgent(db *gorm.DB, userID uint, name, department string, avgResponseTimeSec int) error {
	user := &models.User{
		ID:       userID,
		Username: "user_" + name,
		Email:    name + "@test.com",
		Name:     name,
		Role:     "agent",
	}
	if err := db.Create(user).Error; err != nil {
		return err
	}
	agent := &models.Agent{
		UserID:          userID,
		Department:      department,
		AvgResponseTime: avgResponseTimeSec,
		Status:          "online",
	}
	return db.Create(agent).Error
}

func TestGamificationService_GetLeaderboard(t *testing.T) {
	db := newGamificationTestDB(t)
	svc := NewGamificationService(db)

	// 创建测试用户和客服
	start := time.Now().Add(-48 * time.Hour)
	end := time.Now()

	_ = createTestAgent(db, 1, "张三", "技术支持", 300)  // 快速响应
	_ = createTestAgent(db, 2, "李四", "技术支持", 600)  // 中速响应
	_ = createTestAgent(db, 3, "王五", "客户服务", 1200) // 慢速响应

	// 创建测试工单
	now := time.Now()
	tickets := []models.Ticket{
		{
			CustomerID: 1,
			AgentID:    uintPtr(1),
			Status:     "resolved",
			ResolvedAt: &[]time.Time{now.Add(-24 * time.Hour)}[0],
			Priority:   "1",
			Title:      "工单1",
		},
		{
			CustomerID: 2,
			AgentID:    uintPtr(1),
			Status:     "resolved",
			ResolvedAt: &[]time.Time{now.Add(-12 * time.Hour)}[0],
			Priority:   "2",
			Title:      "工单2",
		},
		{
			CustomerID: 3,
			AgentID:    uintPtr(2),
			Status:     "resolved",
			ResolvedAt: &[]time.Time{now.Add(-18 * time.Hour)}[0],
			Priority:   "1",
			Title:      "工单3",
		},
	}
	for _, ticket := range tickets {
		if err := db.Create(&ticket).Error; err != nil {
			t.Fatalf("create ticket: %v", err)
		}
	}

	// 创建CSAT评价
	csats := []models.CustomerSatisfaction{
		{
			TicketID:   1,
			CustomerID: 1,
			AgentID:    uintPtr(1),
			Rating:     5,
			CreatedAt:  time.Now().Add(-23 * time.Hour),
		},
		{
			TicketID:   2,
			CustomerID: 2,
			AgentID:    uintPtr(1),
			Rating:     4,
			CreatedAt:  time.Now().Add(-11 * time.Hour),
		},
		{
			TicketID:   3,
			CustomerID: 3,
			AgentID:    uintPtr(2),
			Rating:     3,
			CreatedAt:  time.Now().Add(-17 * time.Hour),
		},
	}
	for _, csat := range csats {
		if err := db.Create(&csat).Error; err != nil {
			t.Fatalf("create csat: %v", err)
		}
	}

	req := &LeaderboardRequest{
		StartDate: start,
		EndDate:   end,
		Limit:     10,
	}

	resp, err := svc.GetLeaderboard(context.Background(), req)
	if err != nil {
		t.Fatalf("GetLeaderboard failed: %v", err)
	}

	if len(resp.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(resp.Entries))
	}

	// 验证排名
	if resp.Entries[0].Rank != 1 {
		t.Errorf("expected rank 1, got %d", resp.Entries[0].Rank)
	}

	// 验证徽章应用
	topEntry := resp.Entries[0]
	if len(topEntry.Badges) == 0 {
		t.Error("expected badges for top performer")
	}
}

func TestGamificationService_GetLeaderboard_Validation(t *testing.T) {
	db := newGamificationTestDB(t)
	svc := NewGamificationService(db)

	now := time.Now()

	tests := []struct {
		name    string
		req     *LeaderboardRequest
		wantErr bool
	}{
		{
			name: "valid request",
			req: &LeaderboardRequest{
				StartDate: now.Add(-24 * time.Hour),
				EndDate:   now,
				Limit:     10,
			},
			wantErr: false,
		},
		{
			name:    "nil request",
			req:     nil,
			wantErr: true,
		},
		{
			name: "missing start date",
			req: &LeaderboardRequest{
				EndDate: now,
				Limit:   10,
			},
			wantErr: true,
		},
		{
			name: "missing end date",
			req: &LeaderboardRequest{
				StartDate: now.Add(-24 * time.Hour),
				Limit:     10,
			},
			wantErr: true,
		},
		{
			name: "end before start",
			req: &LeaderboardRequest{
				StartDate: now,
				EndDate:   now.Add(-24 * time.Hour),
				Limit:     10,
			},
			wantErr: true,
		},
		{
			name: "zero limit defaults to 10",
			req: &LeaderboardRequest{
				StartDate: now.Add(-24 * time.Hour),
				EndDate:   now,
				Limit:     0,
			},
			wantErr: false,
		},
		{
			name: "limit over 100 capped",
			req: &LeaderboardRequest{
				StartDate: now.Add(-24 * time.Hour),
				EndDate:   now,
				Limit:     200,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.GetLeaderboard(context.Background(), tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetLeaderboard() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGamificationService_GetLeaderboard_WithDepartment(t *testing.T) {
	db := newGamificationTestDB(t)
	svc := NewGamificationService(db)

	_ = createTestAgent(db, 1, "技术张三", "技术支持", 300)
	_ = createTestAgent(db, 2, "客服李四", "客户服务", 600)

	now := time.Now()
	req := &LeaderboardRequest{
		StartDate:  now.Add(-24 * time.Hour),
		EndDate:    now,
		Limit:      10,
		Department: "技术支持",
	}

	resp, err := svc.GetLeaderboard(context.Background(), req)
	if err != nil {
		t.Fatalf("GetLeaderboard failed: %v", err)
	}

	// 只应该包含技术支持部门
	for _, entry := range resp.Entries {
		if entry.Department != "技术支持" {
			t.Errorf("expected only 技术支持 department, got %s", entry.Department)
		}
	}
}

func TestGamificationService_GetLeaderboard_NoActivity(t *testing.T) {
	db := newGamificationTestDB(t)
	svc := NewGamificationService(db)

	// 创建客服但没有活动
	_ = createTestAgent(db, 1, "张三", "技术支持", 300)

	now := time.Now()
	req := &LeaderboardRequest{
		StartDate: now.Add(-24 * time.Hour),
		EndDate:   now,
		Limit:     10,
	}

	resp, err := svc.GetLeaderboard(context.Background(), req)
	if err != nil {
		t.Fatalf("GetLeaderboard failed: %v", err)
	}

	// 没有活动的客服不应该出现在排行榜
	if len(resp.Entries) != 0 {
		t.Fatalf("expected 0 entries for agents with no activity, got %d", len(resp.Entries))
	}
}

func TestComputeGamificationScore(t *testing.T) {
	tests := []struct {
		name               string
		resolved           int64
		csatAvg            float64
		csatCount          int64
		avgResponseTimeSec int64
		wantPositive       bool
	}{
		{
			name:               "high performer",
			resolved:           50,
			csatAvg:            4.8,
			csatCount:          20,
			avgResponseTimeSec: 300,
			wantPositive:       true,
		},
		{
			name:               "low csat samples",
			resolved:           10,
			csatAvg:            5.0,
			csatCount:          2,
			avgResponseTimeSec: 300,
			wantPositive:       true,
		},
		{
			name:               "slow response time penalty",
			resolved:           10,
			csatAvg:            4.0,
			csatCount:          5,
			avgResponseTimeSec: 5000,
			wantPositive:       true,
		},
		{
			name:               "minimal activity",
			resolved:           1,
			csatAvg:            3.0,
			csatCount:          1,
			avgResponseTimeSec: 100,
			wantPositive:       true,
		},
		{
			name:               "no response time",
			resolved:           10,
			csatAvg:            4.0,
			csatCount:          5,
			avgResponseTimeSec: 0,
			wantPositive:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := computeGamificationScore(tt.resolved, tt.csatAvg, tt.csatCount, tt.avgResponseTimeSec)
			if (score > 0) != tt.wantPositive {
				t.Errorf("computeGamificationScore() = %v, wantPositive %v", score, tt.wantPositive)
			}
		})
	}
}

func TestApplyBadges(t *testing.T) {
	tests := []struct {
		name             string
		entries          []LeaderboardEntry
		wantBadgesInFirst bool
	}{
		{
			name: "single top resolver gets badge",
			entries: []LeaderboardEntry{
				{
					AgentID:         1,
					AgentName:       "张三",
					ResolvedTickets: 50,
					CSATAvg:         4.5,
					CSATCount:       10,
					AvgResponseTime: 300,
				},
				{
					AgentID:         2,
					AgentName:       "李四",
					ResolvedTickets: 30,
					CSATAvg:         4.0,
					CSATCount:       8,
					AvgResponseTime: 400,
				},
			},
			wantBadgesInFirst: true,
		},
		{
			name: "top csat gets badge",
			entries: []LeaderboardEntry{
				{
					AgentID:         1,
					AgentName:       "张三",
					ResolvedTickets: 30,
					CSATAvg:         5.0,
					CSATCount:       10,
					AvgResponseTime: 300,
				},
				{
					AgentID:         2,
					AgentName:       "李四",
					ResolvedTickets: 50,
					CSATAvg:         4.0,
					CSATCount:       8,
					AvgResponseTime: 400,
				},
			},
			wantBadgesInFirst: true,
		},
		{
			name: "fastest response gets badge",
			entries: []LeaderboardEntry{
				{
					AgentID:         1,
					AgentName:       "张三",
					ResolvedTickets: 30,
					CSATAvg:         4.0,
					CSATCount:       8,
					AvgResponseTime: 100,
				},
				{
					AgentID:         2,
					AgentName:       "李四",
					ResolvedTickets: 30,
					CSATAvg:         4.0,
					CSATCount:       8,
					AvgResponseTime: 500,
				},
			},
			wantBadgesInFirst: true,
		},
		{
			name:             "empty entries",
			entries:          []LeaderboardEntry{},
			wantBadgesInFirst: false,
		},
		{
			name: "all-rounder gets multiple badges",
			entries: []LeaderboardEntry{
				{
					AgentID:         1,
					AgentName:       "全能张三",
					ResolvedTickets: 100,
					CSATAvg:         5.0,
					CSATCount:       20,
					AvgResponseTime: 50,
				},
				{
					AgentID:         2,
					AgentName:       "李四",
					ResolvedTickets: 10,
					CSATAvg:         3.0,
					CSATCount:       2,
					AvgResponseTime: 1000,
				},
			},
			wantBadgesInFirst: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			applyBadges(tt.entries)
			if len(tt.entries) > 0 {
				hasBadges := len(tt.entries[0].Badges) > 0
				if hasBadges != tt.wantBadgesInFirst {
					t.Errorf("applyBadges() first entry has badges = %v, want %v", hasBadges, tt.wantBadgesInFirst)
				}
			}
		})
	}
}
