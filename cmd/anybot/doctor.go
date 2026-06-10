package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/tty00a381/anybot/adapters/onebot11"
	"github.com/tty00a381/anybot/app/host"
)

func runDoctor(args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	configPath := fs.String("config", host.DefaultConfigPath, "配置文件")
	connect := fs.Bool("connect", false, "检查远端动作接口是否可连接")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := host.LoadConfig(*configPath)
	if err != nil {
		return err
	}
	if _, err := host.NewLogger(cfg.Runtime.LogLevel, stderr); err != nil {
		return err
	}
	workDir := host.WorkDirForConfig(*configPath)
	lock, err := host.LoadPluginLock(host.PluginLockPath(workDir))
	if err != nil {
		return err
	}
	if err := host.ValidateConfigWithLock(doctorValidationConfig(cfg, lock), host.EmptyRegistry(), lock); err != nil {
		return withExternalPluginHint(err, workDir)
	}
	adapterCfg := cfgAdapter(cfg)
	if err := checkListen(adapterCfg); err != nil {
		return err
	}
	if err := checkListenerToken(adapterCfg); err != nil {
		return err
	}
	if *connect {
		if err := checkRemote(adapterCfg); err != nil {
			return err
		}
	}
	enabled := doctorEnabledPlugins(cfg, lock)
	abs, _ := filepath.Abs(*configPath)
	printSummary(abs, cfg, enabled)
	printDoctorExternalPluginHint(cfg, lock, workDir)
	fmt.Fprintf(stdout, "配置可用：%s\n", abs)
	return nil
}

func doctorValidationConfig(cfg host.Config, lock host.PluginLock) host.Config {
	if len(cfg.Plugins) == 0 {
		return cfg
	}
	out := cfg
	out.Plugins = make(map[string]host.PluginEntry, len(cfg.Plugins))
	for id, entry := range cfg.Plugins {
		out.Plugins[id] = entry
	}
	disabled := false
	for _, item := range lock.Plugins {
		if item.ID == "" || item.Module == "" {
			continue
		}
		entry, ok := out.Plugins[item.ID]
		if !ok || !cliPluginEntryEnabled(entry) {
			continue
		}
		entry.Enabled = &disabled
		out.Plugins[item.ID] = entry
	}
	return out
}

func doctorEnabledPlugins(cfg host.Config, lock host.PluginLock) []string {
	enabled := make([]string, 0, len(cfg.Plugins))
	for _, item := range lock.Plugins {
		if item.ID == "" {
			continue
		}
		entry, ok := cfg.Plugins[item.ID]
		if ok && cliPluginEntryEnabled(entry) {
			enabled = append(enabled, item.ID)
		}
	}
	return enabled
}

func printDoctorExternalPluginHint(cfg host.Config, lock host.PluginLock, dir string) {
	if !lockHasEnabledExternalPlugins(cfg, lock) {
		return
	}
	fmt.Fprintf(stdout, "外部插件：基础配置已检查；完整插件配置请运行 anybot up -dir %s，或构建后运行 ./anybot-bot plugin check\n", shellQuote(dir))
}

func cfgAdapter(cfg host.Config) onebot11.Config {
	return onebot11.Config{Protocol: cfg.Adapter.Protocol, Transport: cfg.Adapter.Transport}
}

func checkListen(cfg onebot11.Config) error {
	if cfg.Transport.Type != "reverse_ws" && (cfg.Transport.Type != "http" || cfg.Transport.Listen == "") {
		return nil
	}
	ln, err := net.Listen("tcp", cfg.Transport.Listen)
	if err != nil {
		return fmt.Errorf("无法监听 %s: %w", cfg.Transport.Listen, err)
	}
	return ln.Close()
}

func checkListenerToken(cfg onebot11.Config) error {
	if !transportListens(cfg) || listenIsLocal(cfg.Transport.Listen) || accessTokenAvailable(cfg) {
		return nil
	}
	return fmt.Errorf("%s 监听非本机地址时必须配置可用访问令牌", cfg.Transport.Type)
}

func transportListens(cfg onebot11.Config) bool {
	return cfg.Transport.Type == "reverse_ws" || (cfg.Transport.Type == "http" && cfg.Transport.Listen != "")
}

func listenIsLocal(listen string) bool {
	host, _, err := net.SplitHostPort(listen)
	if err != nil {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func accessTokenAvailable(cfg onebot11.Config) bool {
	return cfg.Transport.AccessToken != ""
}

func checkRemote(cfg onebot11.Config) error {
	switch cfg.Transport.Type {
	case "http":
		return checkHTTPAction(cfg)
	case "websocket":
		return checkWebSocketHandshake(cfg)
	default:
		return nil
	}
}

func checkHTTPAction(cfg onebot11.Config) error {
	url := strings.TrimRight(cfg.Transport.URL, "/") + "/get_version_info"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader([]byte(`{}`)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	addHeaders(req.Header, cfg)
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("无法连接动作接口 %s: %w", cfg.Transport.URL, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("动作接口返回 %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var action onebot11.Response
	if err := json.Unmarshal(data, &action); err != nil {
		return fmt.Errorf("动作接口响应不是 OneBot 动作响应: %w", err)
	}
	if !action.OK() {
		detail := action.Message
		if detail == "" {
			detail = action.Wording
		}
		return fmt.Errorf("动作接口失败: status=%s retcode=%d %s", action.Status, action.RetCode, detail)
	}
	return nil
}

func checkWebSocketHandshake(cfg onebot11.Config) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, cfg.Transport.URL, &websocket.DialOptions{HTTPHeader: headers(cfg)})
	if err != nil {
		return fmt.Errorf("无法连接 WebSocket %s: %w", cfg.Transport.URL, err)
	}
	return conn.Close(websocket.StatusNormalClosure, "anybot doctor")
}

func headers(cfg onebot11.Config) http.Header {
	header := make(http.Header, len(cfg.Transport.Headers)+1)
	addHeaders(header, cfg)
	return header
}

func addHeaders(header http.Header, cfg onebot11.Config) {
	for key, value := range cfg.Transport.Headers {
		header.Add(key, value)
	}
	if token := accessToken(cfg); token != "" {
		header.Set("Authorization", "Bearer "+token)
	}
}

func accessToken(cfg onebot11.Config) string {
	return cfg.Transport.AccessToken
}

func printSummary(path string, cfg host.Config, plugins []string) {
	fmt.Fprintf(stdout, "配置文件：%s\n", path)
	fmt.Fprintf(stdout, "协议：%s\n", cfg.Adapter.Protocol)
	fmt.Fprintf(stdout, "传输：%s\n", cfg.Adapter.Transport.Type)
	switch cfg.Adapter.Transport.Type {
	case "reverse_ws":
		fmt.Fprintf(stdout, "监听：%s\n", cfg.Adapter.Transport.Listen)
		fmt.Fprintf(stdout, "路径：%s\n", doctorPath(cfg.Adapter.Transport.Path))
	case "http", "websocket":
		fmt.Fprintf(stdout, "URL：%s\n", cfg.Adapter.Transport.URL)
	}
	if len(plugins) == 0 {
		fmt.Fprintln(stdout, "插件：无")
	} else {
		fmt.Fprintf(stdout, "插件：%s\n", strings.Join(plugins, ", "))
	}
}

func doctorPath(path string) string {
	if path == "" {
		return "/"
	}
	return path
}
