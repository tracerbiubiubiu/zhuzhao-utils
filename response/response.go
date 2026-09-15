package response

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/tracerbiubiubiu/zhuzhao-utils/errcode"
)

// Response 统一响应结构
type Response struct {
	Code      int         `json:"code"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data"`
	RequestID string      `json:"request_id,omitempty"`
}

// PageData 分页响应数据
type PageData struct {
	List     interface{} `json:"list"`
	Total    int64       `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
}

// OK 成功响应
func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:      0,
		Message:   "success",
		Data:      data,
		RequestID: c.GetString("request_id"),
	})
}

// OKWithMessage 成功响应（自定义消息）
func OKWithMessage(c *gin.Context, message string, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:      0,
		Message:   message,
		Data:      data,
		RequestID: c.GetString("request_id"),
	})
}

// OKPage 分页响应
func OKPage(c *gin.Context, list interface{}, total int64, page, pageSize int) {
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data: PageData{
			List:     list,
			Total:    total,
			Page:     page,
			PageSize: pageSize,
		},
		RequestID: c.GetString("request_id"),
	})
}

// Fail 失败响应
func Fail(c *gin.Context, httpStatus, code int, message string) {
	c.JSON(httpStatus, Response{
		Code:      code,
		Message:   message,
		Data:      nil,
		RequestID: c.GetString("request_id"),
	})
}

// Error 按业务错误码响应（HTTP 状态由调用方指定）
func Error(c *gin.Context, httpStatus int, err *errcode.Error) {
	Fail(c, httpStatus, err.Code, err.Message)
}

// BadRequest 参数错误
func BadRequest(c *gin.Context, message string) {
	if message == "" {
		message = errcode.ErrInvalidParams.Message
	}
	Fail(c, http.StatusBadRequest, errcode.ErrInvalidParams.Code, message)
}

// Unauthorized 未认证（默认 10002）
func Unauthorized(c *gin.Context, message string) {
	if message == "" {
		message = errcode.ErrUnauthorized.Message
	}
	Fail(c, http.StatusUnauthorized, errcode.ErrUnauthorized.Code, message)
}

// UnauthorizedError 未认证（指定业务码，如 20003）
func UnauthorizedError(c *gin.Context, err *errcode.Error) {
	Error(c, http.StatusUnauthorized, err)
}

// Forbidden 无权限（默认 10003，兼容旧调用）
func Forbidden(c *gin.Context, message string) {
	if message == "" {
		message = errcode.ErrForbidden.Message
	}
	Fail(c, http.StatusForbidden, errcode.ErrForbidden.Code, message)
}

// ForbiddenError 无权限（指定业务码，如 70001 / 70003 / 30003）
func ForbiddenError(c *gin.Context, err *errcode.Error) {
	Error(c, http.StatusForbidden, err)
}

// ServiceUnavailable 鉴权链路不可用（503 + 10008）
func ServiceUnavailable(c *gin.Context) {
	Error(c, http.StatusServiceUnavailable, errcode.ErrServiceUnavailable)
}

// TooManyRequests 请求过于频繁（429）
func TooManyRequests(c *gin.Context, err *errcode.Error) {
	Error(c, http.StatusTooManyRequests, err)
}

// NotFound 资源不存在
func NotFound(c *gin.Context, message string) {
	Fail(c, http.StatusNotFound, errcode.ErrNotFound.Code, message)
}

// Conflict 冲突
func Conflict(c *gin.Context, message string) {
	Fail(c, http.StatusConflict, errcode.ErrConflict.Code, message)
}

// InternalError 内部错误
func InternalError(c *gin.Context, message string) {
	Fail(c, http.StatusInternalServerError, errcode.ErrInternal.Code, message)
}
