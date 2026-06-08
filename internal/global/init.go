package global

import (
	"context"
	"fmt"
	"log"
	"os"


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

// InitConfig initializes viper config
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

// InitLogger initializes zap logger
func InitLogger() {
	writeSyncer := getLogWriter()
	encoder := getEncoder()

	var l zapcore.Level
	if err := l.UnmarshalText([]byte(Config.Log.Level)); err != nil {
		l = zapcore.InfoLevel
	}

	core := zapcore.NewCore(encoder, writeSyncer, l)
	
	// Also log to console in debug mode
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

// InitDB initializes GORM MySQL connection
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

	sqlDB.SetMaxIdleConns(Config.MySQL.MaxIdleConns)
	sqlDB.SetMaxOpenConns(Config.MySQL.MaxOpenConns)

	DB = db
	Logger.Info("MySQL initialized successfully")
}

// InitRedis initializes Redis client
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

// InitCasbin initializes Casbin enforcer
func InitCasbin() {
	// Initialize a Gorm adapter and use it in a Casbin enforcer
	adapter, err := gormadapter.NewAdapterByDB(DB)
	if err != nil {
		Logger.Fatal("Failed to initialize casbin adapter", zap.Error(err))
	}

	// Create casbin enforcer
	enforcer, err := casbin.NewEnforcer("config/rbac_model.conf", adapter)
	if err != nil {
		Logger.Fatal("Failed to create casbin enforcer", zap.Error(err))
	}

	err = enforcer.LoadPolicy()
	if err != nil {
		Logger.Fatal("Failed to load casbin policy", zap.Error(err))
	}

	Enforcer = enforcer
	Logger.Info("Casbin initialized successfully")
}
