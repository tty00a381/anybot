package core

import (
	"context"
	"log/slog"
	"runtime/debug"
	"slices"
	"sync"
	"time"
)

// Recover 捕获处理链中的 panic，记录调用栈，并把 panic 转成 PanicError。
func Recover(loggers ...*slog.Logger) Middleware {
	return func(next Handler) Handler {
		return func(c *Context) (err error) {
			defer func() {
				if recovered := recover(); recovered != nil {
					logger := c.Logger()
					if len(loggers) > 0 && loggers[0] != nil {
						logger = loggers[0]
					}
					logger.Error("处理panic", "panic", recovered, "stack", string(debug.Stack()))
					c.Stop()
					err = &PanicError{Value: recovered, Stack: debug.Stack()}
				}
			}()
			return next(c)
		}
	}
}

// Trace 在每次路由处理结束后记录耗时、路由名称和事件关键信息。
func Trace(loggers ...*slog.Logger) Middleware {
	return func(next Handler) Handler {
		return func(c *Context) error {
			start := time.Now()
			err := next(c)
			logger := c.Logger()
			if len(loggers) > 0 && loggers[0] != nil {
				logger = loggers[0]
			}
			event := c.Event()
			if event == nil {
				fields := []any{"duration", time.Since(start)}
				if err != nil {
					fields = append(fields, "error", err)
				}
				logger.Info("路由完成", fields...)
				return err
			}
			fields := []any{
				"protocol", event.Protocol,
				"type", event.Type,
				"detail_type", event.DetailType,
				"user_id", event.UserID,
				"group_id", event.GroupID,
				"group_role", event.GroupRole,
				"route", routeName(c.Route()),
				"duration", time.Since(start),
			}
			if err != nil {
				fields = append(fields, "error", err)
			}
			logger.Info("路由完成", fields...)
			return err
		}
	}
}

// Timeout 为后续处理函数派生带超时的 context；timeout 不大于 0 时不生效。
func Timeout(timeout time.Duration) Middleware {
	return func(next Handler) Handler {
		return func(c *Context) error {
			if timeout <= 0 {
				return next(c)
			}
			ctx, cancel := context.WithTimeout(c.Context, timeout)
			defer cancel()
			original := c.Context
			c.Context = ctx
			defer func() {
				c.Context = original
			}()
			return next(c)
		}
	}
}

// OnlyPrivate 仅允许私聊消息进入后续处理链，其他事件会停止传播。
func OnlyPrivate() Middleware {
	return func(next Handler) Handler {
		return func(c *Context) error {
			if !c.IsPrivate() {
				c.Stop()
				return nil
			}
			return next(c)
		}
	}
}

// OnlyGroup 仅允许群消息进入后续处理链，其他事件会停止传播。
func OnlyGroup() Middleware {
	return func(next Handler) Handler {
		return func(c *Context) error {
			if !c.IsGroup() {
				c.Stop()
				return nil
			}
			return next(c)
		}
	}
}

// SuperUser 仅允许指定用户进入后续处理链，未授权时返回 ErrUnauthorized。
func SuperUser(ids ...any) Middleware {
	allowed := stringSet(ids...)
	return func(next Handler) Handler {
		return func(c *Context) error {
			if !slices.Contains(allowed, c.UserID()) {
				c.Stop()
				return ErrUnauthorized
			}
			return next(c)
		}
	}
}

// RequireSuperUser 仅允许 App 级超级用户进入后续处理链，未授权时返回 ErrUnauthorized。
func RequireSuperUser() Middleware {
	return func(next Handler) Handler {
		return func(c *Context) error {
			if !c.IsSuperUser() {
				c.Stop()
				return ErrUnauthorized
			}
			return next(c)
		}
	}
}

// RateLimit 使用当前会话键做简单的进程内限速。
func RateLimit(limit int, window time.Duration) Middleware {
	return RateLimitBy(limit, window, nil)
}

// RateLimitBy 使用自定义键做进程内限速；键为空时回退到会话、用户或全局维度。
func RateLimitBy(limit int, window time.Duration, keyFunc func(*Context) string) Middleware {
	type bucket struct {
		start time.Time
		count int
	}
	var mu sync.Mutex
	buckets := map[string]bucket{}
	return func(next Handler) Handler {
		return func(c *Context) error {
			if limit <= 0 || window <= 0 {
				return next(c)
			}
			now := time.Now()
			key := rateLimitKey(c, keyFunc)
			mu.Lock()
			item := buckets[key]
			if item.start.IsZero() || now.Sub(item.start) >= window {
				item = bucket{start: now}
			}
			item.count++
			buckets[key] = item
			mu.Unlock()
			if item.count > limit {
				c.Stop()
				return ErrRateLimited
			}
			return next(c)
		}
	}
}

func rateLimitKey(c *Context, keyFunc func(*Context) string) string {
	if keyFunc != nil {
		if key := keyFunc(c); key != "" {
			return key
		}
	}
	if key := c.ConversationID(); key != "" {
		return key
	}
	if key := c.UserID(); key != "" {
		return key
	}
	return "global"
}

func routeName(route *Route) string {
	if route == nil {
		return ""
	}
	return route.name
}
