package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/coder/websocket"
	"github.com/tty00a381/anybot/adapters/onebot11"
)

func TestRunInitAndDoctor(t *testing.T) {
	dir := t.TempDir()
	if err := run([]string{"init", "-dir", dir, "-module", "example.com/demo"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "main.go")); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(dir, "anybot.yaml")
	data := []byte(`protocol: onebot11
transport:
  type: reverse_ws
  listen: "127.0.0.1:0"
`)
	if err := os.WriteFile(config, data, 0o644); err != nil {
		t.Fatal(err)
	}
	out, errOut, restore := captureCLIOutput(t)
	defer restore()
	if err := run([]string{"doctor", "-config", config}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "传输：reverse_ws") || !strings.Contains(out.String(), "配置可用：") {
		t.Fatalf("doctor 输出不符合预期:\n%s", out.String())
	}
	if errOut.Len() != 0 {
		t.Fatalf("doctor 不应输出警告:\n%s", errOut.String())
	}

	wsConfig := filepath.Join(dir, "websocket.yaml")
	wsData := []byte(`protocol: onebot11
transport:
  type: websocket
  url: "ws://127.0.0.1:6700/"
`)
	if err := os.WriteFile(wsConfig, wsData, 0o644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	errOut.Reset()
	if err := run([]string{"doctor", "-config", wsConfig}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "URL：ws://127.0.0.1:6700/") {
		t.Fatalf("websocket doctor 输出不符合预期:\n%s", out.String())
	}

	badPath := filepath.Join(dir, "bad-path.yaml")
	badPathData := []byte(`protocol: onebot11
transport:
  type: reverse_ws
  listen: "127.0.0.1:0"
  path: "onebot"
`)
	if err := os.WriteFile(badPath, badPathData, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"doctor", "-config", badPath}); err == nil {
		t.Fatal("doctor should reject path without leading slash")
	}
}

func TestRunNewPlugin(t *testing.T) {
	dir := t.TempDir()
	if err := run([]string{"new", "plugin", "hello-world", "-dir", dir}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "plugins", "hello_world", "hello_world.go")); err != nil {
		t.Fatal(err)
	}
}

func TestRunVersion(t *testing.T) {
	old := version
	version = "v1.2.3"
	defer func() {
		version = old
	}()
	out, _, restore := captureCLIOutput(t)
	defer restore()
	if err := run([]string{"version"}); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "anybot v1.2.3\n" {
		t.Fatalf("version 输出 = %q", got)
	}
}

func TestRunHelpOutput(t *testing.T) {
	out, _, restore := captureCLIOutput(t)
	defer restore()
	if err := run([]string{"help"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "anybot init") || !strings.Contains(out.String(), "anybot doctor") {
		t.Fatalf("help 输出不符合预期:\n%s", out.String())
	}
}

func TestRunDoctorPrintsWarnings(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "anybot.yaml")
	data := []byte(`protocol: onebot11
transport:
  type: reverse_ws
  listen: "0.0.0.0:0"
  access_token_env: EMPTY_TOKEN
`)
	if err := os.WriteFile(config, data, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EMPTY_TOKEN", "")
	_, errOut, restore := captureCLIOutput(t)
	defer restore()
	if err := run([]string{"doctor", "-config", config}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(errOut.String(), "环境变量 EMPTY_TOKEN 未设置") {
		t.Fatalf("doctor 警告不符合预期:\n%s", errOut.String())
	}
}

func TestRunDoctorConnectsHTTPAction(t *testing.T) {
	called := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/get_version_info" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("method = %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("authorization = %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("X-Test") != "yes" {
			t.Errorf("X-Test = %q", r.Header.Get("X-Test"))
		}
		called <- struct{}{}
		_, _ = w.Write([]byte(`{"status":"ok","retcode":0,"data":{"app_name":"NapCat"}}`))
	}))
	defer server.Close()

	dir := t.TempDir()
	config := filepath.Join(dir, "anybot.yaml")
	data := []byte(`protocol: onebot11
transport:
  type: http
  url: "` + server.URL + `"
  access_token_env: TOKEN
  headers:
    X-Test: yes
`)
	if err := os.WriteFile(config, data, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TOKEN", "secret")
	if err := run([]string{"doctor", "-config", config, "-connect"}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-called:
	default:
		t.Fatal("doctor did not call HTTP action")
	}
}

func TestRunDoctorConnectsWebSocket(t *testing.T) {
	called := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("authorization = %q", r.Header.Get("Authorization"))
		}
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		called <- struct{}{}
		_ = conn.Close(websocket.StatusNormalClosure, "done")
	}))
	defer server.Close()

	dir := t.TempDir()
	config := filepath.Join(dir, "anybot.yaml")
	data := []byte(`protocol: onebot11
transport:
  type: websocket
  url: "ws` + strings.TrimPrefix(server.URL, "http") + `"
  access_token: secret
`)
	if err := os.WriteFile(config, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"doctor", "-config", config, "-connect"}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-called:
	default:
		t.Fatal("doctor did not open websocket")
	}
}

func TestDoctorWarnings(t *testing.T) {
	t.Setenv("EMPTY_TOKEN", "")
	cfg := onebot11.Config{Transport: onebot11.TransportConfig{
		Type:           "reverse_ws",
		Listen:         "0.0.0.0:6700",
		AccessTokenEnv: "EMPTY_TOKEN",
	}}
	warnings := doctorWarnings(cfg)
	want := []string{
		"环境变量 EMPTY_TOKEN 未设置",
		"反向 WebSocket 监听非本机地址且未配置可用访问令牌",
	}
	if !reflect.DeepEqual(warnings, want) {
		t.Fatalf("warnings=%#v want %#v", warnings, want)
	}

	t.Setenv("TOKEN", "secret")
	cfg.Transport.Listen = "127.0.0.1:6700"
	cfg.Transport.AccessTokenEnv = "TOKEN"
	if warnings := doctorWarnings(cfg); len(warnings) != 0 {
		t.Fatalf("warnings=%#v", warnings)
	}
}

func captureCLIOutput(t *testing.T) (*bytes.Buffer, *bytes.Buffer, func()) {
	t.Helper()
	oldStdout := stdout
	oldStderr := stderr
	var out bytes.Buffer
	var errOut bytes.Buffer
	stdout = &out
	stderr = &errOut
	return &out, &errOut, func() {
		stdout = oldStdout
		stderr = oldStderr
	}
}
