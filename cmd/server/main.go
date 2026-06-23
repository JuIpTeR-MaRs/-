package main

import (
	"context"
	"fmt"

	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"dorm-repair-system/internal/global"
	"dorm-repair-system/internal/model"
	"dorm-repair-system/internal/router"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func main() {
	// 1. 初始化全局配置
	global.InitConfig("config/config.yaml")

	// 2. 初始化日志记录器
	global.InitLogger()
	defer global.Logger.Sync()

	// 3. 初始化 MySQL 数据库连接
	global.InitDB()
	
	// 自动同步数据库表结构
	err := global.DB.AutoMigrate(&model.User{}, &model.WorkOrder{}, &model.Notice{}, &model.InspectionOrder{}, &model.Consumable{}, &model.WorkOrderConsumable{})
	if err != nil {
		global.Logger.Fatal("Failed to auto migrate tables", zap.Error(err))
	}
	global.Logger.Info("Database tables migrated successfully")

	// 初始化物料库存种子数据
	global.SeedDefaultConsumables()
	// 初始化默认用户账户种子数据
	global.SeedDefaultUsers()

	// 4. 初始化 Redis 客户端
	global.InitRedis()

	// 5. 初始化 Casbin 权限管理器
	global.InitCasbin()

	// 6. 初始化并配置 Gin 路由器
	gin.SetMode(global.Config.Server.Mode)
	r := router.SetupRouter()

	// 7. 优雅启动与关机配置
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", global.Config.Server.Port),
		Handler: r,
	}

	go func() {
		global.Logger.Info(fmt.Sprintf("Server is running at http://127.0.0.1:%d", global.Config.Server.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			global.Logger.Fatal("Failed to listen and serve", zap.Error(err))
		}
	}()

	// 监听中断与终止信号以实现优雅退出，设定最长等待时间为 5 秒
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	global.Logger.Info("Shutdown Server ...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		global.Logger.Fatal("Server Shutdown:", zap.Error(err))
	}
	
	global.Logger.Info("Server exiting")
}
