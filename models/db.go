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
	DB.AutoMigrate(&User{}, &Task{}, &VisitorSummary{}, &Visitor{}, &Comment{}, &Faq{}, &Conversation{}, &ConversationMember{}, &Message{}, &Transaction{}, &Weather{})

	// 删除 messages 表中已从模型移除的冗余列（GORM AutoMigrate 不会自动删列）
	if DB.Migrator().HasColumn(&Message{}, "sender_username") {
		DB.Migrator().DropColumn(&Message{}, "sender_username")
	}
	if DB.Migrator().HasColumn(&Message{}, "sender_avatar") {
		DB.Migrator().DropColumn(&Message{}, "sender_avatar")
	}

	// Fix historical daily UV data (recalculate from visitor table)
	err = VisitorSummary{}.FixDailyUV()
	if err != nil {
		slog.Warn("FixDailyUV failed", "error", err)
	}

	// Seed FAQ data
	SeedFaqs()

	// Sync FAQ to JSON file
	if err := SyncFaqToFile(); err != nil {
		slog.Error("Failed to sync FAQ to file", "error", err)
	}

	return DB
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
