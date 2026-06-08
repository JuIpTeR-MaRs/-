package global

import (
	"dorm-repair-system/internal/config"

	"github.com/casbin/casbin/v3"
	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var (
	Config   *config.Config
	DB       *gorm.DB
	Redis    *redis.Client
	Logger   *zap.Logger
	Enforcer *casbin.Enforcer
)
