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
	authController        controller.AuthController        = controller.NewAuthController()
	userController        controller.User                  = controller.User{}
	taskController        controller.TaskController        = controller.TaskController{}
	docController         controller.DocController         = controller.DocController{}
	commentController     controller.CommentController     = controller.CommentController{}
	faqController         controller.FaqController         = controller.FaqController{}
	transactionController controller.TransactionController = *controller.NewTransactionController()
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

	// avatars

	// Health check
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "pong"})
	})

	v1 := r.Group("api/v1")
	{
		auth := v1.Group("/auth")
		{
			auth.POST("/login", authController.Login)
			auth.POST("/register", authController.Register)
			auth.POST("/logout", middleware.AuthRequired(), authController.Logout)
			// 刷新用户信息，判断当前是否已登录，显示头像、用户名、在线状态等
			auth.GET("/me", middleware.AuthRequired(), authController.Me)
			auth.POST("/setting_user", middleware.AuthRequired(), authController.SettingUser)
		}
		v1.Static("/avatar", "/app/assets/avatar")
		user := v1.Group("/user")
		{
			user.GET("", userController.GetAllUsers)
			// 暂时用不上
			// user.GET("/:id", userController.GetUserById)
			// user.GET("/name/:name", userController.GetUserByName)
			// user.POST("", userController.AddUser)
			// user.PUT("/:id", userController.UpdateUser)
			// user.DELETE("/:id", userController.DeleteUser)
		}
		// Music
		music := v1.Group("/music")
		{
			music.GET("", controller.MusicList)
			music.Static("/file", "/app/assets/music")
		}
		// weather
		v1.GET("/weather", controller.GetWeather)
		// holiday
		v1.GET("/holiday", controller.GetHoliday)
		// task
		task := v1.Group("/task")
		{
			task.GET("", taskController.ListTasks)
			task.POST("", taskController.AddTask)
			task.PUT("/:id", taskController.ToggleActive)
			task.DELETE("/:id", taskController.DeleteTask)
		}
		// news
		v1.GET("/hot_search", controller.HotSearch)
		v1.GET("/36kr", func(c *gin.Context) {
			c.JSON(200, gin.H{"message": "因36kr改用字节火山引擎需要真人滑块验证，接口已弃用"})
		})
		doc := v1.Group("/docs")
		{
			doc.GET("", docController.List)
			doc.GET("/:id", docController.Detail)
			doc.POST("", docController.Save)
			doc.PUT("/:id", middleware.AuthRequired(), docController.Update)
			doc.DELETE("/:id", middleware.AuthRequired(), docController.Delete)
		}
		// Visitor stats (Cookie/Session + uuid + Redis)
		visitor := v1.Group("/visitor")
		{
			// 获取访客统计总览和访客列表，包括总 PV/UV、今日 PV/UV、近 7 天 UV、活跃访客数、总浏览时长，以及当前访客信息
			visitor.GET("", controller.GetVisitor)
			// 获取每日访客统计明细，返回 visitor_summary 表里的所有日期数据，适合做日趋势图
			visitor.GET("/daily", controller.VisitorDaily)
			// 统计所有历史日期的 PV 和 UV 总和，返回一个汇总结果
			visitor.GET("/pv_uv", controller.VisitorPvUv)
			// 记录一次访问行为。通常是前端首次进入页面时调用，用来新增或更新访客记录，同时累加 PV/UV，并写入访客位置、设备、浏览器、停留信息等
			visitor.POST("/visit", controller.RecordVisit)
			// 心跳上报接口。前端定时调用，用来累计浏览时长、更新最后访问时间，必要时同步用户名
			visitor.POST("/heartbeat", controller.VisitorHeartbeat)
		}
		comment := v1.Group("/comment")
		{
			comment.GET("", commentController.ListComments)
			comment.POST("", commentController.AddComment)
			comment.PUT("/:id/like", middleware.AuthRequired(), commentController.LikeComment)      // 点赞
			comment.DELETE("/:id/like", middleware.AuthRequired(), commentController.UnlikeComment) // 取消点赞
		}
		faq := v1.Group("/faq")
		{
			faq.GET("", faqController.ListFAQ)
			faq.POST("", faqController.AddFAQ)
		}
		// Chat — New conversation system
		chat := v1.Group("/chat")
		{
			// 获取联系人列表
			chat.GET("/conversations", middleware.AuthRequired(), controller.GetConversations)
			// 获取历史消息
			chat.GET("/conversations/:id/messages", middleware.AuthRequired(), controller.GetConversationMessagesV2)
			// 发送消息
			chat.POST("/messages", middleware.AuthRequired(), controller.PostMessage)
			// chat.GET("/groups", middleware.AuthRequired(), controller.GetGroups)                    // 群聊信息
			// chat.POST("/conversations", middleware.AuthRequired(), controller.CreateConversationV2) // 创建/获取私聊会话
			chat.PUT("/conversations/:id/read", middleware.AuthRequired(), controller.MarkConversationReadV2)
			chat.GET("/ws", func(c *gin.Context) { // WebSocket
				controller.HandleWebSocket(c.Writer, c.Request)
			})
		}
		// Transaction
		transaction := v1.Group("/transactions")
		transaction.Use(middleware.AuthRequired())
		{
			transaction.GET("", transactionController.List)
			transaction.GET("/summary", transactionController.Summary)
			transaction.GET("/months", transactionController.Months)
			transaction.GET("/categories", transactionController.CategoryBreakdown)
			transaction.GET("/top-merchants", transactionController.TopMerchants)
			transaction.GET("/hot-merchants", transactionController.HotMerchants)
			transaction.GET("/monthly", transactionController.MonthlyBreakdown)
			transaction.POST("/filter", transactionController.FilterByMonth)
			transaction.POST("/import", transactionController.ImportCSV)
			transaction.DELETE("", transactionController.Delete)
		}
	}
	// v1 end
	return r
}
