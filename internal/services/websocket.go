package services

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
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
	// TODO: 实现消息持久化

	// 转发给 AI 服务处理
	// TODO: 集成 AI 服务

	// 广播消息
	c.Hub.broadcast <- message
}

func (c *WebSocketClient) handleWebRTCOffer(message WebSocketMessage) {
	// 处理 WebRTC offer
	// TODO: 集成 WebRTC 服务
	logrus.Infof("Received WebRTC offer from session %s", c.SessionID)
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
