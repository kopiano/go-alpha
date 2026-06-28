package routes

import (
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"

	"go-alpha/controller"
	"go-alpha/middleware"
)

func SetupRouter() *gin.Engine {
	r := gin.Default()

	// Cookie, Session
	store := cookie.NewStore([]byte("something-very-secret"))
	r.Use(sessions.Sessions("my-session", store))

	// CORS
	r.Use(middleware.CORS())

	userCtrl := controller.User{}

	// Health check
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "pong"})
	})

	// User CRUD
	userGroup := r.Group("/api/v1")
	{
		userGroup.GET("/user", userCtrl.GetAllUsers)
		userGroup.GET("/user/:id", userCtrl.GetUserById)
		userGroup.GET("/user/name/:name", userCtrl.GetUserByName)
		userGroup.POST("/user", userCtrl.AddUser)
		userGroup.PUT("/user/:id", userCtrl.UpdateUser)
		userGroup.DELETE("/user/:id", userCtrl.DeleteUser)
	}

	return r
}
