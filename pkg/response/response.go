package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Response struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

const (
	CodeSuccess = 0
	CodeError   = 1
)

func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code: CodeSuccess,
		Msg:  "success",
		Data: data,
	})
}

func Error(c *gin.Context, code int, msg string) {
	c.JSON(http.StatusOK, Response{ // Always return 200 HTTP status, handle logic error by custom Code
		Code: code,
		Msg:  msg,
		Data: nil,
	})
}

func ErrorWithStatus(c *gin.Context, httpStatus int, code int, msg string) {
	c.JSON(httpStatus, Response{
		Code: code,
		Msg:  msg,
		Data: nil,
	})
}
