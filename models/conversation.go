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
	// 通过子查询查找两人之间已有的私聊：先找 conversation_member 中同时包含 userA 和 userB 的 conversation_id
	var conv Conversation
	subQuery := db.Table("conversation_member").
		Select("conversation_id").
		Where("user_id IN ?", []uint{userA, userB}).
		Group("conversation_id").
		Having("COUNT(DISTINCT user_id) = 2")

	err := db.Where("id IN (?)", subQuery).Where("type = ?", "private").First(&conv).Error
	if err == nil {
		return &conv, nil
	}

	// 不存在则创建（加锁防止并发重复创建）
	conv = Conversation{Type: "private"}
	if err := db.Create(&conv).Error; err != nil {
		return nil, err
	}
	if err := db.Create(&ConversationMember{ConversationID: conv.ID, UserID: userA, JoinedAt: time.Now(), LastReadAt: time.Now()}).Error; err != nil {
		return nil, err
	}
	if err := db.Create(&ConversationMember{ConversationID: conv.ID, UserID: userB, JoinedAt: time.Now(), LastReadAt: time.Now()}).Error; err != nil {
		return nil, err
	}
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
