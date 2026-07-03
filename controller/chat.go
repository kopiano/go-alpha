package controller

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"go-alpha/models"
	"go-alpha/response"
)

// ContactInfo 联系人信息（/user_info 接口返回结构）
type ContactInfo struct {
	UserID         uint   `json:"user_id"`
	Username       string `json:"username"`
	Avatar         string `json:"avatar"`
	Online         bool   `json:"online"`
	ConversationID string `json:"conversation_id"`
	LastMsg        string `json:"last_msg"`
	LastMsgType    int    `json:"last_msg_type"`
	LastTime       string `json:"last_time"`
	Unread         int64  `json:"unread"`
}

// ─── GET /api/v1/chat/:id/messages ───
// ─── GET /api/v1/chat/:id/messages ───
func GetMessages(c *gin.Context) {
	convID := c.Param("id")
	if convID == "" {
		response.Failed("Invalid conversation ID", c)
		return
	}

	userID := c.GetUint("userId")
	if userID > 0 {
		// 群聊会话："g_{groupID}" 格式
		if len(convID) > 2 && convID[:2] == "g_" {
			groupIDStr := convID[2:]
			groupID, err := strconv.ParseUint(groupIDStr, 10, 64)
			if err == nil {
				memberIDs := models.GetGroupMembers(models.DB, uint(groupID))
				isMember := false
				for _, mid := range memberIDs {
					if mid == userID {
						isMember = true
						break
					}
				}
				if !isMember {
					response.Failed("Not a member of this conversation", c)
					return
				}
			}
		} else {
			// 私聊会话
			userA := models.PrivateConvUserA(convID)
			userB := models.PrivateConvUserB(convID)
			if userID != userA && userID != userB {
				response.Failed("Not a member of this conversation", c)
				return
			}
		}
	}

	limitStr := c.DefaultQuery("limit", "100")
	offsetStr := c.DefaultQuery("offset", "0")
	limit, _ := strconv.Atoi(limitStr)
	offset, _ := strconv.Atoi(offsetStr)
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	msgs, err := models.GetConversationMessages(models.DB, convID, limit, offset)
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

func PostMessage(c *gin.Context) {
	var body struct {
		ConversationID string `json:"conversation_id"`
		RecipientID    uint   `json:"recipient_id"`
		ReceiverID     uint   `json:"receiver_id"`
		ChatType       string `json:"chat_type"`
		GroupID        uint   `json:"group_id"`
		MessageType    int    `json:"message_type"`
		Content        string `json:"content"`
		FileName       string `json:"file_name"`
		FileURL        string `json:"file_url"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Failed("invalid request body", c)
		return
	}
	if body.MessageType == 0 || body.MessageType == models.MsgText {
		if body.Content == "" {
			response.Failed("content is required for text/reply messages", c)
			return
		}
	}

	senderID := c.GetUint("userId")
	if senderID == 0 {
		senderID = 1
	}

	// ── 私聊消息 ──
	if body.ChatType == "private" {
		receiverID := body.ReceiverID
		if receiverID == 0 {
			receiverID = body.RecipientID
		}
		if receiverID == 0 || receiverID == senderID {
			response.Failed("Invalid receiver", c)
			return
		}
		convID, _ := models.FindOrCreatePrivateConv(models.DB, senderID, receiverID)

		if body.MessageType == 0 {
			body.MessageType = models.MsgText
		}
		if body.Content == "" {
			switch body.MessageType {
			case models.MsgImage:
				body.Content = "[图片]"
			case models.MsgFile:
				if body.FileName != "" {
					body.Content = body.FileName
				} else {
					body.Content = "[文件]"
				}
			case models.MsgEmoji:
				body.Content = "[表情]"
			}
		}

		msg := models.Message{
			ConversationID: convID,
			ChatType:       "private",
			SenderID:       senderID,
			ReceiverID:     receiverID,
			MessageType:    body.MessageType,
			Content:        body.Content,
		}

		if err := models.SaveMessage(models.DB, &msg); err != nil {
			response.Failed("Failed to save message", c)
			return
		}

		var sender models.User
		models.DB.First(&sender, senderID)
		BroadcastMessageWithSender(msg, sender.Username, sender.Avatar)

		invalidateChatConvCache(convID)
		invalidateChatUserInfoCache(receiverID)

		response.Success("Message sent", gin.H{
			"id":              msg.ID,
			"conversation_id": msg.ConversationID,
			"chat_type":       "private",
			"sender_id":       msg.SenderID,
			"receiver_id":     msg.ReceiverID,
			"sender_username": sender.Username,
			"sender_avatar":   sender.Avatar,
			"message_type":    msg.MessageType,
			"content":         msg.Content,
			"status":          msg.Status,
			"created_at":      msg.CreatedAt,
			"updated_at":      msg.UpdatedAt,
		}, c)
		return
	}

	// ── 群聊消息 ──
	if body.ChatType == "group" && body.GroupID > 0 {
		memberIDs := models.GetGroupMembers(models.DB, body.GroupID)
		isMember := false
		for _, mid := range memberIDs {
			if mid == senderID {
				isMember = true
				break
			}
		}
		if !isMember {
			response.Failed("You are not a member of this group", c)
			return
		}

		convID := "g_" + strconv.FormatUint(uint64(body.GroupID), 10)

		if body.MessageType == 0 {
			body.MessageType = models.MsgText
		}
		if body.Content == "" {
			switch body.MessageType {
			case models.MsgImage:
				body.Content = "[图片]"
			case models.MsgFile:
				if body.FileName != "" {
					body.Content = body.FileName
				} else {
					body.Content = "[文件]"
				}
			case models.MsgEmoji:
				body.Content = "[表情]"
			}
		}

		msg := models.Message{
			ConversationID: convID,
			ChatType:       "group",
			SenderID:       senderID,
			GroupID:        body.GroupID,
			MessageType:    body.MessageType,
			Content:        body.Content,
		}

		if err := models.SaveMessage(models.DB, &msg); err != nil {
			response.Failed("Failed to save message", c)
			return
		}

		var sender models.User
		models.DB.First(&sender, senderID)
		BroadcastMessageWithSender(msg, sender.Username, sender.Avatar)

		response.Success("Message sent", gin.H{
			"id":              msg.ID,
			"conversation_id": msg.ConversationID,
			"chat_type":       "group",
			"group_id":        body.GroupID,
			"sender_id":       msg.SenderID,
			"sender_username": sender.Username,
			"sender_avatar":   sender.Avatar,
			"message_type":    msg.MessageType,
			"content":         msg.Content,
			"status":          1,
			"created_at":      msg.CreatedAt,
			"updated_at":      msg.UpdatedAt,
		}, c)
		return
	}

	// ── 私聊消息 ──
	convID := body.ConversationID
	if convID == "" && body.RecipientID > 0 {
		if body.RecipientID == senderID {
			response.Failed("Cannot send message to yourself", c)
			return
		}
		convID, _ = models.FindOrCreatePrivateConv(models.DB, senderID, body.RecipientID)
	}
	if convID == "" {
		response.Failed("conversation_id or recipient_id is required", c)
		return
	}

	userA := models.PrivateConvUserA(convID)
	userB := models.PrivateConvUserB(convID)
	if senderID != userA && senderID != userB {
		response.Failed("You are not a member of this conversation", c)
		return
	}

	if body.MessageType == 0 {
		body.MessageType = models.MsgText
	}
	if body.Content == "" {
		switch body.MessageType {
		case models.MsgImage:
			body.Content = "[图片]"
		case models.MsgFile:
			if body.FileName != "" {
				body.Content = body.FileName
			} else {
				body.Content = "[文件]"
			}
		case models.MsgEmoji:
			body.Content = "[表情]"
		}
	}

	recipientID := models.PrivateConvRecipient(senderID, convID)

	msg := models.Message{
		ConversationID: convID,
		ChatType:       "private",
		SenderID:       senderID,
		ReceiverID:     recipientID,
		MessageType:    body.MessageType,
		Content:        body.Content,
	}

	if err := models.SaveMessage(models.DB, &msg); err != nil {
		response.Failed("Failed to save message", c)
		return
	}

	var sender models.User
	models.DB.First(&sender, senderID)
	BroadcastMessageWithSender(msg, sender.Username, sender.Avatar)

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
		"status":          1,
		"created_at":      msg.CreatedAt,
		"updated_at":      msg.UpdatedAt,
	}, c)
}

func GetChatUserInfo(c *gin.Context) {
	userID := c.GetUint("userId")
	if userID == 0 {
		userID = 1
	}

	// 尝试读 Redis 缓存
	ctx := context.Background()
	cacheKey := "chat:user_info:" + strconv.Itoa(int(userID))
	if cached, err := models.RDB.Get(ctx, cacheKey).Result(); err == nil {
		var data map[string]any
		if json.Unmarshal([]byte(cached), &data) == nil {
			response.Success("ok", data, c)
			return
		}
	}

	// 从 messages 表获取用户参与的所有私聊会话
	type convSummary struct {
		convID string
	}
	otherMap := make(map[uint]*convSummary) // partnerUserID → summary

	// 查询该用户参与的所有会话（含对方 ID）
	type rawConv struct {
		ConversationID string
		PartnerID      uint
	}
	var rawConvs []rawConv
	// 我发送的消息 → 对方
	models.DB.Model(&models.Message{}).
		Select("DISTINCT conversation_id, receiver_id AS partner_id").
		Where("sender_id = ? AND receiver_id > 0", userID).
		Scan(&rawConvs)
	// 对方发给我的消息 → 对方
	models.DB.Model(&models.Message{}).
		Select("DISTINCT conversation_id, sender_id AS partner_id").
		Where("receiver_id = ? AND sender_id != ?", userID, userID).
		Scan(&rawConvs)

	for _, r := range rawConvs {
		if r.PartnerID == 0 || r.ConversationID == "" {
			continue
		}
		if _, ok := otherMap[r.PartnerID]; !ok {
			otherMap[r.PartnerID] = &convSummary{
				convID: r.ConversationID,
			}
		}
	}

	// 始终包含所有用户（不含自己），方便新注册用户即时展示
	var allUsers []struct{ ID uint }
	models.DB.Model(&models.User{}).Select("id").Where("id != ?", userID).Find(&allUsers)
	for _, u := range allUsers {
		if _, ok := otherMap[u.ID]; !ok {
			otherMap[u.ID] = &convSummary{}
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

		online := ChatHub.IsUserOnline(uid) || recentOnline[uid]

		lastMsg := ""
		lastMsgType := 0
		lastTime := ""
		var unread int64

		if info.convID != "" {
			var latest struct {
				ID          uint
				Content     string
				MessageType int
				CreatedAt   time.Time
				SenderID    uint
			}
			models.DB.Model(&models.Message{}).
				Where("conversation_id = ?", info.convID).
				Order("created_at DESC").Limit(1).
				Select("id, content, message_type, created_at, sender_id").Scan(&latest)
			if latest.ID > 0 {
				switch latest.MessageType {
				case models.MsgText:
					lastMsg = latest.Content
				case models.MsgEmoji:
					lastMsg = "[表情]"
				case models.MsgImage:
					lastMsg = "[图片]"
				case models.MsgFile:
					lastMsg = "[文件]"
				default:
					lastMsg = latest.Content
				}
				lastMsgType = latest.MessageType
				lastTime = latest.CreatedAt.Format("2006-01-02 15:04:05")
			}

			models.DB.Model(&models.Message{}).
				Where("conversation_id = ? AND sender_id = ? AND receiver_id = ? AND status = 0",
					info.convID, uid, userID).
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

	sortByLastTime(result)

	if result == nil {
		result = []ContactInfo{}
	}

	// 查询 Team 群聊
	type teamMember struct {
		UserID   uint   `json:"user_id"`
		Username string `json:"username"`
		Avatar   string `json:"avatar"`
	}
	teamInfo := gin.H{"id": 0, "name": "", "members": []teamMember{}}
	var teamGroup models.Group
	if err := models.DB.Where("name = ?", "Team").First(&teamGroup).Error; err == nil {
		memberIDs := models.GetGroupMembers(models.DB, teamGroup.ID)
		members := make([]teamMember, 0, len(memberIDs))
		for _, mid := range memberIDs {
			var u models.User
			if models.DB.Select("id, username, avatar").First(&u, mid).Error == nil {
				members = append(members, teamMember{UserID: u.ID, Username: u.Username, Avatar: u.Avatar})
			}
		}
		teamInfo = gin.H{"id": teamGroup.ID, "name": teamGroup.Name, "members": members}
	}

	data := gin.H{"contacts": result, "total": len(result), "team": teamInfo}
	if jsonData, err := json.Marshal(data); err == nil {
		models.RDB.Set(ctx, cacheKey, string(jsonData), 5*time.Second)
	}

	response.Success("ok", data, c)
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
// 从 messages 表查询会话伙伴（无需 conversation_member 表）
func invalidateChatUserInfoCache(userID uint) {
	if models.RDB == nil {
		return
	}
	delKey := func(uid uint) {
		models.RDB.Del(context.Background(), "chat:user_info:"+strconv.Itoa(int(uid)))
	}

	delKey(userID)

	var partnerIDs []uint
	models.DB.Model(&models.Message{}).
		Select("DISTINCT receiver_id").
		Where("sender_id = ? AND receiver_id > 0", userID).
		Pluck("receiver_id", &partnerIDs)
	models.DB.Model(&models.Message{}).
		Select("DISTINCT sender_id").
		Where("receiver_id = ? AND sender_id != ?", userID, userID).
		Pluck("sender_id", &partnerIDs)

	for _, uid := range partnerIDs {
		delKey(uid)
	}
}

// invalidateChatConvCache 清除指定会话中所有成员的 chat:user_info 缓存
// 从 messages 表查询会话成员（无需 conversation_member 表）
func invalidateChatConvCache(convID string) {
	if models.RDB == nil {
		return
	}
	ctx := context.Background()
	var allIDs []uint
	models.DB.Model(&models.Message{}).
		Select("DISTINCT sender_id").
		Where("conversation_id = ?", convID).
		Pluck("sender_id", &allIDs)
	models.DB.Model(&models.Message{}).
		Select("DISTINCT receiver_id").
		Where("conversation_id = ? AND receiver_id > 0", convID).
		Pluck("receiver_id", &allIDs)

	for _, uid := range allIDs {
		models.RDB.Del(ctx, "chat:user_info:"+strconv.Itoa(int(uid)))
	}
}

// ─── GET /api/v1/chat/team ───
// 获取 Team 群组信息（含成员列表）
func GetTeam(c *gin.Context) {
	teamGroup := models.GetTeamGroup(models.DB)
	if teamGroup == nil {
		response.Success("ok", gin.H{"id": 0, "name": "", "members": []gin.H{}}, c)
		return
	}
	memberIDs := models.GetGroupMembers(models.DB, teamGroup.ID)
	type memberInfo struct {
		UserID   uint   `json:"user_id"`
		Username string `json:"username"`
		Avatar   string `json:"avatar"`
	}
	members := make([]memberInfo, 0, len(memberIDs))
	for _, mid := range memberIDs {
		var u models.User
		if models.DB.Select("id, username, avatar").First(&u, mid).Error == nil {
			members = append(members, memberInfo{UserID: u.ID, Username: u.Username, Avatar: u.Avatar})
		}
	}
	response.Success("ok", gin.H{
		"id":      teamGroup.ID,
		"name":    teamGroup.Name,
		"members": members,
	}, c)
}

// ─── POST /api/v1/chat/conversations ───
// 获取或创建与指定用户的私聊会话
func CreateConversation(c *gin.Context) {
	var body struct {
		UserID uint `json:"user_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.UserID == 0 {
		response.Failed("user_id is required", c)
		return
	}
	senderID := c.GetUint("userId")
	if senderID == 0 || senderID == body.UserID {
		response.Failed("Invalid user_id", c)
		return
	}
	convID, _ := models.FindOrCreatePrivateConv(models.DB, senderID, body.UserID)
	response.Success("ok", gin.H{"conversation_id": convID}, c)
}

// ─── PUT /api/v1/chat/conversations/:id/read ───
// 标记会话已读
func MarkConversationRead(c *gin.Context) {
	convID := c.Param("id")
	if convID == "" {
		response.Failed("conversation_id is required", c)
		return
	}
	userID := c.GetUint("userId")
	models.MarkConversationRead(models.DB, convID, userID)
	response.Success("ok", gin.H{}, c)
}
