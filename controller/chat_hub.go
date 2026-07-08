package controller

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"go-alpha/models"
	"go-alpha/response"
)

// ─── WebSocket 升级器 ───────────────────────────────────────────

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // 开发环境允许所有来源，生产环境应限制
	},
}

// ─── 消息格式 ────────────────────────────────────────────────────

// WSMessage WebSocket 通信的消息格式
type WSMessage struct {
	Type           string `json:"type"`            // legacy category
	Event          string `json:"event,omitempty"` // message.new | message.edit | message.delete | conversation.read | user.online | user.offline | typing.start | typing.stop
	UserID         uint   `json:"user_id,omitempty"`
	Username       string `json:"username,omitempty"`
	Avatar         string `json:"avatar,omitempty"`
	MsgType        string `json:"msg_type,omitempty"` // text | emoji | image | file
	ConversationID string `json:"conversation_id,omitempty"`
	Content        string `json:"content,omitempty"`
	FileName       string `json:"file_name,omitempty"`
	FileURL        string `json:"file_url,omitempty"`
	Time           string `json:"time,omitempty"`
	Payload        any    `json:"payload,omitempty"`
}

// ─── 客户端连接 ──────────────────────────────────────────────────

// Client 代表一个 WebSocket 连接
type Client struct {
	hub      *Hub
	conn     *websocket.Conn
	send     chan []byte
	userID   uint
	username string
	avatar   string
}

// readPump 从 WebSocket 读取消息并广播到 hub
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(8192) // 8KB 最大消息
	c.conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		return nil
	})

	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				slog.Warn("WebSocket read error", "userID", c.userID, "error", err)
			}
			break
		}

		var msg WSMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			slog.Warn("Invalid JSON from client", "userID", c.userID)
			continue
		}

		msg.UserID = c.userID
		msg.Username = c.username
		msg.Avatar = c.avatar
		msg.Time = time.Now().Format("15:04")
		if msg.Type == "" {
			msg.Type = "message"
		}

		// 广播到所有客户端
		data, _ := json.Marshal(msg)
		c.hub.broadcast <- data
	}
}

// writePump 将消息从 hub 写入 WebSocket 连接
func (c *Client) writePump() {
	ticker := time.NewTicker(5 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			// 心跳 ping
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// ─── Hub（消息中心）──────────────────────────────────────────────

// Hub 管理所有 WebSocket 客户端连接
type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
}

// NewHub 创建新的 Hub 实例
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// Run 启动 hub 的主循环
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			slog.Info("WebSocket client connected", "userID", client.userID, "username", client.username, "total", len(h.clients))

			// 广播在线用户列表
			h.broadcastOnlineUsers("user.online", client.userID)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
			slog.Info("WebSocket client disconnected", "userID", client.userID, "total", len(h.clients))

			// 广播在线用户列表
			h.broadcastOnlineUsers("user.offline", client.userID)

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					// 客户端消费速度跟不上，跳过
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// broadcastOnlineUsers 广播当前在线用户列表
func (h *Hub) broadcastOnlineUsers(event string, uid uint) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	type OnlineUser struct {
		UserID   uint   `json:"user_id"`
		Username string `json:"username"`
		Avatar   string `json:"avatar"`
	}

	users := make([]OnlineUser, 0, len(h.clients))
	seen := make(map[uint]bool)
	for client := range h.clients {
		if !seen[client.userID] {
			seen[client.userID] = true
			users = append(users, OnlineUser{
				UserID:   client.userID,
				Username: client.username,
				Avatar:   client.avatar,
			})
		}
	}

	msg := map[string]interface{}{
		"type":    "presence",
		"event":   event,
		"user_id": uid,
		"users":   users,
	}
	data, _ := json.Marshal(msg)

	for client := range h.clients {
		select {
		case client.send <- data:
		default:
		}
	}
}

// IsUserOnline 检查指定用户是否通过 WebSocket 在线
func (h *Hub) IsUserOnline(userID uint) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for client := range h.clients {
		if client.userID == userID {
			return true
		}
	}
	return false
}

// OnlineUserSet 返回当前在线用户的去重快照。
func (h *Hub) OnlineUserSet() map[uint]bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	users := make(map[uint]bool, len(h.clients))
	for client := range h.clients {
		users[client.userID] = true
	}
	return users
}

// ─── 全局 Hub 单例 ──────────────────────────────────────────────

var ChatHub = NewHub()

func init() {
	go ChatHub.Run()
}

// ─── WebSocket 处理函数 ─────────────────────────────────────────

