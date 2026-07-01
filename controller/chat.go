package controller

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"go-alpha/models"
	"go-alpha/response"
)

// ─── GET /api/v1/chat/conversations ───
func GetConversations(c *gin.Context) {
	userID := c.GetUint("userId")
	if userID == 0 {
		userID = 1
	}

	convs, err := models.GetUserConversations(models.DB, userID)
	if err != nil {
		response.Failed("Failed to fetch conversations", c)
		return
	}
	if convs == nil {
		convs = []models.Conversation{}
	}

	// 为每个会话附加最后一条消息和未读数
	type ConvWithMeta struct {
		models.Conversation
		LastMessage *models.Message `json:"last_message,omitempty"`
		UnreadCount int64           `json:"unread_count"`
	}
	result := make([]ConvWithMeta, 0, len(convs))
	for _, conv := range convs {
		var lastMsg models.Message
		models.DB.Where("conversation_id = ? AND status != ?", conv.ID, models.StatusDeleted).
			Order("created_at DESC").First(&lastMsg)

		var lastReadAt time.Time
		for _, m := range conv.Members {
			if m.UserID == userID {
				lastReadAt = m.LastReadAt
				break
			}
		}
		unread, _ := models.GetUnreadCount(models.DB, conv.ID, userID, lastReadAt)

		result = append(result, ConvWithMeta{
			Conversation: conv,
			LastMessage:  &lastMsg,
			UnreadCount:  unread,
		})
	}

	response.Success("ok", gin.H{"conversations": result}, c)
}

// ─── POST /api/v1/chat/conversations ───
func CreateConversation(c *gin.Context) {
	var body struct {
		UserID uint `json:"user_id"` // 对方用户 ID
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.UserID == 0 {
		response.Failed("user_id is required", c)
		return
	}

	currentUserID := c.GetUint("userId")
	if currentUserID == 0 {
		currentUserID = 1
	}

	conv, err := models.FindOrCreatePrivateConv(models.DB, currentUserID, body.UserID)
	if err != nil {
		response.Failed("Failed to create conversation", c)
		return
	}
	response.Success("ok", conv, c)
}

// ─── GET /api/v1/chat/conversations/:id/messages ───
func GetMessages(c *gin.Context) {
	convID, _ := strconv.Atoi(c.Param("id"))
	if convID == 0 {
		response.Failed("Invalid conversation ID", c)
		return
	}

	limitStr := c.DefaultQuery("limit", "100")
	offsetStr := c.DefaultQuery("offset", "0")
	limit, _ := strconv.Atoi(limitStr)
	offset, _ := strconv.Atoi(offsetStr)
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	msgs, err := models.GetConversationMessages(models.DB, uint(convID), limit, offset)
	if err != nil {
		response.Failed("Failed to fetch messages", c)
		return
	}
	if msgs == nil {
		msgs = []models.Message{}
	}
	response.Success("ok", gin.H{"messages": msgs}, c)
}

// ─── POST /api/v1/chat/messages ───
func PostMessage(c *gin.Context) {
	var msg models.Message
	if err := c.ShouldBindJSON(&msg); err != nil {
		response.Failed("Invalid message format", c)
		return
	}
	if msg.Content == "" || msg.ConversationID == 0 {
		response.Failed("content and conversation_id are required", c)
		return
	}
	if msg.SenderID == 0 {
		msg.SenderID = c.GetUint("userId")
		if msg.SenderID == 0 {
			msg.SenderID = 1
		}
	}
	if msg.MessageType == 0 {
		msg.MessageType = models.MsgText
	}
	msg.Status = models.StatusActive

	if err := models.SaveMessage(models.DB, &msg); err != nil {
		response.Failed("Failed to save message", c)
		return
	}

	// 更新会话时间
	models.DB.Model(&models.Conversation{}).Where("id = ?", msg.ConversationID).
		Update("updated_at", time.Now())

	// 广播
	BroadcastMessage(msg)

	response.Success("Message sent", msg, c)
}

// ─── PUT /api/v1/chat/messages/:id/recall ───
func RecallMessage(c *gin.Context) {
	msgID, _ := strconv.Atoi(c.Param("id"))
	senderID := c.GetUint("userId")
	if senderID == 0 {
		senderID = 1
	}

	if err := models.RecallMessage(models.DB, uint(msgID), senderID); err != nil {
		response.Failed("Failed to recall message", c)
		return
	}

	// 广播撤回事件
	data, _ := json.Marshal(gin.H{
		"type":    "recall",
		"msg_id":  msgID,
		"user_id": senderID,
	})
	ChatHub.broadcast <- data

	response.Success("Message recalled", nil, c)
}

// ─── PUT /api/v1/chat/conversations/:id/read ───
func MarkConversationRead(c *gin.Context) {
	convID, _ := strconv.Atoi(c.Param("id"))
	userID := c.GetUint("userId")
	if userID == 0 {
		userID = 1
	}

	models.DB.Model(&models.ConversationMember{}).
		Where("conversation_id = ? AND user_id = ?", convID, userID).
		Update("last_read_at", time.Now())

	response.Success("Marked as read", nil, c)
}

// ─── GET /api/v1/chat/users ───
func GetChatUsers(c *gin.Context) {
	var users []models.User
	models.DB.Select("id, username, email, avatar, status, last_login_at").Find(&users)
	if users == nil {
		users = []models.User{}
	}
	response.Success("ok", gin.H{"users": users}, c)
}
