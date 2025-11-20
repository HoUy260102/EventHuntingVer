package routers

import (
	"EventHunting/controllers"
	"EventHunting/middlewares"

	"github.com/gin-gonic/gin"
)

func Register(router *gin.RouterGroup) {
	//Auth
	authRouter := router.Group("auth")
	{
		authRouter.POST("/login", controllers.Login)
		authRouter.POST("/logout", controllers.Logout)
		authRouter.POST("renew-access-token", controllers.RenewAccessToken)
		authRouter.GET("/:provider", controllers.BeginGoogleAuth)
		authRouter.GET("/:provider/callback", controllers.OAuthCallback)
	}

	//Account
	accountRouter := router.Group("accounts")
	{
		accountRouter.POST("/create", middlewares.AuthorizeJWTMiddleware(), middlewares.RBACMiddleware("add_account"), controllers.CreateAccount)
		accountRouter.PATCH("/:id/update", middlewares.AuthorizeJWTMiddleware(), middlewares.RBACMiddleware("update_account"), controllers.UpdateAccount)
		accountRouter.PATCH("/:id/upload-avatar", middlewares.AuthorizeJWTMiddleware(), middlewares.RBACMiddleware("update_account"), controllers.UploadAvatar)
		accountRouter.PATCH("/:id/lock", middlewares.AuthorizeJWTMiddleware(), middlewares.RBACMiddleware("lock_account"), controllers.LockAccount)
		accountRouter.PATCH("/:id/unlock", middlewares.AuthorizeJWTMiddleware(), middlewares.RBACMiddleware("unlock_account"), controllers.UnlockAccount)
		accountRouter.GET("/:id/detail", middlewares.AuthorizeJWTMiddleware(), controllers.GetAccount)
		accountRouter.PATCH("/:id/soft-delete", middlewares.AuthorizeJWTMiddleware(), middlewares.RBACMiddleware("soft-delete_account"), controllers.SoftDeleteAccount)
		accountRouter.PATCH("/:id/restore", middlewares.AuthorizeJWTMiddleware(), middlewares.RBACMiddleware("restore_account"), controllers.RestoreAccount)
		accountRouter.GET("/search", controllers.FindAccounts)
		accountRouter.GET("/all1", controllers.FindAccountsByKeyset)
	}

	//Permisson
	permissonRouter := router.Group("permissions")
	{
		permissonRouter.Use(middlewares.AuthorizeJWTMiddleware())
		permissonRouter.POST("/add", middlewares.RBACMiddleware("add_permission"), controllers.CreatePermission)
		permissonRouter.PATCH("/:id/lock", middlewares.RBACMiddleware("lock_permission"), controllers.LockPermission)
		permissonRouter.PATCH("/:id/unlock", middlewares.RBACMiddleware("unlock_permission"), controllers.UnLockPermission)
		permissonRouter.PATCH("/:id/disable", middlewares.RBACMiddleware("disable_permission"), controllers.DisablePermission)
		permissonRouter.PATCH("/:id/enable", middlewares.RBACMiddleware("enable_permission"), controllers.EnablePermission)
		permissonRouter.GET("/all", middlewares.RBACMiddleware("read_permission"), controllers.FindPermissions)
	}

	//Role
	roleRouter := router.Group("/roles")
	{
		roleRouter.Use(middlewares.AuthorizeJWTMiddleware())
		roleRouter.POST("/:id/permissions/add", middlewares.RBACMiddleware("assign_permission"), controllers.AssignPermissionsToRole)
		roleRouter.POST("/add", middlewares.RBACMiddleware("add_role"), controllers.CreateRole)
		roleRouter.PATCH("/:id/permissions/remove", middlewares.RBACMiddleware("remove_permission"), controllers.RemovePermissionFromRole)
		roleRouter.GET("/:id/permissions/all", middlewares.RBACMiddleware("read_permission"), controllers.GetPermissionFromRole)
		roleRouter.PATCH("/:id/permissions/delete", middlewares.RBACMiddleware("remove_permission"), controllers.RemovePermissionFromRole)
		roleRouter.PATCH("/:id/soft-delete", middlewares.RBACMiddleware("soft-delete_role"), controllers.SoftDeleteRole)
		roleRouter.PATCH("/:id/restore", middlewares.RBACMiddleware("restore_role"), controllers.RestoreRole)
	}

	//Tag
	tagRouter := router.Group("tags")
	{
		tagRouter.POST("/add", middlewares.AuthorizeJWTMiddleware(), middlewares.RBACMiddleware("add_tag"), controllers.CreateTag)
		tagRouter.PATCH("/:id/update", middlewares.AuthorizeJWTMiddleware(), middlewares.RBACMiddleware("update_tag"), controllers.UpdateTag)
		tagRouter.PATCH("/:id/soft-delete", middlewares.AuthorizeJWTMiddleware(), middlewares.RBACMiddleware("soft-delete_tag"), controllers.SoftDeleteTag)
		tagRouter.PATCH("/:id/restore", middlewares.AuthorizeJWTMiddleware(), middlewares.RBACMiddleware("restore_tag"), controllers.RestoreTag)
		tagRouter.GET("", controllers.FindTag)
	}

	//Topic
	topicRouter := router.Group("topics")
	{
		topicRouter.POST("/add", middlewares.AuthorizeJWTMiddleware(), middlewares.RBACMiddleware("add_topic"), controllers.CreateTopic)
		topicRouter.PATCH("/:id/update", middlewares.AuthorizeJWTMiddleware(), middlewares.RBACMiddleware("update_topic"), controllers.UpdateTopic)
		topicRouter.PATCH("/:id/soft-delete", middlewares.AuthorizeJWTMiddleware(), middlewares.RBACMiddleware("soft-delete_topic"), controllers.SoftDeleteTopic)
		topicRouter.PATCH("/:id/restore", middlewares.AuthorizeJWTMiddleware(), middlewares.RBACMiddleware("restore_topic"), controllers.RestoreTopic)
		topicRouter.GET("", controllers.FindTopic)
	}

	//Blog
	blogRouter := router.Group("blogs")
	{
		blogRouter.POST("/add", middlewares.AuthorizeJWTMiddleware(), controllers.CreateBlog)
		blogRouter.PATCH("/:id/update", middlewares.AuthorizeJWTMiddleware(), controllers.UpdateBlog)
		blogRouter.PATCH("/:id/soft-delete", middlewares.AuthorizeJWTMiddleware(), controllers.SoftDeleteBlog)
		blogRouter.PATCH("/:id/restore", middlewares.AuthorizeJWTMiddleware(), controllers.RestoreBlog)
		blogRouter.PATCH("/:id/lock-comment", middlewares.AuthorizeJWTMiddleware(), controllers.LockComment)
		blogRouter.PATCH("/:id/unlock-comment", middlewares.AuthorizeJWTMiddleware(), controllers.UnLockComment)
		blogRouter.GET("/search", controllers.GetListBlogs)
		blogRouter.GET("/:id/detail", middlewares.OptionalAuthMiddleware(), controllers.GetBlog)
		blogRouter.GET("/:id/comments", controllers.GetCommentFromBlog)
	}

	//Event
	eventRouter := router.Group("events")
	{
		eventRouter.POST("/add", middlewares.AuthorizeJWTMiddleware(), controllers.CreateEvent)
		eventRouter.PATCH("/:id/update", middlewares.AuthorizeJWTMiddleware(), controllers.UpdateEvent)
		eventRouter.GET("/:id/detail", middlewares.OptionalAuthMiddleware(), controllers.GetEvent)
		eventRouter.GET("/search", controllers.GetListEvents)
		eventRouter.GET("/:id/ticket_types", controllers.GetListTicketTypes)
		eventRouter.POST("/:id/registration", middlewares.AuthorizeJWTMiddleware(), controllers.RegistrationEvent)
	}

	//Comment
	commentRouter := router.Group("comments")
	{
		commentRouter.Use(middlewares.AuthorizeJWTMiddleware())
		commentRouter.POST("/add", controllers.CreateComment)
		commentRouter.PATCH("/:id/update", controllers.UpdateComment)
		commentRouter.PATCH("/:id/soft-delete", controllers.SoftDeleteComment)
		commentRouter.PATCH("/:id/restore", controllers.RestoreComment)
		commentRouter.GET("/:id/reply", controllers.GetCommentReplies)
	}

	//Ticket Type
	ticketTypeRouter := router.Group("ticket_types")
	{
		ticketTypeRouter.Use(middlewares.AuthorizeJWTMiddleware())
		ticketTypeRouter.POST("/add", controllers.CreateTicketType)
		ticketTypeRouter.PATCH("/:id/update", controllers.UpdateTicketType)
	}

	//Media
	mediaRouter := router.Group("medias")
	{
		mediaRouter.POST("/upload", middlewares.MaxBodySizeMiddleware(10*1024*1024), controllers.UploadMedia)
	}

	router.GET("/vnpay_return", func(c *gin.Context) {
		controllers.HandleCallbackVNPAY(c)
	})
}