// HandleWebSocket 处理 WebSocket 连接升级
func HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	userID, err := authenticateWebSocketRequest(r)
	if err != nil {
		slog.Warn("WebSocket auth failed", "error", err)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("WebSocket upgrade failed", "error", err)
		return
	}

	var user models.User
	if err := models.DB.Select("id, username, avatar").First(&user, userID).Error; err != nil {
		slog.Warn("WebSocket user lookup failed", "userID", userID, "error", err)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		_ = conn.Close()
		return
	}

	client := &Client{
		hub:      ChatHub,
		conn:     conn,
		send:     make(chan []byte, 128),
		userID:   user.ID,
		username: user.Username,
		avatar:   user.Avatar,
	}

	ChatHub.register <- client
	client.sendPresenceSnapshot()

	go client.writePump()
	go client.readPump()
}

func authenticateWebSocketRequest(r *http.Request) (uint, error) {
	token := ""
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		token = strings.TrimPrefix(authHeader, "Bearer ")
	}
	if token == "" {
		token = r.URL.Query().Get("token")
	}
	if token == "" {
		return 0, http.ErrNoCookie
	}
	claims, err := response.ParseToken(token)
	if err != nil {
		return 0, err
	}
	return claims.Id, nil
}

func (c *Client) sendPresenceSnapshot() {
	c.hub.mu.RLock()
	defer c.hub.mu.RUnlock()
	type OnlineUser struct {
		UserID   uint   `json:"user_id"`
		Username string `json:"username"`
		Avatar   string `json:"avatar"`
	}
	users := make([]OnlineUser, 0, len(c.hub.clients))
	seen := make(map[uint]bool)
	for client := range c.hub.clients {
		if seen[client.userID] {
			continue
		}
		seen[client.userID] = true
		users = append(users, OnlineUser{UserID: client.userID, Username: client.username, Avatar: client.avatar})
	}
	data, _ := json.Marshal(map[string]any{
		"type":  "presence",
		"event": "presence.snapshot",
		"users": users,
	})
	select {
	case c.send <- data:
	default:
	}
}

// BroadcastMessageWithSender 广播消息到所有 WebSocket 客户端（带发送者信息，避免重复查 DB）
func BroadcastMessageWithSender(msg models.Message, senderUsername, senderAvatar string) {
	// 映射消息类型到字符串
	msgTypeStr := "text"
	switch msg.MessageType {
	case models.MsgEmoji:
		msgTypeStr = "emoji"
	case models.MsgImage:
		msgTypeStr = "image"
	case models.MsgFile:
		msgTypeStr = "file"
	}

	wsMsg := map[string]interface{}{
		"type":            "message",
		"event":           "message.new",
		"chat_type":       msg.ChatType,
		"id":              msg.ID,
		"conversation_id": msg.ConversationID,
		"sender_id":       msg.SenderID,
		"sender_username": senderUsername,
		"sender_avatar":   senderAvatar,
		"username":        senderUsername,
		"message_type":    msg.MessageType,
		"msg_type":        msgTypeStr,
		"content":         msg.Content,
		"file_name":       msg.FileName,
		"file_url":        msg.FileURL,
		"status":          1,
		"created_at":      msg.CreatedAt.Format(time.RFC3339),
		"time":            msg.CreatedAt.Format("15:04"),
		"payload": map[string]any{
			"conversation_id": msg.ConversationID,
			"sender_id":       msg.SenderID,
			"message_id":      msg.ID,
		},
	}
	data, err := json.Marshal(wsMsg)
	if err != nil {
		slog.Error("Failed to marshal broadcast message", "error", err)
		return
	}
	ChatHub.broadcast <- data
}

// BroadcastMessage 从 HTTP 接口广播消息到所有 WebSocket 客户端（保持向后兼容）
func BroadcastMessage(msg models.Message) {
	var user models.User
	models.DB.First(&user, msg.SenderID)
	BroadcastMessageWithSender(msg, user.Username, user.Avatar)
}

func BroadcastConversationRead(convID string, userID uint) {
	data, _ := json.Marshal(map[string]any{
		"type":            "conversation",
		"event":           "conversation.read",
		"conversation_id": convID,
		"user_id":         userID,
		"time":            time.Now().Format(time.RFC3339),
	})
	ChatHub.broadcast <- data
}

func BroadcastConversationUpdate(convID string, payload map[string]any) {
	if payload == nil {
		payload = map[string]any{}
	}
	payload["conversation_id"] = convID
	payload["time"] = time.Now().Format(time.RFC3339)
	payload["event"] = "conversation.update"
	payload["type"] = "conversation"
	data, _ := json.Marshal(payload)
	ChatHub.broadcast <- data
}
