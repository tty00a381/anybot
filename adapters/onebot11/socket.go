package onebot11

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/coder/websocket"
)

type socketPeer struct {
	logger *slog.Logger

	connMu  sync.RWMutex
	conn    *websocket.Conn
	writeMu sync.Mutex

	pendingMu sync.Mutex
	pending   map[string]chan pendingResult
	nextEcho  atomic.Uint64
}

type pendingResult struct {
	response *Response
	err      error
}

func newSocketPeer(logger *slog.Logger) *socketPeer {
	return &socketPeer{
		logger:  logger,
		pending: map[string]chan pendingResult{},
	}
}

func (p *socketPeer) callRaw(ctx context.Context, action string, params any) (*Response, error) {
	if action == "" {
		return nil, errors.New("onebot11: action is required")
	}
	echo := fmt.Sprintf("anybot-%d", p.nextEcho.Add(1))
	ch := make(chan pendingResult, 1)

	p.pendingMu.Lock()
	p.pending[echo] = ch
	p.pendingMu.Unlock()

	payload, err := json.Marshal(map[string]any{
		"action": action,
		"params": params,
		"echo":   echo,
	})
	if err != nil {
		p.removePending(echo)
		return nil, err
	}

	conn := p.currentConn()
	if conn == nil {
		p.removePending(echo)
		return nil, errors.New("onebot11: websocket is not connected")
	}

	p.writeMu.Lock()
	err = conn.Write(ctx, websocket.MessageText, payload)
	p.writeMu.Unlock()
	if err != nil {
		p.removePending(echo)
		return nil, err
	}

	select {
	case result := <-ch:
		return result.response, result.err
	case <-ctx.Done():
		p.removePending(echo)
		return nil, ctx.Err()
	}
}

func (p *socketPeer) handleFrame(ctx context.Context, data []byte, sink func(context.Context, *Event) error) error {
	var probe struct {
		Echo    string          `json:"echo"`
		Status  json.RawMessage `json:"status"`
		RetCode *flexibleInt64  `json:"retcode"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return err
	}
	if probe.Echo != "" && (len(probe.Status) > 0 || probe.RetCode != nil) {
		var response Response
		if err := json.Unmarshal(data, &response); err != nil {
			return err
		}
		response.Raw = append(response.Raw[:0], data...)
		p.completePending(probe.Echo, pendingResult{response: &response})
		return nil
	}

	var event Event
	if err := json.Unmarshal(data, &event); err != nil {
		return err
	}
	return sink(ctx, &event)
}

func (p *socketPeer) currentConn() *websocket.Conn {
	p.connMu.RLock()
	defer p.connMu.RUnlock()
	return p.conn
}

func (p *socketPeer) setConn(conn *websocket.Conn) {
	p.connMu.Lock()
	p.conn = conn
	p.connMu.Unlock()
}

func (p *socketPeer) replaceConn(conn *websocket.Conn) *websocket.Conn {
	p.connMu.Lock()
	old := p.conn
	p.conn = conn
	p.connMu.Unlock()
	if old != nil && old != conn {
		p.failPending(errors.New("onebot11: websocket connection replaced"))
	}
	return old
}

func (p *socketPeer) removePending(echo string) {
	p.pendingMu.Lock()
	delete(p.pending, echo)
	p.pendingMu.Unlock()
}

func (p *socketPeer) completePending(echo string, result pendingResult) {
	p.pendingMu.Lock()
	ch, ok := p.pending[echo]
	if ok {
		delete(p.pending, echo)
	}
	p.pendingMu.Unlock()
	if ok {
		ch <- result
		close(ch)
	}
}

func (p *socketPeer) failPending(err error) {
	p.pendingMu.Lock()
	pending := p.pending
	p.pending = map[string]chan pendingResult{}
	p.pendingMu.Unlock()
	for _, ch := range pending {
		ch <- pendingResult{err: err}
		close(ch)
	}
}

func authHeaders(opts options) http.Header {
	header := make(http.Header, len(opts.header)+1)
	for key, values := range opts.header {
		header[key] = append([]string(nil), values...)
	}
	if opts.accessToken != "" {
		header["Authorization"] = []string{"Bearer " + opts.accessToken}
	}
	return header
}
