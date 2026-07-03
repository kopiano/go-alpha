package controller

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"go-alpha/models"
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
	Type     string `json:"type"`               // "message" | "typing" | "online" | "ping"
	UserID   uint   `json:"user_id,omitempty"`
	Username string `json:"username,omitempty"`
	Avatar   string `json:"avatar,omitempty"`
	MsgType  string `json:"msg_type,omitempty"`  // text | emoji | image | file
	Content  string `json:"content,omitempty"`
	FileName string `json:"file_name,omitempty"`
	FileURL  string `json:"file_url,omitempty"`
	Time     string `json:"time,omitempty"`
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
			h.broadcastOnlineUsers()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
			slog.Info("WebSocket client disconnected", "userID", client.userID, "total", len(h.clients))

			// 广播在线用户列表
			h.broadcastOnlineUsers()

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
func (h *Hub) broadcastOnlineUsers() {
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
		"type":  "online",
		"users": users,
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

// ─── 全局 Hub 单例 ──────────────────────────────────────────────

var ChatHub = NewHub()

func init() {
	go ChatHub.Run()
}

// ─── WebSocket 处理函数 ─────────────────────────────────────────

// HandleWebSocket 处理 WebSocket 连接升级
func HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("WebSocket upgrade failed", "error", err)
		return
	}

	// 从查询参数获取用户信息
	userID := r.URL.Query().Get("user_id")
	username := r.URL.Query().Get("username")
	avatar := r.URL.Query().Get("avatar")

	var uid uint
	if userID != "" {
		uidStr, _ := strconv.ParseUint(userID, 10, 64)
		uid = uint(uidStr)
	}

	client := &Client{
		hub:      ChatHub,
		conn:     conn,
		send:     make(chan []byte, 128),
		userID:   uid,
		username: username,
		avatar:   avatar,
	}

	ChatHub.register <- client

	go client.writePump()
	go client.readPump()
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
	case models.MsgSystem:
		msgTypeStr = "system"
	case models.MsgReply:
		msgTypeStr = "reply"
	}

	// 从 metadata 提取文件名和下载地址
	var meta map[string]interface{}
	json.Unmarshal(msg.Metadata, &meta)
	fileName, _ := meta["file_name"].(string)
	fileURL, _ := meta["file_url"].(string)

	// 如果回复了消息，查询原消息的发送者用户名和内容
	var replyToUsername, replyToContent string
	if msg.ReplyToMessageID != nil && *msg.ReplyToMessageID > 0 {
		var repliedMsg models.Message
		if err := models.DB.First(&repliedMsg, *msg.ReplyToMessageID).Error; err == nil {
			replyToContent = repliedMsg.Content
			var repliedUser models.User
			if err := models.DB.First(&repliedUser, repliedMsg.SenderID).Error; err == nil {
				replyToUsername = repliedUser.Username
			}
		}
	}

	wsMsg := map[string]interface{}{
		"type":               "message",
		"id":                 msg.ID,
		"conversation_id":    msg.ConversationID,
		"sender_id":          msg.SenderID,
		"sender_username":    senderUsername,
		"sender_avatar":      senderAvatar,
		"username":           senderUsername,
		"message_type":       msg.MessageType,
		"msg_type":           msgTypeStr,
		"content":            msg.Content,
		"file_name":          fileName,
		"file_url":           fileURL,
		"status":             msg.Status,
		"created_at":         msg.CreatedAt.Format(time.RFC3339),
		"time":               msg.CreatedAt.Format("15:04"),
		"reply_to_message_id": msg.ReplyToMessageID,
		"reply_to_username":   replyToUsername,
		"reply_to_content":    replyToContent,
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
