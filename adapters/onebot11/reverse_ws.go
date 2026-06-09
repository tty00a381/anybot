package onebot11

import (
	"context"
	"errors"
	"net/http"

	"github.com/coder/websocket"
	"github.com/tty00a381/anybot/core"
)

type reverseWSServer struct {
	addr string
	opts options
	peer *socketPeer
}

func newReverseWSServer(addr string, opts options) *reverseWSServer {
	return &reverseWSServer{
		addr: addr,
		opts: opts,
		peer: newSocketPeer(opts.logger),
	}
}

func (s *reverseWSServer) Start(ctx context.Context, sink func(context.Context, *Event) error) error {
	if s.addr == "" {
		return errors.New("onebot11: reverse websocket addr is required")
	}
	if err := s.opts.checkPublicListener("reverse_ws", s.addr); err != nil {
		return err
	}
	server := newHTTPServer(s.addr, s.handler(sink))
	s.opts.emitAdapterState(ctx, core.AdapterState{
		Protocol:  Protocol,
		Kind:      core.AdapterStateDisconnected,
		Transport: "reverse_ws",
		Reason:    "waiting for reverse websocket connection",
	})
	errc := make(chan error, 1)
	go func() {
		s.opts.logger.Info("反向WS监听中", "addr", s.addr, "path", s.opts.path)
		err := server.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		errc <- err
	}()
	select {
	case <-ctx.Done():
		s.closeCurrentConn()
		shutdownHTTPServer(server)
		err := <-errc
		if err != nil {
			return err
		}
		return ctx.Err()
	case err := <-errc:
		return err
	}
}

func (s *reverseWSServer) handler(sink func(context.Context, *Event) error) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(s.opts.path, func(w http.ResponseWriter, r *http.Request) {
		if !s.authorized(r) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			logFrameError(s.opts.logger, err)
			return
		}
		conn.SetReadLimit(s.opts.maxEventBytes)
		s.handleConn(r.Context(), conn, r.RemoteAddr, sink)
	})
	return mux
}

func (s *reverseWSServer) CallRaw(ctx context.Context, action string, params any) (*Response, error) {
	return s.peer.callRaw(ctx, action, params)
}

func (s *reverseWSServer) handleConn(ctx context.Context, conn *websocket.Conn, remote string, sink func(context.Context, *Event) error) {
	if old := s.peer.replaceConn(conn); old != nil && old != conn {
		_ = old.Close(websocket.StatusNormalClosure, "anybot connection replaced")
	}
	info := ConnectionEvent{Transport: "reverse_ws", Addr: s.addr, Path: s.opts.path, Remote: remote}
	info.State = ConnectionConnected
	s.opts.emitConnection(ctx, info)
	s.opts.logger.Info("反向WS已连接")
	defer func() {
		info.State = ConnectionDisconnected
		s.opts.emitConnection(ctx, info)
		s.peer.disconnectConn(conn, actionUnavailable("onebot11: reverse websocket disconnected"))
		_ = conn.Close(websocket.StatusNormalClosure, "anybot disconnect")
	}()

	for {
		messageType, data, err := conn.Read(ctx)
		if err != nil {
			if errors.Is(err, websocket.ErrMessageTooBig) {
				logFrameError(s.opts.logger, err)
			}
			return
		}
		if messageType != websocket.MessageText && messageType != websocket.MessageBinary {
			continue
		}
		if err := s.opts.checkFrameSize(len(data)); err != nil {
			logFrameError(s.opts.logger, err)
			continue
		}
		if err := s.peer.handleFrame(ctx, data, sink); err != nil {
			logFrameError(s.opts.logger, err)
		}
	}
}

func (s *reverseWSServer) closeCurrentConn() {
	conn := s.peer.currentConn()
	if conn != nil {
		_ = conn.CloseNow()
	}
}

func (s *reverseWSServer) authorized(r *http.Request) bool {
	if s.opts.accessToken == "" {
		return true
	}
	if r.Header.Get("Authorization") == "Bearer "+s.opts.accessToken {
		return true
	}
	return r.URL.Query().Get("access_token") == s.opts.accessToken
}
