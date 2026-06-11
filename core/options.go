package core

import (
	"log/slog"
	"os"
	"runtime"
)

// ErrorHandler 处理路由处理函数或中间件返回的错误。
type ErrorHandler func(*Context, error)

// EventKeyFunc 返回事件串行调度使用的键；空键表示不参与串行化。
type EventKeyFunc func(*Event) string

// Option 调整 App 的运行时配置。
type Option func(*App)

// WithAdapter 配置运行时使用的协议适配器。
func WithAdapter(adapter Adapter) Option {
	return func(app *App) {
		app.adapter = adapter
	}
}

// WithLogger 配置运行时日志器；nil 会被忽略。
func WithLogger(logger *slog.Logger) Option {
	return func(app *App) {
		if logger != nil {
			app.logger = logger
		}
	}
}

// WithStore 配置会话存储；nil 会被忽略。
func WithStore(store Store) Option {
	return func(app *App) {
		if store != nil {
			app.store = store
		}
	}

}

// WithSuperUsers 配置 App 级超级用户 ID，供 RequireSuperUser 和 Context.IsSuperUser 使用。
func WithSuperUsers(ids ...string) Option {
	return func(app *App) {
		app.superUsers = append([]string(nil), ids...)
	}
}

// WithWorkers 配置并发处理事件的 worker 数；不大于 0 时由适配器回调同步处理。
func WithWorkers(workers int) Option {
	return func(app *App) {
		app.workers = workers
	}
}

// WithBuffer 配置事件队列大小；不大于 0 的值会被忽略。
func WithBuffer(size int) Option {
	return func(app *App) {
		if size > 0 {
			app.buffer = size
		}
	}
}

// WithObserverBuffer 配置异步事件观察者的并发队列容量；不大于 0 的值会被忽略。
func WithObserverBuffer(size int) Option {
	return func(app *App) {
		if size > 0 {
			app.observerSlots = make(chan struct{}, size)
		}
	}
}

// WithErrorHandler 注册错误处理器。
func WithErrorHandler(handler ErrorHandler) Option {
	return func(app *App) {
		app.OnError(handler)
	}
}

// WithSerialBy 按自定义事件键串行处理同键事件，适合保护会话状态流转。
func WithSerialBy(fn EventKeyFunc) Option {
	return func(app *App) {
		app.serial = fn
		if app.locks == nil {
			app.locks = newKeyedLocks()
		}
	}
}

// WithSerialByConversation 让同一会话内的事件按顺序处理。
func WithSerialByConversation() Option {
	return WithSerialBy(func(event *Event) string {
		if event == nil {
			return ""
		}
		return event.ConversationID()
	})
}

func defaultLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
}

func defaultWorkers() int {
	return max(4, runtime.GOMAXPROCS(0))
}

func defaultObserverBuffer() int {
	return 1024
}
