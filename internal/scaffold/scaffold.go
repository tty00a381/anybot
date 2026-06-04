package scaffold

import (
	"bytes"
	"fmt"
	"go/format"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// ProjectOptions 配置项目脚手架的目标目录、模块名和覆盖策略。
type ProjectOptions struct {
	Dir    string
	Module string
	Force  bool
}

// PluginOptions 配置插件脚手架的目标项目、插件名和覆盖策略。
type PluginOptions struct {
	Dir   string
	Name  string
	Force bool
}

type scaffoldFile struct {
	Name    string
	Content string
}

// InitProject 写入一个使用 OneBot v11 反向 WebSocket 的最小机器人项目。
func InitProject(opts ProjectOptions) error {
	if opts.Dir == "" {
		opts.Dir = "."
	}
	if opts.Module == "" {
		opts.Module = "example.com/bot"
	}
	files := []scaffoldFile{
		{Name: "go.mod", Content: render(projectGoMod, opts)},
		{Name: "main.go", Content: mustFormat(render(projectMain, opts))},
		{Name: "anybot.yaml", Content: render(projectConfig, opts)},
		{Name: ".env.example", Content: "ONEBOT_ACCESS_TOKEN=\n"},
		{Name: "README.md", Content: render(projectReadme, opts)},
	}
	if err := checkFileConflicts(opts.Dir, files, opts.Force); err != nil {
		return err
	}
	for _, file := range files {
		if err := writeFile(filepath.Join(opts.Dir, file.Name), file.Content, opts.Force); err != nil {
			return err
		}
	}
	return nil
}

// NewPlugin 在 plugins/<name> 下写入显式安装的插件骨架。
func NewPlugin(opts PluginOptions) error {
	if opts.Dir == "" {
		opts.Dir = "."
	}
	if opts.Name == "" {
		return fmt.Errorf("插件名不能为空")
	}
	pkg := packageName(opts.Name)
	data := struct {
		Name    string
		Package string
	}{
		Name:    opts.Name,
		Package: pkg,
	}
	dir := filepath.Join(opts.Dir, "plugins", pkg)
	return writeFile(filepath.Join(dir, pkg+".go"), mustFormat(render(pluginFile, data)), opts.Force)
}

func writeFile(path, content string, force bool) error {
	if !force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("%s 已存在；使用 -force 覆盖", path)
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func checkFileConflicts(dir string, files []scaffoldFile, force bool) error {
	if force {
		return nil
	}
	for _, file := range files {
		path := filepath.Join(dir, file.Name)
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("%s 已存在；使用 -force 覆盖", path)
		}
	}
	return nil
}

func render(src string, data any) string {
	tpl := template.Must(template.New("scaffold").Parse(src))
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		panic(err)
	}
	return buf.String()
}

func mustFormat(src string) string {
	out, err := format.Source([]byte(src))
	if err != nil {
		return src
	}
	return string(out)
}

func packageName(name string) string {
	name = strings.ToLower(name)
	var b strings.Builder
	lastSep := false
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			lastSep = false
		case r >= '0' && r <= '9' && b.Len() > 0:
			b.WriteRune(r)
			lastSep = false
		case r == '_' || r == '-' || r == ' ':
			if b.Len() > 0 && !lastSep {
				b.WriteByte('_')
				lastSep = true
			}
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return "plugin"
	}
	if token.Lookup(out).IsKeyword() {
		return out + "_plugin"
	}
	return out
}

const projectGoMod = `module {{.Module}}

go 1.24
`

const projectMain = `package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/tty00a381/anybot"
	"github.com/tty00a381/anybot/adapters/onebot11"
)

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "-h" || os.Args[1] == "--help") {
		fmt.Println("用法：go run .")
		fmt.Println("配置：编辑 anybot.yaml，并让 OneBot v11 协议端连接反向 WebSocket")
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	adapter, err := onebot11.LoadAdapter("anybot.yaml")
	if err != nil {
		log.Fatal(err)
	}

	app := anybot.New(anybot.WithAdapter(adapter))
	app.Use(anybot.Recover(), anybot.Trace())

	app.Command("ping").Handle(func(c *anybot.Context) error {
		_, err := c.ReplyText("pong")
		return err
	})

	if err := app.Run(ctx); err != nil && ctx.Err() == nil {
		log.Fatal(err)
	}
}
`

const projectConfig = `protocol: onebot11
transport:
  # AnyBot 监听该地址，OneBot v11 协议端通过反向 WebSocket 主动连接。
  type: reverse_ws
  listen: "127.0.0.1:6700"
  path: "/"
  access_token_env: ONEBOT_ACCESS_TOKEN
  action_timeout: 10s
`

const projectReadme = `# AnyBot 机器人

本项目由 ` + "`anybot init`" + ` 生成，模块名为 ` + "`{{.Module}}`" + `。它默认使用 OneBot v11 反向 WebSocket；下面以 NapCat 为例，其他 OneBot v11 协议端也按同样方式接入。

## 运行

` + "```sh" + `
go mod tidy
anybot doctor
go run . --help
go run .
` + "```" + `

## 协议端配置

在 OneBot v11 协议端中启用反向 WebSocket，并连接到：

` + "```text" + `
ws://127.0.0.1:6700/
` + "```" + `

如果设置了访问令牌，请在运行前配置环境变量，并在协议端中填写同一个值：

` + "```sh" + `
export ONEBOT_ACCESS_TOKEN=你的令牌
` + "```" + `

## 目录

- ` + "`main.go`" + `：机器人入口。
- ` + "`anybot.yaml`" + `：本地运行配置，启动时由 ` + "`onebot11.LoadAdapter`" + ` 读取。
- ` + "`.env.example`" + `：环境变量示例。
- ` + "`plugins/`" + `：插件目录，按需生成。

## 插件

使用以下命令生成插件：

` + "```sh" + `
anybot new plugin hello
` + "```" + `

然后在 ` + "`main.go`" + ` 中显式安装插件：

` + "```go" + `
if err := app.UsePlugin(hello.New()); err != nil {
	log.Fatal(err)
}
` + "```" + `

## 部署

生产环境建议先构建二进制，再用 systemd、supervisor 或容器托管进程。请给协议端与机器人配置相同的访问令牌，并避免把反向 WebSocket 端口直接暴露到公网。
`

const pluginFile = `package {{.Package}}

import "github.com/tty00a381/anybot"

// Config 配置 {{.Name}} 插件。
type Config struct {
	Enabled bool ` + "`yaml:\"enabled\"`" + `
}

// New 创建 {{.Name}} 插件。
func New(config ...Config) anybot.Plugin {
	cfg := Config{Enabled: true}
	if len(config) > 0 {
		cfg = config[0]
	}
	return anybot.PluginFunc{
		Info: anybot.Manifest{
			Name: "{{.Name}}",
			Version: "0.1.0",
			Description: "{{.Name}} 插件",
			Config: cfg,
		},
		Fn: func(app *anybot.App) error {
			if !cfg.Enabled {
				return nil
			}
			app.Command("{{.Name}}").Handle(func(c *anybot.Context) error {
				_, err := c.ReplyText("{{.Name}} 已启动")
				return err
			})
			return nil
		},
	}
}
`
