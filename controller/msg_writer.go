package controller

import (
	"encoding/json"
	"log/slog"
	"time"

	"go-alpha/models"
)

// ─── 异步批量消息写入器 ───

// QueuedMsg 待写入的消息
type QueuedMsg struct {
	Msg  *models.Message
	Done chan<- error // 通知调用方写入完成（可选，nil 表示不等待）
}

var (
	msgQueue    = make(chan QueuedMsg, 1024)
	writeTicker = time.NewTicker(100 * time.Millisecond)
)

func init() {
	go msgWriter()
}

// msgWriter 从 channel 读取消息，每 100ms 或满 100 条批量写入 MySQL
func msgWriter() {
	batch := make([]*models.Message, 0, 100)
	doneChans := make([]chan<- error, 0, 100)

	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := models.BatchCreateMessages(models.DB, batch); err != nil {
			slog.Error("Batch insert messages failed", "count", len(batch), "error", err)
			for _, ch := range doneChans {
				ch <- err
			}
		} else {
			for _, ch := range doneChans {
				ch <- nil
			}
		}
		batch = batch[:0]
		doneChans = doneChans[:0]
	}

	for {
		select {
		case q := <-msgQueue:
			batch = append(batch, q.Msg)
			if q.Done != nil {
				doneChans = append(doneChans, q.Done)
			}
			if len(batch) >= 100 {
				flush()
			}
		case <-writeTicker.C:
			flush()
		}
	}
}

// EnqueueMessage 将消息加入写入队列（非阻塞，队列满时丢弃）
func EnqueueMessage(msg *models.Message) {
	select {
	case msgQueue <- QueuedMsg{Msg: msg}:
	default:
		slog.Warn("Message queue full, dropping message", "convID", msg.ConversationID)
	}
}

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
		if err := models.RDB.Publish(msgCtx, redisPubChan, string(data)).Err(); err != nil {
			slog.Error("Redis publish failed", "error", err)
		}
	}
}

var msgCtx = msgCtxType{}

type msgCtxType struct{}

// SubscribeMessages 订阅 Redis 频道并将消息推送到本地 Hub
func SubscribeMessages() {
	if models.RDB == nil {
		slog.Warn("Redis not available, Pub/Sub disabled")
		return
	}
	sub := models.RDB.Subscribe(msgCtx, redisPubChan)
	ch := sub.Channel()

	slog.Info("Redis Pub/Sub subscribed", "channel", redisPubChan)

	go func() {
		for msg := range ch {
			// 解析消息并推送到本地 Hub 客户端
			var payload map[string]interface{}
			if err := json.Unmarshal([]byte(msg.Payload), &payload); err != nil {
				continue
			}
			ChatHub.broadcastRaw(payload)
		}
	}()
}
