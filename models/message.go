package models

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

// MessageType 消息类型
const (
	MsgText  = 1 // 文本
	MsgEmoji = 2 // Emoji / 表情包
	MsgImage = 3 // 图片
	MsgFile  = 4 // 文件附件
)

// Message 聊天消息
type Message struct {
	ID             uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	ConversationID string `gorm:"type:varchar(64);not null;index:idx_chat_message_conv_created,priority:1;index:idx_chat_message_conv_id,priority:1" json:"conversation_id"`

	// sender / receiver
	ChatType   string `gorm:"type:varchar(20);not null;index" json:"chat_type"` // 消息类型：用户"private"、群聊"group"
	SenderID   uint   `gorm:"index:idx_chat_message_conv_sender_status,priority:2;index:idx_chat_message_sender_receiver,priority:1;not null" json:"sender_id"`
	ReceiverID uint   `gorm:"index:idx_chat_message_sender_receiver,priority:2;index:idx_chat_message_receiver_status,priority:1" json:"receiver_id"` // 私聊
	GroupID    uint   `gorm:"index" json:"group_id"`                                                                                                  // 群聊id
	// message
	Content     string         `gorm:"type:text;not null" json:"content"` // 内容
	MessageType int            `gorm:"default:1" json:"message_type"`     // 消息类型：1-4
	FileName    string         `gorm:"type:varchar(255)" json:"file_name"`
	FileURL     string         `gorm:"type:varchar(255)" json:"file_url"`
	ReplyToID   *uint          `gorm:"index" json:"reply_to_id,omitempty"`
	EditedAt    *time.Time     `json:"edited_at,omitempty"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	// status
	Status int `gorm:"index:idx_chat_message_conv_sender_status,priority:3;index:idx_chat_message_receiver_status,priority:2;default:0" json:"status"` // 消息状态：0未读，1为已读

	CreatedAt time.Time `gorm:"index:idx_chat_message_conv_created,priority:2;index:idx_chat_message_sender_receiver,priority:3" json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ─── 私聊会话辅助函数 ──────────────────────────────────────────

// PrivateConvID 计算私聊会话的确定性 conversation_id
// 采用字符串形式避免位运算方案在用户 ID 较大时发生碰撞。
func PrivateConvID(userA, userB uint) string {
	if userA == userB {
		return ""
	}
	if userA > userB {
		userA, userB = userB, userA
	}
	return fmt.Sprintf("p_%d_%d", userA, userB)
}

func parsePrivateConvUsers(convID string) (uint, uint, bool) {
	if strings.HasPrefix(convID, "p_") {
		parts := strings.Split(convID[2:], "_")
		if len(parts) != 2 {
			return 0, 0, false
		}
		a, err1 := strconv.ParseUint(parts[0], 10, 64)
		b, err2 := strconv.ParseUint(parts[1], 10, 64)
		if err1 != nil || err2 != nil {
			return 0, 0, false
		}
		return uint(a), uint(b), true
	}
	id, err := strconv.ParseUint(convID, 10, 64)
	if err != nil {
		return 0, 0, false
	}
	cid := uint(id)
	lo := cid & ((1 << 20) - 1)
	hi := cid >> 20
	if lo == 0 || hi == 0 {
		return 0, 0, false
	}
	return hi, lo, true
}

// PrivateConvRecipient 返回私聊中对方用户的 ID
func PrivateConvRecipient(senderID uint, convID string) uint {
	a, b, ok := parsePrivateConvUsers(convID)
	if !ok {
		return 0
	}
	if senderID == a {
		return b
	}
	if senderID == b {
		return a
	}
	return 0
}

// PrivateConvUserA 返回私聊中 ID 较小的用户（约定为 UserA）
func PrivateConvUserA(convID string) uint {
	a, _, ok := parsePrivateConvUsers(convID)
	if !ok {
		return 0
	}
	return a
}

// PrivateConvUserB 返回私聊中 ID 较大的用户（约定为 UserB）
func PrivateConvUserB(convID string) uint {
	_, b, ok := parsePrivateConvUsers(convID)
	if !ok {
		return 0
	}
	return b
}

// ─── 已读状态（使用 Message.Status 字段）────────────────────────

// MarkConversationRead 将用户在该会话的未读消息标记为已读
func MarkConversationRead(db *gorm.DB, convID string, userID uint) {
	db.Model(&Message{}).
		Where("conversation_id = ? AND receiver_id = ? AND status = 0", convID, userID).
		Update("status", 1)
	var lastID uint
	db.Model(&Message{}).Where("conversation_id = ?", convID).Select("IFNULL(MAX(id),0)").Scan(&lastID)
	db.Model(&ConversationMember{}).
		Where("conversation_id = ? AND user_id = ?", convID, userID).
		Updates(map[string]any{
			"last_read_at":         time.Now(),
			"last_read_message_id": lastID,
			"unread_count":         0,
		})
}

// ─── Sender info populated from User table ─────────────────────

// MessageWithSender 带发送者信息的消息（API 响应用）
type MessageWithSender struct {
	Message
	SenderUsername string `json:"sender_username"`
	SenderAvatar   string `json:"sender_avatar"`
}

// PopulateSender 从 User 表填充发送者信息
func (m *Message) PopulateSender(db *gorm.DB) MessageWithSender {
	var user User
	db.First(&user, m.SenderID)
	return MessageWithSender{
		Message:        *m,
		SenderUsername: user.Username,
		SenderAvatar:   user.Avatar,
	}
}

// PopulateSenderForMessages 批量填充发送者信息
func PopulateSenderForMessages(db *gorm.DB, msgs []Message) []MessageWithSender {
	if len(msgs) == 0 {
		return nil
	}

	userIDs := make(map[uint]bool)
	for _, m := range msgs {
		userIDs[m.SenderID] = true
	}
	ids := make([]uint, 0, len(userIDs))
	for id := range userIDs {
		ids = append(ids, id)
	}
	var users []User
	db.Where("id IN ?", ids).Find(&users)
	userMap := make(map[uint]User)
	for _, u := range users {
		userMap[u.ID] = u
	}

	result := make([]MessageWithSender, len(msgs))
	for i, m := range msgs {
		u := userMap[m.SenderID]
		result[i] = MessageWithSender{
			Message:        m,
			SenderUsername: u.Username,
			SenderAvatar:   u.Avatar,
		}
	}
	return result
}

// ─── Redis 缓存 ────────────────────────────────────────────────

const msgCacheTTL = 5 * time.Minute

func msgCacheKey(convID string) string {
	return fmt.Sprintf("chat:messages:%s", convID)
}

func cacheMessages(convID string, msgs []Message) {
	if RDB == nil {
		return
	}
	data, err := json.Marshal(msgs)
	if err != nil {
		return
	}
	ctx := context.Background()
	RDB.Set(ctx, msgCacheKey(convID), data, msgCacheTTL)
}

func getCachedMessages(convID string) ([]Message, bool) {
	if RDB == nil {
		return nil, false
	}
	ctx := context.Background()
	data, err := RDB.Get(ctx, msgCacheKey(convID)).Bytes()
	if err != nil {
		return nil, false
	}
	var msgs []Message
	if err := json.Unmarshal(data, &msgs); err != nil {
		return nil, false
	}
	return msgs, true
}

func invalidateMsgCache(convID string) {
	if RDB == nil {
		return
	}
	ctx := context.Background()
	RDB.Del(ctx, msgCacheKey(convID))
}

// ─── 数据库操作 ────────────────────────────────────────────────

// GetConversationMessages 获取会话消息（分页）
func GetConversationMessages(db *gorm.DB, convID string, limit, offset int) ([]Message, error) {
	if offset == 0 && limit >= 500 {
		if cached, ok := getCachedMessages(convID); ok {
			return cached, nil
		}
	}

	var msgs []Message
	err := db.Where("conversation_id = ?", convID).
		Order("created_at DESC").Limit(limit).Offset(offset).Find(&msgs).Error
	if err != nil {
		return nil, err
	}

	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}

	if offset == 0 && limit >= 500 && len(msgs) > 0 {
		cacheMessages(convID, msgs)
	}
	return msgs, nil
}

