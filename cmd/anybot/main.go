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
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/tty00a381/anybot/adapters/onebot11"
	"github.com/tty00a381/anybot/internal/scaffold"
)

var version = "dev"
var stdout io.Writer = os.Stdout
var stderr io.Writer = os.Stderr

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		usage()
		return nil
	}
	switch args[0] {
	case "init":
		return runInit(args[1:])
	case "new":
		return runNew(args[1:])
	case "run":
		return runBot(args[1:])
	case "doctor":
		return runDoctor(args[1:])
	case "version":
		fmt.Fprintf(stdout, "anybot %s\n", version)
		return nil
	case "help", "-h", "--help":
		usage()
		return nil
	default:
		return fmt.Errorf("未知命令 %q", args[0])
	}
}

func runInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	module := fs.String("module", "example.com/bot", "Go 模块路径")
	dir := fs.String("dir", ".", "目标目录")
	force := fs.Bool("force", false, "覆盖已有文件")
	if err := fs.Parse(args); err != nil {
		return err
	}
	return scaffold.InitProject(scaffold.ProjectOptions{Dir: *dir, Module: *module, Force: *force})
}

func runNew(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("用法：anybot new plugin <名称>")
	}
	switch args[0] {
	case "plugin":
		fs := flag.NewFlagSet("new plugin", flag.ContinueOnError)
		dir := fs.String("dir", ".", "目标项目目录")
		force := fs.Bool("force", false, "覆盖已有文件")
		name, flagArgs, err := splitPluginArgs(args[1:])
		if err != nil {
			return err
		}
		if err := fs.Parse(flagArgs); err != nil {
			return err
		}
		if name == "" {
			return fmt.Errorf("用法：anybot new plugin <名称>")
		}
		return scaffold.NewPlugin(scaffold.PluginOptions{Dir: *dir, Name: name, Force: *force})
	default:
		return fmt.Errorf("未知生成器 %q", args[0])
	}
}

func splitPluginArgs(args []string) (string, []string, error) {
	var name string
	var flagArgs []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-dir" || arg == "--dir":
			if i+1 >= len(args) {
				return "", nil, fmt.Errorf("%s 需要值", arg)
			}
			flagArgs = append(flagArgs, arg, args[i+1])
			i++
		case strings.HasPrefix(arg, "-dir=") || strings.HasPrefix(arg, "--dir="):
			flagArgs = append(flagArgs, arg)
		case arg == "-force" || arg == "--force":
			flagArgs = append(flagArgs, arg)
		case strings.HasPrefix(arg, "-"):
			flagArgs = append(flagArgs, arg)
		default:
			if name != "" {
				return "", nil, fmt.Errorf("只能指定一个插件名")
			}
			name = arg
		}
	}
	return name, flagArgs, nil
}

