package onebot11

import (
	"context"

	"github.com/tty00a381/anybot"
)

// Transport 定义 OneBot v11 事件接收与动作调用所需的底层传输能力。
type Transport interface {
	Start(context.Context, func(context.Context, *Event) error) error
	CallRaw(context.Context, string, any) (*Response, error)
}

// Adapter 将 OneBot v11 传输接入 AnyBot 运行时。
type Adapter struct {
	transport Transport
	client    *Client
}

// New 基于自定义传输创建适配器，适合扩展或测试场景。
func New(transport Transport) *Adapter {
	return &Adapter{
		transport: transport,
		client:    &Client{transport: transport},
	}
}

func newAdapter(transport Transport, opts options) *Adapter {
	return &Adapter{
		transport: transport,
		client:    &Client{transport: transport, actionTimeout: opts.actionTimeout},
	}
}

// Protocol 返回适配器所属协议。
func (a *Adapter) Protocol() anybot.Protocol {
	return anybot.ProtocolOneBot11
}

// Start 启动底层传输，并把 OneBot v11 事件标准化后投递给运行时。
func (a *Adapter) Start(ctx context.Context, emit anybot.EmitFunc) error {
	return a.transport.Start(ctx, func(eventCtx context.Context, event *Event) error {
		return emit(eventCtx, event.Normalize())
	})
}

// Client 返回 OneBot v11 类型化动作客户端。
func (a *Adapter) Client() anybot.ActionClient {
	return a.client
}

// WebSocket 创建正向 WebSocket 适配器，由 AnyBot 主动连接协议端。
func WebSocket(url string, opts ...Option) *Adapter {
	cfg := newOptions(opts)
	return newAdapter(newWebSocketClient(url, cfg), cfg)
}

// ReverseWS 创建反向 WebSocket 服务端适配器，由 NapCat 主动连接 AnyBot。
func ReverseWS(addr string, opts ...Option) *Adapter {
	cfg := newOptions(opts)
	return newAdapter(newReverseWSServer(addr, cfg), cfg)
}

// HTTP 创建 HTTP API 与事件回调适配器；listenAddr 为空时仅启用动作调用。
func HTTP(apiURL, listenAddr string, opts ...Option) *Adapter {
	cfg := newOptions(opts)
	return newAdapter(newHTTPTransport(apiURL, listenAddr, cfg), cfg)
}
