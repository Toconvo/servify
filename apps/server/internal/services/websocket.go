package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"servify/apps/server/internal/models"
)

type WebSocketMessage struct {
	Type      string      `json:"type"`
	Data      interface{} `json:"data"`
	SessionID string      `json:"session_id"`
	Timestamp time.Time   `json:"timestamp"`
}

type WebSocketClient struct {
	ID        string
	SessionID string
	Conn      *websocket.Conn
	Send      chan WebSocketMessage
	Hub       *WebSocketHub
}

type WebSocketHub struct {
	clients    map[string]*WebSocketClient
	broadcast  chan WebSocketMessage
	register   chan *WebSocketClient
	unregister chan *WebSocketClient
	mutex      sync.RWMutex
	// 可选：用于直接在WS层调用AI服务（未设置时则仅广播）
	aiService AIServiceInterface
	// 可选：用于触发“转人工”流程（未设置则仅返回提示）
	transferService *SessionTransferService
	// 可选：用于将文本消息落库（如未设置则仅记录日志）
	db *gorm.DB
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // 生产环境需要验证源
	},
}

func NewWebSocketHub() *WebSocketHub {
	return &WebSocketHub{
		clients:    make(map[string]*WebSocketClient),
		broadcast:  make(chan WebSocketMessage),
		register:   make(chan *WebSocketClient),
		unregister: make(chan *WebSocketClient),
	}
}

// SetAIService 为WebSocketHub注入AI服务（可选）
func (h *WebSocketHub) SetAIService(ai AIServiceInterface) {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	h.aiService = ai
}

// SetSessionTransferService 为 WebSocketHub 注入会话转接服务（可选）
func (h *WebSocketHub) SetSessionTransferService(svc *SessionTransferService) {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	h.transferService = svc
}

// SetDB 为 WebSocketHub 注入可选的数据库实例，用于持久化消息
func (h *WebSocketHub) SetDB(db *gorm.DB) {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	h.db = db
}

func (h *WebSocketHub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mutex.Lock()
			h.clients[client.ID] = client
			h.mutex.Unlock()
			logrus.Infof("Client %s connected", client.ID)

		case client := <-h.unregister:
			h.mutex.Lock()
			if _, ok := h.clients[client.ID]; ok {
				delete(h.clients, client.ID)
				close(client.Send)
				logrus.Infof("Client %s disconnected", client.ID)
			}
			h.mutex.Unlock()

		case message := <-h.broadcast:
			h.mutex.RLock()
			for _, client := range h.clients {
				if message.SessionID == "" || client.SessionID == message.SessionID {
					select {
					case client.Send <- message:
					default:
						close(client.Send)
						delete(h.clients, client.ID)
					}
				}
			}
			h.mutex.RUnlock()
		}
	}
}

func (h *WebSocketHub) HandleWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logrus.Error("WebSocket upgrade failed:", err)
		return
	}

	sessionID := c.Query("session_id")
	if sessionID == "" {
		sessionID = fmt.Sprintf("session_%d", time.Now().UnixNano())
	}

	client := &WebSocketClient{
		ID:        fmt.Sprintf("client_%d", time.Now().UnixNano()),
		SessionID: sessionID,
		Conn:      conn,
		Send:      make(chan WebSocketMessage, 256),
		Hub:       h,
	}

	h.register <- client

	go client.writePump()
	go client.readPump()
}

func (c *WebSocketClient) readPump() {
	defer func() {
		c.Hub.unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(512)
	c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, messageBytes, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logrus.Errorf("WebSocket error: %v", err)
			}
			break
		}

		var message WebSocketMessage
		if err := json.Unmarshal(messageBytes, &message); err != nil {
			logrus.Error("Invalid message format:", err)
			continue
		}

		message.SessionID = c.SessionID
		message.Timestamp = time.Now()

		// 处理不同类型的消息
		switch message.Type {
		case "text-message":
			c.handleTextMessage(message)
		case "webrtc-offer":
			c.handleWebRTCOffer(message)
		case "webrtc-answer":
			c.handleWebRTCAnswer(message)
		case "webrtc-candidate":
			c.handleWebRTCCandidate(message)
		default:
			logrus.Warnf("Unknown message type: %s", message.Type)
		}
	}
}

