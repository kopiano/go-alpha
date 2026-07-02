package controller

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/datatypes"

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

	type ConvWithMeta struct {
		models.Conversation
		LastMessage *models.MessageWithSender `json:"last_message,omitempty"`
		UnreadCount int64                      `json:"unread_count"`
	}
	result := make([]ConvWithMeta, 0, len(convs))
	for _, conv := range convs {
		var lastMsg models.Message
		models.DB.Where("conversation_id = ? AND status != ?", conv.ID, models.StatusDeleted).
			Order("created_at DESC").First(&lastMsg)

		var lastMsgWithSender *models.MessageWithSender
		if lastMsg.ID > 0 {
			mws := lastMsg.PopulateSender(models.DB)
			lastMsgWithSender = &mws
		}

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
			LastMessage:  lastMsgWithSender,
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

	result := models.PopulateSenderForMessages(models.DB, msgs)
	response.Success("ok", gin.H{"messages": result}, c)
}

// ─── POST /api/v1/chat/messages ───
// 发送消息：支持 conversation_id 或 recipient_id，自动创建会话
func PostMessage(c *gin.Context) {
	var body struct {
		ConversationID uint   `json:"conversation_id"`
		RecipientID    uint   `json:"recipient_id"`
		MessageType    int    `json:"message_type"`
		Content        string `json:"content"`
		FileName       string `json:"file_name"`
		FileURL        string `json:"file_url"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Content == "" {
		response.Failed("content is required", c)
		return
	}

	senderID := c.GetUint("userId")
	if senderID == 0 {
		senderID = 1
	}

	// 确定 conversation_id：优先使用传入的，否则通过 recipient_id 查找/创建
	convID := body.ConversationID
	if convID == 0 && body.RecipientID > 0 {
		conv, _ := models.FindOrCreatePrivateConv(models.DB, senderID, body.RecipientID)
		if conv != nil {
			convID = conv.ID
		}
	}
	if convID == 0 {
		response.Failed("conversation_id or recipient_id is required", c)
		return
	}

	if body.MessageType == 0 {
		body.MessageType = models.MsgText
	}

	// 构建 metadata
	var meta datatypes.JSON
	if body.FileName != "" || body.FileURL != "" {
		meta, _ = json.Marshal(map[string]string{
			"file_name": body.FileName,
			"file_url":  body.FileURL,
		})
	}

	msg := models.Message{
		ConversationID: convID,
		SenderID:       senderID,
		MessageType:    body.MessageType,
		Content:        body.Content,
		Metadata:       meta,
		Status:         models.StatusActive,
	}

	if err := models.SaveMessage(models.DB, &msg); err != nil {
		response.Failed("Failed to save message", c)
		return
	}

	// 更新会话时间
	models.DB.Model(&models.Conversation{}).Where("id = ?", convID).
		Update("updated_at", time.Now())

	// 广播
	BroadcastMessage(msg)

	// 返回带发送者信息的消息
	result := msg.PopulateSender(models.DB)
	response.Success("Message sent", result, c)
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

	// 5 分钟内活跃的视为在线（不依赖 logout 更新 status）
	fiveMinAgo := time.Now().Add(-5 * time.Minute)
	var activeCount int64
	models.DB.Model(&models.User{}).Where("last_login_at >= ?", fiveMinAgo).Count(&activeCount)

	response.Success("ok", gin.H{"users": users, "active_count": activeCount}, c)
}