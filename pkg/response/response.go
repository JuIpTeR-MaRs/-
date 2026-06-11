package response

import (
	"dorm-repair-system/pkg/e"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

type Response struct {
	Code e.ErrCode   `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data,omitempty"`
}

func Success(c *gin.Context, data interface{}) {
	c.JSON(e.Success.HttpStatus(), Response{
		Code: e.Success,
		Msg:  e.Success.Msg(),
		Data: data,
	})
}

func Fail(c *gin.Context, code e.ErrCode, customMsg ...string) {
	msg := code.Msg()
	if len(customMsg) > 0 && customMsg[0] != "" {
		msg = customMsg[0]
	}
	c.JSON(code.HttpStatus(), Response{
		Code: code,
		Msg:  msg,
		Data: nil,
	})
}

// TranslateError converts validator validation errors to user-friendly messages
func TranslateError(err error) string {
	errs, ok := err.(validator.ValidationErrors)
	if !ok {
		return err.Error()
	}

	for _, e := range errs {
		switch e.Field() {
		case "Content":
			switch e.Tag() {
			case "required":
				return "报修内容不能为空"
			case "min":
				return "报修内容长度不能少于10个字符"
			case "max":
				return "报修内容长度不能超过200个字符"
			}
		case "ContactPhone":
			return "联系电话必须为11位数字"
		case "Username":
			return "用户名长度需在3到30个字符之间"
		case "Password":
			return "密码长度需在6到30个字符之间"
		case "Role":
			return "用户角色必须为 Student, Worker 或 Housemaster"
		}
	}
	return err.Error()
}

// Deprecated: Error is kept for compatibility with legacy components
func Error(c *gin.Context, code int, msg string) {
	c.JSON(e.Success.HttpStatus(), Response{
		Code: e.ErrCode(code),
		Msg:  msg,
		Data: nil,
	})
}

// Deprecated: ErrorWithStatus is kept for compatibility
func ErrorWithStatus(c *gin.Context, httpStatus int, code int, msg string) {
	c.JSON(httpStatus, Response{
		Code: e.ErrCode(code),
		Msg:  msg,
		Data: nil,
	})
}
