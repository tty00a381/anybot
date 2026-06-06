package onebot11

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/tty00a381/anybot/core"
)

func TestWebSocketClientEventAndCall(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "done")
		err = conn.Write(r.Context(), websocket.MessageText, []byte(`{
			"post_type":"message",
			"message_type":"private",
			"user_id":42,
			"message":"ping"
		}`))
		if err != nil {
			t.Error(err)
			return
		}
		_, data, err := conn.Read(r.Context())
		if err != nil {
			t.Error(err)
			return
		}
		var req struct {
			Action string          `json:"action"`
			Echo   string          `json:"echo"`
			Params json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal(data, &req); err != nil {
			t.Error(err)
			return
		}
		if req.Action != "get_login_info" {
			t.Errorf("action = %q", req.Action)
		}
		resp := `{"status":"ok","retcode":0,"data":{"user_id":10000,"nickname":"bot"},"echo":"` + req.Echo + `"}`
		_ = conn.Write(r.Context(), websocket.MessageText, []byte(resp))
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	states := make(chan ConnectionEvent, 4)
	transport := newWebSocketClient("ws"+strings.TrimPrefix(server.URL, "http"), newOptions([]Option{
		WithConnectionHook(func(_ context.Context, event ConnectionEvent) {
			states <- event
		}),
	}))
	events := make(chan *Event, 1)
	errc := make(chan error, 1)
	go func() {
		errc <- transport.Start(ctx, func(_ context.Context, event *Event) error {
			events <- event
			return nil
		})
	}()
	select {
	case event := <-events:
		if event.UserID != 42 || event.Message.Text() != "ping" {
			t.Fatalf("event = %#v", event)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event")
	}
	select {
	case event := <-states:
		if event.State != ConnectionConnected || event.Transport != "websocket" {
			t.Fatalf("connection event = %#v", event)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for connection event")
	}
	resp, err := transport.CallRaw(ctx, "get_login_info", nil)
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		UserID int64  `json:"user_id"`
		Nick   string `json:"nickname"`
	}
	if err := resp.Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.UserID != 10000 {
		t.Fatalf("out = %#v", out)
	}
	cancel()
	select {
	case <-errc:
	case <-time.After(2 * time.Second):
		t.Fatal("transport did not stop")
	}
	select {
	case event := <-states:
		if event.State != ConnectionDisconnected || event.Transport != "websocket" {
			t.Fatalf("connection event = %#v", event)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for disconnection event")
	}
}

func TestWebSocketClientReportsDisconnectedOnDialFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "not a websocket", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	states := make(chan core.AdapterState, 2)
	transport := newWebSocketClient("ws"+strings.TrimPrefix(server.URL, "http"), newOptions([]Option{
		WithReconnectInterval(time.Hour),
	}))
	transport.opts.state.SetSink(func(_ context.Context, state core.AdapterState) {
		states <- state
	})

	ctx, cancel := context.WithCancel(context.Background())
	errc := make(chan error, 1)
	go func() {
		errc <- transport.Start(ctx, func(context.Context, *Event) error {
			return nil
		})
	}()
	defer func() {
		cancel()
		select {
		case err := <-errc:
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("start err = %v", err)
			}
		case <-time.After(time.Second):
			t.Fatal("websocket client did not stop")
		}
	}()

	select {
	case state := <-states:
		if state.Kind != core.AdapterStateDisconnected || state.ActionReady || state.Transport != "websocket" {
			t.Fatalf("state = %#v", state)
		}
		if state.Err == nil || !strings.Contains(state.Reason, "dial failed") {
			t.Fatalf("state = %#v", state)
		}
	case <-time.After(time.Second):
		t.Fatal("dial failure state was not reported")
	}
}

func TestSocketPeerAcceptsStringRetCodeProbe(t *testing.T) {
	peer := newSocketPeer(nil)
	ch := make(chan pendingResult, 1)
	peer.pending["echo-1"] = ch

	called := false
	err := peer.handleFrame(context.Background(), []byte(`{"retcode":"0","data":{"ok":true},"echo":"echo-1"}`), func(context.Context, *Event) error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("动作响应不应被当作事件投递")
	}
	result := <-ch
	if result.err != nil {
		t.Fatal(result.err)
	}
	if result.response == nil || result.response.Echo != "echo-1" || result.response.RetCode != 0 {
		t.Fatalf("response=%#v", result.response)
	}
}

func TestSocketPeerUnavailableError(t *testing.T) {
	peer := newSocketPeer(nil)
	_, err := peer.callRaw(context.Background(), "get_login_info", nil)
	if !errors.Is(err, core.ErrActionUnavailable) {
		t.Fatalf("err = %v", err)
	}
}

func TestSocketPeerAcceptsHeartbeatStatusObject(t *testing.T) {
	peer := newSocketPeer(nil)
	events := make(chan *Event, 1)
	err := peer.handleFrame(context.Background(), []byte(`{
		"time":1,
		"self_id":10000,
		"post_type":"meta_event",
		"meta_event_type":"heartbeat",
		"status":{"online":true,"good":true},
		"interval":30000
	}`), func(_ context.Context, event *Event) error {
		events <- event
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case event := <-events:
		if event.PostType != "meta_event" || event.MetaEventType != "heartbeat" {
			t.Fatalf("event=%#v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("heartbeat event was not emitted")
	}
}
