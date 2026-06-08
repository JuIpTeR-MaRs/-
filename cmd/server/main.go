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
	// 1. Initialize Config
	global.InitConfig("config/config.yaml")

	// 2. Initialize Logger
	global.InitLogger()
	defer global.Logger.Sync()

	// 3. Initialize MySQL and GORM
	global.InitDB()
	
	// Auto migrate tables
	err := global.DB.AutoMigrate(&model.User{}, &model.WorkOrder{}, &model.Notice{}, &model.InspectionOrder{})
	if err != nil {
		global.Logger.Fatal("Failed to auto migrate tables", zap.Error(err))
	}
	global.Logger.Info("Database tables migrated successfully")

	// 4. Initialize Redis
	global.InitRedis()

	// 5. Initialize Casbin
	global.InitCasbin()

	// 6. Setup Gin Router
	gin.SetMode(global.Config.Server.Mode)
	r := router.SetupRouter()

	// 7. Start Server gracefully
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

	// Wait for interrupt signal to gracefully shutdown the server with a timeout of 5 seconds.
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
