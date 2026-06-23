package repository

import (
	"context"
	"dorm-repair-system/internal/global"
	"dorm-repair-system/internal/model"
)

type IUserRepository interface {
	CreateUser(ctx context.Context, user *model.User) error
	GetUserByUsername(ctx context.Context, username string) (*model.User, error)
	GetUserByID(ctx context.Context, id uint) (*model.User, error)
	GetUsersByRole(ctx context.Context, role model.RoleEnum) ([]model.User, error)
}

type UserRepository struct{}

func NewUserRepository() IUserRepository {
	return &UserRepository{}
}

// CreateUser 创建用户
func (r *UserRepository) CreateUser(ctx context.Context, user *model.User) error {
	return global.DB.WithContext(ctx).Create(user).Error
}

// GetUserByUsername 根据用户名查询用户
func (r *UserRepository) GetUserByUsername(ctx context.Context, username string) (*model.User, error) {
	var user model.User
	err := global.DB.WithContext(ctx).Where("username = ?", username).First(&user).Error
	return &user, err
}

// GetUserByID 根据 ID 查询用户
func (r *UserRepository) GetUserByID(ctx context.Context, id uint) (*model.User, error) {
	var user model.User
	err := global.DB.WithContext(ctx).First(&user, id).Error
	return &user, err
}

// GetUsersByRole 根据角色获取用户列表
func (r *UserRepository) GetUsersByRole(ctx context.Context, role model.RoleEnum) ([]model.User, error) {
	var users []model.User
	err := global.DB.WithContext(ctx).Where("role = ?", role).Find(&users).Error
	return users, err
}
