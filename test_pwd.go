package main

import (
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"dorm-repair-system/internal/global"
	"dorm-repair-system/internal/model"
)

func main() {
	global.InitConfig("config/config.yaml")
	global.InitLogger()
	global.InitDB()
	
	hash, _ := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)
	global.DB.Model(&model.User{}).Where("1 = 1").Update("password", string(hash))
	fmt.Println("Passwords updated successfully to 123456")
}
