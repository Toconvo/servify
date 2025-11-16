package services

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"servify/apps/server/internal/models"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// AgentService 人工客服服务
type AgentService struct {
	db          *gorm.DB
	logger      *logrus.Logger
	onlineAgents sync.Map // map[uint]*AgentInfo - 在线客服列表
	agentQueues  sync.Map // map[uint]chan *models.Session - 客服会话队列
}

// NewAgentService 创建人工客服服务
func NewAgentService(db *gorm.DB, logger *logrus.Logger) *AgentService {
	if logger == nil {
		logger = logrus.New()
	}

	service := &AgentService{
		db:     db,
		logger: logger,
	}

	// 启动后台任务
	go service.backgroundTasks()

	return service
}

// AgentInfo 在线客服信息
type AgentInfo struct {
	UserID          uint                    `json:"user_id"`
	Username        string                  `json:"username"`
	Name            string                  `json:"name"`
	Department      string                  `json:"department"`
	Skills          []string                `json:"skills"`
	Status          string                  `json:"status"` // online, busy, away
	MaxConcurrent   int                     `json:"max_concurrent"`
	CurrentLoad     int                     `json:"current_load"`
	Rating          float64                 `json:"rating"`
	AvgResponseTime int                     `json:"avg_response_time"`
	LastActivity    time.Time               `json:"last_activity"`
	ConnectedAt     time.Time               `json:"connected_at"`
	Sessions        map[string]*models.Session `json:"-"` // 当前处理的会话
}

// AgentCreateRequest 创建客服请求
type AgentCreateRequest struct {
	UserID         uint   `json:"user_id" binding:"required"`
	Department     string `json:"department"`
	Skills         string `json:"skills"`
	MaxConcurrent  int    `json:"max_concurrent"`
}

// AgentUpdateRequest 更新客服请求
type AgentUpdateRequest struct {
	Department    *string  `json:"department"`
	Skills        *string  `json:"skills"`
	Status        *string  `json:"status"`
	MaxConcurrent *int     `json:"max_concurrent"`
}

// CreateAgent 创建客服
func (s *AgentService) CreateAgent(ctx context.Context, req *AgentCreateRequest) (*models.Agent, error) {
	// 验证用户是否存在
	var user models.User
	if err := s.db.First(&user, req.UserID).Error; err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	// 检查是否已经是客服
	var existingAgent models.Agent
	if err := s.db.Where("user_id = ?", req.UserID).First(&existingAgent).Error; err == nil {
		return nil, fmt.Errorf("user is already an agent")
	}

	// 创建客服记录
	agent := &models.Agent{
		UserID:        req.UserID,
		Department:    req.Department,
		Skills:        req.Skills,
		Status:        "offline",
		MaxConcurrent: req.MaxConcurrent,
		CurrentLoad:   0,
		Rating:        5.0,
	}

	if agent.MaxConcurrent <= 0 {
		agent.MaxConcurrent = 5
	}

	if err := s.db.Create(agent).Error; err != nil {
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}

	// 更新用户角色
	s.db.Model(&models.User{}).Where("id = ?", req.UserID).Update("role", "agent")

	s.logger.Infof("Created agent for user %d", req.UserID)

	// 返回完整的客服信息
	return s.GetAgentByUserID(ctx, req.UserID)
}

// GetAgentByUserID 根据用户ID获取客服信息
func (s *AgentService) GetAgentByUserID(ctx context.Context, userID uint) (*models.Agent, error) {
	var agent models.Agent
	err := s.db.Preload("User").
		Preload("Tickets", func(db *gorm.DB) *gorm.DB {
			return db.Where("status NOT IN ?", []string{"closed"}).Order("created_at DESC")
		}).
		Where("user_id = ?", userID).
		First(&agent).Error

	if err != nil {
		return nil, fmt.Errorf("agent not found: %w", err)
	}

	return &agent, nil
}

// AgentGoOnline 客服上线
func (s *AgentService) AgentGoOnline(ctx context.Context, userID uint) error {
	// 更新数据库状态
	if err := s.db.Model(&models.Agent{}).
		Where("user_id = ?", userID).
		Update("status", "online").Error; err != nil {
		return fmt.Errorf("failed to update agent status: %w", err)
	}

	// 获取客服信息
	agent, err := s.GetAgentByUserID(ctx, userID)
	if err != nil {
		return err
	}

	// 添加到在线客服列表
	agentInfo := &AgentInfo{
		UserID:          agent.UserID,
		Username:        agent.User.Username,
		Name:            agent.User.Name,
		Department:      agent.Department,
		Skills:          s.parseSkills(agent.Skills),
		Status:          "online",
		MaxConcurrent:   agent.MaxConcurrent,
		CurrentLoad:     agent.CurrentLoad,
		Rating:          agent.Rating,
		AvgResponseTime: agent.AvgResponseTime,
		LastActivity:    time.Now(),
		ConnectedAt:     time.Now(),
		Sessions:        make(map[string]*models.Session),
	}

	s.onlineAgents.Store(userID, agentInfo)

	// 创建会话队列
	queue := make(chan *models.Session, agent.MaxConcurrent*2)
	s.agentQueues.Store(userID, queue)

	s.logger.Infof("Agent %d (%s) went online", userID, agent.User.Username)

	return nil
}

