package controller

import (
	"dorm-repair-system/internal/service"
	"dorm-repair-system/pkg/response"

	"github.com/gin-gonic/gin"
)

type UserController struct {
	userService *service.UserService
}

func NewUserController() *UserController {
	return &UserController{
		userService: service.NewUserService(),
	}
}

func (ctrl *UserController) Register(c *gin.Context) {
	var input service.RegisterInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, response.CodeError, err.Error())
		return
	}

	if err := ctrl.userService.Register(&input); err != nil {
		response.Error(c, response.CodeError, err.Error())
		return
	}

	response.Success(c, nil)
}

func (ctrl *UserController) Login(c *gin.Context) {
	var input service.LoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, response.CodeError, err.Error())
		return
	}

	output, err := ctrl.userService.Login(&input)
	if err != nil {
		response.Error(c, response.CodeError, err.Error())
		return
	}

	response.Success(c, output)
}

func (ctrl *UserController) GetUserInfo(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		response.Error(c, response.CodeError, "unauthorized")
		return
	}

	user, err := ctrl.userService.GetUserInfo(userID.(uint))
	if err != nil {
		response.Error(c, response.CodeError, "failed to get user info")
		return
	}

	response.Success(c, user)
}
