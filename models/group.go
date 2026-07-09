package models

import (
	"context"
	"log/slog"
	"time"

	"gorm.io/gorm"
)

// Group 群组
type Group struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"type:varchar(100);not null" json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

func (Group) TableName() string { return "chat_group" }

type GroupMember struct {
	ID       uint      `gorm:"primaryKey" json:"id"`
	GroupID  uint      `gorm:"index;not null" json:"group_id"`
	UserID   uint      `gorm:"index;not null" json:"user_id"`
	JoinedAt time.Time `json:"joined_at"`
}

func (GroupMember) TableName() string { return "chat_group_member" }

func invalidateTeamCache() {
	if RDB == nil {
		return
	}
	_ = RDB.Del(context.Background(), "chat:group_info").Err()
}

// EnsureTeamGroup 确保 Group 群组存在，并将所有用户加为成员
func EnsureTeamGroup(db *gorm.DB) {
	var group Group
	err := db.Where("name = ?", "Group").First(&group).Error
	if err != nil {
		group = Group{Name: "Group"}
		db.Create(&group)
		slog.Info("Group created", "id", group.ID)
		invalidateTeamCache()
	}

	var users []User
	db.Select("id").Find(&users)
	added := 0
	for _, u := range users {
		var count int64
		db.Model(&GroupMember{}).Where("group_id = ? AND user_id = ?", group.ID, u.ID).Count(&count)
		if count == 0 {
			db.Create(&GroupMember{GroupID: group.ID, UserID: u.ID, JoinedAt: time.Now()})
			added++
		}
	}
	if added > 0 {
		slog.Info("New members joined Group", "count", added, "total", len(users))
		invalidateTeamCache()
	}
}

// AddUserToTeam 将用户加入 Group 群组
func AddUserToTeam(db *gorm.DB, userID uint) {
	var group Group
	if err := db.Where("name = ?", "Group").First(&group).Error; err != nil {
		return
	}
	var count int64
	db.Model(&GroupMember{}).Where("group_id = ? AND user_id = ?", group.ID, userID).Count(&count)
	if count == 0 {
		db.Create(&GroupMember{GroupID: group.ID, UserID: userID, JoinedAt: time.Now()})
		invalidateTeamCache()
	}
}

// GetTeamGroup 获取 Group 群组信息（含成员）
func GetTeamGroup(db *gorm.DB) *Group {
	var group Group
	if err := db.Where("name = ?", "Group").First(&group).Error; err != nil {
		return nil
	}
	return &group
}

// GetGroupMembers 获取群组成员 ID 列表
func GetGroupMembers(db *gorm.DB, groupID uint) []uint {
	var members []GroupMember
	db.Where("group_id = ?", groupID).Find(&members)
	ids := make([]uint, len(members))
	for i, m := range members {
		ids[i] = m.UserID
	}
	return ids
}
