package main

import (
	"log/slog"

	_ "go-alpha/logger"
	"go-alpha/models"
	"go-alpha/routes"
)

func main() {
	slog.Info("Starting server")

	db := models.SetupMySQL()
	defer models.CloseMysqlDB(db)
	models.SetupRedis()

	r := routes.SetupRouter()
	slog.Info("Server listening on :8080")
	if err := r.Run(":8080"); err != nil {
		slog.Error("Server failed", "error", err)
	}
}
