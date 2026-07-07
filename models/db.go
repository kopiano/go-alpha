package models

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"

	"go-alpha/config"
)

var (
	DB  *gorm.DB
	RDB *redis.Client
)

func SetupMySQL() *gorm.DB {
	dsn := config.GetYamlDsn()
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
		// 设置全局表名禁用复数
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
		},
	})
	if err != nil {
		slog.Error("Failed to connect MySQL", "error", err)
		os.Exit(1)
	}
	slog.Info("MySQL connected successfully")
	DB = db
	DB.AutoMigrate(&User{}, &Task{}, &VisitorSummary{}, &Visitor{}, &Comment{}, &CommentLike{}, &Faq{}, &Message{}, &Transaction{}, &Weather{}, &Group{}, &GroupMember{}, &Md{}, &Conversation{}, &ConversationMember{})
	if DB.Migrator().HasColumn(&Md{}, "visibility") {
		DB.Exec("ALTER TABLE `md` MODIFY COLUMN `visibility` TINYINT(1) NOT NULL")
	}
	if DB.Migrator().HasColumn(&Md{}, "edit_permission") {
		DB.Exec("ALTER TABLE `md` MODIFY COLUMN `edit_permission` TINYINT(1) NOT NULL")
	}
	if DB.Migrator().HasColumn(&Md{}, "contributors") {
		DB.Exec("UPDATE `md` SET `contributors` = JSON_ARRAY(`user_id`) WHERE `contributors` IS NULL OR JSON_LENGTH(`contributors`) = 0")
	}

	migrateChatData(DB)

	// Fix historical daily UV data (recalculate from visitor table)
	err = VisitorSummary{}.FixDailyUV()
	if err != nil {
		slog.Warn("FixDailyUV failed", "error", err)
	}

	return DB
}

func migrateChatData(db *gorm.DB) {
	if db.Migrator().HasTable("conversation_read") {
		db.Migrator().DropTable("conversation_read")
	}

	// backfill conversations and members from existing messages
	var convs []struct {
		ConversationID string
		ChatType       string
		LastMessageAt  time.Time
		LastMessageID  uint
		LastSenderID   uint
		LastMsgCount   int64
	}
	db.Model(&Message{}).
		Select("conversation_id, chat_type, MAX(created_at) as last_message_at, MAX(id) as last_message_id, MAX(sender_id) as last_sender_id, COUNT(*) as last_msg_count").
		Group("conversation_id, chat_type").
		Scan(&convs)
	for _, row := range convs {
		if row.ConversationID == "" {
			continue
		}
		var last Message
		db.Where("conversation_id = ?", row.ConversationID).Order("created_at DESC, id DESC").First(&last)
		var conv Conversation
		if err := db.First(&conv, "id = ?", row.ConversationID).Error; err != nil {
			conv = Conversation{
				ID:              row.ConversationID,
				Type:            row.ChatType,
				LastMessageID:   last.ID,
				LastMessageAt:   last.CreatedAt,
				LastMessageText: last.Content,
				LastMessageType: last.MessageType,
				LastSenderID:    last.SenderID,
				UpdatedAt:       last.CreatedAt,
			}
			db.Create(&conv)
		}
	}
	var msgs []Message
	db.Find(&msgs)
	for _, msg := range msgs {
		ensureConversationForMessage(db, msg)
	}
}

func ensureConversationForMessage(db *gorm.DB, msg Message) {
	if msg.ConversationID == "" {
		return
	}
	convType := ConversationTypePrivate
	if msg.ChatType == "group" {
		convType = ConversationTypeGroup
	}
	if err := db.First(&Conversation{}, "id = ?", msg.ConversationID).Error; err != nil {
		db.Create(&Conversation{
			ID:              msg.ConversationID,
			Type:            convType,
			LastMessageID:   msg.ID,
			LastMessageAt:   msg.CreatedAt,
			LastMessageText: msg.Content,
			LastMessageType: msg.MessageType,
			LastSenderID:    msg.SenderID,
			CreatedBy:       msg.SenderID,
			UpdatedAt:       msg.CreatedAt,
		})
	}
	joinConversationMember(db, msg.ConversationID, msg.SenderID, msg.CreatedAt)
	if msg.ReceiverID > 0 {
		joinConversationMember(db, msg.ConversationID, msg.ReceiverID, msg.CreatedAt)
	}
	if msg.GroupID > 0 {
		var memberIDs []uint
		memberIDs = GetGroupMembers(db, msg.GroupID)
		for _, uid := range memberIDs {
			joinConversationMember(db, msg.ConversationID, uid, msg.CreatedAt)
		}
	}
}

func joinConversationMember(db *gorm.DB, convID string, userID uint, joinedAt time.Time) {
	if convID == "" || userID == 0 {
		return
	}
	var existing ConversationMember
	if err := db.Where("conversation_id = ? AND user_id = ? AND left_at IS NULL", convID, userID).First(&existing).Error; err == nil {
		return
	}
	member := ConversationMember{ConversationID: convID, UserID: userID, JoinedAt: joinedAt}
	db.Where("conversation_id = ? AND user_id = ?", convID, userID).Attrs(member).FirstOrCreate(&member)
}

func SetupRedis() {
	RDB = redis.NewClient(&redis.Options{
		Addr:         config.Conf.Redis.Addr(),
		Password:     config.Conf.Redis.Password,
		DB:           config.Conf.Redis.DB,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := RDB.Ping(ctx).Err(); err != nil {
		slog.Error("Failed to connect Redis", "error", err)
		os.Exit(1)
	}

	slog.Info("Redis connected successfully")
}

func CloseMysqlDB(db *gorm.DB) {
	sqlDB, err := DB.DB()
	if err != nil {
		slog.Error("Failed to close connection from database", "error", err)
	}
	sqlDB.SetMaxIdleConns(10)               // 最大空闲连接数
	sqlDB.SetMaxOpenConns(100)              // 最多可容纳
	sqlDB.SetConnMaxLifetime(time.Hour * 4) // 连接最大复用时间
	sqlDB.Close()
}
