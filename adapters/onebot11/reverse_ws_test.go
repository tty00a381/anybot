package onebot11

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/tty00a381/anybot/core"
)

func TestReverseWSAuthorization(t *testing.T) {
	server := newReverseWSServer("127.0.0.1:0", newOptions([]Option{WithAccessToken("secret")}))
	req := httptest.NewRequest("GET", "/?access_token=secret", nil)
	if !server.authorized(req) {
		t.Fatal("query token should be accepted")
	}
	req = httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer secret")
	if !server.authorized(req) {
		t.Fatal("bearer token should be accepted")
	}
	req = httptest.NewRequest("GET", "/", nil)
	if server.authorized(req) {
		t.Fatal("missing token should be rejected")
	}
}

func TestReverseWSEventAndCall(t *testing.T) {
	states := make(chan ConnectionEvent, 4)
	server := newReverseWSServer("127.0.0.1:0", newOptions([]Option{
		WithAccessToken("secret"),
		WithConnectionHook(func(_ context.Context, event ConnectionEvent) {
			states <- event
		}),
	}))
	events := make(chan *Event, 1)
	httpServer := httptest.NewServer(server.handler(func(_ context.Context, event *Event) error {
		events <- event
		return nil
	}))
	defer httpServer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(httpServer.URL, "http")+"/?access_token=secret", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "done")

	if err := conn.Write(ctx, websocket.MessageText, []byte(`{"post_type":"message","message_type":"private","self_id":10000,"user_id":42,"message":"ping"}`)); err != nil {
		t.Fatal(err)
	}
	select {
	case event := <-events:
		if event.UserID != 42 || event.Message.Text() != "ping" {
			t.Fatalf("event = %#v", event)
		}
	case <-ctx.Done():
		t.Fatal("等待事件超时")
	}
	select {
	case event := <-states:
		if event.State != ConnectionConnected || event.Transport != "reverse_ws" || event.Remote == "" {
			t.Fatalf("connection event = %#v", event)
		}
	case <-ctx.Done():
		t.Fatal("等待连接状态超时")
	}

	responses := make(chan *Response, 1)
	errs := make(chan error, 1)
	go func() {
		response, err := server.CallRaw(ctx, "get_login_info", nil)
		if err != nil {
			errs <- err
			return
		}
		responses <- response
	}()
	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var req struct {
		Action string `json:"action"`
		Echo   string `json:"echo"`
	}
	if err := json.Unmarshal(data, &req); err != nil {
		t.Fatal(err)
	}
	if req.Action != "get_login_info" || req.Echo == "" {
		t.Fatalf("request = %#v", req)
	}
	resp := `{"status":"ok","retcode":0,"data":{"user_id":10000,"nickname":"bot"},"echo":"` + req.Echo + `"}`
	if err := conn.Write(ctx, websocket.MessageText, []byte(resp)); err != nil {
		t.Fatal(err)
	}
	select {
	case response := <-responses:
		var out struct {
			UserID int64  `json:"user_id"`
			Nick   string `json:"nickname"`
		}
		if err := response.Decode(&out); err != nil {
			t.Fatal(err)
		}
		if out.UserID != 10000 || out.Nick != "bot" {
			t.Fatalf("response = %#v", out)
		}
	case err := <-errs:
		t.Fatal(err)
	case <-ctx.Done():
		t.Fatal("等待动作响应超时")
	}
}

func TestReverseWSReportsAdapterState(t *testing.T) {
	adapter := ReverseWS("127.0.0.1:0")
	states := make(chan core.AdapterState, 4)
	adapter.SetStateSink(func(_ context.Context, state core.AdapterState) {
		states <- state
	})
	server := adapter.transport.(*reverseWSServer)
	httpServer := httptest.NewServer(server.handler(func(context.Context, *Event) error {
		return nil
	}))
	defer httpServer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(httpServer.URL, "http")+"/", nil)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case state := <-states:
		if state.Kind != core.AdapterStateReady || !state.ActionReady || state.Protocol != core.ProtocolOneBot11 {
			t.Fatalf("state = %#v", state)
		}
	case <-ctx.Done():
		t.Fatal("等待 ready state 超时")
	}
	if err := conn.Close(websocket.StatusNormalClosure, "done"); err != nil {
		t.Fatal(err)
	}
	select {
	case state := <-states:
		if state.Kind != core.AdapterStateDisconnected || state.ActionReady {
			t.Fatalf("state = %#v", state)
		}
	case <-ctx.Done():
		t.Fatal("等待 disconnected state 超时")
	}
}

func TestReverseWSReportsWaitingStateOnStart(t *testing.T) {
	adapter := ReverseWS("127.0.0.1:0")
	states := make(chan core.AdapterState, 4)
	adapter.SetStateSink(func(_ context.Context, state core.AdapterState) {
		states <- state
	})

	ctx, cancel := context.WithCancel(context.Background())
	errc := make(chan error, 1)
	go func() {
		errc <- adapter.Start(ctx, func(context.Context, *core.Event) error {
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
			t.Fatal("reverse ws server did not stop")
		}
	}()

	select {
	case state := <-states:
		if state.Kind != core.AdapterStateDisconnected || state.ActionReady || state.Transport != "reverse_ws" {
			t.Fatalf("state = %#v", state)
		}
		if !strings.Contains(state.Reason, "waiting") {
			t.Fatalf("reason = %q", state.Reason)
		}
	case <-time.After(time.Second):
		t.Fatal("waiting state was not reported")
	}
}

func TestReverseWSPendingCallFailsOnDisconnect(t *testing.T) {
	states := make(chan ConnectionEvent, 2)
	server := newReverseWSServer("127.0.0.1:0", newOptions([]Option{
		WithConnectionHook(func(_ context.Context, event ConnectionEvent) {
			states <- event
		}),
	}))
	httpServer := httptest.NewServer(server.handler(func(context.Context, *Event) error {
		return nil
	}))
	defer httpServer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(httpServer.URL, "http")+"/", nil)
	if err != nil {
		t.Fatal(err)
	}

	select {
	case event := <-states:
		if event.State != ConnectionConnected {
			t.Fatalf("connection event = %#v", event)
		}
	case <-ctx.Done():
		t.Fatal("等待连接状态超时")
	}

	errs := make(chan error, 1)
	go func() {
		_, err := server.CallRaw(ctx, "get_login_info", nil)
		errs <- err
	}()
	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var req struct {
		Action string `json:"action"`
		Echo   string `json:"echo"`
	}
	if err := json.Unmarshal(data, &req); err != nil {
		t.Fatal(err)
	}
	if req.Action != "get_login_info" || req.Echo == "" {
		t.Fatalf("request = %#v", req)
	}
	if err := conn.Close(websocket.StatusNormalClosure, "done"); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-errs:
		if err == nil || !strings.Contains(err.Error(), "disconnected") {
			t.Fatalf("err = %v", err)
		}
	case <-ctx.Done():
		t.Fatal("等待 pending action 失败超时")
	}
}
