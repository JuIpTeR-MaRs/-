package global

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"dorm-repair-system/internal/model"

	casbin "github.com/casbin/casbin/v3"
	gormadapter "github.com/casbin/gorm-adapter/v3"
	"github.com/go-redis/redis/v8"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// InitConfig 初始化 Viper 配置
func InitConfig(configPath string) {
	viper.SetConfigFile(configPath)
	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Error reading config file: %v", err)
	}

	if err := viper.Unmarshal(&Config); err != nil {
		log.Fatalf("Error unmarshaling config: %v", err)
	}
	fmt.Println("Config loaded successfully.")
}

// InitLogger 初始化 Zap 日志
func InitLogger() {
	writeSyncer := getLogWriter()
	encoder := getEncoder()

	var l zapcore.Level
	if err := l.UnmarshalText([]byte(Config.Log.Level)); err != nil {
		l = zapcore.InfoLevel
	}

	core := zapcore.NewCore(encoder, writeSyncer, l)
	
	// 调试模式下也输出到控制台
	if Config.Server.Mode == "debug" {
		consoleEncoder := zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig())
		core = zapcore.NewTee(
			core,
			zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), zapcore.DebugLevel),
		)
	}

	Logger = zap.New(core, zap.AddCaller())
	Logger.Info("Logger initialized successfully")
}

func getEncoder() zapcore.Encoder {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	return zapcore.NewJSONEncoder(encoderConfig)
}

func getLogWriter() zapcore.WriteSyncer {
	lumberJackLogger := &lumberjack.Logger{
		Filename:   Config.Log.Filename,
		MaxSize:    Config.Log.MaxSize,
		MaxBackups: Config.Log.MaxBackups,
		MaxAge:     Config.Log.MaxAge,
		Compress:   true,
	}
	return zapcore.AddSync(lumberJackLogger)
}

// InitDB 初始化 MySQL 数据库连接（使用 GORM）
func InitDB() {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		Config.MySQL.User,
		Config.MySQL.Password,
		Config.MySQL.Host,
		Config.MySQL.Port,
		Config.MySQL.DBName,
	)

	var gormLogger gormlogger.Interface
	if Config.MySQL.LogMode {
		gormLogger = gormlogger.Default.LogMode(gormlogger.Info)
	} else {
		gormLogger = gormlogger.Default.LogMode(gormlogger.Silent)
	}

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		Logger.Fatal("Failed to connect to MySQL", zap.Error(err))
	}

	sqlDB, err := db.DB()
	if err != nil {
		Logger.Fatal("Failed to get sql.DB", zap.Error(err))
	}

	// 配置连接池优化性能
	sqlDB.SetMaxIdleConns(Config.MySQL.MaxIdleConns)
	sqlDB.SetMaxOpenConns(Config.MySQL.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(1 * time.Hour)
	sqlDB.SetConnMaxIdleTime(20 * time.Minute)

	DB = db
	Logger.Info("MySQL initialized successfully")
}

// SeedDefaultConsumables 初始化默认耗材数据
func SeedDefaultConsumables() {
	var count int64
	DB.Model(&model.Consumable{}).Count(&count)
	if count == 0 {
		items := []model.Consumable{
			{Name: "LED日光灯管", Stock: 100, Unit: "根"},
			{Name: "水龙头合金阀芯", Stock: 50, Unit: "个"},
			{Name: "五孔电插座面板", Stock: 80, Unit: "个"},
		}
		for _, item := range items {
			DB.Create(&item)
		}
		Logger.Info("Initial consumables seeded successfully")
	}
}

