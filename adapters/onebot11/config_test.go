package onebot11

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadAdapterFromConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "core.yaml")
	t.Setenv("TOKEN_FROM_ENV", "secret")
	data := []byte(`protocol: onebot11
transport:
  type: reverse_ws
  listen: "127.0.0.1:6700"
  path: "/onebot"
  access_token_env: TOKEN_FROM_ENV
  max_event_bytes: 2048
  action_timeout: 2s
`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	adapter, err := LoadAdapter(path)
	if err != nil {
		t.Fatal(err)
	}
	client := adapter.Client().(*Client)
	if client.actionTimeout != 2*time.Second {
		t.Fatalf("action timeout = %s", client.actionTimeout)
	}
	server := adapter.transport.(*reverseWSServer)
	if server.opts.path != "/onebot" || server.opts.accessToken != "secret" || server.opts.maxEventBytes != 2048 {
		t.Fatalf("options = %#v", server.opts)
	}
}

func TestLoadConfigRejectsYMLExtension(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "core.yml")
	if err := os.WriteFile(path, []byte("protocol: onebot11\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadConfig(path)
	if err == nil || !strings.Contains(err.Error(), "必须使用 .yaml 扩展名") {
		t.Fatalf("err = %v", err)
	}
}

func TestConfigValidation(t *testing.T) {
	cfg := Config{Protocol: "onebot12", Transport: TransportConfig{Type: "reverse_ws", Listen: "127.0.0.1:6700"}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("protocol should be rejected")
	}
	cfg = Config{Protocol: "onebot11", Transport: TransportConfig{Type: "websocket"}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("missing url should be rejected")
	}
	cfg = Config{Protocol: "onebot11", Transport: TransportConfig{Type: "reverse_ws", Listen: "127.0.0.1:6700", Path: "onebot"}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("bad path should be rejected")
	}
	cfg = Config{Protocol: "onebot11", Transport: TransportConfig{Type: "websocket", URL: "http://127.0.0.1:6700"}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("websocket should reject http url")
	}
	cfg = Config{Protocol: "onebot11", Transport: TransportConfig{Type: "http", URL: "ws://127.0.0.1:6700"}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("http should reject ws url")
	}
}
