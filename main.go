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

	// 确保 Team 群组存在
	models.EnsureTeamGroup(db)

	// 启动定时抓取热搜（每 6 小时）
	scheduler.StartHotSearchScheduler()

	r := routes.SetupRouter()
	slog.Info("Server listening on :8000")
	if err := r.Run(":8000"); err != nil {
		slog.Error("Server failed", "error", err)
	}
}
