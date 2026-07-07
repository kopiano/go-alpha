package models

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"gorm.io/gorm"
)

const (
	ConversationTypePrivate = "private"
	ConversationTypeGroup   = "group"
)

type Conversation struct {
	ID              string    `gorm:"type:varchar(64);primaryKey" json:"id"`
	Type            string    `gorm:"type:varchar(16);not null;index:idx_chat_conversation_type_lastmsg,priority:1;index:idx_chat_conversation_created_type,priority:2" json:"type"`
	Title           string    `gorm:"type:varchar(120)" json:"title"`
	Avatar          string    `gorm:"type:varchar(255)" json:"avatar"`
	LastMessageID   uint      `gorm:"index" json:"last_message_id"`
	LastMessageAt   time.Time `gorm:"index:idx_chat_conversation_type_lastmsg,priority:2" json:"last_message_at"`
	LastMessageText string    `gorm:"type:varchar(255)" json:"last_message_text"`
	LastMessageType int       `gorm:"default:1" json:"last_message_type"`
	LastSenderID    uint      `gorm:"index" json:"last_sender_id"`
	IsPinned        bool      `gorm:"default:false;index" json:"is_pinned"`
	IsMuted         bool      `gorm:"default:false" json:"is_muted"`
	CreatedBy       uint      `gorm:"index;not null" json:"created_by"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `gorm:"index:idx_chat_conversation_type_lastmsg,priority:3;index:idx_chat_conversation_created_type,priority:1" json:"updated_at"`
}

func (Conversation) TableName() string { return "chat_conversation" }

type ConversationMember struct {
	ID                uint           `gorm:"primaryKey" json:"id"`
	ConversationID    string         `gorm:"type:varchar(64);not null;index:idx_chat_conv_member,priority:1;uniqueIndex:uk_chat_conv_user,priority:1" json:"conversation_id"`
	UserID            uint           `gorm:"not null;index:idx_chat_conv_member,priority:2;uniqueIndex:uk_chat_conv_user,priority:2;index:idx_chat_conv_user_left,priority:1" json:"user_id"`
	LastReadMessageID uint           `gorm:"default:0;index:idx_chat_conv_user_left,priority:3" json:"last_read_message_id"`
	LastReadAt        time.Time      `gorm:"index" json:"last_read_at"`
	PinnedAt          *time.Time     `gorm:"index" json:"pinned_at"`
	MutedUntil        *time.Time     `json:"muted_until"`
	JoinedAt          time.Time      `json:"joined_at"`
	LeftAt            gorm.DeletedAt `gorm:"index:idx_chat_conv_member,priority:3;index:idx_chat_conv_user_left,priority:2" json:"left_at"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
}

func (ConversationMember) TableName() string { return "chat_conversation_member" }

type ConversationListItem struct {
	ConversationID  string                   `json:"conversation_id"`
	Type            string                   `json:"type"`
	Title           string                   `json:"title"`
	Avatar          string                   `json:"avatar"`
	LastMessage     string                   `json:"last_message"`
	LastMessageType int                      `json:"last_message_type"`
	LastMessageAt   time.Time                `json:"last_message_at"`
	LastSenderID    uint                     `json:"last_sender_id"`
	UnreadCount     int64                    `json:"unread_count"`
	IsPinned        bool                     `json:"is_pinned"`
	IsMuted         bool                     `json:"is_muted"`
	Members         []ConversationMemberUser `json:"members,omitempty"`
}

type ConversationMemberUser struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Avatar   string `json:"avatar"`
}

func conversationCacheKey(userID uint) string {
	return fmt.Sprintf("chat:conversations:%d", userID)
}

func invalidateConversationListCache(userIDs ...uint) {
	if RDB == nil {
		return
	}
	ctx := context.Background()
	for _, uid := range userIDs {
		RDB.Del(ctx, conversationCacheKey(uid))
	}
}

