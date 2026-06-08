package router

import (
	"dorm-repair-system/internal/controller"
	"dorm-repair-system/internal/middleware"

	"github.com/gin-gonic/gin"
)

func SetupRouter() *gin.Engine {
	r := gin.New()
	
	// Global middlewares
	r.Use(middleware.Cors())
	r.Use(middleware.GinLogger())
	r.Use(middleware.CustomRecovery())

	// Serve the frontend pages
	r.StaticFile("/", "./index.html")
	r.StaticFile("/login", "./login.html")

	// Controllers
	userCtrl := controller.NewUserController()
	workOrderCtrl := controller.NewWorkOrderController()

	api := r.Group("/api/v1")
	
	// Public routes
	auth := api.Group("/auth")
	{
		auth.POST("/register", userCtrl.Register)
		auth.POST("/login", userCtrl.Login)
	}

	// Test Route for Panic Recovery
	api.GET("/test-panic", workOrderCtrl.TestPanic)

	// Protected routes
	protected := api.Group("")
	protected.Use(middleware.JWTAuth())
	{
		user := protected.Group("/user")
		{
			user.GET("/info", userCtrl.GetUserInfo)
		}

		workorder := protected.Group("/workorder")
		workorder.Use(middleware.CasbinRBAC())
		{
			workorder.POST("", workOrderCtrl.CreateOrder)
			workorder.GET("", workOrderCtrl.ListOrders)
			workorder.POST("/:id/evaluate", workOrderCtrl.EvaluateOrder)
			workorder.PUT("/:id/assign", workOrderCtrl.AssignWorker)
			workorder.PUT("/:id/status", workOrderCtrl.UpdateStatusByWorker)
		}

		stats := protected.Group("/stats")
		stats.Use(middleware.CasbinRBAC())
		{
			stats.GET("/worker-leaderboard", workOrderCtrl.GetWorkerLeaderboard)
		}
	}

	return r
}
