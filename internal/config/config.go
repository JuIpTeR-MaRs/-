package config

// ServerConfig 服务运行配置
type ServerConfig struct {
	Port int    `mapstructure:"port"` // 端口
	Mode string `mapstructure:"mode"` // 运行模式
}

// MySQLConfig MySQL 数据库配置
type MySQLConfig struct {
	Host         string `mapstructure:"host"`           // 主机地址
	Port         int    `mapstructure:"port"`           // 端口
	User         string `mapstructure:"user"`           // 用户名
	Password     string `mapstructure:"password"`       // 密码
	DBName       string `mapstructure:"dbname"`         // 数据库名称
	MaxIdleConns int    `mapstructure:"max_idle_conns"` // 最大闲置连接数
	MaxOpenConns int    `mapstructure:"max_open_conns"` // 最大打开连接数
	LogMode      bool   `mapstructure:"log_mode"`       // 是否开启日志模式
}

// RedisConfig Redis 配置
type RedisConfig struct {
	Host     string `mapstructure:"host"`     // 主机地址
	Port     int    `mapstructure:"port"`     // 端口
	Password string `mapstructure:"password"` // 密码
	DB       int    `mapstructure:"db"`       // 数据库索引
}

// JWTConfig JWT 签名配置
type JWTConfig struct {
	Secret string `mapstructure:"secret"` // 密钥
	Expire int64  `mapstructure:"expire"` // 过期时间（秒）
}

// LogConfig 日志配置
type LogConfig struct {
	Level      string `mapstructure:"level"`       // 日志级别
	Filename   string `mapstructure:"filename"`    // 写入文件名
	MaxSize    int    `mapstructure:"max_size"`    // 单个文件最大大小（MB）
	MaxBackups int    `mapstructure:"max_backups"` // 最大备份数
	MaxAge     int    `mapstructure:"max_age"`     // 最大保存天数
}

// Config 全局配置结构体
type Config struct {
	Server ServerConfig `mapstructure:"server"`
	MySQL  MySQLConfig  `mapstructure:"mysql"`
	Redis  RedisConfig  `mapstructure:"redis"`
	JWT    JWTConfig    `mapstructure:"jwt"`
	Log    LogConfig    `mapstructure:"log"`
}