func (c *WebSocketClient) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.Conn.WriteJSON(message); err != nil {
				logrus.Error("WriteJSON error:", err)
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *WebSocketClient) handleTextMessage(message WebSocketMessage) {
	// 保存消息到数据库
	if err := c.persistTextMessage(message); err != nil {
		logrus.Warnf("Failed to persist text message: %v", err)
		// 不影响消息处理流程，继续执行
	}

	// 转发给 AI 服务处理
	go c.processMessageWithAI(message)

	// 广播消息
	c.Hub.broadcast <- message
}

func (c *WebSocketClient) handleWebRTCOffer(message WebSocketMessage) {
	// 处理 WebRTC offer
	// 集成 WebRTC 服务处理
	logrus.Infof("Received WebRTC offer from session %s", c.SessionID)

	// 这里应该调用 WebRTC 服务来处理 offer
	// 并将 answer 返回给客户端
	/*
		if webrtcService != nil {
			answer, err := webrtcService.HandleOffer(c.SessionID, message.Data)
			if err != nil {
				logrus.Errorf("Failed to handle WebRTC offer: %v", err)
				return
			}

			// 发送 answer 回客户端
			response := WebSocketMessage{
				Type:      "webrtc-answer",
				Data:      answer,
				SessionID: c.SessionID,
				Timestamp: time.Now(),
			}
			c.Send <- response
		}
	*/

	// 当前实现：简单转发给同一会话的其他客户端
	c.Hub.broadcast <- message
}

func (c *WebSocketClient) handleWebRTCAnswer(message WebSocketMessage) {
	// 处理 WebRTC answer
	logrus.Infof("Received WebRTC answer from session %s", c.SessionID)
}

func (c *WebSocketClient) handleWebRTCCandidate(message WebSocketMessage) {
	// 处理 ICE candidate
	logrus.Infof("Received ICE candidate from session %s", c.SessionID)
}

func (h *WebSocketHub) SendToSession(sessionID string, message WebSocketMessage) {
	h.broadcast <- WebSocketMessage{
		Type:      message.Type,
		Data:      message.Data,
		SessionID: sessionID,
		Timestamp: time.Now(),
	}
}

func (h *WebSocketHub) GetClientCount() int {
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	return len(h.clients)
}

// persistTextMessage 持久化文本消息
func (c *WebSocketClient) persistTextMessage(message WebSocketMessage) error {
	// 当前简单实现：记录到日志
	// 生产环境中应该保存到数据库中的 messages 表
	logrus.WithFields(logrus.Fields{
		"session_id": c.SessionID,
		"client_id":  c.ID,
		"type":       message.Type,
		"timestamp":  message.Timestamp,
	}).Info("Text message persisted")

	// 若未配置数据库，则直接返回
	hub := c.Hub
	hub.mutex.RLock()
	db := hub.db
	hub.mutex.RUnlock()
	if db == nil {
		return nil
	}

	// 确保会话存在（以 SessionID 作为主键），若不存在则创建
	var sess models.Session
	if err := db.First(&sess, "id = ?", c.SessionID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			now := time.Now()
			sess = models.Session{
				ID:        c.SessionID,
				Status:    "active",
				Platform:  "web",
				StartedAt: now,
				CreatedAt: now,
				UpdatedAt: now,
			}
			if err := db.Create(&sess).Error; err != nil {
				return fmt.Errorf("create session: %w", err)
			}
		} else {
			return err
		}
	}

	// 提取文本内容
	var content string
	switch v := message.Data.(type) {
	case map[string]interface{}:
		if s, ok := v["content"].(string); ok {
			content = s
		}
	case string:
		content = v
	default:
		// 其他格式不处理
	}

	// 插入消息记录
	m := &models.Message{
		SessionID: c.SessionID,
		UserID:    0,
		Content:   content,
		Type:      "text",
		Sender:    "user",
		CreatedAt: time.Now(),
	}
	if err := db.Create(m).Error; err != nil {
		return fmt.Errorf("persist message: %w", err)
	}
	return nil
}

