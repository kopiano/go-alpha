package controller

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"go-alpha/models"
)

// ─── Redis Pub/Sub 广播 ───

const redisPubChan = "chat:messages"

// PublishMessage 将消息广播到 Redis Pub/Sub 频道
func PublishMessage(msg models.Message, senderUsername, senderAvatar string) {
	msgTypeStr := "text"
	switch msg.MessageType {
	case models.MsgEmoji:
		msgTypeStr = "emoji"
	case models.MsgImage:
		msgTypeStr = "image"
	case models.MsgFile:
		msgTypeStr = "file"
	case models.MsgSystem:
		msgTypeStr = "system"
	case models.MsgReply:
		msgTypeStr = "reply"
	}

	var replyToUsername, replyToContent string
	if msg.ReplyToMessageID != nil && *msg.ReplyToMessageID > 0 {
		var repliedMsg models.Message
		if err := models.DB.First(&repliedMsg, *msg.ReplyToMessageID).Error; err == nil {
			replyToContent = repliedMsg.Content
			var repliedUser models.User
			if err := models.DB.First(&repliedUser, repliedMsg.SenderID).Error; err == nil {
				replyToUsername = repliedUser.Username
			}
		}
	}

	wsMsg := map[string]interface{}{
		"type":               "message",
		"sender_id":          msg.SenderID,
		"sender_username":    senderUsername,
		"sender_avatar":      senderAvatar,
		"username":           senderUsername,
		"message_type":       msg.MessageType,
		"msg_type":           msgTypeStr,
		"content":            msg.Content,
		"conversation_id":    msg.ConversationID,
		"reply_to_message_id": msg.ReplyToMessageID,
		"reply_to_username":   replyToUsername,
		"reply_to_content":    replyToContent,
		"status":             msg.Status,
		"created_at":         msg.CreatedAt.Format(time.RFC3339),
		"time":               msg.CreatedAt.Format("15:04"),
	}

	data, err := json.Marshal(wsMsg)
	if err != nil {
		slog.Error("Failed to marshal pub/sub message", "error", err)
		return
	}

	if models.RDB != nil {
		if err := models.RDB.Publish(context.Background(), redisPubChan, string(data)).Err(); err != nil {
			slog.Error("Redis publish failed", "error", err)
		}
	}
}

// SubscribeMessages 订阅 Redis 频道并将消息推送到本地 Hub
func SubscribeMessages() {
	if models.RDB == nil {
		slog.Warn("Redis not available, Pub/Sub disabled")
		return
	}
	sub := models.RDB.Subscribe(context.Background(), redisPubChan)
	ch := sub.Channel()

	slog.Info("Redis Pub/Sub subscribed", "channel", redisPubChan)

	go func() {
		for msg := range ch {
			ChatHub.broadcast <- []byte(msg.Payload)
		}
	}()
}
