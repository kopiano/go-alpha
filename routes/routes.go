package routes

import (
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"

	"go-alpha/controller"
	"go-alpha/logger"
	"go-alpha/middleware"
)

var (
	authController    controller.AuthController    = controller.NewAuthController()
	userController    controller.User              = controller.User{}
	taskController    controller.TaskController    = controller.TaskController{}
	docController     controller.DocController     = controller.DocController{}
	commentController controller.CommentController = controller.CommentController{}
)

func SetupRouter() *gin.Engine {
	r := gin.New()
	r.Use(logger.GinMiddleware())
	r.Use(gin.Recovery())

	// Cookie, Session
	store := cookie.NewStore([]byte("something-very-secret"))
	r.Use(sessions.Sessions("my-session", store))

	// CORS
	r.Use(middleware.CORS())

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
		userGroup.GET("/user", userController.GetAllUsers)
		userGroup.GET("/user/:id", userController.GetUserById)
		userGroup.GET("/user/name/:name", userController.GetUserByName)
		userGroup.POST("/user", userController.AddUser)
		userGroup.PUT("/user/:id", userController.UpdateUser)
		userGroup.DELETE("/user/:id", userController.DeleteUser)
	}

	// Hot search
	r.GET("/api/v1/hot_search", controller.HotSearch)

	// Task
	taskGroup := r.Group("/api/v1")
	{
		taskGroup.GET("/task", taskController.ListTasks)
		taskGroup.POST("/task", taskController.AddTask)
		taskGroup.PUT("/task/:id", taskController.ToggleActive)
		taskGroup.DELETE("/task/:id", taskController.DeleteTask)
	}

	// Music
	r.GET("/api/v1/music", controller.MusicList)

	// Visitor stats (Cookie/Session + uuid + Redis)
	r.POST("/api/v1/visit", controller.RecordVisit)
	r.POST("/api/v1/visit/heartbeat", controller.VisitorHeartbeat)
	r.GET("/api/v1/visitor", controller.GetVisitor)

	// Doc
	docGroup := r.Group("/api/v1/doc")
	{
		docGroup.GET("/list", docController.List)
		docGroup.POST("/save", docController.Save)
	}

	// Comment
	r.GET("/api/v1/comment", commentController.ListComments)
	r.POST("/api/v1/comment", commentController.AddComment)
	r.POST("/api/v1/comment/:id/likes", commentController.LikesComment) // 点赞
	return r
}