// processMessageWithAI 使用 AI 处理消息
func (c *WebSocketClient) processMessageWithAI(message WebSocketMessage) {
	// 若未注入AI服务，直接返回
	h := c.Hub
	h.mutex.RLock()
	ai := h.aiService
	transferSvc := h.transferService
	db := h.db
	h.mutex.RUnlock()
	if ai == nil {
		logrus.WithFields(logrus.Fields{
			"session_id":   c.SessionID,
			"message_type": message.Type,
		}).Debug("AI service not configured; skipping AI processing")
		return
	}

	// 提取文本内容
	var content string
	switch v := message.Data.(type) {
	case map[string]interface{}:
		if s, ok := v["content"].(string); ok {
			content = s
		}
	case string:
		content = v
	default:
		// 非预期格式
		logrus.Warnf("Unsupported message data type for AI processing: %T", v)
		return
	}
	if strings.TrimSpace(content) == "" {
		return
	}

	// 若会话已分配人工客服，则停止 AI 自动回复（避免“人机抢答”）
	if db != nil {
		var sess models.Session
		if err := db.Select("id", "agent_id", "status").First(&sess, "id = ?", c.SessionID).Error; err == nil {
			if sess.AgentID != nil && sess.Status != "ended" {
				return
			}
		}
	}

	// 触发“转人工”流程（优先于 AI 正常回答）
	if transferSvc != nil {
		var history []models.Message
		if db != nil {
			_ = db.Where("session_id = ?", c.SessionID).
				Order("created_at DESC").
				Limit(6).
				Find(&history).Error
		}
		if ai.ShouldTransferToHuman(content, history) {
			go func(sessionID string) {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()

				result, err := transferSvc.TransferToHuman(ctx, &TransferRequest{
					SessionID: sessionID,
					Reason:    "user_request",
				})
				if err != nil {
					c.Hub.SendToSession(sessionID, WebSocketMessage{
						Type: "ai-response",
						Data: map[string]interface{}{
							"content":    "转接人工客服失败：" + err.Error(),
							"confidence": 1.0,
							"source":     "system",
						},
						SessionID: sessionID,
						Timestamp: time.Now(),
					})
					return
				}

				respText := "我来为您转接人工客服，请稍等..."
				if result.IsWaiting {
					respText = "我来为您转接人工客服，当前暂无可用客服，已进入等待队列。"
				} else if result.NewAgentID != 0 {
					respText = fmt.Sprintf("我来为您转接人工客服，已为您分配客服（ID=%d）。", result.NewAgentID)
				}
				c.Hub.SendToSession(sessionID, WebSocketMessage{
					Type: "ai-response",
					Data: map[string]interface{}{
						"content":    respText,
						"confidence": 1.0,
						"source":     "system",
					},
					SessionID: sessionID,
					Timestamp: time.Now(),
				})
			}(c.SessionID)
			return
		}
	}

	// 异步调用AI
	go func(sessionID string, text string) {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		resp, err := ai.ProcessQuery(ctx, text, sessionID)
		if err != nil {
			logrus.Errorf("AI processing failed: %v", err)
			return
		}
		// 推送AI回复
		c.Hub.SendToSession(sessionID, WebSocketMessage{
			Type: "ai-response",
			Data: map[string]interface{}{
				"content":    resp.Content,
				"confidence": resp.Confidence,
				"source":     resp.Source,
			},
			SessionID: sessionID,
			Timestamp: time.Now(),
		})
	}(c.SessionID, content)
}
