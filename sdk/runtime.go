package sdk

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/tty00a381/anybot/core"
)

// Manifest 描述插件作者提供的展示信息。
type Manifest struct {
	Name        string
	Version     string
	Description string
}

// Environment 描述运行框架授予插件的宿主能力。
type Environment struct {
	DataDir               string
	PluginID              string
	ConfigStore           ConfigStore
	AllowGlobalMiddleware bool
}

// InstallOption 调整本次插件安装使用的宿主能力。
type InstallOption func(*Environment)

// WithEnvironment 注入完整宿主能力，供 host 统一安装插件时使用。
func WithEnvironment(env Environment) InstallOption {
	return func(target *Environment) {
		if target == nil {
			return
		}
		*target = env
		target.DataDir = strings.TrimSpace(target.DataDir)
		target.PluginID = strings.TrimSpace(target.PluginID)
	}
}

// WithDataDir 注入插件数据根目录；运行框架会在其下为每个插件 ID 创建独立目录。
func WithDataDir(root string) InstallOption {
	return func(env *Environment) {
		if env != nil {
			env.DataDir = strings.TrimSpace(root)
		}
	}
}

// WithPluginID 注入当前插件的本地安装 ID；状态、配置和数据目录都使用它定位。
func WithPluginID(id string) InstallOption {
	return func(env *Environment) {
		if env != nil {
			env.PluginID = strings.TrimSpace(id)
		}
	}
}

// WithConfigStore 注入插件配置写回能力；通常由框架配置文件加载层使用。
func WithConfigStore(store ConfigStore) InstallOption {
	return func(env *Environment) {
		if env != nil {
			env.ConfigStore = store
		}
	}
}

// Install 将 SDK 插件安装到运行时。普通插件代码不需要接触 core.App 的内部安装细节。
func Install(app *App, plugins ...Plugin) error {
	if app == nil {
		return fmt.Errorf("anybot: app is nil")
	}
	for _, plugin := range plugins {
		if err := InstallWith(app, Environment{}, plugin); err != nil {
			return err
		}
	}
	return nil
}

// InstallDefault 使用插件定义的默认配置安装插件，主要供插件测试和嵌入式程序使用。
// 它不会注入 PluginID、配置写回或插件数据目录；需要这些宿主能力时使用
// InstallDefaultWith 或交给 anybot 运行框架安装。
func InstallDefault(app *App, definitions ...Definition) error {
	if app == nil {
		return fmt.Errorf("anybot: app is nil")
	}
	for _, definition := range definitions {
		if err := InstallDefaultWith(app, Environment{}, definition); err != nil {
			return err
		}
	}
	return nil
}

// InstallDefaultWith 使用默认配置和显式宿主能力安装插件定义。
func InstallDefaultWith(app *App, env Environment, definition Definition) error {
	if app == nil {
		return fmt.Errorf("anybot: app is nil")
	}
	if definition == nil {
		return nil
	}
	plugin, err := definition.Build()
	if err != nil {
		return err
	}
	return InstallWith(app, env, plugin)
}

// InstallDefaultWithID 使用默认配置和显式稳定 PluginID 安装插件定义。
// 适合嵌入式机器人直接安装多个插件时隔离状态、任务和路由名称。
func InstallDefaultWithID(app *App, id string, definition Definition) error {
	return InstallDefaultWith(app, Environment{PluginID: id}, definition)
}

// InstallWith 使用显式宿主能力安装 SDK 插件。
func InstallWith(app *App, env Environment, plugin Plugin) error {
	return InstallCoreWith(coreApp(app), env, plugin)
}

// InstallWithID 使用显式稳定 PluginID 安装已构建好的插件对象。
func InstallWithID(app *App, id string, plugin Plugin) error {
	return InstallWith(app, Environment{PluginID: id}, plugin)
}

