# 插件作者指南

这份文档写给第一次给 AnyBot 写插件的人。主路径只有一条：写一个独立 Go module，导出 `Plugin`，用 `testkit` 测试，再通过 `anybot plugin add` 装进机器人工作目录。

## 1. 生成插件

需要 Go 1.24 或更新版本。

```sh
anybot dev plugin weather \
  -dir ../anybot-weather \
  -module github.com/you/anybot-weather
cd ../anybot-weather
go test ./...
```

`-module` 应该填真实 Go module 路径。`example.com/...` 只适合临时练习，不适合发布，也不适合让朋友安装。

## 2. 插件形态

普通插件只需要导出：

```go
var Plugin = sdk.Define(sdk.Spec[Config]{
	Manifest: sdk.Manifest{Name: "weather", Version: "0.1.0", Description: "天气提醒"},
	DefaultConfig: Config{Command: "weather"},
	Setup: func(ctx *sdk.Context, cfg Config) error {
		ctx.Command(cfg.Command).Handle(func(c *sdk.EventContext) error {
			_, err := c.ReplyText("weather ok")
			return err
		})
		return nil
	},
})
```

这段代码定义的是“插件类型”，不是某次运行实例。`Manifest` 是展示给宿主和用户看的插件信息；`DefaultConfig` 是第一次生成配置文件时使用的默认部署配置；`Setup` 是宿主完成配置解析后安装路由、状态、后台任务和对话流的逻辑入口。

不要在普通插件里直接依赖 `core`。`sdk.Context` 已经提供命令、事件规则、回复、主动发送、后台任务、状态、数据目录、配置写回和对话流。

## 3. 配置、状态、数据

- `Config`：部署配置和用户明确设置的选项，写在 `plugins.d/<PluginID>.yaml`。
- `State[T]` / `Session`：运行时记忆、会话、计数、小型 JSON 状态。
- `ctx.DataDir()`：插件私有文件、缓存、索引，或插件确实需要自管数据库时使用。

`PluginID` 由机器人工作目录生成，不是插件作者维护的 ID，也不是 `Manifest.Name`。改插件展示名不会迁移运行时数据。

## 4. 测试

```go
func TestPlugin(t *testing.T) {
	app := testkit.NewApp()
	if err := app.InstallDefault(Plugin); err != nil {
		t.Fatal(err)
	}
	if err := app.DispatchText("/weather"); err != nil {
		t.Fatal(err)
	}
	if got := app.LastReplyText(); got == "" {
		t.Fatal("expected reply")
	}
}
```

测试普通回复、权限、状态和对话流时优先使用 `github.com/tty00a381/anybot/sdk/testkit`。

## 5. 本地安装

```sh
anybot plugin add github.com/you/anybot-weather -replace ../anybot-weather -dir ../mybot
anybot plugin status -dir ../mybot
anybot plugin inspect <id> -dir ../mybot
anybot plugin config <id> command=weather -dir ../mybot
anybot plugin enable <id> -dir ../mybot
anybot up -dir ../mybot
```

基础 CLI 在外部插件尚未构建前只知道安装锁，会提示“待构建”。`anybot up` 构建生成宿主后，`./anybot-bot plugin check` 会使用完整插件注册表做最终校验。

## 6. 发布

```sh
go test ./...
git tag v0.1.0
go list -m github.com/you/anybot-weather@v0.1.0
```

别人安装固定版本：

```sh
anybot plugin add github.com/you/anybot-weather@v0.1.0 -dir mybot
anybot plugin enable <id> -dir mybot
anybot up -dir mybot
```

## 7. 何时导入协议包

只回复文本或发送协议无关消息时，继续只用 `sdk`。需要 OneBot v11 群管、文件、撤回、好友列表、NapCat 扩展动作时，可以在事件处理函数里导入 `github.com/tty00a381/anybot/adapters/onebot11` 并使用 `onebot11.ClientFrom(c)` 或 `onebot11.EventFrom(c)`。
