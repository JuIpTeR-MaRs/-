package router

import (
	"dorm-repair-system/internal/controller"
	"dorm-repair-system/internal/middleware"
	"dorm-repair-system/internal/repository"
	"dorm-repair-system/internal/service"

	"github.com/gin-gonic/gin"
)

func SetupRouter() *gin.Engine {
	r := gin.New()
	
	// Global middlewares
	r.Use(middleware.Cors())
	r.Use(middleware.TraceIDMiddleware())
	r.Use(middleware.GinLogger())
	r.Use(middleware.CustomRecovery())

	// Serve the frontend pages
	r.StaticFile("/", "./index.html")
	r.StaticFile("/login", "./login.html")

	// 1. Dependency Injection Wiring (Clean Architecture)
	userRepo := repository.NewUserRepository()
	workOrderRepo := repository.NewWorkOrderRepository()

	userService := service.NewUserService(userRepo)
	workOrderService := service.NewWorkOrderService(workOrderRepo)

	userCtrl := controller.NewUserController(userService)
	workOrderCtrl := controller.NewWorkOrderController(workOrderService)

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
			user.GET("/workers", userCtrl.GetWorkers)
		}

		// RESTfulized Routes Alignment
		workorders := protected.Group("/workorders")
		workorders.Use(middleware.CasbinRBAC())
		{
			// Student submits workorder protected by Redis Rate Limiter middleware (10 capacity, 1 refilled token/sec)
			workorders.POST("", middleware.RateLimiterMiddleware(10, 1), workOrderCtrl.CreateOrder)
			workorders.GET("", workOrderCtrl.ListOrders)
			workorders.POST("/:id/evaluations", workOrderCtrl.EvaluateOrder)
			workorders.PUT("/:id/assignment", workOrderCtrl.AssignWorker)
			workorders.PUT("/:id/status", workOrderCtrl.UpdateStatusByWorker)

			// Grab order via worker
			workorders.PUT("/:id/grab", workOrderCtrl.GrabOrder)
			// Worker completes order specifying used consumables
			workorders.POST("/:id/completion", workOrderCtrl.CompleteOrderWithConsumables)
		}

		stats := protected.Group("/stats")
		stats.Use(middleware.CasbinRBAC())
		{
			stats.GET("/worker-leaderboard", workOrderCtrl.GetWorkerLeaderboard)
			// Analytics and metrics endpoints
			stats.GET("/locations", workOrderCtrl.GetLocationStats)
			stats.GET("/efficiency", workOrderCtrl.GetWorkerEfficiency)
		}
	}

	return r
}