// InstallCoreWith 将 SDK 插件安装到底层 core.App，供运行框架装配插件时使用。
func InstallCoreWith(app *core.App, env Environment, plugin Plugin) error {
	if app == nil {
		return fmt.Errorf("anybot: app is nil")
	}
	env.DataDir = strings.TrimSpace(env.DataDir)
	env.PluginID = strings.TrimSpace(env.PluginID)
	if env.PluginID != "" {
		if err := ValidatePluginID(env.PluginID); err != nil {
			return err
		}
	}
	if plugin == nil {
		return nil
	}
	manifest := plugin.Manifest()
	if err := plugin.Setup(newCoreContext(app, manifest, WithEnvironment(env))); err != nil {
		if manifest.Name != "" {
			return fmt.Errorf("anybot: 安装插件 %s 失败: %w", manifest.Name, err)
		}
		return fmt.Errorf("anybot: 安装插件失败: %w", err)
	}
	return nil
}

// App 是 SDK 暴露的运行时句柄，常用于插件测试和嵌入式程序。
type App struct {
	runtime *core.App
}

// Option 调整底层运行时配置，常用于插件测试。
type Option = core.Option

// Adapter 把具体聊天协议接入 AnyBot 运行时。
type Adapter = core.Adapter

// EmitFunc 是适配器向运行时投递标准化事件的函数。
type EmitFunc = core.EmitFunc

// Event 是协议标准化后的事件。
type Event = core.Event

// ErrorHandler 处理插件路由处理函数或中间件返回的错误。
type ErrorHandler func(*EventContext, error)

// ObserverHandler 异步观察事件，不参与路由控制。
type ObserverHandler = core.ObserverHandler

// Hook 是 App 生命周期钩子函数。
type Hook = core.Hook

// Match 描述规则匹配结果。
type Match = core.Match

// Store 是会话存储接口。
type Store = core.Store

// Session 是基于固定键前缀的 Store 视图。
type Session = core.Session

// MemoryStore 是 goroutine 安全的进程内 Store。
type MemoryStore = core.MemoryStore

// FileStore 是 goroutine 安全的文件持久化 Store。
type FileStore = core.FileStore

// ActionClient 是协议无关的消息发送接口。
type ActionClient = core.ActionClient

// ReplyTarget 描述主动消息的发送目标。
type ReplyTarget = core.ReplyTarget

// MessageReceipt 是协议无关的消息发送回执。
type MessageReceipt = core.MessageReceipt

// Protocol 是协议标识。
type Protocol = core.Protocol

// PanicError 包装处理函数或中间件中恢复到的 panic 值和调用栈。
type PanicError = core.PanicError

// TaskFunc 是生命周期托管任务函数。
type TaskFunc = core.TaskFunc

// TaskOption 配置生命周期托管任务。
type TaskOption = core.TaskOption

// AdapterState 描述动作通道状态。
type AdapterState = core.AdapterState

// AdapterStateHook 观察动作通道状态变化。
type AdapterStateHook = core.AdapterStateHook

var (
	// ErrActionUnavailable 表示动作客户端当前暂不可用。
	ErrActionUnavailable = core.ErrActionUnavailable
	// ErrReplyTargetUnavailable 表示当前事件或目标无法映射到可回复的会话。
	ErrReplyTargetUnavailable = core.ErrReplyTargetUnavailable
	// ErrSessionUnavailable 表示当前事件或运行时无法映射到请求的会话维度。
	ErrSessionUnavailable = core.ErrSessionUnavailable
	// ErrPass 表示当前路由主动让出处理权。
	ErrPass = core.ErrPass
	// ErrStop 表示当前事件应停止向后续路由传播。
	ErrStop = core.ErrStop
	// ErrUnauthorized 表示当前事件没有通过权限检查。
	ErrUnauthorized = core.ErrUnauthorized
	// ErrRateLimited 表示当前限速窗口已被耗尽。
	ErrRateLimited = core.ErrRateLimited
)

// NewMemoryStore 创建默认内存存储。
func NewMemoryStore() *MemoryStore { return core.NewMemoryStore() }

// NewFileStore 打开或创建一个 JSON 文件存储。
func NewFileStore(path string) (*FileStore, error) { return core.NewFileStore(path) }

// NewApp 创建测试或嵌入式运行时。常规插件安装逻辑应使用 Context 上的窄接口。
func NewApp(opts ...Option) *App { return &App{runtime: core.New(opts...)} }

