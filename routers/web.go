package routers

import (
	"EventHunting/controllers"
	"EventHunting/middlewares"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func Register(router *gin.RouterGroup, redisClient *redis.Client) {
	//Auth
	authRouter := router.Group("auth")
	{
		authRouter.POST("/login", func(c *gin.Context) {
			controllers.Login(c, redisClient)
		})
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
		tagRouter.POST("/add", middlewares.RBACMiddleware("add_tag"), controllers.CreateTag)
		tagRouter.PATCH("/:id/update", middlewares.RBACMiddleware("update_tag"), controllers.UpdateTag)
		tagRouter.PATCH("/:id/soft-delete", middlewares.RBACMiddleware("soft-delete_tag"), controllers.SoftDeleteTag)
		tagRouter.PATCH("/:id/restore", middlewares.RBACMiddleware("restore_tag"), controllers.RestoreTag)
	}

	//Blog
	blogRouter := router.Group("blogs")
	{
		blogRouter.POST("/add", controllers.CreateBlog)
		blogRouter.GET("/search", controllers.GetListBlogs)
		blogRouter.GET("/:id/detail", controllers.GetBlog)
		blogRouter.GET("/:id/comments", controllers.GetCommentFromBlog)
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
}
