package services

import (
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	"sync"
	"time"
)

type MessageRouter struct {
	platforms map[string]PlatformAdapter
	aiService *AIService
	wsHub     *WebSocketHub
	mutex     sync.RWMutex
}

type PlatformAdapter interface {
	SendMessage(chatID, message string) error
	ReceiveMessage() <-chan UnifiedMessage
	GetPlatformType() PlatformType
	Start() error
	Stop() error
}

type PlatformType string

const (
	PlatformWeb      PlatformType = "web"
	PlatformTelegram PlatformType = "telegram"
	PlatformWeChat   PlatformType = "wechat"
	PlatformQQ       PlatformType = "qq"
	PlatformFeishu   PlatformType = "feishu"
)

type UnifiedMessage struct {
	ID          string                 `json:"id"`
	PlatformID  string                 `json:"platform_id"`
	UserID      string                 `json:"user_id"`
	Content     string                 `json:"content"`
	Type        MessageType            `json:"type"`
	Timestamp   time.Time              `json:"timestamp"`
	Attachments []Attachment           `json:"attachments,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

type MessageType string

const (
	MessageTypeText  MessageType = "text"
	MessageTypeImage MessageType = "image"
	MessageTypeFile  MessageType = "file"
	MessageTypeAudio MessageType = "audio"
	MessageTypeVideo MessageType = "video"
)

type Attachment struct {
	Type string `json:"type"`
	URL  string `json:"url"`
	Name string `json:"name"`
	Size int64  `json:"size"`
}

type RouteRule struct {
	Platform  PlatformType `json:"platform"`
	Condition string       `json:"condition"`
	Action    string       `json:"action"`
	Priority  int          `json:"priority"`
}

func NewMessageRouter(aiService *AIService, wsHub *WebSocketHub) *MessageRouter {
	return &MessageRouter{
		platforms: make(map[string]PlatformAdapter),
		aiService: aiService,
		wsHub:     wsHub,
	}
}

func (r *MessageRouter) RegisterPlatform(platformID string, adapter PlatformAdapter) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.platforms[platformID] = adapter
	logrus.Infof("Registered platform adapter: %s", platformID)
}

func (r *MessageRouter) UnregisterPlatform(platformID string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if adapter, exists := r.platforms[platformID]; exists {
		adapter.Stop()
		delete(r.platforms, platformID)
		logrus.Infof("Unregistered platform adapter: %s", platformID)
	}
}

func (r *MessageRouter) Start() error {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	for platformID, adapter := range r.platforms {
		go r.handlePlatformMessages(platformID, adapter)
		if err := adapter.Start(); err != nil {
			logrus.Errorf("Failed to start platform %s: %v", platformID, err)
			return err
		}
	}

	logrus.Info("Message router started")
	return nil
}

func (r *MessageRouter) Stop() error {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	for platformID, adapter := range r.platforms {
		if err := adapter.Stop(); err != nil {
			logrus.Errorf("Failed to stop platform %s: %v", platformID, err)
		}
	}

	logrus.Info("Message router stopped")
	return nil
}

func (r *MessageRouter) handlePlatformMessages(platformID string, adapter PlatformAdapter) {
	messageChan := adapter.ReceiveMessage()

	for message := range messageChan {
		logrus.Infof("Received message from platform %s: %s", platformID, message.Content)

		// 路由消息
		if err := r.routeMessage(platformID, message); err != nil {
			logrus.Errorf("Failed to route message: %v", err)
		}
	}
}

func (r *MessageRouter) routeMessage(platformID string, message UnifiedMessage) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1. 保存消息到数据库
	// TODO: 实现消息持久化

	// 2. 如果是 Web 平台，直接通过 WebSocket 处理
	if platformID == string(PlatformWeb) {
		return r.handleWebMessage(ctx, message)
	}

	// 3. 其他平台的消息处理
	return r.handleExternalPlatformMessage(ctx, platformID, message)
}

func (r *MessageRouter) handleWebMessage(ctx context.Context, message UnifiedMessage) error {
	// AI 处理消息
	aiResponse, err := r.aiService.ProcessQuery(ctx, message.Content, message.UserID)
	if err != nil {
		logrus.Errorf("AI processing failed: %v", err)
		return err
	}

	// 发送回复
	response := WebSocketMessage{
		Type: "ai-response",
		Data: map[string]interface{}{
			"content":    aiResponse.Content,
			"confidence": aiResponse.Confidence,
			"source":     aiResponse.Source,
		},
		SessionID: message.UserID,
		Timestamp: time.Now(),
	}

	r.wsHub.SendToSession(message.UserID, response)
	return nil
}

func (r *MessageRouter) handleExternalPlatformMessage(ctx context.Context, platformID string, message UnifiedMessage) error {
	// AI 处理消息
	aiResponse, err := r.aiService.ProcessQuery(ctx, message.Content, message.UserID)
	if err != nil {
		logrus.Errorf("AI processing failed: %v", err)
		return err
	}

	// 发送回复到原平台
	r.mutex.RLock()
	adapter, exists := r.platforms[platformID]
	r.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("platform adapter not found: %s", platformID)
	}

	err = adapter.SendMessage(message.UserID, aiResponse.Content)
	if err != nil {
		return fmt.Errorf("failed to send message to platform %s: %w", platformID, err)
	}

	return nil
}

func (r *MessageRouter) BroadcastMessage(message UnifiedMessage) error {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	for platformID, adapter := range r.platforms {
		if err := adapter.SendMessage(message.UserID, message.Content); err != nil {
			logrus.Errorf("Failed to broadcast message to platform %s: %v", platformID, err)
		}
	}

	return nil
}

func (r *MessageRouter) GetPlatformStats() map[string]interface{} {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	stats := make(map[string]interface{})
	stats["total_platforms"] = len(r.platforms)
	stats["active_platforms"] = make([]string, 0, len(r.platforms))

	for platformID := range r.platforms {
		stats["active_platforms"] = append(stats["active_platforms"].([]string), platformID)
	}

	return stats
}

// Telegram 适配器示例
type TelegramAdapter struct {
	botToken string
	chatID   string
	msgChan  chan UnifiedMessage
	stopChan chan struct{}
}

func NewTelegramAdapter(botToken, chatID string) *TelegramAdapter {
	return &TelegramAdapter{
		botToken: botToken,
		chatID:   chatID,
		msgChan:  make(chan UnifiedMessage, 100),
		stopChan: make(chan struct{}),
	}
}

func (t *TelegramAdapter) SendMessage(chatID, message string) error {
	// 实现 Telegram 消息发送
	logrus.Infof("Sending Telegram message to %s: %s", chatID, message)
	// TODO: 实现实际的 Telegram API 调用
	return nil
}

func (t *TelegramAdapter) ReceiveMessage() <-chan UnifiedMessage {
	return t.msgChan
}

func (t *TelegramAdapter) GetPlatformType() PlatformType {
	return PlatformTelegram
}

func (t *TelegramAdapter) Start() error {
	logrus.Info("Starting Telegram adapter")
	// TODO: 实现 Telegram webhook 或 polling
	return nil
}

func (t *TelegramAdapter) Stop() error {
	logrus.Info("Stopping Telegram adapter")
	close(t.stopChan)
	return nil
}

// 微信适配器示例
type WeChatAdapter struct {
	appID     string
	appSecret string
	msgChan   chan UnifiedMessage
	stopChan  chan struct{}
}

func NewWeChatAdapter(appID, appSecret string) *WeChatAdapter {
	return &WeChatAdapter{
		appID:     appID,
		appSecret: appSecret,
		msgChan:   make(chan UnifiedMessage, 100),
		stopChan:  make(chan struct{}),
	}
}

func (w *WeChatAdapter) SendMessage(chatID, message string) error {
	// 实现微信消息发送
	logrus.Infof("Sending WeChat message to %s: %s", chatID, message)
	// TODO: 实现实际的微信 API 调用
	return nil
}

func (w *WeChatAdapter) ReceiveMessage() <-chan UnifiedMessage {
	return w.msgChan
}

func (w *WeChatAdapter) GetPlatformType() PlatformType {
	return PlatformWeChat
}

func (w *WeChatAdapter) Start() error {
	logrus.Info("Starting WeChat adapter")
	// TODO: 实现微信消息接收
	return nil
}

func (w *WeChatAdapter) Stop() error {
	logrus.Info("Stopping WeChat adapter")
	close(w.stopChan)
	return nil
}
