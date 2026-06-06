package sdk

import (
	"log/slog"
	"regexp"
	"time"

	"github.com/tty00a381/anybot/core"
)

// Manifest 描述插件名称、版本、说明和可选配置。
type Manifest = core.Manifest

// EventContext 是事件处理函数的上下文。
type EventContext = core.Context

// App 是底层运行时。常规插件应优先使用 sdk.Context 上的窄接口。
type App = core.App

// Option 调整底层运行时配置，常用于插件测试。
type Option = core.Option

// Adapter 把具体聊天协议接入 AnyBot 运行时。
type Adapter = core.Adapter

// EmitFunc 是适配器向运行时投递标准化事件的函数。
type EmitFunc = core.EmitFunc

// Route 表示一条事件路由。
type Route = core.Route

// Observer 表示一个旁路事件观察者。
type Observer = core.Observer

// Event 是协议标准化后的事件。
type Event = core.Event

// Handler 处理已经匹配成功的事件。
type Handler = core.Handler

// ErrorHandler 处理路由处理函数或中间件返回的错误。
type ErrorHandler = core.ErrorHandler

// Hook 是 App 生命周期钩子函数。
type Hook = core.Hook

// Middleware 包装处理函数。
type Middleware = core.Middleware

// Match 描述规则匹配结果。
type Match = core.Match

// Rule 判断路由是否应处理当前事件。
type Rule = core.Rule

// RuleFunc 将普通函数适配为 Rule。
type RuleFunc = core.RuleFunc

// Store 是会话存储接口。
type Store = core.Store

// Session 是基于固定键前缀的 Store 视图。
type Session = core.Session

// MemoryStore 是 goroutine 安全的进程内 Store。
type MemoryStore = core.MemoryStore

// FileStore 是 goroutine 安全的文件持久化 Store。
type FileStore = core.FileStore

// ActionClient 是协议无关的动作调用接口。
type ActionClient = core.ActionClient

// ReplyTarget 描述主动消息的发送目标。
type ReplyTarget = core.ReplyTarget

// MessageReceipt 是协议无关的消息发送回执。
type MessageReceipt = core.MessageReceipt

// ActionResponse 是协议无关的动作响应封套。
type ActionResponse = core.ActionResponse

// Protocol 是协议标识。
type Protocol = core.Protocol

// ActionError 描述一次由协议端明确拒绝或返回失败状态的动作调用。
type ActionError = core.ActionError

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

const (
	// ProtocolOneBot11 表示 OneBot v11 协议。
	ProtocolOneBot11 = core.ProtocolOneBot11
)

var (
	// ErrActionUnavailable 表示动作客户端当前暂不可用。
	ErrActionUnavailable = core.ErrActionUnavailable
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
func NewApp(opts ...Option) *App { return core.New(opts...) }

// WithAdapter 配置运行时使用的协议适配器。
func WithAdapter(adapter Adapter) Option { return core.WithAdapter(adapter) }

// WithStore 配置测试或嵌入式运行时使用的会话存储。
func WithStore(store Store) Option { return core.WithStore(store) }

// WithSuperUsers 配置测试或嵌入式运行时的超级用户 ID。
func WithSuperUsers(ids ...string) Option { return core.WithSuperUsers(ids...) }

// NewTestContext 创建测试用事件上下文。
func NewTestContext(app *App, event *Event) *EventContext { return core.NewTestContext(app, event) }

// NewSession 创建指定键前缀下的会话视图。
func NewSession(store Store, key string) *Session { return core.NewSession(store, key) }

// Any 匹配所有事件。
func Any() Rule { return core.Any() }

// All 要求所有规则都匹配。
func All(rules ...Rule) Rule { return core.All(rules...) }

// AnyOf 在任意规则匹配时通过。
func AnyOf(rules ...Rule) Rule { return core.AnyOf(rules...) }

// Not 在规则不匹配时通过。
func Not(rule Rule) Rule { return core.Not(rule) }

// EventType 匹配标准化事件类型。
func EventType(kind string) Rule { return core.EventType(kind) }

// DetailType 匹配标准化事件细分类型。
func DetailType(kind string) Rule { return core.DetailType(kind) }

// MessageEvent 匹配消息事件。
func MessageEvent() Rule { return core.MessageEvent() }

// Group 匹配群消息。
func Group() Rule { return core.Group() }

// Private 匹配私聊消息。
func Private() Rule { return core.Private() }

// FromUser 匹配指定发送者 ID。
func FromUser(ids ...any) Rule { return core.FromUser(ids...) }

// FromSelf 匹配由当前机器人账号自己发送的消息。
func FromSelf() Rule { return core.FromSelf() }

// NotFromSelf 排除当前机器人账号自己发送的消息。
func NotFromSelf() Rule { return core.NotFromSelf() }

// InGroup 匹配指定群 ID。
func InGroup(ids ...any) Rule { return core.InGroup(ids...) }

// Mentioned 匹配提及指定用户的消息；未传 ID 时匹配任意提及。
func Mentioned(ids ...any) Rule { return core.Mentioned(ids...) }

// ToMe 匹配提及机器人或私聊消息。
func ToMe() Rule { return core.ToMe() }

// Contains 匹配包含指定文本的消息。
func Contains(substr string) Rule { return core.Contains(substr) }

// Prefix 匹配指定前缀的消息。
func Prefix(prefix string) Rule { return core.Prefix(prefix) }

// CommandRule 使用默认前缀匹配命令。
func CommandRule(names ...string) Rule { return core.CommandRule(names...) }

// CommandWithPrefixes 使用显式前缀集合匹配命令。
func CommandWithPrefixes(prefixes []string, names ...string) Rule {
	return core.CommandWithPrefixes(prefixes, names...)
}

// RegexRule 使用正则表达式字符串匹配事件文本。
func RegexRule(pattern string) Rule { return core.RegexRule(pattern) }

// RegexpRule 使用已编译的正则表达式匹配事件文本。
func RegexpRule(re *regexp.Regexp) Rule { return core.RegexpRule(re) }

// RequireSuperUser 只允许宿主级超级用户继续执行。
func RequireSuperUser() Middleware { return core.RequireSuperUser() }

// Recover 捕获处理链中的 panic。
func Recover(loggers ...*slog.Logger) Middleware { return core.Recover(loggers...) }

// Trace 记录路由处理耗时和事件关键信息。
func Trace(loggers ...*slog.Logger) Middleware { return core.Trace(loggers...) }

// Timeout 为后续处理函数派生带超时的 context。
func Timeout(timeout time.Duration) Middleware { return core.Timeout(timeout) }

// OnlyPrivate 仅允许私聊消息进入后续处理链。
func OnlyPrivate() Middleware { return core.OnlyPrivate() }

// OnlyGroup 仅允许群消息进入后续处理链。
func OnlyGroup() Middleware { return core.OnlyGroup() }

// SuperUser 仅允许指定用户进入后续处理链。
func SuperUser(ids ...any) Middleware { return core.SuperUser(ids...) }

// RateLimit 按会话限制事件处理频率。
func RateLimit(limit int, window time.Duration) Middleware {
	return core.RateLimit(limit, window)
}

// RateLimitBy 使用自定义键做进程内限速。
func RateLimitBy(limit int, window time.Duration, keyFunc func(*EventContext) string) Middleware {
	return core.RateLimitBy(limit, window, keyFunc)
}

// TaskImmediate 让周期任务启动后立即执行一次。
func TaskImmediate() TaskOption { return core.TaskImmediate() }

// TaskCritical 让任务失败时停止 App.Run，并把任务错误作为运行错误返回。
func TaskCritical() TaskOption { return core.TaskCritical() }