// WithAdapter 配置运行时使用的协议适配器。
func WithAdapter(adapter Adapter) Option { return core.WithAdapter(adapter) }

// WithStore 配置测试或嵌入式运行时使用的会话存储。
func WithStore(store Store) Option { return core.WithStore(store) }

// WithLogger 配置测试或嵌入式运行时使用的日志器。
func WithLogger(logger *slog.Logger) Option { return core.WithLogger(logger) }

// WithSuperUsers 配置测试或嵌入式运行时的超级用户 ID。
func WithSuperUsers(ids ...string) Option { return core.WithSuperUsers(ids...) }

// NewTestContext 创建测试用事件上下文。
func NewTestContext(app *App, event *Event) *EventContext {
	return newEventContext(core.NewTestContext(coreApp(app), event))
}

// NewSession 创建指定键前缀下的会话视图。
func NewSession(store Store, key string) *Session { return core.NewSession(store, key) }

// Any 匹配所有事件。
func Any() Rule { return wrapRule(core.Any()) }

// All 要求所有规则都匹配。
func All(rules ...Rule) Rule { return wrapRule(core.All(coreRules(rules)...)) }

// AnyOf 在任意规则匹配时通过。
func AnyOf(rules ...Rule) Rule { return wrapRule(core.AnyOf(coreRules(rules)...)) }

// Not 在规则不匹配时通过。
func Not(rule Rule) Rule {
	rules := coreRules([]Rule{rule})
	if len(rules) == 0 {
		return wrapRule(core.Not(nil))
	}
	return wrapRule(core.Not(rules[0]))
}

// EventType 匹配标准化事件类型。
func EventType(kind string) Rule { return wrapRule(core.EventType(kind)) }

// DetailType 匹配标准化事件细分类型。
func DetailType(kind string) Rule { return wrapRule(core.DetailType(kind)) }

// MessageEvent 匹配消息事件。
func MessageEvent() Rule { return wrapRule(core.MessageEvent()) }

// Group 匹配群消息。
func Group() Rule { return wrapRule(core.Group()) }

// Private 匹配私聊消息。
func Private() Rule { return wrapRule(core.Private()) }

// FromUser 匹配指定发送者 ID。
func FromUser(ids ...any) Rule { return wrapRule(core.FromUser(ids...)) }

// FromSelf 匹配由当前机器人账号自己发送的消息。
func FromSelf() Rule { return wrapRule(core.FromSelf()) }

// NotFromSelf 排除当前机器人账号自己发送的消息。
func NotFromSelf() Rule { return wrapRule(core.NotFromSelf()) }

// InGroup 匹配指定群 ID。
func InGroup(ids ...any) Rule { return wrapRule(core.InGroup(ids...)) }

// Mentioned 匹配提及指定用户的消息；未传 ID 时匹配任意提及。
func Mentioned(ids ...any) Rule { return wrapRule(core.Mentioned(ids...)) }

// ToMe 匹配提及机器人或私聊消息。
func ToMe() Rule { return wrapRule(core.ToMe()) }

// Contains 匹配包含指定文本的消息。
func Contains(substr string) Rule { return wrapRule(core.Contains(substr)) }

// Prefix 匹配指定前缀的消息。
func Prefix(prefix string) Rule { return wrapRule(core.Prefix(prefix)) }

// CommandRule 使用默认前缀匹配命令。
func CommandRule(names ...string) Rule { return wrapRule(core.CommandRule(names...)) }

// CommandWithPrefixes 使用显式前缀集合匹配命令。
func CommandWithPrefixes(prefixes []string, names ...string) Rule {
	return wrapRule(core.CommandWithPrefixes(prefixes, names...))
}

// RegexRule 使用正则表达式字符串匹配事件文本。
func RegexRule(pattern string) Rule { return wrapRule(core.RegexRule(pattern)) }

// RegexpRule 使用已编译的正则表达式匹配事件文本。
func RegexpRule(re *regexp.Regexp) Rule { return wrapRule(core.RegexpRule(re)) }

