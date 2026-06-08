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

	"golang.org/x/mod/modfile"
	modmodule "golang.org/x/mod/module"
)

// ProjectOptions 配置项目脚手架的目标目录、模块名和覆盖策略。
type ProjectOptions struct {
	Dir    string
	Module string
	Force  bool
}

// PluginOptions 配置插件脚手架的目标项目、插件名和覆盖策略。
type PluginOptions struct {
	Dir           string
	Name          string
	Module        string
	Force         bool
	AnyBotVersion string
	AnyBotReplace string
}

// PluginResult 描述插件脚手架写入结果。
type PluginResult struct {
	Name       string
	Package    string
	Module     string
	Standalone bool
	TestReady  bool
	Files      []string
}

type scaffoldFile struct {
	Name    string
	Content string
}

type pluginScaffoldData struct {
	Name          string
	Package       string
	Manifest      string
	Command       string
	Module        string
	AnyBotVersion string
	AnyBotReplace string
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
		{Name: "core.yaml", Content: render(projectConfig, opts)},
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

// NewPlugin 写入可被 anybot 加载的插件 SDK 骨架。
func NewPlugin(opts PluginOptions) (PluginResult, error) {
	if opts.Dir == "" {
		opts.Dir = "."
	}
	if opts.Name == "" {
		return PluginResult{}, fmt.Errorf("插件名不能为空")
	}
	opts.Module = strings.TrimSpace(opts.Module)
	pkg := packageName(opts.Name)
	data := pluginScaffoldData{
		Name:          pkg,
		Package:       pkg,
		Manifest:      pkg,
		Command:       pkg,
		Module:        opts.Module,
		AnyBotVersion: anybotVersion(opts.AnyBotVersion),
		AnyBotReplace: strings.TrimSpace(opts.AnyBotReplace),
	}
	if opts.Module != "" {
		return newStandalonePlugin(opts, data)
	}
	dir := filepath.Join(opts.Dir, "plugins", pkg)
	path := filepath.Join(dir, pkg+".go")
	if err := writeFile(path, mustFormat(render(pluginFile, data)), opts.Force); err != nil {
		return PluginResult{}, err
	}
	return PluginResult{Name: data.Manifest, Package: pkg, Files: []string{path}}, nil
}

func newStandalonePlugin(opts PluginOptions, data pluginScaffoldData) (PluginResult, error) {
	if err := modmodule.CheckPath(opts.Module); err != nil {
		return PluginResult{}, fmt.Errorf("插件模块路径 %q 无效: %w", opts.Module, err)
	}
	goMod, err := pluginGoMod(data)
	if err != nil {
		return PluginResult{}, err
	}
	files := []scaffoldFile{
		{Name: "go.mod", Content: goMod},
		{Name: data.Package + ".go", Content: mustFormat(render(pluginFile, data))},
		{Name: data.Package + "_test.go", Content: mustFormat(render(pluginTestFile, data))},
		{Name: "README.md", Content: render(pluginReadme, data)},
	}
	testReady := data.AnyBotReplace != ""
	if testReady {
		files = insertScaffoldFile(files, 1, scaffoldFile{Name: "go.sum", Content: pluginGoSumFile})
	}
	if err := checkFileConflicts(opts.Dir, files, opts.Force); err != nil {
		return PluginResult{}, err
	}
	paths := make([]string, 0, len(files))
	for _, file := range files {
		path := filepath.Join(opts.Dir, file.Name)
		if err := writeFile(path, file.Content, opts.Force); err != nil {
			return PluginResult{}, err
		}
		paths = append(paths, path)
	}
	return PluginResult{Name: data.Manifest, Package: data.Package, Module: opts.Module, Standalone: true, TestReady: testReady, Files: paths}, nil
}

func insertScaffoldFile(files []scaffoldFile, index int, file scaffoldFile) []scaffoldFile {
	files = append(files, scaffoldFile{})
	copy(files[index+1:], files[index:])
	files[index] = file
	return files
}

func anybotVersion(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return "v0.0.0"
	}
	return version
}

