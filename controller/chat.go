package controller

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/datatypes"

	"go-alpha/models"
	"go-alpha/response"
)

// ContactInfo 联系人信息（/user_info 接口返回结构）
type ContactInfo struct {
	UserID         uint   `json:"user_id"`
	Username       string `json:"username"`
	Avatar         string `json:"avatar"`
	Online         bool   `json:"online"`
	ConversationID uint   `json:"conversation_id"`
	LastMsg        string `json:"last_msg"`
	LastMsgType    int    `json:"last_msg_type"`
	LastTime       string `json:"last_time"`
	Unread         int64  `json:"unread"`
}

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

	// 清除双方缓存，新联系人立即可见
	invalidateChatUserInfoCache(currentUserID)
	invalidateChatUserInfoCache(body.UserID)

	response.Success("ok", conv, c)
}

// ─── GET /api/v1/chat/conversations/:id/messages ───
func GetMessages(c *gin.Context) {
	convID, _ := strconv.Atoi(c.Param("id"))
	if convID == 0 {
		response.Failed("Invalid conversation ID", c)
		return
	}

	// 验证用户是会话成员
	userID := c.GetUint("userId")
	if userID > 0 {
		var count int64
		models.DB.Model(&models.ConversationMember{}).
			Where("conversation_id = ? AND user_id = ?", convID, userID).
			Count(&count)
		if count == 0 {
			response.Failed("Not a member of this conversation", c)
			return
		}
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
		ConversationID   uint   `json:"conversation_id"`
		RecipientID      uint   `json:"recipient_id"`
		MessageType      int    `json:"message_type"`
		Content          string `json:"content"`
		FileName         string `json:"file_name"`
		FileURL          string `json:"file_url"`
		ReplyToMessageID uint   `json:"reply_to_message_id"`
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

	// 处理回复
	var replyToID *uint
	if body.ReplyToMessageID > 0 {
		replyToID = &body.ReplyToMessageID
	}

	msg := models.Message{
		ConversationID:   convID,
		SenderID:         senderID,
		MessageType:      body.MessageType,
		Content:          body.Content,
		Metadata:         meta,
		Status:           models.StatusActive,
		ReplyToMessageID: replyToID,
	}

	if err := models.SaveMessage(models.DB, &msg); err != nil {
		response.Failed("Failed to save message", c)
		return
	}

	// 更新会话时间
	models.DB.Model(&models.Conversation{}).Where("id = ?", convID).
		Update("updated_at", time.Now())

	// 查询发送者信息（一次查询，广播 + 响应共用）
	var sender models.User
	models.DB.First(&sender, senderID)

	// 广播消息（避免 BroadcastMessage 再查一次 DB）
	BroadcastMessageWithSender(msg, sender.Username, sender.Avatar)

	// 清除相关用户信息缓存，使下次 /user_info 请求获取最新数据
	invalidateChatConvCache(convID)
	if body.RecipientID > 0 {
		invalidateChatUserInfoCache(body.RecipientID)
	}

	response.Success("Message sent", gin.H{
		"id":              msg.ID,
		"conversation_id": msg.ConversationID,
		"sender_id":       msg.SenderID,
		"sender_username": sender.Username,
		"sender_avatar":   sender.Avatar,
		"message_type":    msg.MessageType,
		"content":         msg.Content,
		"status":          msg.Status,
		"created_at":      msg.CreatedAt,
		"updated_at":      msg.UpdatedAt,
	}, c)
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

// ─── GET /api/v1/chat/team ───
// 返回团队群聊信息
func GetTeam(c *gin.Context) {
	var conv models.Conversation
	if err := models.DB.Where("type = ? AND name = ?", "group", "Team").Preload("Members").First(&conv).Error; err != nil {
		response.Failed("Team conversation not found", c)
		return
	}

	type MemberInfo struct {
		UserID   uint   `json:"user_id"`
		Username string `json:"username"`
		Avatar   string `json:"avatar"`
	}
	members := make([]MemberInfo, 0, len(conv.Members))
	for _, m := range conv.Members {
		var user models.User
		if models.DB.Select("id, username, avatar").First(&user, m.UserID).Error == nil {
			members = append(members, MemberInfo{UserID: user.ID, Username: user.Username, Avatar: user.Avatar})
		}
	}

	response.Success("ok", gin.H{
		"id":      conv.ID,
		"name":    conv.Name,
		"members": members,
	}, c)
}

// ─── GET /api/v1/chat/user_info ───
// 返回当前用户的联系人列表，含在线状态 / 最新消息 / 未读数
func GetChatUserInfo(c *gin.Context) {
	userID := c.GetUint("userId")
	if userID == 0 {
		userID = 1
	}

	// 获取当前用户的所有会话（含成员）
	convs, err := models.GetUserConversations(models.DB, userID)
	if err != nil {
		response.Failed("查询失败", c)
		return
	}

	// 收集所有其他参与者 ID → 会话信息
	type convInfo struct {
		convID     uint
		lastReadAt time.Time
	}
	otherMap := make(map[uint]*convInfo) // userID → conversation info
	for _, conv := range convs {
		for _, m := range conv.Members {
			if m.UserID == userID {
				// 记录当前用户在该会话中的 last_read_at
				if existing, ok := otherMap[userID]; ok {
					existing.lastReadAt = m.LastReadAt
				}
			} else {
				if existing, ok := otherMap[m.UserID]; !ok {
					otherMap[m.UserID] = &convInfo{convID: conv.ID, lastReadAt: time.Time{}}
				} else if existing.convID == 0 {
					existing.convID = conv.ID
				}
			}
		}
	}
	// 补全每个会话中当前用户对应的 last_read_at
	for _, conv := range convs {
		for _, m := range conv.Members {
			if m.UserID == userID {
				for _, info := range otherMap {
					if info.convID == conv.ID {
						info.lastReadAt = m.LastReadAt
					}
				}
			}
		}
	}

	// 如果没有会话，返回所有用户（不含自己）
	if len(otherMap) == 0 {
		var allUsers []struct{ ID uint }
		models.DB.Model(&models.User{}).Select("id").Where("id != ?", userID).Find(&allUsers)
		for _, u := range allUsers {
			otherMap[u.ID] = &convInfo{}
		}
	}

	// 批量查询在线状态（last_login_at >= 5 分钟内 或 WebSocket 在线）
	fiveMinAgo := time.Now().Add(-5 * time.Minute)
	var recentlyActive []struct {
		ID uint
	}
	otherIDs := mapKeys(otherMap)
	models.DB.Model(&models.User{}).
		Where("id IN ? AND last_login_at >= ?", otherIDs, fiveMinAgo).
		Pluck("id", &recentlyActive)
	recentOnline := make(map[uint]bool)
	for _, u := range recentlyActive {
		recentOnline[u.ID] = true
	}

	result := make([]ContactInfo, 0, len(otherIDs))

	for uid, info := range otherMap {
		var contact models.User
		if err := models.DB.Select("id, username, avatar").First(&contact, uid).Error; err != nil {
			continue
		}

		// 在线判断：WebSocket 连接存在 或 5 分钟内登录过
		online := ChatHub.IsUserOnline(uid) || recentOnline[uid]

		lastMsg := ""
		lastMsgType := 0
		lastTime := ""
		var unread int64

		if info.convID > 0 {
			// 查询会话中最新一条消息（不限发送者）
			var latest struct {
				ID          uint
				Content     string
				MessageType int
				CreatedAt   time.Time
				SenderID    uint
			}
			models.DB.Model(&models.Message{}).
				Where("conversation_id = ? AND status != ?", info.convID, models.StatusDeleted).
				Order("created_at DESC").Limit(1).
				Select("id, content, message_type, created_at, sender_id").Scan(&latest)
			if latest.ID > 0 {
				// 根据消息类型生成展示文本
				switch latest.MessageType {
				case models.MsgText, models.MsgReply:
					lastMsg = latest.Content
				case models.MsgEmoji:
					lastMsg = "[表情]"
				case models.MsgImage:
					lastMsg = "[图片]"
				case models.MsgFile:
					lastMsg = "[文件]"
				case models.MsgSystem:
					lastMsg = "[系统消息]"
				default:
					lastMsg = latest.Content
				}
				lastMsgType = latest.MessageType
				lastTime = latest.CreatedAt.Format("2006-01-02 15:04:05")
			}

			// 未读消息数：对方发送的、时间在 last_read_at 之后、状态正常的消息
			models.DB.Model(&models.Message{}).
				Where("conversation_id = ? AND sender_id = ? AND status = ? AND created_at > ?",
					info.convID, uid, models.StatusActive, info.lastReadAt).
				Count(&unread)
		}

		result = append(result, ContactInfo{
			UserID:         contact.ID,
			Username:       contact.Username,
			Avatar:         contact.Avatar,
			Online:         online,
			ConversationID: info.convID,
			LastMsg:        lastMsg,
			LastMsgType:    lastMsgType,
			LastTime:       lastTime,
			Unread:         unread,
		})
	}

	// 按最后消息时间降序排序（无消息的排在后面）
	sortByLastTime(result)

	if result == nil {
		result = []ContactInfo{}
	}

	// 查找团队群聊
	var teamConv struct {
		ID      uint
		Name    string
	}
	models.DB.Raw("SELECT id, name FROM conversation WHERE type = ? AND name = ? LIMIT 1", "group", "Team").Scan(&teamConv)
	var teamMembers []gin.H
	if teamConv.ID > 0 {
		var members []struct {
			UserID   uint
			Username string
			Avatar   string
		}
		models.DB.Table("conversation_member").Select("user_id, username, avatar").
			Joins("JOIN user ON user.id = conversation_member.user_id").
			Where("conversation_id = ?", teamConv.ID).Scan(&members)
		for _, m := range members {
			teamMembers = append(teamMembers, gin.H{
				"user_id":  m.UserID,
				"username": m.Username,
				"avatar":   m.Avatar,
			})
		}
	}

	response.Success("ok", gin.H{
		"contacts": result,
		"total":    len(result),
		"team": gin.H{
			"id":      teamConv.ID,
			"name":    teamConv.Name,
			"members": teamMembers,
		},
	}, c)
}

// sortByLastTime 按 LastTime 降序排序，无消息的排在最后
func sortByLastTime(items []ContactInfo) {
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			swap := false
			if items[i].LastTime == "" && items[j].LastTime != "" {
				swap = true
			} else if items[i].LastTime != "" && items[j].LastTime != "" && items[i].LastTime < items[j].LastTime {
				swap = true
			}
			if swap {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}

// mapKeys 提取 map 的 key 为 slice
func mapKeys[K comparable, V any](m map[K]V) []K {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// invalidateChatUserInfoCache 清除指定用户及其所有会话伙伴的 chat:user_info 缓存
func invalidateChatUserInfoCache(userID uint) {
	if models.RDB == nil {
		return
	}
	ctx := context.Background()

	var partnerIDs []uint
	models.DB.Model(&models.ConversationMember{}).
		Where("conversation_id IN (?)",
			models.DB.Table("conversation_member").Select("conversation_id").Where("user_id = ?", userID),
		).
		Where("user_id != ?", userID).
		Distinct("user_id").
		Pluck("user_id", &partnerIDs)

	allIDs := append(partnerIDs, userID)
	for _, uid := range allIDs {
		models.RDB.Del(ctx, "chat:user_info:"+strconv.Itoa(int(uid)))
	}
}

// invalidateChatConvCache 清除指定会话中所有成员的 chat:user_info 缓存
func invalidateChatConvCache(convID uint) {
	if models.RDB == nil {
		return
	}
	ctx := context.Background()
	var memberIDs []uint
	models.DB.Model(&models.ConversationMember{}).
		Where("conversation_id = ?", convID).Pluck("user_id", &memberIDs)
	for _, uid := range memberIDs {
		models.RDB.Del(ctx, "chat:user_info:"+strconv.Itoa(int(uid)))
	}
}