// AgentGoOffline 客服下线
func (s *AgentService) AgentGoOffline(ctx context.Context, userID uint) error {
	// 更新数据库状态
	if err := s.db.Model(&models.Agent{}).
		Where("user_id = ?", userID).
		Update("status", "offline").Error; err != nil {
		return fmt.Errorf("failed to update agent status: %w", err)
	}

	// 从在线列表中移除
	s.onlineAgents.Delete(userID)

	// 关闭会话队列
	if queue, ok := s.agentQueues.LoadAndDelete(userID); ok {
		close(queue.(chan *models.Session))
	}

	s.logger.Infof("Agent %d went offline", userID)

	return nil
}

// UpdateAgentStatus 更新客服状态
func (s *AgentService) UpdateAgentStatus(ctx context.Context, userID uint, status string) error {
	validStatuses := map[string]bool{"online": true, "busy": true, "away": true}
	if !validStatuses[status] {
		return fmt.Errorf("invalid status: %s", status)
	}

	// 更新数据库
	if err := s.db.Model(&models.Agent{}).
		Where("user_id = ?", userID).
		Update("status", status).Error; err != nil {
		return fmt.Errorf("failed to update agent status: %w", err)
	}

	// 更新内存中的状态
	if agentInfo, ok := s.onlineAgents.Load(userID); ok {
		info := agentInfo.(*AgentInfo)
		info.Status = status
		info.LastActivity = time.Now()
		s.onlineAgents.Store(userID, info)
	}

	s.logger.Infof("Agent %d status updated to %s", userID, status)

	return nil
}

// AssignSessionToAgent 将会话分配给客服
func (s *AgentService) AssignSessionToAgent(ctx context.Context, sessionID string, agentID uint) error {
	// 获取客服信息
	agentInfo, ok := s.onlineAgents.Load(agentID)
	if !ok {
		return fmt.Errorf("agent %d is not online", agentID)
	}

	info := agentInfo.(*AgentInfo)

	// 检查客服是否可以接受新会话
	if info.CurrentLoad >= info.MaxConcurrent {
		return fmt.Errorf("agent %d is at maximum capacity", agentID)
	}

	if info.Status == "offline" {
		return fmt.Errorf("agent %d is offline", agentID)
	}

	// 获取会话信息
	var session models.Session
	if err := s.db.First(&session, "id = ?", sessionID).Error; err != nil {
		return fmt.Errorf("session not found: %w", err)
	}

	// 更新会话分配
	if err := s.db.Model(&models.Session{}).
		Where("id = ?", sessionID).
		Updates(map[string]interface{}{
			"agent_id": agentID,
			"status":   "assigned",
		}).Error; err != nil {
		return fmt.Errorf("failed to assign session: %w", err)
	}

	// 更新客服负载
	info.CurrentLoad++
	info.Sessions[sessionID] = &session
	info.LastActivity = time.Now()
	s.onlineAgents.Store(agentID, info)

	// 更新数据库中的负载
	s.db.Model(&models.Agent{}).
		Where("user_id = ?", agentID).
		Update("current_load", info.CurrentLoad)

	s.logger.Infof("Assigned session %s to agent %d", sessionID, agentID)

	return nil
}

// ReleaseSessionFromAgent 从客服释放会话
func (s *AgentService) ReleaseSessionFromAgent(ctx context.Context, sessionID string, agentID uint) error {
	// 更新会话状态
	if err := s.db.Model(&models.Session{}).
		Where("id = ? AND agent_id = ?", sessionID, agentID).
		Updates(map[string]interface{}{
			"status":   "ended",
			"ended_at": time.Now(),
		}).Error; err != nil {
		return fmt.Errorf("failed to release session: %w", err)
	}

	// 更新客服负载
	if agentInfo, ok := s.onlineAgents.Load(agentID); ok {
		info := agentInfo.(*AgentInfo)
		if info.CurrentLoad > 0 {
			info.CurrentLoad--
		}
		delete(info.Sessions, sessionID)
		info.LastActivity = time.Now()
		s.onlineAgents.Store(agentID, info)

		// 更新数据库中的负载
		s.db.Model(&models.Agent{}).
			Where("user_id = ?", agentID).
			Update("current_load", info.CurrentLoad)
	}

	s.logger.Infof("Released session %s from agent %d", sessionID, agentID)

	return nil
}

