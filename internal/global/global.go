package global

import (
	"dorm-repair-system/internal/config"

	"github.com/casbin/casbin/v3"
	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var (
	Config   *config.Config    // 全局配置
	DB       *gorm.DB          // 数据库连接实例
	Redis    *redis.Client     // Redis 客户端实例
	Logger   *zap.Logger       // 日志记录器
	Enforcer *casbin.Enforcer  // Casbin 权限执行器
)
