package models

import (
	"log/slog"
	"time"

	"gorm.io/gorm"
)

const TeamConvName = "Team"

// EnsureTeamConversation 确保团队群聊存在，并将所有用户加入
func EnsureTeamConversation(db *gorm.DB) {
	var conv Conversation
	err := db.Where("type = ? AND name = ?", "group", TeamConvName).First(&conv).Error

	if err != nil {
		// 创建群聊
		conv = Conversation{Type: "group", Name: TeamConvName}
		if err := db.Create(&conv).Error; err != nil {
			slog.Error("Failed to create team conversation", "error", err)
			return
		}
		slog.Info("Team conversation created", "id", conv.ID)
	}

	// 将所有用户加为成员
	var users []User
	db.Select("id").Find(&users)

	added := 0
	for _, u := range users {
		var count int64
		db.Model(&ConversationMember{}).
			Where("conversation_id = ? AND user_id = ?", conv.ID, u.ID).
			Count(&count)
		if count == 0 {
			db.Create(&ConversationMember{
				ConversationID: conv.ID,
				UserID:         u.ID,
				JoinedAt:       time.Now(),
				LastReadAt:     time.Now(),
			})
			added++
		}
	}

	if added > 0 {
		// 更新群聊时间以触发排序
		db.Model(&conv).Update("updated_at", time.Now())
		slog.Info("New members joined team", "count", added, "total", len(users))
	}
}

// GetTeamConversation 获取团队群聊
func AddUserToTeam(db *gorm.DB, userID uint) {
	var conv Conversation
	if err := db.Where("type = ? AND name = ?", "group", TeamConvName).First(&conv).Error; err != nil {
		return
	}
	var count int64
	db.Model(&ConversationMember{}).
		Where("conversation_id = ? AND user_id = ?", conv.ID, userID).
		Count(&count)
	if count == 0 {
		db.Create(&ConversationMember{
			ConversationID: conv.ID,
			UserID:         userID,
			JoinedAt:       time.Now(),
			LastReadAt:     time.Now(),
		})
	}
}

func GetTeamConversation(db *gorm.DB) *Conversation {
	var conv Conversation
	err := db.Where("type = ? AND name = ?", "group", TeamConvName).Preload("Members").First(&conv).Error
	if err != nil {
		return nil
	}
	return &conv
}
