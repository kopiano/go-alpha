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
	}

	wsMsg := map[string]interface{}{
		"type":               "message",
		"event":              "message.new",
		"chat_type":          msg.ChatType,
		"sender_id":          msg.SenderID,
		"sender_username":    senderUsername,
		"sender_avatar":      senderAvatar,
		"username":           senderUsername,
		"message_type":       msg.MessageType,
		"msg_type":           msgTypeStr,
		"content":            msg.Content,
		"conversation_id":    msg.ConversationID,
		"status": 1,
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
