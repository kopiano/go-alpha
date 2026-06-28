package models

import (
	"context"
	"fmt"
	"log"
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
		log.Fatalf("Failed to connect MySQL: %v", err)
	}
	fmt.Println("MySQL connected successfully")
	DB = db
	DB.AutoMigrate(&User{})
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
		log.Fatalf("Failed to connect Redis: %v", err)
	}

	fmt.Println("Redis connected successfully")
}

func CloseMysqlDB(db *gorm.DB) {
	sqlDB, err := DB.DB()
	if err != nil {
		log.Fatal("Failed to close connection from database")
	}
	sqlDB.SetMaxIdleConns(10)               // 最大空闲连接数
	sqlDB.SetMaxOpenConns(100)              // 最多可容纳
	sqlDB.SetConnMaxLifetime(time.Hour * 4) // 连接最大复用时间
	sqlDB.Close()
}
