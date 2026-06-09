package onebot11

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/coder/websocket"
	"github.com/tty00a381/anybot/core"
)

type webSocketClient struct {
	url  string
	opts options
	peer *socketPeer
}

func newWebSocketClient(url string, opts options) *webSocketClient {
	return &webSocketClient{
		url:  url,
		opts: opts,
		peer: newSocketPeer(opts.logger),
	}
}

func (c *webSocketClient) Start(ctx context.Context, sink func(context.Context, *Event) error) error {
	if c.url == "" {
		return errors.New("onebot11: websocket url is required")
	}
	backoff := c.opts.reconnectInterval
	for {
		err := c.connectOnce(ctx, sink)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if backoff <= 0 {
			backoff = time.Second
		}
		c.opts.logger.Warn("正向WS已断开", "error", err, "retry_in", backoff)
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
		backoff *= 2
		if c.opts.reconnectMax > 0 && backoff > c.opts.reconnectMax {
			backoff = c.opts.reconnectMax
		}
	}
}

func (c *webSocketClient) CallRaw(ctx context.Context, action string, params any) (*Response, error) {
	return c.peer.callRaw(ctx, action, params)
}

func (c *webSocketClient) connectOnce(ctx context.Context, sink func(context.Context, *Event) error) error {
	dialCtx, cancel := context.WithTimeout(ctx, c.opts.dialTimeout)
	defer cancel()

	conn, _, err := websocket.Dial(dialCtx, c.url, &websocket.DialOptions{
		HTTPHeader: authHeaders(c.opts),
	})
	if err != nil {
		if ctx.Err() == nil {
			c.opts.emitAdapterState(ctx, core.AdapterState{
				Protocol:  Protocol,
				Kind:      core.AdapterStateDisconnected,
				Transport: "websocket",
				Reason:    "websocket dial failed",
				Err:       err,
			})
		}
		return err
	}
	conn.SetReadLimit(c.opts.maxEventBytes)

	c.peer.setConn(conn)
	info := ConnectionEvent{State: ConnectionConnected, Transport: "websocket", URL: c.url}
	c.opts.emitConnection(ctx, info)
	c.opts.logger.Info("正向WS已连接", "url", c.url)
	defer func() {
		info.State = ConnectionDisconnected
		c.opts.emitConnection(ctx, info)
		c.peer.setConn(nil)
		_ = conn.Close(websocket.StatusNormalClosure, "anybot reconnect")
		c.peer.failPending(actionUnavailable("onebot11: websocket disconnected"))
	}()

	for {
		messageType, data, err := conn.Read(ctx)
		if err != nil {
			if errors.Is(err, websocket.ErrMessageTooBig) {
				logFrameError(c.opts.logger, err)
			}
			return err
		}
		if messageType != websocket.MessageText && messageType != websocket.MessageBinary {
			continue
		}
		if err := c.opts.checkFrameSize(len(data)); err != nil {
			logFrameError(c.opts.logger, err)
			continue
		}
		if err := c.peer.handleFrame(ctx, data, sink); err != nil {
			logFrameError(c.opts.logger, err)
		}
	}
}

func logFrameError(logger *slog.Logger, err error) {
	if logger != nil {
		logger.Warn("WS帧已忽略", "error", err)
	}
}
