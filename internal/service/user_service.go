package service

import (
	"errors"

	"dorm-repair-system/internal/model"
	"dorm-repair-system/internal/repository"
	"dorm-repair-system/pkg/utils"

	"gorm.io/gorm"
)

type UserService struct {
	userRepo *repository.UserRepository
}

func NewUserService() *UserService {
	return &UserService{
		userRepo: repository.NewUserRepository(),
	}
}

type RegisterInput struct {
	Username    string `json:"username" binding:"required,min=3,max=30"`
	Password    string `json:"password" binding:"required,min=6,max=30"`
	Role        string `json:"role" binding:"required,oneof=Student Worker Admin"`
	Phone       string `json:"phone"`
	RealName    string `json:"real_name"`
}

func (s *UserService) Register(input *RegisterInput) error {
	// Check if user exists
	_, err := s.userRepo.GetUserByUsername(input.Username)
	if err == nil {
		return errors.New("username already exists")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	hashedPassword, err := utils.HashPassword(input.Password)
	if err != nil {
		return err
	}

	user := &model.User{
		Username:    input.Username,
		Password:    hashedPassword,
		Role:        model.RoleEnum(input.Role),
		Phone:       input.Phone,
		RealName:    input.RealName,
	}

	return s.userRepo.CreateUser(user)
}

type LoginInput struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginOutput struct {
	Token string `json:"token"`
	User  *model.User `json:"user"`
}

func (s *UserService) Login(input *LoginInput) (*LoginOutput, error) {
	user, err := s.userRepo.GetUserByUsername(input.Username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("invalid username or password")
		}
		return nil, err
	}

	if !utils.CheckPasswordHash(input.Password, user.Password) {
		return nil, errors.New("invalid username or password")
	}

	token, err := utils.GenerateToken(user.ID, user.Username, string(user.Role))
	if err != nil {
		return nil, err
	}

	return &LoginOutput{
		Token: token,
		User:  user,
	}, nil
}

func (s *UserService) GetUserInfo(userID uint) (*model.User, error) {
	return s.userRepo.GetUserByID(userID)
}
