package models

import (
	"time"

	"gorm.io/gorm"
)

// ─── 会话查询 ──────────────────────────────────────────────

// FindOrCreatePrivateConv 查找或创建私聊会话
// 无需读写任何额外表：直接计算 deterministic conversation_id
func FindOrCreatePrivateConv(db *gorm.DB, userA, userB uint) (convID string, err error) {
	if userA == userB {
		return "", gorm.ErrRecordNotFound
	}
	return PrivateConvID(userA, userB), nil
}

// LegacyConversation 保留旧实现兼容，新的会话模型使用 chat_conversation.go
type LegacyConversation struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	UpdatedAt time.Time `json:"updated_at"`
}

func GetUserConversations(db *gorm.DB, userID uint) ([]LegacyConversation, error) {
	type convRow struct {
		ConversationID string
		ChatType       string
		LastMsgTime    time.Time
	}
	var rows []convRow

	err := db.Model(&Message{}).
		Select("conversation_id, chat_type, MAX(created_at) as last_msg_time").
		Where("(sender_id = ? OR receiver_id = ?)", userID, userID).
		Group("conversation_id, chat_type").
		Order("last_msg_time DESC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	result := make([]LegacyConversation, 0, len(rows))
	for _, r := range rows {
		result = append(result, LegacyConversation{
			ID:        r.ConversationID,
			Type:      r.ChatType,
			UpdatedAt: r.LastMsgTime,
		})
	}
	return result, nil
}

// GetConversationPartner 获取私聊会话中的对方用户 ID
func GetConversationPartner(db *gorm.DB, convID string, currentUserID uint) uint {
	return PrivateConvRecipient(currentUserID, convID)
}

// GetUnreadCount 获取未读消息数
func GetUnreadCount(db *gorm.DB, convID string, userID uint) (int64, error) {
	var count int64
	err := db.Model(&Message{}).
		Where("conversation_id = ? AND sender_id != ? AND receiver_id = ? AND status = 0",
			convID, userID, userID).
		Count(&count).Error
	return count, err
}
