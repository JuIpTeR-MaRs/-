package main

import (
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"dorm-repair-system/internal/global"
	"dorm-repair-system/internal/model"
)

// 用于批量重置数据库中所有用户的密码为 123456 的脚本
func main() {
	// 初始化配置、日志和数据库连接
	global.InitConfig("config/config.yaml")
	global.InitLogger()
	global.InitDB()
	
	// 生成 123456 的 bcrypt 哈希值并更新所有用户的密码
	hash, _ := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)
	global.DB.Model(&model.User{}).Where("1 = 1").Update("password", string(hash))
	fmt.Println("Passwords updated successfully to 123456")
}
