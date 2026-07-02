package models

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// MessageType 消息类型
const (
	MsgText   = 1 // 文本
	MsgEmoji  = 2 // Emoji / 表情包
	MsgImage  = 3 // 图片
	MsgFile   = 4 // 文件附件
	MsgSystem = 7 // 系统通知（加入/退出群聊等）
	MsgReply  = 8 // 回复消息
)

// MsgStatus 消息状态
const (
	StatusActive   = 1 // 正常
	StatusRecalled = 2 // 已撤回
	StatusDeleted  = 3 // 已删除
)

// Message 聊天消息
type Message struct {
	ID               uint           `gorm:"primaryKey" json:"id"`
	ConversationID   uint           `gorm:"index;not null" json:"conversation_id"`
	SenderID         uint           `gorm:"index;not null" json:"sender_id"`
	MessageType      int            `gorm:"default:1;not null" json:"message_type"` // 1-8
	Content          string         `gorm:"type:text" json:"content"`
	ReplyToMessageID *uint          `gorm:"default:null" json:"reply_to_message_id,omitempty"`
	Status           int            `gorm:"default:1" json:"status"` // 1=active 2=recalled 3=deleted
	Metadata         datatypes.JSON `gorm:"type:json;default:null" json:"metadata,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`
}

// ─── Sender info populated from User table (not stored in messages) ───

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
	userIDs := make(map[uint]bool)
	for _, m := range msgs {
		userIDs[m.SenderID] = true
	}
	var users []User
	ids := make([]uint, 0, len(userIDs))
	for id := range userIDs {
		ids = append(ids, id)
	}
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

// ─── 数据库操作 ───

// GetConversationMessages 获取会话消息（分页）
func GetConversationMessages(db *gorm.DB, convID uint, limit, offset int) ([]Message, error) {
	var msgs []Message
	err := db.Where("conversation_id = ? AND status != ?", convID, StatusDeleted).
		Order("created_at DESC").Limit(limit).Offset(offset).Find(&msgs).Error
	if err != nil {
		return nil, err
	}
	// 反转为正序
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
	return msgs, nil
}

// SaveMessage 保存消息
func SaveMessage(db *gorm.DB, msg *Message) error {
	return db.Create(msg).Error
}

// RecallMessage 撤回消息
func RecallMessage(db *gorm.DB, msgID, senderID uint) error {
	return db.Model(&Message{}).Where("id = ? AND sender_id = ?", msgID, senderID).
		Update("status", StatusRecalled).Error
}

// GetUnreadCount 获取未读消息数
func GetUnreadCount(db *gorm.DB, convID, userID uint, lastReadAt time.Time) (int64, error) {
	var count int64
	err := db.Model(&Message{}).
		Where("conversation_id = ? AND sender_id != ? AND created_at > ? AND status = ?",
			convID, userID, lastReadAt, StatusActive).
		Count(&count).Error
	return count, err
}
