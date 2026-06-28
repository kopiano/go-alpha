package main

import (
	"log"

	"go-alpha/models"
	"go-alpha/routes"
)

func main() {
	// Initialize databases.
	db := models.SetupMySQL()
	defer models.CloseMysqlDB(db)
	models.SetupRedis()

	// Setup router and start
	r := routes.SetupRouter()
	log.Fatal(r.Run(":8000"))
}
