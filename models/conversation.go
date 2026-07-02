package models

import (
	"time"

	"gorm.io/gorm"
)

// Conversation 会话（1v1 或群聊）
type Conversation struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Type      string         `gorm:"type:varchar(20);default:private;not null" json:"type"` // "private" | "group"
	Name      string         `gorm:"type:varchar(255)" json:"name,omitempty"`               // 群聊名称
	Avatar    string         `gorm:"type:varchar(500)" json:"avatar,omitempty"`              // 群头像
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	Members   []ConversationMember `gorm:"foreignKey:ConversationID" json:"members,omitempty"`
}

// ConversationMember 会话成员
type ConversationMember struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	ConversationID uint      `gorm:"index;not null" json:"conversation_id"`
	UserID         uint      `gorm:"index;not null" json:"user_id"`
	LastReadAt     time.Time `json:"last_read_at"` // 最后已读时间
	JoinedAt       time.Time `json:"joined_at"`
}

// FindOrCreatePrivateConv 查找或创建 1v1 私聊会话
func FindOrCreatePrivateConv(db *gorm.DB, userA, userB uint) (*Conversation, error) {
	// 查找两人之间已有的私聊
	var conv Conversation
	err := db.Where("type = ?", "private").
		Joins("JOIN conversation_member cm1 ON cm1.conversation_id = conversations.id AND cm1.user_id = ?", userA).
		Joins("JOIN conversation_member cm2 ON cm2.conversation_id = conversations.id AND cm2.user_id = ?", userB).
		First(&conv).Error
	if err == nil {
		return &conv, nil
	}

	// 不存在则创建
	conv = Conversation{Type: "private"}
	db.Create(&conv)
	db.Create(&ConversationMember{ConversationID: conv.ID, UserID: userA, JoinedAt: time.Now(), LastReadAt: time.Now()})
	db.Create(&ConversationMember{ConversationID: conv.ID, UserID: userB, JoinedAt: time.Now(), LastReadAt: time.Now()})
	return &conv, nil
}

// GetUserConversations 获取用户的所有会话
func GetUserConversations(db *gorm.DB, userID uint) ([]Conversation, error) {
	var conversations []Conversation
	err := db.Where("id IN (?)",
		db.Table("conversation_member").Select("conversation_id").Where("user_id = ?", userID),
	).Preload("Members").Order("updated_at DESC").Find(&conversations).Error
	return conversations, err
}
