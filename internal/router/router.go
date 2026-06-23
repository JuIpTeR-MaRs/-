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
	
	// 全局中间件
	r.Use(middleware.Cors())
	r.Use(middleware.TraceIDMiddleware())
	r.Use(middleware.GinLogger())
	r.Use(middleware.CustomRecovery())

	// 静态文件服务：托管前端页面
	r.StaticFile("/", "./index.html")
	r.StaticFile("/login", "./login.html")

	// 1. 依赖注入组装 (Clean Architecture 架构)
	userRepo := repository.NewUserRepository()
	workOrderRepo := repository.NewWorkOrderRepository()

	userService := service.NewUserService(userRepo)
	workOrderService := service.NewWorkOrderService(workOrderRepo)

	userCtrl := controller.NewUserController(userService)
	workOrderCtrl := controller.NewWorkOrderController(workOrderService)

	api := r.Group("/api/v1")
	
	// 开放的认证路由
	auth := api.Group("/auth")
	{
		auth.POST("/register", userCtrl.Register)
		auth.POST("/login", userCtrl.Login)
	}

	// 异常恢复测试路由
	api.GET("/test-panic", workOrderCtrl.TestPanic)

	// 受 JWT 登录认证保护的路由
	protected := api.Group("")
	protected.Use(middleware.JWTAuth())
	{
		user := protected.Group("/user")
		{
			user.GET("/info", userCtrl.GetUserInfo)
			user.GET("/workers", userCtrl.GetWorkers)
		}

		// 符合 RESTful 规范的工单路由，由 Casbin 执行基于角色的访问控制 (RBAC)
		workorders := protected.Group("/workorders")
		workorders.Use(middleware.CasbinRBAC())
		{
			// 学生创建工单：受限流中间件限制（容量 10，每秒补充 1 个令牌）
			workorders.POST("", middleware.RateLimiterMiddleware(10, 1), workOrderCtrl.CreateOrder)
			workorders.GET("", workOrderCtrl.ListOrders)
			workorders.POST("/:id/evaluations", workOrderCtrl.EvaluateOrder)
			workorders.PUT("/:id/assignment", workOrderCtrl.AssignWorker)
			workorders.PUT("/:id/status", workOrderCtrl.UpdateStatusByWorker)

			// 师傅抢单接口
			workorders.PUT("/:id/grab", workOrderCtrl.GrabOrder)
			// 师傅完成工单（扣减库存）
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