// SeedDefaultUsers 初始化默认测试账号
func SeedDefaultUsers() {
	users := []model.User{
		{
			Username: "admin",
			Password: "$2a$10$wO0X54261iO7bI2uL4gE8u.O9nU9G5/t00iRIf9j.UfR0X07Y.y0O", // 123456
			Role:     model.RoleAdmin,
			Phone:    "13800138000",
			RealName: "系统管理员",
		},
		{
			Username: "housemaster1",
			Password: "$2a$10$wO0X54261iO7bI2uL4gE8u.O9nU9G5/t00iRIf9j.UfR0X07Y.y0O", // 123456
			Role:     model.RoleHousemaster,
			Phone:    "13800138002",
			RealName: "李宿管老师",
		},
		{
			Username: "student1",
			Password: "$2a$10$wO0X54261iO7bI2uL4gE8u.O9nU9G5/t00iRIf9j.UfR0X07Y.y0O", // 123456
			Role:     model.RoleStudent,
			Phone:    "13900139000",
			RealName: "张三同学",
		},
		{
			Username: "worker1",
			Password: "$2a$10$wO0X54261iO7bI2uL4gE8u.O9nU9G5/t00iRIf9j.UfR0X07Y.y0O", // 123456
			Role:     model.RoleWorker,
			Phone:    "13700137000",
			RealName: "李四维修工",
		},
	}

	for _, u := range users {
		var count int64
		DB.Model(&model.User{}).Where("username = ?", u.Username).Count(&count)
		if count == 0 {
			if err := DB.Create(&u).Error; err != nil {
				Logger.Error("Failed to seed user", zap.String("username", u.Username), zap.Error(err))
			} else {
				Logger.Info("Seeded default user", zap.String("username", u.Username))
			}
		}
	}
}


// InitRedis 初始化 Redis 客户端
func InitRedis() {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", Config.Redis.Host, Config.Redis.Port),
		Password: Config.Redis.Password,
		DB:       Config.Redis.DB,
	})

	_, err := client.Ping(context.Background()).Result()
	if err != nil {
		Logger.Fatal("Failed to connect to Redis", zap.Error(err))
	}

	Redis = client
	Logger.Info("Redis initialized successfully")
}

// InitCasbin 初始化 Casbin 权限管理器
func InitCasbin() {
	// 初始化 GORM 适配器
	adapter, err := gormadapter.NewAdapterByDB(DB)
	if err != nil {
		Logger.Fatal("Failed to initialize casbin adapter", zap.Error(err))
	}

	// 创建 Casbin 执行器
	enforcer, err := casbin.NewEnforcer("config/rbac_model.conf", adapter)
	if err != nil {
		Logger.Fatal("Failed to create casbin enforcer", zap.Error(err))
	}

	err = enforcer.LoadPolicy()
	if err != nil {
		Logger.Fatal("Failed to load casbin policy", zap.Error(err))
	}

	Enforcer = enforcer
	
	// 自动初始化 Casbin 策略以防接口被拦截
	seedCasbinPolicies()

	Logger.Info("Casbin initialized successfully")
}

func seedCasbinPolicies() {
	policies := [][]string{
		{"Student", "/api/v1/workorders", "POST"},
		{"Student", "/api/v1/workorders", "GET"},
		{"Student", "/api/v1/workorders/:id/evaluations", "POST"},
		{"Student", "/api/v1/stats/worker-leaderboard", "GET"},
		{"Worker", "/api/v1/workorders", "GET"},
		{"Worker", "/api/v1/workorders/:id/status", "PUT"},
		{"Worker", "/api/v1/stats/worker-leaderboard", "GET"},
		{"Housemaster", "/api/v1/workorders", "GET"},
		{"Housemaster", "/api/v1/workorders/:id/assignment", "PUT"},
		{"Housemaster", "/api/v1/stats/worker-leaderboard", "GET"},

		// 抢单、完工及统计相关权限策略
		{"Worker", "/api/v1/workorders/:id/grab", "PUT"},
		{"Worker", "/api/v1/workorders/:id/completion", "POST"},
		{"Housemaster", "/api/v1/stats/locations", "GET"},
		{"Housemaster", "/api/v1/stats/efficiency", "GET"},
		{"Admin", "/api/v1/stats/locations", "GET"},
		{"Admin", "/api/v1/stats/efficiency", "GET"},
	}

	seeded := false
	for _, p := range policies {
		has, err := Enforcer.HasPolicy(p[0], p[1], p[2])
		if err == nil && !has {
			_, err = Enforcer.AddPolicy(p[0], p[1], p[2])
			if err == nil {
				seeded = true
			}
		}
	}
	if seeded {
		Logger.Info("RESTful Casbin policies seeded successfully")
		_ = Enforcer.SavePolicy()
	}
}
