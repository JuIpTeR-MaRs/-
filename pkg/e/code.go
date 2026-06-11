package e

import "net/http"

type ErrCode int

const (
	Success       ErrCode = 0
	InvalidParams ErrCode = 40001
	Unauthorized  ErrCode = 40101
	Forbidden     ErrCode = 40301
	NotFound      ErrCode = 40401
	ServerPanic   ErrCode = 50001

	// Business Errors
	UserAlreadyExists ErrCode = 20001
	InvalidPassword   ErrCode = 20002
	OrderStateError   ErrCode = 30001
	OrderAuthError    ErrCode = 30002
)

var codeToMsg = map[ErrCode]string{
	Success:           "success",
	InvalidParams:     "请求参数错误",
	Unauthorized:      "认证失败或已过期",
	Forbidden:         "权限不足",
	NotFound:          "资源未找到",
	ServerPanic:       "系统繁忙，请稍后再试",
	UserAlreadyExists: "用户名已存在",
	InvalidPassword:   "用户名或密码错误",
	OrderStateError:   "工单状态不满足当前操作",
	OrderAuthError:    "无权操作此工单",
}

func (c ErrCode) Msg() string {
	if msg, ok := codeToMsg[c]; ok {
		return msg
	}
	return "未知错误"
}

func (c ErrCode) HttpStatus() int {
	switch c {
	case Success:
		return http.StatusOK
	case InvalidParams:
		return http.StatusBadRequest
	case Unauthorized:
		return http.StatusUnauthorized
	case Forbidden:
		return http.StatusForbidden
	case NotFound:
		return http.StatusNotFound
	case ServerPanic:
		return http.StatusInternalServerError
	default:
		return http.StatusOK
	}
}
