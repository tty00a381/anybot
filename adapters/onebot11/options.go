package onebot11

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"
)

type options struct {
	accessToken       string
	path              string
	header            http.Header
	logger            *slog.Logger
	connectionHooks   []ConnectionHook
	dialTimeout       time.Duration
	actionTimeout     time.Duration
	reconnectInterval time.Duration
	reconnectMax      time.Duration
	state             *adapterState
}

// ConnectionState 表示 WebSocket 传输的连接状态。
type ConnectionState string

const (
	// ConnectionConnected 表示 WebSocket 已建立连接。
	ConnectionConnected ConnectionState = "connected"
	// ConnectionDisconnected 表示 WebSocket 已断开连接。
	ConnectionDisconnected ConnectionState = "disconnected"
)

// ConnectionEvent 描述一次 WebSocket 连接状态变化及其传输端信息。
type ConnectionEvent struct {
	State     ConnectionState
	Transport string
	URL       string
	Addr      string
	Path      string
	Remote    string
}

// ConnectionHook 接收 WebSocket 连接状态变化通知。
type ConnectionHook func(context.Context, ConnectionEvent)

// Option 调整 OneBot v11 传输配置。
type Option func(*options)

// WithAccessToken 配置访问令牌，用于 Bearer token 鉴权和反向连接校验。
func WithAccessToken(token string) Option {
	return func(opts *options) {
		opts.accessToken = token
	}
}

// WithPath 配置 HTTP 事件回调或反向 WebSocket 的请求路径。
func WithPath(path string) Option {
	return func(opts *options) {
		if path != "" {
			opts.path = path
		}
	}
}

// WithHeader 添加出站连接使用的 HTTP 头。
func WithHeader(key, value string) Option {
	return func(opts *options) {
		if opts.header == nil {
			opts.header = make(http.Header)
		}
		opts.header.Add(key, value)
	}
}

// WithLogger 配置适配器日志器；nil 会被忽略。
func WithLogger(logger *slog.Logger) Option {
	return func(opts *options) {
		if logger != nil {
			opts.logger = logger
		}
	}
}

// WithConnectionHook 注册 WebSocket 连接状态钩子。
func WithConnectionHook(hook ConnectionHook) Option {
	return func(opts *options) {
		if hook != nil {
			opts.connectionHooks = append(opts.connectionHooks, hook)
		}
	}
}

// WithDialTimeout 配置正向 WebSocket 拨号超时。
func WithDialTimeout(timeout time.Duration) Option {
	return func(opts *options) {
		if timeout > 0 {
			opts.dialTimeout = timeout
		}
	}
}

// WithActionTimeout 配置动作调用默认超时；调用上下文已有 deadline 时不会覆盖。
func WithActionTimeout(timeout time.Duration) Option {
	return func(opts *options) {
		if timeout > 0 {
			opts.actionTimeout = timeout
		}
	}
}

// WithReconnectInterval 配置正向 WebSocket 初始重连间隔。
func WithReconnectInterval(interval time.Duration) Option {
	return func(opts *options) {
		if interval > 0 {
			opts.reconnectInterval = interval
		}
	}
}

// WithReconnectMaxInterval 配置正向 WebSocket 最大重连间隔。
func WithReconnectMaxInterval(interval time.Duration) Option {
	return func(opts *options) {
		if interval > 0 {
			opts.reconnectMax = interval
		}
	}
}

func newOptions(opts []Option) options {
	out := options{
		path:              "/",
		header:            make(http.Header),
		logger:            slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})),
		dialTimeout:       10 * time.Second,
		actionTimeout:     10 * time.Second,
		reconnectInterval: 3 * time.Second,
		reconnectMax:      30 * time.Second,
		state:             newAdapterState(),
	}
	for _, opt := range opts {
		opt(&out)
	}
	return out
}

func (opts options) emitConnection(ctx context.Context, event ConnectionEvent) {
	opts.emitConnectionState(ctx, event)
	for _, hook := range opts.connectionHooks {
		if hook != nil {
			hook(ctx, event)
		}
	}
}
