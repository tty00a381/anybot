package core

import (
	"errors"
	"fmt"
)

var (
	// ErrPass 表示当前路由主动让出处理权，后续路由继续匹配。
	ErrPass = errors.New("anybot: pass route")
	// ErrStop 表示当前事件应停止向后续路由传播。
	ErrStop = errors.New("anybot: stop route")
	// ErrUnauthorized 表示当前事件没有通过权限检查。
	ErrUnauthorized = errors.New("anybot: unauthorized")
	// ErrRateLimited 表示当前限速窗口已被耗尽。
	ErrRateLimited = errors.New("anybot: rate limited")
	// ErrActionUnavailable 表示动作客户端当前暂不可用，通常是连接尚未建立或已临时断开。
	ErrActionUnavailable = errors.New("anybot: action unavailable")
)

// ActionError 描述一次由协议端明确拒绝或返回失败状态的动作调用。
type ActionError struct {
	Action  string
	Status  string
	RetCode int
	Message string
	Wording string
}

func (e *ActionError) Error() string {
	detail := e.Message
	if detail == "" {
		detail = e.Wording
	}
	if detail == "" {
		detail = "action failed"
	}
	return fmt.Sprintf("%s: status=%s retcode=%d: %s", e.Action, e.Status, e.RetCode, detail)
}

// PanicError 包装处理函数或中间件中恢复到的 panic 值和调用栈。
type PanicError struct {
	Value any
	Stack []byte
}

func (e *PanicError) Error() string {
	return fmt.Sprintf("anybot: panic: %v", e.Value)
}
