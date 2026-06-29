package routes

import (
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"

	"go-alpha/controller"
	"go-alpha/middleware"
)

var (
	authController controller.AuthController = controller.NewAuthController()
	docCtrl        controller.DocController  = controller.DocController{}
)

func SetupRouter() *gin.Engine {
	r := gin.Default()

	// Cookie, Session
	store := cookie.NewStore([]byte("something-very-secret"))
	r.Use(sessions.Sessions("my-session", store))

	// CORS
	r.Use(middleware.CORS())

	userCtrl := controller.User{}
	taskCtrl := controller.TaskController{}

	// Health check
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "pong"})
	})

	// avatars
	r.Static("/api/v1/avatar", "/app/assets/avatar")

	// auth
	authGroup := r.Group("/api/v1")
	{
		authGroup.POST("/login", authController.Login)
		authGroup.POST("/register", authController.Register)
		authGroup.POST("/logout", middleware.AuthRequired(), authController.Logout)
		authGroup.GET("/me", middleware.AuthRequired(), authController.Me)
	}

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

	// Hot search
	r.GET("/api/v1/hot_search", controller.HotSearch)

	// Task
	taskGroup := r.Group("/api/v1")
	{
		taskGroup.GET("/task", taskCtrl.ListTasks)
		taskGroup.POST("/task", taskCtrl.AddTask)
		taskGroup.PUT("/task/:id", taskCtrl.ToggleActive)
		taskGroup.DELETE("/task/:id", taskCtrl.DeleteTask)
	}

	// Music
	r.GET("/api/v1/music", controller.MusicList)

	// Visitor stats (Cookie/Session + uuid + Redis)
	r.POST("/api/v1/visit", controller.RecordVisit)
	r.GET("/api/v1/visitor", controller.GetVisitor)

	// Doc
	docGroup := r.Group("/api/v1/doc")
	{
		docGroup.GET("/list", docCtrl.List)
		docGroup.POST("/save", docCtrl.Save)
	}

	return r
}
