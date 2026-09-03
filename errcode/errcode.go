package errcode

// Error 业务错误，包含错误码和消息
type Error struct {
	Code    int
	Message string
}

func (e *Error) Error() string {
	return e.Message
}

// New 创建业务错误。各业务系统在本包之外用 New 定义自己的错误码段，
// 避免码值冲突：通用段 10000-10999 归本包，20000 起按模块分段留给业务侧。
func New(code int, message string) *Error {
	return &Error{Code: code, Message: message}
}

// 通用错误码 10000-10999
var (
	ErrInternal               = New(10000, "服务器内部错误")
	ErrInvalidParams          = New(10001, "参数错误")
	ErrUnauthorized           = New(10002, "未授权")
	ErrForbidden              = New(10003, "禁止访问")
	ErrNotFound               = New(10004, "资源不存在")
	ErrConflict               = New(10005, "资源冲突")
	ErrConcurrentModification = New(10006, "数据已被修改，请刷新后重试")
	ErrTooManyReqs            = New(10007, "请求过于频繁")
	ErrServiceUnavailable     = New(10008, "服务暂时不可用")
)