// FindAvailableAgent 查找可用的客服
func (s *AgentService) FindAvailableAgent(ctx context.Context, skills []string, priority string) (*AgentInfo, error) {
	var bestAgent *AgentInfo
	var bestScore float64 = -1

	s.onlineAgents.Range(func(key, value interface{}) bool {
		info := value.(*AgentInfo)

		// 检查可用性
		if info.Status != "online" || info.CurrentLoad >= info.MaxConcurrent {
			return true
		}

		// 计算匹配分数
		score := s.calculateAgentScore(info, skills, priority)
		if score > bestScore {
			bestScore = score
			bestAgent = info
		}

		return true
	})

	if bestAgent == nil {
		return nil, fmt.Errorf("no available agent found")
	}

	return bestAgent, nil
}

// GetOnlineAgents 获取在线客服列表
func (s *AgentService) GetOnlineAgents(ctx context.Context) []*AgentInfo {
	var agents []*AgentInfo

	s.onlineAgents.Range(func(key, value interface{}) bool {
		info := value.(*AgentInfo)
		agents = append(agents, info)
		return true
	})

	return agents
}

// GetAgentStats 获取客服统计信息
func (s *AgentService) GetAgentStats(ctx context.Context, agentID *uint) (*AgentStats, error) {
	stats := &AgentStats{}

	query := s.db.Model(&models.Agent{})
	if agentID != nil {
		query = query.Where("user_id = ?", *agentID)
	}

	// 总客服数
	query.Count(&stats.Total)

	// 在线客服数
	onlineCount := 0
	s.onlineAgents.Range(func(key, value interface{}) bool {
		onlineCount++
		return true
	})
	stats.Online = int64(onlineCount)

	// 繁忙客服数
	busyCount := 0
	s.onlineAgents.Range(func(key, value interface{}) bool {
		info := value.(*AgentInfo)
		if info.Status == "busy" || info.CurrentLoad >= info.MaxConcurrent {
			busyCount++
		}
		return true
	})
	stats.Busy = int64(busyCount)

	// 平均响应时间
	var avgResponseTime float64
	s.db.Model(&models.Agent{}).
		Select("AVG(avg_response_time)").
		Row().Scan(&avgResponseTime)
	stats.AvgResponseTime = int64(avgResponseTime)

	// 平均评分
	var avgRating float64
	s.db.Model(&models.Agent{}).
		Select("AVG(rating)").
		Row().Scan(&avgRating)
	stats.AvgRating = avgRating

	return stats, nil
}

// calculateAgentScore 计算客服匹配分数
func (s *AgentService) calculateAgentScore(agent *AgentInfo, requiredSkills []string, priority string) float64 {
	score := 0.0

	// 基础分数：客服评分
	score += agent.Rating

	// 负载分数：负载越低分数越高
	loadRatio := float64(agent.CurrentLoad) / float64(agent.MaxConcurrent)
	score += (1 - loadRatio) * 3

	// 响应时间分数：响应时间越短分数越高
	if agent.AvgResponseTime > 0 {
		responseScore := 300.0 / float64(agent.AvgResponseTime) // 300秒作为基准
		if responseScore > 2 {
			responseScore = 2
		}
		score += responseScore
	}

	// 技能匹配分数
	if len(requiredSkills) > 0 {
		matchedSkills := 0
		for _, required := range requiredSkills {
			for _, agentSkill := range agent.Skills {
				if required == agentSkill {
					matchedSkills++
					break
				}
			}
		}
		skillRatio := float64(matchedSkills) / float64(len(requiredSkills))
		score += skillRatio * 2
	}

	return score
}

// parseSkills 解析技能字符串
func (s *AgentService) parseSkills(skillsStr string) []string {
	if skillsStr == "" {
		return []string{}
	}

	skills := []string{}
	for _, skill := range strings.Split(skillsStr, ",") {
		skill = strings.TrimSpace(skill)
		if skill != "" {
			skills = append(skills, skill)
		}
	}

	return skills
}

// backgroundTasks 后台任务
func (s *AgentService) backgroundTasks() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.cleanupInactiveAgents()
		s.updateAgentMetrics()
	}
}

// cleanupInactiveAgents 清理不活跃的客服
func (s *AgentService) cleanupInactiveAgents() {
	timeout := 5 * time.Minute

	s.onlineAgents.Range(func(key, value interface{}) bool {
		info := value.(*AgentInfo)
		if time.Since(info.LastActivity) > timeout {
			s.logger.Warnf("Agent %d appears inactive, marking as away", info.UserID)
			s.UpdateAgentStatus(context.Background(), info.UserID, "away")
		}
		return true
	})
}

// updateAgentMetrics 更新客服指标
func (s *AgentService) updateAgentMetrics() {
	// 这里可以实现更复杂的指标计算逻辑
	// 例如：计算平均响应时间、处理工单数等
}

// AgentStats 客服统计信息
type AgentStats struct {
	Total           int64   `json:"total"`
	Online          int64   `json:"online"`
	Busy            int64   `json:"busy"`
	AvgResponseTime int64   `json:"avg_response_time"`
	AvgRating       float64 `json:"avg_rating"`
}