func GetUserConversationsV2(db *gorm.DB, userID uint) ([]ConversationListItem, error) {
	var memberships []ConversationMember
	if err := db.Where("user_id = ? AND left_at IS NULL", userID).Find(&memberships).Error; err != nil {
		return nil, err
	}
	if len(memberships) == 0 {
		return []ConversationListItem{}, nil
	}

	convIDs := make([]string, 0, len(memberships))
	memberMap := make(map[string]ConversationMember, len(memberships))
	for _, m := range memberships {
		convIDs = append(convIDs, m.ConversationID)
		memberMap[m.ConversationID] = m
	}

	var conversations []Conversation
	if err := db.Where("id IN ?", convIDs).Find(&conversations).Error; err != nil {
		return nil, err
	}
	convMap := make(map[string]Conversation, len(conversations))
	for _, c := range conversations {
		convMap[c.ID] = c
	}

	unreadMap := make(map[string]int64, len(convIDs))
	type unreadRow struct {
		ConversationID string
		UnreadCount    int64
	}
	var unreadRows []unreadRow
	if err := db.Model(&Message{}).
		Select("messages.conversation_id, COUNT(*) AS unread_count").
		Joins("JOIN chat_conversation_member cm ON cm.conversation_id = messages.conversation_id AND cm.user_id = ?", userID).
		Where("messages.conversation_id IN ? AND messages.sender_id <> ? AND messages.id > cm.last_read_message_id", convIDs, userID).
		Group("messages.conversation_id").
		Scan(&unreadRows).Error; err == nil {
		for _, row := range unreadRows {
			unreadMap[row.ConversationID] = row.UnreadCount
		}
	}

	privatePeerIDs := make(map[uint]struct{})
	groupConvIDs := make([]string, 0)
	for _, mem := range memberships {
		conv := convMap[mem.ConversationID]
		if conv.ID == "" {
			continue
		}
		if conv.Type == ConversationTypePrivate {
			if peerID := PrivateConvRecipient(userID, conv.ID); peerID > 0 {
				privatePeerIDs[peerID] = struct{}{}
			}
		} else if conv.Type == ConversationTypeGroup {
			groupConvIDs = append(groupConvIDs, conv.ID)
		}
	}

	userIDs := make([]uint, 0, len(privatePeerIDs))
	for uid := range privatePeerIDs {
		userIDs = append(userIDs, uid)
	}
	groupMemberRows := make([]ConversationMember, 0)
	groupUserIDs := make(map[uint]struct{})
	if len(groupConvIDs) > 0 {
		var groupMembers []ConversationMember
		if err := db.Where("conversation_id IN ? AND left_at IS NULL", groupConvIDs).Find(&groupMembers).Error; err == nil {
			groupMemberRows = groupMembers
			for _, m := range groupMembers {
				groupUserIDs[m.UserID] = struct{}{}
			}
		}
	}

	for uid := range groupUserIDs {
		userIDs = append(userIDs, uid)
	}

	userMap := make(map[uint]User, len(userIDs))
	if len(userIDs) > 0 {
		var users []User
		if err := db.Select("id, username, avatar").Where("id IN ?", userIDs).Find(&users).Error; err == nil {
			for _, u := range users {
				userMap[u.ID] = u
			}
		}
	}

	groupMembersByConv := make(map[string][]ConversationMemberUser, len(groupConvIDs))
	for _, m := range groupMemberRows {
		if u, ok := userMap[m.UserID]; ok {
			groupMembersByConv[m.ConversationID] = append(groupMembersByConv[m.ConversationID], ConversationMemberUser{
				UserID:   u.ID,
				Username: u.Username,
				Avatar:   u.Avatar,
			})
		}
	}

	result := make([]ConversationListItem, 0, len(memberships))
	for _, mem := range memberships {
		conv := convMap[mem.ConversationID]
		if conv.ID == "" {
			continue
		}
		item := ConversationListItem{
			ConversationID:  conv.ID,
			Type:            conv.Type,
			Title:           conv.Title,
			Avatar:          conv.Avatar,
			LastMessage:     conv.LastMessageText,
			LastMessageType: conv.LastMessageType,
			LastMessageAt:   conv.LastMessageAt,
			LastSenderID:    conv.LastSenderID,
			IsPinned:        conv.IsPinned || mem.PinnedAt != nil,
			IsMuted:         conv.IsMuted || mem.MutedUntil != nil,
			UnreadCount:     0,
		}
		if conv.Type == ConversationTypePrivate {
			peerID := PrivateConvRecipient(userID, conv.ID)
			if peerID > 0 {
				if u, ok := userMap[peerID]; ok {
					item.Members = []ConversationMemberUser{{UserID: u.ID, Username: u.Username, Avatar: u.Avatar}}
					if item.Title == "" {
						item.Title = u.Username
					}
					if item.Avatar == "" {
						item.Avatar = u.Avatar
					}
				}
			}
		} else {
			item.Members = groupMembersByConv[conv.ID]
		}
		if mem, ok := memberMap[conv.ID]; ok {
			item.UnreadCount = unreadMap[conv.ID]
			if item.UnreadCount == 0 && mem.LastReadMessageID == 0 {
				item.UnreadCount = unreadMap[conv.ID]
			}
		}
		result = append(result, item)
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].LastMessageAt.IsZero() && result[j].LastMessageAt.IsZero() {
			return false
		}
		if result[i].LastMessageAt.IsZero() {
			return false
		}
		if result[j].LastMessageAt.IsZero() {
			return true
		}
		return result[i].LastMessageAt.After(result[j].LastMessageAt)
	})
	return result, nil
}