func runBot(args []string) error {
	cmdArgs := append([]string{"run", "."}, args...)
	cmd := exec.Command("go", cmdArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

func runDoctor(args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	configPath := fs.String("config", "anybot.yaml", "配置文件")
	connect := fs.Bool("connect", false, "检查远端 NapCat 是否可连接")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := onebot11.LoadConfig(*configPath)
	if err != nil {
		return err
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	for _, warning := range doctorWarnings(cfg) {
		fmt.Fprintf(stderr, "警告：%s\n", warning)
	}
	switch cfg.Transport.Type {
	case "reverse_ws":
		ln, err := net.Listen("tcp", cfg.Transport.Listen)
		if err != nil {
			return fmt.Errorf("无法监听 %s: %w", cfg.Transport.Listen, err)
		}
		_ = ln.Close()
	case "http", "websocket":
		if *connect {
			if err := checkRemote(cfg); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("不支持传输 %q", cfg.Transport.Type)
	}
	abs, _ := filepath.Abs(*configPath)
	printDoctorSummary(abs, cfg)
	fmt.Fprintf(stdout, "配置可用：%s\n", abs)
	return nil
}

func printDoctorSummary(path string, cfg onebot11.Config) {
	fmt.Fprintf(stdout, "配置文件：%s\n", path)
	fmt.Fprintf(stdout, "传输：%s\n", cfg.Transport.Type)
	switch cfg.Transport.Type {
	case "reverse_ws":
		fmt.Fprintf(stdout, "监听：%s\n", cfg.Transport.Listen)
		fmt.Fprintf(stdout, "路径：%s\n", doctorPath(cfg.Transport.Path))
	case "http", "websocket":
		fmt.Fprintf(stdout, "URL：%s\n", cfg.Transport.URL)
	}
}

func doctorWarnings(cfg onebot11.Config) []string {
	var warnings []string
	if cfg.Transport.AccessTokenEnv != "" && os.Getenv(cfg.Transport.AccessTokenEnv) == "" {
		warnings = append(warnings, fmt.Sprintf("环境变量 %s 未设置", cfg.Transport.AccessTokenEnv))
	}
	if cfg.Transport.Type == "reverse_ws" && !reverseListenIsLocal(cfg.Transport.Listen) && !accessTokenAvailable(cfg) {
		warnings = append(warnings, "反向 WebSocket 监听非本机地址且未配置可用访问令牌")
	}
	return warnings
}

func accessTokenAvailable(cfg onebot11.Config) bool {
	if cfg.Transport.AccessToken != "" {
		return true
	}
	return cfg.Transport.AccessTokenEnv != "" && os.Getenv(cfg.Transport.AccessTokenEnv) != ""
}

func reverseListenIsLocal(listen string) bool {
	host, _, err := net.SplitHostPort(listen)
	if err != nil {
		return false
	}
	switch host {
	case "127.0.0.1", "localhost", "::1":
		return true
	default:
		return false
	}
}

func doctorPath(path string) string {
	if path == "" {
		return "/"
	}
	return path
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
	addDoctorHeaders(req.Header, cfg)
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("无法连接 NapCat HTTP %s: %w", cfg.Transport.URL, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("NapCat HTTP 返回 %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var action onebot11.Response
	if err := json.Unmarshal(data, &action); err != nil {
		return fmt.Errorf("NapCat HTTP 响应不是 OneBot 动作响应: %w", err)
	}
	if !action.OK() {
		detail := action.Message
		if detail == "" {
			detail = action.Wording
		}
		return fmt.Errorf("NapCat HTTP 动作失败: status=%s retcode=%d %s", action.Status, action.RetCode, detail)
	}
	return nil
}

func checkWebSocketHandshake(cfg onebot11.Config) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, cfg.Transport.URL, &websocket.DialOptions{
		HTTPHeader: doctorHeaders(cfg),
	})
	if err != nil {
		return fmt.Errorf("无法连接 NapCat WebSocket %s: %w", cfg.Transport.URL, err)
	}
	return conn.Close(websocket.StatusNormalClosure, "anybot doctor")
}

func doctorHeaders(cfg onebot11.Config) http.Header {
	header := make(http.Header, len(cfg.Transport.Headers)+1)
	addDoctorHeaders(header, cfg)
	return header
}

func addDoctorHeaders(header http.Header, cfg onebot11.Config) {
	for key, value := range cfg.Transport.Headers {
		header.Add(key, value)
	}
	if token := doctorAccessToken(cfg); token != "" {
		header.Set("Authorization", "Bearer "+token)
	}
}

func doctorAccessToken(cfg onebot11.Config) string {
	if cfg.Transport.AccessToken != "" {
		return cfg.Transport.AccessToken
	}
	if cfg.Transport.AccessTokenEnv == "" {
		return ""
	}
	return os.Getenv(cfg.Transport.AccessTokenEnv)
}

func usage() {
	fmt.Fprintln(stdout, `anybot 命令：
  anybot init [-module 模块名] [-dir 目录] [-force]
  anybot new plugin <名称> [-dir 目录] [-force]
  anybot run [go run 参数...]
  anybot doctor [-config anybot.yaml] [-connect]
  anybot version`)
}
