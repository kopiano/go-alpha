package main

import (
	"log/slog"

	_ "go-alpha/logger"
	"go-alpha/models"
	"go-alpha/routes"
	"go-alpha/scheduler"
)

func main() {
	slog.Info("Starting server")

	db := models.SetupMySQL()
	defer models.CloseMysqlDB(db)
	models.SetupRedis()

	// 启动定时抓取热搜（每 6 小时）
	scheduler.StartHotSearchScheduler()

	r := routes.SetupRouter()
	slog.Info("Server listening on :8080")
	if err := r.Run(":8080"); err != nil {
		slog.Error("Server failed", "error", err)
	}
}