// RequireSuperUser 只允许框架级超级用户继续执行。
func RequireSuperUser() Middleware { return wrapMiddleware(core.RequireSuperUser()) }

// Recover 捕获处理链中的 panic。
func Recover(loggers ...*slog.Logger) Middleware { return wrapMiddleware(core.Recover(loggers...)) }

// Trace 记录路由处理耗时和事件关键信息。
func Trace(loggers ...*slog.Logger) Middleware { return wrapMiddleware(core.Trace(loggers...)) }

// Timeout 为后续处理函数派生带超时的 context。
func Timeout(timeout time.Duration) Middleware { return wrapMiddleware(core.Timeout(timeout)) }

// OnlyPrivate 仅允许私聊消息进入后续处理链。
func OnlyPrivate() Middleware { return wrapMiddleware(core.OnlyPrivate()) }

// OnlyGroup 仅允许群消息进入后续处理链。
func OnlyGroup() Middleware { return wrapMiddleware(core.OnlyGroup()) }

// SuperUser 仅允许指定用户进入后续处理链。
func SuperUser(ids ...any) Middleware { return wrapMiddleware(core.SuperUser(ids...)) }

// RateLimit 按会话限制事件处理频率。
func RateLimit(limit int, window time.Duration) Middleware {
	return wrapMiddleware(core.RateLimit(limit, window))
}

// RateLimitBy 使用自定义键做进程内限速。
func RateLimitBy(limit int, window time.Duration, keyFunc func(*EventContext) string) Middleware {
	var coreKeyFunc func(*core.Context) string
	if keyFunc != nil {
		coreKeyFunc = func(c *core.Context) string {
			return keyFunc(newEventContext(c))
		}
	}
	return wrapMiddleware(core.RateLimitBy(limit, window, coreKeyFunc))
}

// TaskImmediate 让周期任务启动后立即执行一次。
func TaskImmediate() TaskOption { return core.TaskImmediate() }

// TaskCritical 让任务失败时停止 App.Run，并把任务错误作为运行错误返回。
func TaskCritical() TaskOption { return core.TaskCritical() }

// Runtime 返回底层 core.App，供测试、嵌入式程序或框架适配代码使用。
func (a *App) Runtime() *core.App {
	return coreApp(a)
}

// Dispatch 将标准化事件直接投递给运行时。
func (a *App) Dispatch(ctx context.Context, event *Event) error {
	if a == nil || a.runtime == nil {
		return fmt.Errorf("anybot: app is nil")
	}
	return a.runtime.Dispatch(ctx, event)
}

// Use 注册全局中间件。
func (a *App) Use(middleware ...Middleware) {
	if a != nil && a.runtime != nil {
		a.runtime.Use(coreMiddleware(middleware)...)
	}
}

// OnError 注册错误处理器。
func (a *App) OnError(handler ErrorHandler) {
	if a == nil || a.runtime == nil || handler == nil {
		return
	}
	a.runtime.OnError(func(c *core.Context, err error) {
		handler(newEventContext(c), err)
	})
}

// OnMessage 注册消息事件路由。
func (a *App) OnMessage(rules ...Rule) *Route {
	if a == nil || a.runtime == nil {
		return nil
	}
	return newRoute(nil, a.runtime.OnMessage(coreRules(rules)...))
}

// Command 注册命令路由。
func (a *App) Command(names ...string) *Route {
	if a == nil || a.runtime == nil {
		return nil
	}
	return newRoute(nil, a.runtime.Command(names...))
}

func coreApp(app *App) *core.App {
	if app == nil {
		return nil
	}
	return app.runtime
}

func wrapRule(rule core.Rule) Rule {
	return RuleFunc(func(ctx context.Context, c *EventContext) (Match, bool) {
		return rule.Match(ctx, c.UnsafeCoreContext())
	})
}

func wrapMiddleware(middleware core.Middleware) Middleware {
	return func(next Handler) Handler {
		wrapped := middleware(coreHandler(next))
		return func(c *EventContext) error {
			return wrapped(c.UnsafeCoreContext())
		}
	}
}
