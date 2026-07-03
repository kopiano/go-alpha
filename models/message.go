package models

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
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

// MsgStatus 消息状态
const (
	StatusActive   = 1 // 正常
	StatusRecalled = 2 // 已撤回
	StatusDeleted  = 3 // 已删除
)

// Message 聊天消息
type Message struct {
	ID             uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	ConversationID string `gorm:"type:varchar(64);not null;index" json:"conversation_id"`

	// sender / receiver
	ChatType   string `gorm:"type:varchar(20);not null;index" json:"chat_type"` // "private" | "group"
	SenderID   uint   `gorm:"index;not null" json:"sender_id"`
	ReceiverID uint   `gorm:"index" json:"receiver_id"` // 私聊
	GroupID    uint   `gorm:"index" json:"group_id"`    // 群聊

	// message
	Content     string `gorm:"type:text;not null" json:"content"` // 内容
	MessageType int    `gorm:"default:1" json:"message_type"`     // 消息类型：1-4

	// status
	Status int `gorm:"default:0" json:"status"` // 消息状态：0未读，1为已读

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ─── 私聊会话辅助函数 ──────────────────────────────────────────

// PrivateConvID 计算私聊会话的确定性 conversation_id
func PrivateConvID(userA, userB uint) uint {
	if userA < userB {
		return (userA << 20) | userB
	}
	return (userB << 20) | userA
}

// PrivateConvRecipient 返回私聊中对方用户的 ID
func PrivateConvRecipient(senderID uint, convID string) uint {
	id, _ := strconv.ParseUint(convID, 10, 64)
	cid := uint(id)
	lo := cid & ((1 << 20) - 1)
	hi := cid >> 20
	if senderID == lo {
		return hi
	}
	return lo
}

// PrivateConvUserA 返回私聊中 ID 较小的用户（约定为 UserA）
func PrivateConvUserA(convID string) uint {
	id, _ := strconv.ParseUint(convID, 10, 64)
	return uint(id) >> 20
}

// PrivateConvUserB 返回私聊中 ID 较大的用户（约定为 UserB）
func PrivateConvUserB(convID string) uint {
	id, _ := strconv.ParseUint(convID, 10, 64)
	return uint(id) & ((1 << 20) - 1)
}

// ─── 已读状态（使用 Message.Status 字段）────────────────────────

// MarkConversationRead 将用户在该会话的未读消息标记为已读
func MarkConversationRead(db *gorm.DB, convID string, userID uint) {
	db.Model(&Message{}).
		Where("conversation_id = ? AND receiver_id = ? AND status = 0", convID, userID).
		Update("status", 1)
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

// SaveMessage 保存消息
func SaveMessage(db *gorm.DB, msg *Message) error {
	err := db.Create(msg).Error
	if err == nil && msg.ConversationID != "" {
		invalidateMsgCache(msg.ConversationID)
	}
	return err
}
