package repository

import (
	"dorm-repair-system/internal/global"
	"dorm-repair-system/internal/model"
)

type UserRepository struct{}

func NewUserRepository() *UserRepository {
	return &UserRepository{}
}

func (r *UserRepository) CreateUser(user *model.User) error {
	return global.DB.Create(user).Error
}

func (r *UserRepository) GetUserByUsername(username string) (*model.User, error) {
	var user model.User
	err := global.DB.Where("username = ?", username).First(&user).Error
	return &user, err
}

func (r *UserRepository) GetUserByID(id uint) (*model.User, error) {
	var user model.User
	err := global.DB.First(&user, id).Error
	return &user, err
}
