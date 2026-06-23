package controller

import (
	"dorm-repair-system/internal/service"
	"dorm-repair-system/pkg/e"
	"dorm-repair-system/pkg/response"

	"github.com/gin-gonic/gin"
)

type UserController struct {
	userService service.IUserService
}

func NewUserController(s service.IUserService) *UserController {
	return &UserController{
		userService: s,
	}
}

func (ctrl *UserController) Register(c *gin.Context) {
	var input service.RegisterInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Fail(c, e.InvalidParams, response.TranslateError(err))
		return
	}

	if err := ctrl.userService.Register(c.Request.Context(), &input); err != nil {
		if err.Error() == "username already exists" {
			response.Fail(c, e.UserAlreadyExists)
		} else {
			response.Fail(c, e.ServerPanic, err.Error())
		}
		return
	}

	response.Success(c, nil)
}

func (ctrl *UserController) Login(c *gin.Context) {
	var input service.LoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Fail(c, e.InvalidParams, response.TranslateError(err))
		return
	}

	output, err := ctrl.userService.Login(c.Request.Context(), &input)
	if err != nil {
		if err.Error() == "invalid username or password" {
			response.Fail(c, e.InvalidPassword)
		} else {
			response.Fail(c, e.ServerPanic, err.Error())
		}
		return
	}

	response.Success(c, output)
}

func (ctrl *UserController) GetUserInfo(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		response.Fail(c, e.Unauthorized)
		return
	}

	user, err := ctrl.userService.GetUserInfo(c.Request.Context(), userID.(uint))
	if err != nil {
		response.Fail(c, e.NotFound, "failed to get user info")
		return
	}

	response.Success(c, user)
}

func (ctrl *UserController) GetWorkers(c *gin.Context) {
	workers, err := ctrl.userService.ListWorkers(c.Request.Context())
	if err != nil {
		response.Fail(c, e.ServerPanic, err.Error())
		return
	}
	response.Success(c, workers)
}