func GetConversationMessagesBefore(db *gorm.DB, convID string, beforeID uint, limit int) ([]Message, error) {
	query := db.Where("conversation_id = ?", convID)
	if beforeID > 0 {
		query = query.Where("id < ?", beforeID)
	}
	var msgs []Message
	if err := query.Order("id DESC").Limit(limit).Find(&msgs).Error; err != nil {
		return nil, err
	}
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
	return msgs, nil
}

// SaveMessage 保存消息
func SaveMessage(db *gorm.DB, msg *Message) error {
	err := db.Create(msg).Error
	if err == nil && msg.ConversationID != "" {
		invalidateMsgCache(msg.ConversationID)
		ensureConversationForMessage(db, *msg)
		if msg.ChatType == ConversationTypePrivate {
			db.Model(&ConversationMember{}).
				Where("conversation_id = ? AND user_id = ?", msg.ConversationID, msg.ReceiverID).
				UpdateColumn("unread_count", gorm.Expr("unread_count + 1"))
		} else if msg.ChatType == ConversationTypeGroup {
			db.Model(&ConversationMember{}).
				Where("conversation_id = ? AND user_id <> ?", msg.ConversationID, msg.SenderID).
				UpdateColumn("unread_count", gorm.Expr("unread_count + 1"))
		}
		db.Model(&Conversation{}).Where("id = ?", msg.ConversationID).Updates(map[string]any{
			"last_message_id":   msg.ID,
			"last_message_at":   msg.CreatedAt,
			"last_message_text": msg.Content,
			"last_message_type": msg.MessageType,
			"last_sender_id":    msg.SenderID,
			"updated_at":        msg.CreatedAt,
		})
		if msg.SenderID > 0 {
			TouchConversationList(msg.SenderID)
		}
	}
	return err
}