func pluginGoMod(data pluginScaffoldData) (string, error) {
	if data.AnyBotReplace == "" && data.AnyBotVersion == "v0.0.0" {
		return "", fmt.Errorf("独立插件需要有效 AnyBot 版本，或使用 -replace 指向本地 AnyBot 源码")
	}
	goMod := render(pluginGoModFile, data)
	file, err := modfile.Parse("go.mod", []byte(goMod), nil)
	if err != nil {
		return "", fmt.Errorf("插件 go.mod 无效: %w", err)
	}
	if data.AnyBotReplace != "" {
		if err := file.AddReplace("github.com/tty00a381/anybot", "", data.AnyBotReplace, ""); err != nil {
			return "", fmt.Errorf("插件 go.mod replace 无效: %w", err)
		}
	}
	out, err := file.Format()
	if err != nil {
		return "", fmt.Errorf("插件 go.mod 格式化失败: %w", err)
	}
	return string(out), nil
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

	"github.com/tty00a381/anybot/core"
	"github.com/tty00a381/anybot/adapters/onebot11"
)

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "-h" || os.Args[1] == "--help") {
		fmt.Println("用法：go run .")
		fmt.Println("配置：编辑 core.yaml，并让 OneBot v11 协议端连接反向 WebSocket")
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	adapter, err := onebot11.LoadAdapter("core.yaml")
	if err != nil {
		log.Fatal(err)
	}

	app := core.New(core.WithAdapter(adapter))
	app.Use(core.Recover(), core.Trace())

	app.Command("ping").Handle(func(c *core.Context) error {
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

const projectReadme = `# AnyBot 核心库机器人

本项目由 ` + "`anybot dev init`" + ` 生成，模块名为 ` + "`{{.Module}}`" + `。它直接使用 core 运行时，默认通过 OneBot v11 反向 WebSocket 接入；下面以 NapCat 为例，其他 OneBot v11 协议端也按同样方式接入。

## 运行

` + "```sh" + `
go mod tidy
anybot dev doctor
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
- ` + "`core.yaml`" + `：本地运行配置，启动时由 ` + "`onebot11.LoadAdapter`" + ` 读取。
- ` + "`.env.example`" + `：环境变量示例。
- ` + "`plugins/`" + `：插件目录，按需生成。

## 插件

当前核心库项目里直接使用的插件，用 ` + "`-in-project`" + ` 生成到 ` + "`plugins/`" + `：

` + "```sh" + `
anybot dev plugin hello -in-project
` + "```" + `

默认推荐把要发布给别人使用的插件生成成独立模块：

` + "```sh" + `
anybot dev plugin hello -dir ../anybot-hello -module example.com/hello
` + "```" + `

项目内插件导出 ` + "`Plugin`" + `，适合直接写 Go 入口时显式安装：

` + "```go" + `
if err := absdk.InstallDefault(app, hello.Plugin); err != nil {
	log.Fatal(err)
}
` + "```" + `

## 部署

生产环境建议先构建二进制，再用 systemd、supervisor 或容器托管进程。请给协议端与机器人配置相同的访问令牌，并避免把反向 WebSocket 端口直接暴露到公网。
`

const pluginFile = `package {{.Package}}

import (
	"fmt"
	"strings"

	absdk "github.com/tty00a381/anybot/sdk"
)

// Config 配置 {{.Name}} 插件。
type Config struct {
	Command string ` + "`yaml:\"command\"`" + `
}

// Validate 校验 {{.Name}} 插件配置。
func (cfg Config) Validate() error {
	if strings.TrimSpace(cfg.Command) == "" {
		return fmt.Errorf("command 不能为空")
	}
	return nil
}

// Plugin 是 {{.Name}} 插件导出的 AnyBot 插件定义。
var Plugin = absdk.Define(
	absdk.Manifest{Name: "{{.Manifest}}", Version: "0.1.0", Description: "{{.Name}} 插件"},
	Config{Command: "{{.Command}}"},
	func(ctx *absdk.Context, cfg Config) error {
		ctx.Command(cfg.Command).
			Name("command").
			Handle(func(c *absdk.EventContext) error {
				_, err := c.ReplyText("{{.Name}} 已启动")
				return err
			})
		return nil
	},
)
`

const pluginTestFile = `package {{.Package}}

import (
	"testing"

	"github.com/tty00a381/anybot/sdk/testkit"
)

func TestPluginRepliesToCommand(t *testing.T) {
	app := testkit.NewApp()
	if err := app.InstallDefault(Plugin); err != nil {
		t.Fatal(err)
	}
	if err := app.DispatchText("/{{.Command}}"); err != nil {
		t.Fatal(err)
	}
	if got := app.LastReplyText(); got != "{{.Name}} 已启动" {
		t.Fatalf("reply = %q", got)
	}
}
`

const pluginReadme = `# {{.Name}}

{{.Name}} 是一个 AnyBot SDK 插件。插件导出 ` + "`Plugin`" + `，运行框架可以通过 ` + "`anybot plugin add`" + ` 安装，也可以在 Go 代码中用 ` + "`absdk.InstallDefault(app, Plugin)`" + ` 显式安装。

## 开发

如果 ` + "`go.mod`" + ` 中带有本地 AnyBot ` + "`replace`" + `，可以直接测试：

` + "```sh" + `
go test ./...
` + "```" + `

如果依赖的是远端 AnyBot 版本，先补齐模块校验和：

` + "```sh" + `
go mod tidy
go test ./...
` + "```" + `

## 本地安装

在机器人工作目录中执行：

` + "```sh" + `
anybot plugin add {{.Module}} -replace <插件目录>
anybot plugin status
anybot plugin config <id> command={{.Command}}
anybot plugin enable <id>
anybot up
` + "```" + `

` + "`anybot up`" + ` 会构建带外部插件的运行框架、同步默认配置并检查插件配置。停止机器人后，也可以运行 ` + "`./anybot-bot plugin check`" + ` 复查。

## 配置

` + "```yaml" + `
enabled: true
config:
  command: {{.Command}}
` + "```" + `

` + "`command`" + ` 是触发插件的命令名。插件会回复 ` + "`{{.Name}} 已启动`" + `。

## 发布

发布前确认 module 路径就是插件包路径，并给仓库打 tag：

` + "```sh" + `
go test ./...
git tag v0.1.0
go list -m {{.Module}}@v0.1.0
` + "```" + `

发布 Go module 版本后，用户可以安装固定版本：

` + "```sh" + `
anybot plugin add {{.Module}}@v0.1.0
anybot plugin status
anybot plugin enable <id>
anybot up
` + "```" + `
`

const pluginGoModFile = `module {{.Module}}

go 1.24.0

require github.com/tty00a381/anybot {{.AnyBotVersion}}

require gopkg.in/yaml.v3 v3.0.1 // indirect
`

const pluginGoSumFile = `gopkg.in/check.v1 v0.0.0-20161208181325-20d25e280405 h1:yhCVgyC4o1eVCa2tZl7eS0r+SDo693bJlVdllGtEeKM=
gopkg.in/check.v1 v0.0.0-20161208181325-20d25e280405/go.mod h1:Co6ibVJAznAaIkqp8huTwlJQCZ016jof/cbN4VW5Yz0=
gopkg.in/yaml.v3 v3.0.1 h1:fxVm/GzAzEWqLHuvctI91KS9hhNmmWOoWu0XTYJS7CA=
gopkg.in/yaml.v3 v3.0.1/go.mod h1:K4uyk7z7BCEPqu6E+C64Yfv1cQ7kz7rIZviUmN+EgEM=
`
