package sdk

import (
	"errors"
	"strings"
	"time"
)

// ErrStoreUnavailable 表示当前运行时没有可用会话存储。
var ErrStoreUnavailable = errors.New("anybot: plugin store unavailable")

// ErrEventContextUnavailable 表示当前缺少事件上下文。
var ErrEventContextUnavailable = errors.New("anybot: event context unavailable")

// ErrStateKeyRequired 表示 typed 状态缺少字段名。
var ErrStateKeyRequired = errors.New("anybot: plugin state key is required")

// State 是绑定到插件命名空间和事件上下文的一段 typed 会话状态。
type State[T any] struct {
	session *Session
	event   *EventContext
	key     string
	err     error
}

// ConversationState 返回当前自然会话下的 typed 状态。
func ConversationState[T any](ctx *Context, event *EventContext, key string) State[T] {
	state := State[T]{event: event, key: key, err: validateStateKey(key)}
	if ctx == nil || ctx.Store() == nil {
		return state
	}
	state.session = ctx.Session(event)
	return state
}

// UserState 返回当前用户维度下的 typed 状态。
func UserState[T any](ctx *Context, event *EventContext, key string) State[T] {
	state := State[T]{event: event, key: key, err: validateStateKey(key)}
	if ctx == nil || ctx.Store() == nil {
		return state
	}
	state.session = ctx.UserSession(event)
	return state
}

// GroupState 返回当前群或频道维度下的 typed 状态。
func GroupState[T any](ctx *Context, event *EventContext, key string) State[T] {
	state := State[T]{event: event, key: key, err: validateStateKey(key)}
	if ctx == nil || ctx.Store() == nil {
		return state
	}
	state.session = ctx.GroupSession(event)
	return state
}

// NamedState 返回当前插件自定义维度下的 typed 状态。
func NamedState[T any](ctx *Context, event *EventContext, key, scope string) State[T] {
	state := State[T]{event: event, key: key, err: validateStateKey(key)}
	if ctx == nil || ctx.Store() == nil {
		return state
	}
	state.session = ctx.SessionBy(scope)
	return state
}

func validateStateKey(key string) error {
	if strings.TrimSpace(key) == "" {
		return ErrStateKeyRequired
	}
	return nil
}

// Load 读取状态；状态不存在时返回零值和 ok=false。
func (s State[T]) Load() (T, bool, error) {
	var out T
	if s.err != nil {
		return out, false, s.err
	}
	if s.session == nil {
		return out, false, ErrStoreUnavailable
	}
	if s.event == nil {
		return out, false, ErrEventContextUnavailable
	}
	ok, err := s.session.LoadJSON(s.event.Context, s.key, &out)
	return out, ok, err
}

// LoadOr 读取状态；状态不存在时返回 fallback。
func (s State[T]) LoadOr(fallback T) (T, error) {
	value, ok, err := s.Load()
	if err != nil {
		return fallback, err
	}
	if !ok {
		return fallback, nil
	}
	return value, nil
}

// Save 写入状态。ttl 大于 0 时，状态会在到期后失效。
func (s State[T]) Save(value T, ttl time.Duration) error {
	if s.err != nil {
		return s.err
	}
	if s.session == nil {
		return ErrStoreUnavailable
	}
	if s.event == nil {
		return ErrEventContextUnavailable
	}
	return s.session.SaveJSON(s.event.Context, s.key, value, ttl)
}

// Delete 删除状态。
func (s State[T]) Delete() error {
	if s.err != nil {
		return s.err
	}
	if s.session == nil {
		return ErrStoreUnavailable
	}
	if s.event == nil {
		return ErrEventContextUnavailable
	}
	return s.session.Delete(s.event.Context, s.key)
}