func CacheConversationList(userID uint, data []ConversationListItem) {
	if RDB == nil {
		return
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return
	}
	RDB.Set(context.Background(), conversationCacheKey(userID), payload, 60*time.Second)
}

func EnsurePrivateConversation(db *gorm.DB, userA, userB uint) (string, error) {
	convID, err := FindOrCreatePrivateConv(db, userA, userB)
	if err != nil {
		return "", err
	}
	if convID == "" {
		return "", gorm.ErrRecordNotFound
	}
	var conv Conversation
	if err := db.First(&conv, "id = ?", convID).Error; err != nil {
		conv = Conversation{
			ID:        convID,
			Type:      ConversationTypePrivate,
			CreatedBy: userA,
		}
		if userA > 0 {
			conv.CreatedBy = userA
		}
		if err := db.Create(&conv).Error; err != nil {
			return "", err
		}
	}
	joinConversationMember(db, convID, userA, time.Now())
	joinConversationMember(db, convID, userB, time.Now())
	return convID, nil
}

func EnsureGroupConversation(db *gorm.DB, groupID uint, memberIDs []uint) string {
	convID := fmt.Sprintf("g_%d", groupID)
	now := time.Now()
	var conv Conversation
	if err := db.First(&conv, "id = ?", convID).Error; err != nil {
		conv = Conversation{ID: convID, Type: ConversationTypeGroup, Title: fmt.Sprintf("Group %d", groupID), CreatedBy: 0, CreatedAt: now, UpdatedAt: now}
		db.Create(&conv)
	}
	for _, uid := range memberIDs {
		joinConversationMember(db, convID, uid, now)
	}
	return convID
}

func TouchConversationList(userIDs ...uint) {
	invalidateConversationListCache(userIDs...)
}

func GetCachedConversationList(userID uint) ([]ConversationListItem, bool) {
	if RDB == nil {
		return nil, false
	}
	data, err := RDB.Get(context.Background(), conversationCacheKey(userID)).Bytes()
	if err != nil {
		return nil, false
	}
	var items []ConversationListItem
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, false
	}
	return items, true
}

func GetConversationMemberIDs(db *gorm.DB, convID string) []uint {
	var memberIDs []uint
	if convID == "" {
		return memberIDs
	}
	db.Model(&ConversationMember{}).
		Where("conversation_id = ? AND left_at IS NULL", convID).
		Pluck("user_id", &memberIDs)
	return memberIDs
}
