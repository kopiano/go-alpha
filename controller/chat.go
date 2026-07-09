package controller

import (
	"context"
	"encoding/json"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"go-alpha/models"
	"go-alpha/response"
)

// ─── Shared chat helpers ───────────────────────────────────────

type postMessageBody struct {
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

func normalizeMessageBody(body *postMessageBody) {
	if body.MessageType == 0 {
		body.MessageType = models.MsgText
	}
	if body.Content != "" {
		return
	}
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

func loadSenderProfile(senderID uint) models.User {
	var sender models.User
	models.DB.Select("id, username, avatar").First(&sender, senderID)
	return sender
}

func successMessagePayload(msg models.Message, sender models.User) gin.H {
	payload := gin.H{
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
	}
	if msg.ChatType != "" {
		payload["chat_type"] = msg.ChatType
	}
	if msg.ReceiverID > 0 {
		payload["receiver_id"] = msg.ReceiverID
	}
	if msg.GroupID > 0 {
		payload["group_id"] = msg.GroupID
	}
	if msg.FileName != "" {
		payload["file_name"] = msg.FileName
	}
	if msg.FileURL != "" {
		payload["file_url"] = msg.FileURL
	}
	return payload
}

func ackMessagePayload(msg models.Message, sender models.User, clientMsgID string) gin.H {
	payload := gin.H{
		"type":            "message",
		"event":           "message.ack",
		"client_msg_id":   clientMsgID,
		"id":              msg.ID,
		"conversation_id": msg.ConversationID,
		"sender_id":       msg.SenderID,
		"sender_username": sender.Username,
		"sender_avatar":   sender.Avatar,
		"message_type":    msg.MessageType,
		"content":         msg.Content,
		"status":          1,
		"delivery_status": "confirmed",
		"created_at":      msg.CreatedAt,
		"time":            msg.CreatedAt.Format("15:04"),
	}
	if msg.ChatType != "" {
		payload["chat_type"] = msg.ChatType
	}
	if msg.ReceiverID > 0 {
		payload["receiver_id"] = msg.ReceiverID
	}
	if msg.GroupID > 0 {
		payload["group_id"] = msg.GroupID
	}
	if msg.FileName != "" {
		payload["file_name"] = msg.FileName
	}
	if msg.FileURL != "" {
		payload["file_url"] = msg.FileURL
	}
	return payload
}

func buildChatMessage(senderID uint, body *postMessageBody) (models.Message, models.User, error) {
	normalizeMessageBody(body)

	switch body.ChatType {
	case "private":
		recipientID := body.ReceiverID
		if recipientID == 0 {
			recipientID = body.RecipientID
		}
		if recipientID == 0 {
			return models.Message{}, models.User{}, gorm.ErrRecordNotFound
		}
		if recipientID == senderID {
			return models.Message{}, models.User{}, gorm.ErrRecordNotFound
		}
		convID := body.ConversationID
		if convID == "" {
			convID = models.PrivateConvID(senderID, recipientID)
		}
		if convID == "" {
			return models.Message{}, models.User{}, gorm.ErrRecordNotFound
		}
		userA := models.PrivateConvUserA(convID)
		userB := models.PrivateConvUserB(convID)
		if senderID != userA && senderID != userB {
			return models.Message{}, models.User{}, gorm.ErrRecordNotFound
		}
		recipientID = models.PrivateConvRecipient(senderID, convID)
		if recipientID == 0 {
			return models.Message{}, models.User{}, gorm.ErrRecordNotFound
		}
		msg := models.Message{
			ConversationID: convID,
			ChatType:       "private",
			SenderID:       senderID,
			ReceiverID:     recipientID,
			MessageType:    body.MessageType,
			Content:        body.Content,
			FileName:       body.FileName,
			FileURL:        body.FileURL,
		}
		sender := loadSenderProfile(senderID)
		return msg, sender, nil
	case "group":
		memberIDs := models.GetGroupMembers(models.DB, body.GroupID)
		isMember := false
		for _, mid := range memberIDs {
			if mid == senderID {
				isMember = true
				break
			}
		}
		if !isMember {
			return models.Message{}, models.User{}, gorm.ErrRecordNotFound
		}
		convID := body.ConversationID
		if convID == "" {
			convID = models.EnsureGroupConversation(models.DB, body.GroupID, memberIDs)
		}
		msg := models.Message{
			ConversationID: convID,
			ChatType:       "group",
			SenderID:       senderID,
			GroupID:        body.GroupID,
			MessageType:    body.MessageType,
			Content:        body.Content,
			FileName:       body.FileName,
			FileURL:        body.FileURL,
		}
		sender := loadSenderProfile(senderID)
		return msg, sender, nil
	default:
		return models.Message{}, models.User{}, gorm.ErrRecordNotFound
	}
}

func persistAndBroadcastMessage(msg *models.Message, sender models.User) error {
	saveStart := time.Now()
	if err := models.SaveMessage(models.DB, msg); err != nil {
		return err
	}
	slog.Info("chat.post_message timing", "chat_type", msg.ChatType, "conversation_id", msg.ConversationID, "save_ms", time.Since(saveStart).Milliseconds())

	broadcastStart := time.Now()
	BroadcastMessageWithSender(*msg, sender.Username, sender.Avatar)
	BroadcastConversationUpdate(msg.ConversationID, map[string]any{
		"last_message":      msg.Content,
		"last_message_type": msg.MessageType,
		"last_sender_id":    msg.SenderID,
	})
	if msg.ChatType == "private" {
		invalidateChatConvCache(msg.ConversationID)
		models.TouchConversationList(msg.SenderID, msg.ReceiverID)
		invalidateChatUserInfoCache(msg.SenderID, msg.ReceiverID)
	} else {
		memberIDs := models.GetConversationMemberIDs(models.DB, msg.ConversationID)
		models.TouchConversationList(memberIDs...)
		invalidateChatUserInfoCache(msg.SenderID, memberIDs...)
	}
	slog.Info("chat.post_message broadcast", "chat_type", msg.ChatType, "conversation_id", msg.ConversationID, "broadcast_ms", time.Since(broadcastStart).Milliseconds())
	return nil
}

func broadcastSavedMessageWithClientID(msg models.Message, senderUsername, senderAvatar, clientMsgID string) {
	msgTypeStr := "text"
	switch msg.MessageType {
	case models.MsgEmoji:
		msgTypeStr = "emoji"
	case models.MsgImage:
		msgTypeStr = "image"
	case models.MsgFile:
		msgTypeStr = "file"
	}

	wsMsg := map[string]any{
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
		"client_msg_id":   clientMsgID,
		"created_at":      msg.CreatedAt.Format(time.RFC3339),
		"time":            msg.CreatedAt.Format("15:04"),
	}
	data, err := json.Marshal(wsMsg)
	if err != nil {
		slog.Error("Failed to marshal broadcast message", "error", err)
		return
	}
	broadcastToAll(data)
}

func chatUserInfoCacheKey(userID uint) string {
	return "chat:user_info:" + strconv.Itoa(int(userID))
}

func chatUserContactsCacheKey(userID uint) string {
	return "chat:user_contacts:" + strconv.Itoa(int(userID))
}

func chatTeamInfoCacheKey() string {
	return "chat:team_info"
}

func getCachedChatUserInfo(userID uint) (map[string]any, bool) {
	if models.RDB == nil {
		return nil, false
	}
	data, err := models.RDB.Get(context.Background(), chatUserInfoCacheKey(userID)).Result()
	if err != nil {
		return nil, false
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(data), &out); err != nil {
		return nil, false
	}
	return out, true
}

func setCachedChatUserInfo(userID uint, data map[string]any) {
	if models.RDB == nil {
		return
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return
	}
	_ = models.RDB.Set(context.Background(), chatUserInfoCacheKey(userID), payload, 30*time.Second).Err()
}

func getCachedChatUserContacts(userID uint) ([]ContactInfo, bool) {
	if models.RDB == nil {
		return nil, false
	}
	data, err := models.RDB.Get(context.Background(), chatUserContactsCacheKey(userID)).Bytes()
	if err != nil {
		return nil, false
	}
	var contacts []ContactInfo
	if err := json.Unmarshal(data, &contacts); err != nil {
		return nil, false
	}
	return contacts, true
}

func setCachedChatUserContacts(userID uint, contacts []ContactInfo) {
	if models.RDB == nil {
		return
	}
	payload, err := json.Marshal(contacts)
	if err != nil {
		return
	}
	_ = models.RDB.Set(context.Background(), chatUserContactsCacheKey(userID), payload, 60*time.Second).Err()
}

func getCachedTeamInfo() (gin.H, bool) {
	if models.RDB == nil {
		return nil, false
	}
	data, err := models.RDB.Get(context.Background(), chatTeamInfoCacheKey()).Bytes()
	if err != nil {
		return nil, false
	}
	var teamInfo gin.H
	if err := json.Unmarshal(data, &teamInfo); err != nil {
		return nil, false
	}
	return teamInfo, true
}

func setCachedTeamInfo(teamInfo gin.H) {
	if models.RDB == nil {
		return
	}
	payload, err := json.Marshal(teamInfo)
	if err != nil {
		return
	}
	_ = models.RDB.Set(context.Background(), chatTeamInfoCacheKey(), payload, 5*time.Minute).Err()
}

func getChatUserContacts(userID uint) []ContactInfo {
	type convSummary struct {
		convID string
	}
	otherMap := make(map[uint]*convSummary)

	type rawConv struct {
		ConversationID string
		PartnerID      uint
	}
	var rawConvs []rawConv
	models.DB.Model(&models.Message{}).
		Select("DISTINCT conversation_id, receiver_id AS partner_id").
		Where("sender_id = ? AND receiver_id > 0", userID).
		Scan(&rawConvs)
	models.DB.Model(&models.Message{}).
		Select("DISTINCT conversation_id, sender_id AS partner_id").
		Where("receiver_id = ? AND sender_id != ?", userID, userID).
		Scan(&rawConvs)

	for _, r := range rawConvs {
		if r.PartnerID == 0 || r.ConversationID == "" {
			continue
		}
		if _, ok := otherMap[r.PartnerID]; !ok {
			otherMap[r.PartnerID] = &convSummary{convID: r.ConversationID}
		}
	}

	otherIDs := mapKeys(otherMap)
	if len(otherIDs) == 0 {
		return []ContactInfo{}
	}

	contactUserMap := make(map[uint]models.User, len(otherIDs))
	var contactUsers []models.User
	models.DB.Select("id, username, avatar").Where("id IN ?", otherIDs).Find(&contactUsers)
	for _, u := range contactUsers {
		contactUserMap[u.ID] = u
	}

	fiveMinAgo := time.Now().Add(-5 * time.Minute)
	var recentlyActive []struct {
		ID uint
	}
	models.DB.Model(&models.User{}).
		Where("id IN ? AND last_login_at >= ?", otherIDs, fiveMinAgo).
		Pluck("id", &recentlyActive)
	recentOnline := make(map[uint]bool, len(recentlyActive))
	for _, u := range recentlyActive {
		recentOnline[u.ID] = true
	}
	onlineUsers := ChatHub.OnlineUserSet()

	convIDs := make([]string, 0, len(otherMap))
	for _, info := range otherMap {
		if info.convID != "" {
			convIDs = append(convIDs, info.convID)
		}
	}

	type latestRow struct {
		ConversationID string
		Content        string
		MessageType    int
		CreatedAt      time.Time
	}
	latestMap := make(map[string]latestRow, len(convIDs))
	if len(convIDs) > 0 {
		type maxRow struct {
			ConversationID string
			MaxID          uint
		}
		var maxRows []maxRow
		if err := models.DB.Model(&models.Message{}).
			Select("conversation_id, MAX(id) AS max_id").
			Where("conversation_id IN ?", convIDs).
			Group("conversation_id").
			Scan(&maxRows).Error; err == nil && len(maxRows) > 0 {
			msgIDs := make([]uint, 0, len(maxRows))
			for _, row := range maxRows {
				if row.MaxID > 0 {
					msgIDs = append(msgIDs, row.MaxID)
				}
			}
			if len(msgIDs) > 0 {
				var latestMsgs []struct {
					ID             uint
					ConversationID string
					Content        string
					MessageType    int
					CreatedAt      time.Time
				}
				models.DB.Model(&models.Message{}).
					Select("id, conversation_id, content, message_type, created_at").
					Where("id IN ?", msgIDs).
					Find(&latestMsgs)
				for _, msg := range latestMsgs {
					latestMap[msg.ConversationID] = latestRow{
						ConversationID: msg.ConversationID,
						Content:        msg.Content,
						MessageType:    msg.MessageType,
						CreatedAt:      msg.CreatedAt,
					}
				}
			}
		}
	}

	unreadMap := make(map[string]int64, len(convIDs))
	if len(convIDs) > 0 {
		type unreadRow struct {
			ConversationID string
			UnreadCount    int64
		}
		var unreadRows []unreadRow
		models.DB.Model(&models.Message{}).
			Select("conversation_id, COUNT(*) AS unread_count").
			Where("conversation_id IN ? AND receiver_id = ? AND status = 0", convIDs, userID).
			Group("conversation_id").
			Scan(&unreadRows)
		for _, row := range unreadRows {
			unreadMap[row.ConversationID] = row.UnreadCount
		}
	}

	result := make([]ContactInfo, 0, len(otherIDs))
	for uid, info := range otherMap {
		contact, ok := contactUserMap[uid]
		if !ok {
			continue
		}
		online := onlineUsers[uid] || recentOnline[uid]
		lastMsg := ""
		lastMsgType := 0
		lastTime := ""
		if info.convID != "" {
			if latest, ok := latestMap[info.convID]; ok {
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
			Unread:         unreadMap[info.convID],
		})
	}
	sortByLastTime(result)
	return result
}

func getChatTeamInfo() gin.H {
	if cached, ok := getCachedTeamInfo(); ok {
		return cached
	}
	teamInfo := gin.H{"id": 0, "name": "", "members": []map[string]any{}}
	var teamGroup models.Group
	if err := models.DB.Where("name = ?", "Group").First(&teamGroup).Error; err != nil {
		setCachedTeamInfo(teamInfo)
		return teamInfo
	}
	memberIDs := models.GetGroupMembers(models.DB, teamGroup.ID)
	members := make([]map[string]any, 0, len(memberIDs))
	if len(memberIDs) > 0 {
		var teamUsers []models.User
		models.DB.Select("id, username, avatar").Where("id IN ?", memberIDs).Find(&teamUsers)
		teamUserMap := make(map[uint]models.User, len(teamUsers))
		for _, u := range teamUsers {
			teamUserMap[u.ID] = u
		}
		for _, mid := range memberIDs {
			if u, ok := teamUserMap[mid]; ok {
				members = append(members, map[string]any{"user_id": u.ID, "username": u.Username, "avatar": u.Avatar})
			}
		}
	}
	teamInfo = gin.H{"id": teamGroup.ID, "name": teamGroup.Name, "members": members}
	setCachedTeamInfo(teamInfo)
	return teamInfo
}

// ─── Message send paths ─────────────────────────────────────────

func sendPrivateChatMessage(c *gin.Context, senderID uint, body *postMessageBody) {
	start := time.Now()
	msg, sender, err := buildChatMessage(senderID, body)
	if err != nil {
		response.Failed("invalid message", c)
		return
	}
	if err := persistAndBroadcastMessage(&msg, sender); err != nil {
		response.Failed("Failed to save message", c)
		return
	}
	slog.Info("chat.post_message timing", "chat_type", "private", "conversation_id", msg.ConversationID, "total_ms", time.Since(start).Milliseconds())
	response.Success("Message sent", successMessagePayload(msg, sender), c)
}

func sendGroupChatMessage(c *gin.Context, senderID uint, body *postMessageBody) {
	start := time.Now()
	msg, sender, err := buildChatMessage(senderID, body)
	if err != nil {
		response.Failed("invalid message", c)
		return
	}
	if err := persistAndBroadcastMessage(&msg, sender); err != nil {
		response.Failed("Failed to save message", c)
		return
	}
	slog.Info("chat.post_message timing", "chat_type", "group", "conversation_id", msg.ConversationID, "group_id", body.GroupID, "total_ms", time.Since(start).Milliseconds())
	response.Success("Message sent", successMessagePayload(msg, sender), c)
}

// ─── API response types ─────────────────────────────────────────

type ConversationUser struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Avatar   string `json:"avatar"`
}

type ConversationResponse struct {
	ConversationID  string             `json:"conversation_id"`
	Type            string             `json:"type"`
	Title           string             `json:"title"`
	Avatar          string             `json:"avatar"`
	LastMessage     string             `json:"last_message"`
	LastMessageType int                `json:"last_message_type"`
	LastMessageAt   time.Time          `json:"last_message_at"`
	UnreadCount     int64              `json:"unread_count"`
	Users           []ConversationUser `json:"users,omitempty"`
}

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

// ─── Conversation APIs ──────────────────────────────────────────

// ─── GET /api/v1/chat/:id/messages ───
// ─── GET /api/v1/chat/:id/messages ───
func GetMessages(c *gin.Context) {
	GetConversationMessagesV2(c)
}

func GetConversations(c *gin.Context) {
	userID := c.GetUint("userId")
	if userID == 0 {
		response.Failed("unauthorized", c)
		return
	}
	if cached, ok := models.GetCachedConversationList(userID); ok {
		out := make([]ConversationResponse, 0, len(cached))
		for _, item := range cached {
			res := ConversationResponse{
				ConversationID:  item.ConversationID,
				Type:            item.Type,
				Title:           item.Title,
				Avatar:          item.Avatar,
				LastMessage:     item.LastMessage,
				LastMessageType: item.LastMessageType,
				LastMessageAt:   item.LastMessageAt,
				UnreadCount:     item.UnreadCount,
			}
			for _, u := range item.Members {
				res.Users = append(res.Users, ConversationUser{UserID: u.UserID, Username: u.Username, Avatar: u.Avatar})
			}
			out = append(out, res)
		}
		response.Success("ok", gin.H{
			"conversations": out,
			"users":         getChatDirectoryUsers(userID, cached),
			"team":          getChatTeamInfo(),
		}, c)
		return
	}
	items, err := models.GetUserConversationsV2(models.DB, userID)
	if err != nil {
		response.Failed("failed to fetch conversations", c)
		return
	}
	out := make([]ConversationResponse, 0, len(items))
	for _, item := range items {
		res := ConversationResponse{
			ConversationID:  item.ConversationID,
			Type:            item.Type,
			Title:           item.Title,
			Avatar:          item.Avatar,
			LastMessage:     item.LastMessage,
			LastMessageType: item.LastMessageType,
			LastMessageAt:   item.LastMessageAt,
			UnreadCount:     item.UnreadCount,
		}
		for _, u := range item.Members {
			res.Users = append(res.Users, ConversationUser{UserID: u.UserID, Username: u.Username, Avatar: u.Avatar})
		}
		out = append(out, res)
	}
	models.CacheConversationList(userID, items)
	response.Success("ok", gin.H{
		"conversations": out,
		"users":         getChatDirectoryUsers(userID, items),
		"team":          getChatTeamInfo(),
	}, c)
}

type chatDirectoryUser struct {
	UserID         uint   `json:"user_id"`
	Username       string `json:"username"`
	Avatar         string `json:"avatar"`
	ConversationID string `json:"conversation_id,omitempty"`
	LastMsg        string `json:"last_msg,omitempty"`
	LastMsgType    int    `json:"last_msg_type,omitempty"`
	LastTime       string `json:"last_time,omitempty"`
	Unread         int64  `json:"unread,omitempty"`
	Online         bool   `json:"online,omitempty"`
}

func getChatDirectoryUsers(userID uint, items []models.ConversationListItem) []chatDirectoryUser {
	conversationMap := make(map[uint]models.ConversationListItem)
	for _, item := range items {
		if item.Type != models.ConversationTypePrivate || len(item.Members) == 0 {
			continue
		}
		conversationMap[item.Members[0].UserID] = item
	}

	var users []models.User
	if err := models.DB.Select("id, username, avatar").Where("id <> ?", userID).Order("id ASC").Find(&users).Error; err != nil {
		return []chatDirectoryUser{}
	}

	onlineUsers := ChatHub.OnlineUserSet()
	fiveMinAgo := time.Now().Add(-5 * time.Minute)
	var recentlyActive []uint
	models.DB.Model(&models.User{}).Where("id <> ? AND last_login_at >= ?", userID, fiveMinAgo).Pluck("id", &recentlyActive)
	recentOnline := make(map[uint]bool, len(recentlyActive))
	for _, uid := range recentlyActive {
		recentOnline[uid] = true
	}

	result := make([]chatDirectoryUser, 0, len(users))
	for _, u := range users {
		item := chatDirectoryUser{
			UserID:   u.ID,
			Username: u.Username,
			Avatar:   u.Avatar,
			Online:   onlineUsers[u.ID] || recentOnline[u.ID],
		}
		if conv, ok := conversationMap[u.ID]; ok {
			item.ConversationID = conv.ConversationID
			item.LastMsg = conv.LastMessage
			item.LastMsgType = conv.LastMessageType
			if !conv.LastMessageAt.IsZero() {
				item.LastTime = conv.LastMessageAt.Format("2006-01-02 15:04:05")
			}
			item.Unread = conv.UnreadCount
		}
		result = append(result, item)
	}
	return result
}

func GetConversationMessagesV2(c *gin.Context) {
	convID := c.Param("id")
	userID := c.GetUint("userId")
	if convID == "" || userID == 0 {
		response.Failed("invalid request", c)
		return
	}
	if !canAccessConversation(models.DB, convID, userID) {
		a := models.PrivateConvUserA(convID)
		b := models.PrivateConvUserB(convID)
		if a > 0 && b > 0 && (userID == a || userID == b) {
			if _, err := models.EnsurePrivateConversation(models.DB, a, b); err != nil {
				response.Failed("failed to ensure conversation", c)
				return
			}
		}
		if strings.HasPrefix(convID, "g_") {
			groupID, err := strconv.ParseUint(strings.TrimPrefix(convID, "g_"), 10, 64)
			if err == nil && groupID > 0 {
				memberIDs := models.GetGroupMembers(models.DB, uint(groupID))
				for _, mid := range memberIDs {
					if mid == userID {
						models.EnsureGroupConversation(models.DB, uint(groupID), memberIDs)
						break
					}
				}
			}
		}
	}
	if !canAccessConversation(models.DB, convID, userID) {
		response.Failed("not a member of this conversation", c)
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	beforeID, _ := strconv.ParseUint(c.DefaultQuery("before_id", "0"), 10, 64)
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if beforeID > 0 {
		msgs, err := models.GetConversationMessagesBefore(models.DB, convID, uint(beforeID), limit)
		if err != nil {
			response.Failed("failed to fetch messages", c)
			return
		}
		response.Success("ok", gin.H{"messages": models.PopulateSenderForMessages(models.DB, msgs)}, c)
		return
	}
	msgs, err := models.GetConversationMessages(models.DB, convID, limit, offset)
	if err != nil {
		response.Failed("failed to fetch messages", c)
		return
	}
	response.Success("ok", gin.H{"messages": models.PopulateSenderForMessages(models.DB, msgs)}, c)
}

func MarkConversationReadV2(c *gin.Context) {
	convID := c.Param("id")
	userID := c.GetUint("userId")
	if convID == "" || userID == 0 {
		response.Failed("conversation_id is required", c)
		return
	}
	if !canAccessConversation(models.DB, convID, userID) {
		slog.Warn("MarkConversationReadV2: skip unread update for inaccessible conversation", "conversation_id", convID, "user_id", userID)
		response.Success("ok", gin.H{}, c)
		return
	}
	models.MarkConversationRead(models.DB, convID, userID)
	models.TouchConversationList(userID)
	BroadcastConversationRead(convID, userID)
	BroadcastConversationUpdate(convID, map[string]any{"unread_count": 0, "is_read": true})
	response.Success("ok", gin.H{}, c)
}

func CreateConversationV2(c *gin.Context) {
	var body struct {
		UserID uint `json:"user_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.UserID == 0 {
		response.Failed("user_id is required", c)
		return
	}
	senderID := c.GetUint("userId")
	if senderID == 0 || senderID == body.UserID {
		response.Failed("invalid user_id", c)
		return
	}
	convID, err := models.EnsurePrivateConversation(models.DB, senderID, body.UserID)
	if err != nil {
		response.Failed("failed to create conversation", c)
		return
	}
	response.Success("ok", gin.H{"conversation_id": convID}, c)
}

func GetGroups(c *gin.Context) {
	response.Success("ok", gin.H{"groups": []gin.H{getChatTeamInfo()}}, c)
}

func canAccessConversation(db *gorm.DB, convID string, userID uint) bool {
	var count int64
	db.Model(&models.ConversationMember{}).
		Where("conversation_id = ? AND user_id = ? AND left_at IS NULL", convID, userID).
		Count(&count)
	if count > 0 {
		return true
	}
	return backfillPrivateConversationMember(db, convID, userID)
}

func backfillPrivateConversationMember(db *gorm.DB, convID string, userID uint) bool {
	peerID := models.PrivateConvRecipient(userID, convID)
	if peerID == 0 {
		return false
	}

	var msgCount int64
	if err := db.Model(&models.Message{}).
		Where("conversation_id = ? AND ((sender_id = ? AND receiver_id = ?) OR (sender_id = ? AND receiver_id = ?))",
			convID, userID, peerID, peerID, userID).
		Count(&msgCount).Error; err != nil || msgCount == 0 {
		return false
	}

	now := time.Now()
	members := []models.ConversationMember{
		{ConversationID: convID, UserID: userID, JoinedAt: now, LastReadAt: now},
		{ConversationID: convID, UserID: peerID, JoinedAt: now, LastReadAt: now},
	}
	for _, member := range members {
		var existing models.ConversationMember
		err := db.Unscoped().
			Where("conversation_id = ? AND user_id = ?", member.ConversationID, member.UserID).
			First(&existing).Error
		if err == nil {
			if existing.LeftAt.Valid {
				if err := db.Model(&existing).Update("left_at", nil).Error; err != nil {
					slog.Warn("backfillPrivateConversationMember: restore member failed", "conversation_id", convID, "user_id", member.UserID, "error", err)
				}
			}
			continue
		}
		if err := db.Create(&member).Error; err != nil {
			slog.Warn("backfillPrivateConversationMember: create member failed", "conversation_id", convID, "user_id", member.UserID, "error", err)
			return false
		}
	}
	return true
}

// ─── Message APIs ───────────────────────────────────────────────

func PostMessage(c *gin.Context) {
	var body postMessageBody
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
		response.Failed("unauthorized", c)
		return
	}
	normalizeMessageBody(&body)

	if body.ChatType == "" {
		switch {
		case body.GroupID > 0:
			body.ChatType = "group"
		case body.ReceiverID > 0 || body.RecipientID > 0:
			body.ChatType = "private"
		}
	}

	switch body.ChatType {
	case "private":
		receiverID := body.ReceiverID
		if receiverID == 0 {
			receiverID = body.RecipientID
		}
		if receiverID == 0 || receiverID == senderID {
			response.Failed("Invalid receiver", c)
			return
		}
		body.ReceiverID = receiverID
		body.ConversationID = models.PrivateConvID(senderID, receiverID)
		sendPrivateChatMessage(c, senderID, &body)
		return
	case "group":
		if body.GroupID > 0 {
			sendGroupChatMessage(c, senderID, &body)
			return
		}
		response.Failed("group_id is required for chat_type=group", c)
		return
	}

	response.Failed("invalid chat_type, expected private or group", c)
}

// ─── User info / presence ──────────────────────────────────────

func GetChatUserInfo(c *gin.Context) {
	userID := c.GetUint("userId")
	if userID == 0 {
		userID = 1
	}
	if cached, ok := getCachedChatUserInfo(userID); ok {
		response.Success("ok", cached, c)
		return
	}
	if cachedContacts, ok := getCachedChatUserContacts(userID); ok {
		data := gin.H{"contacts": cachedContacts, "total": len(cachedContacts), "team": getChatTeamInfo()}
		setCachedChatUserInfo(userID, data)
		response.Success("ok", data, c)
		return
	}

	contacts := getChatUserContacts(userID)
	setCachedChatUserContacts(userID, contacts)
	teamInfo := getChatTeamInfo()
	data := gin.H{"contacts": contacts, "total": len(contacts), "team": teamInfo}
	setCachedChatUserInfo(userID, data)

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

// invalidateChatUserInfoCache 清除指定用户及其会话伙伴的 chat:user_info 缓存
// 调用方应尽量直接传入已知成员，避免额外查询消息表
func invalidateChatUserInfoCache(userID uint, peerIDs ...uint) {
	if models.RDB == nil {
		return
	}
	delKey := func(uid uint) {
		if uid == 0 {
			return
		}
		models.RDB.Del(context.Background(), chatUserInfoCacheKey(uid))
		models.RDB.Del(context.Background(), chatUserContactsCacheKey(uid))
	}

	delKey(userID)

	for _, uid := range peerIDs {
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
	for _, uid := range models.GetConversationMemberIDs(models.DB, convID) {
		models.RDB.Del(ctx, chatUserInfoCacheKey(uid))
		models.RDB.Del(ctx, chatUserContactsCacheKey(uid))
		models.RDB.Del(ctx, "chat:conversations:"+strconv.Itoa(int(uid)))
	}
}

// ─── Legacy compatibility ──────────────────────────────────────

// ─── GET /api/v1/chat/team ───
// 获取 Team 群组信息（含成员列表）
func GetTeam(c *gin.Context) {
	GetGroups(c)
}

// ─── POST /api/v1/chat/conversations ───
// 获取或创建与指定用户的私聊会话
func CreateConversation(c *gin.Context) {
	CreateConversationV2(c)
}

// ─── PUT /api/v1/chat/conversations/:id/read ───
// 标记会话已读
func MarkConversationRead(c *gin.Context) {
	MarkConversationReadV2(c)
}
