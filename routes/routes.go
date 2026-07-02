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
	authController        controller.AuthController          = controller.NewAuthController()
	userController        controller.User                    = controller.User{}
	taskController        controller.TaskController          = controller.TaskController{}
	docController         controller.DocController           = controller.DocController{}
	commentController     controller.CommentController       = controller.CommentController{}
	faqController         controller.FaqController           = controller.FaqController{}
	transactionController controller.TransactionController   = *controller.NewTransactionController()
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
		authGroup.POST("/setting_user", middleware.AuthRequired(), authController.SettingUser)
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
		r.GET("/api/v1/36kr", controller.Kr36Hot)
			r.GET("/api/v1/weather", controller.GetWeather)

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
	r.GET("/api/v1/visitor_daily", controller.VisitorDaily)
	r.GET("/api/v1/visitor_pv_uv", controller.VisitorPvUv)


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

	// FAQ
	r.GET("/api/v1/faq", faqController.ListFAQ)
	r.POST("/api/v1/faq", faqController.AddFAQ)


	// Chat — New conversation system
	chatGroup := r.Group("/api/v1/chat")
	{
		chatGroup.GET("/users", controller.GetChatUsers)
		chatGroup.GET("/conversations", middleware.AuthRequired(), controller.GetConversations)
		chatGroup.POST("/conversations", middleware.AuthRequired(), controller.CreateConversation)
		chatGroup.GET("/conversations/:id/messages", controller.GetMessages)
		chatGroup.PUT("/conversations/:id/read", middleware.AuthRequired(), controller.MarkConversationRead)
		chatGroup.POST("/messages", middleware.AuthRequired(), controller.PostMessage)
		chatGroup.PUT("/messages/:id/recall", middleware.AuthRequired(), controller.RecallMessage)
		chatGroup.GET("/ws", func(c *gin.Context) {
			controller.HandleWebSocket(c.Writer, c.Request)
		})
	}

	// Transaction
	transactionGroup := r.Group("/api/v1/transactions")
	transactionGroup.Use(middleware.AuthRequired())
	{
		transactionGroup.GET("", transactionController.List)                        // GET  /api/v1/transactions
		transactionGroup.POST("/filter", transactionController.FilterByMonth)         // POST /api/v1/transactions/filter
		transactionGroup.POST("/import", transactionController.ImportCSV)            // POST /api/v1/transactions/import
		transactionGroup.GET("/summary", transactionController.Summary)             // GET  /api/v1/transactions/summary
		transactionGroup.GET("/months", transactionController.Months)               // GET  /api/v1/transactions/months
		transactionGroup.GET("/categories", transactionController.CategoryBreakdown) // GET  /api/v1/transactions/categories
		transactionGroup.GET("/monthly", transactionController.MonthlyBreakdown)    // GET  /api/v1/transactions/monthly
		transactionGroup.DELETE("", transactionController.Delete)                   // DELETE /api/v1/transactions
	}

	return r
}
