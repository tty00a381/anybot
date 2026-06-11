package sdk

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/tty00a381/anybot/core"
	"github.com/tty00a381/anybot/sdk/message"
)

// ErrGlobalMiddlewareUnavailable 表示宿主没有授予插件注册全局中间件的能力。
var ErrGlobalMiddlewareUnavailable = errors.New("anybot: global middleware is not available to this plugin")

// Context 是成熟插件的安装上下文，暴露插件应使用的运行时能力。
type Context struct {
	app      *core.App
	router   *core.Router
	manifest Manifest
	env      Environment
}

// NewContext 创建插件安装上下文，主要供运行框架或测试使用。
func NewContext(app *App, manifest Manifest, opts ...InstallOption) *Context {
	return newCoreContext(coreApp(app), manifest, opts...)
}

func newCoreContext(app *core.App, manifest Manifest, opts ...InstallOption) *Context {
	ctx := &Context{app: app, manifest: manifest}
	for _, opt := range opts {
		if opt != nil {
			opt(&ctx.env)
		}
	}
	if app != nil {
		ctx.router = app.Group()
	}
	return ctx
}

// Manifest 返回当前插件清单。
func (c *Context) Manifest() Manifest {
	if c == nil {
		return Manifest{}
	}
	return c.manifest
}

// PluginID 返回当前插件在本地机器人目录中的稳定安装 ID。
func (c *Context) PluginID() string {
	if c == nil {
		return ""
	}
	return c.env.PluginID
}

// Logger 返回框架日志器。
func (c *Context) Logger() *slog.Logger {
	if c == nil || c.app == nil {
		return slog.Default()
	}
	logger := c.app.Logger()
	id := c.PluginID()
	if id == "" {
		return logger
	}
	return logger.With("plugin_id", id, "plugin_name", c.manifest.Name)
}

// Store 返回框架会话存储。
func (c *Context) Store() Store {
	if c == nil || c.app == nil {
		return nil
	}
	return c.app.Store()
}

// Session 返回当前 PluginID 命名空间下的会话存储视图。
func (c *Context) Session(event *EventContext) *Session {
	if event == nil {
		return c.SessionBy("session")
	}
	return c.SessionBy(event.ConversationID())
}

// UserSession 返回当前 PluginID 命名空间下的用户存储视图。
func (c *Context) UserSession(event *EventContext) *Session {
	key := "session:user"
	if event != nil {
		if current := event.Event(); current != nil && current.UserSessionID() != "" {
			key = current.UserSessionID()
		}
	}
	return c.SessionBy(key)
}

// GroupSession 返回当前 PluginID 命名空间下的群或频道存储视图。
func (c *Context) GroupSession(event *EventContext) *Session {
	if event == nil {
		return core.NewUnavailableSession(errors.Join(ErrSessionUnavailable, ErrEventContextUnavailable))
	}
	current := event.Event()
	if current == nil || current.GroupSessionID() == "" {
		return core.NewUnavailableSession(errors.Join(ErrSessionUnavailable, ErrGroupContextUnavailable))
	}
	return c.SessionBy(current.GroupSessionID())
}

// SessionBy 返回当前 PluginID 命名空间下的自定义存储视图。
func (c *Context) SessionBy(key string) *Session {
	if key == "" {
		key = "session"
	}
	return NewSession(c.Store(), scopedName(c.PluginID(), key))
}

// Client 返回当前动作客户端。
func (c *Context) Client() ActionClient {
	if c == nil || c.app == nil {
		return nil
	}
	return c.app.Client()
}

// Send 使用当前动作客户端发送消息。
func (c *Context) Send(ctx context.Context, target ReplyTarget, chain message.Chain) (MessageReceipt, error) {
	if c == nil || c.app == nil || c.app.Client() == nil {
		return MessageReceipt{}, ErrActionUnavailable
	}
	return c.app.Client().Send(ctx, target, chain)
}

// SendText 使用当前动作客户端发送纯文本消息。
func (c *Context) SendText(ctx context.Context, target ReplyTarget, text string) (MessageReceipt, error) {
	return c.Send(ctx, target, message.New(message.Text(text)))
}

// Use 注册插件级中间件，只作用于当前插件通过 Context 注册的后续路由。
func (c *Context) Use(middleware ...Middleware) {
	if c != nil && c.router != nil {
		c.router.Use(coreMiddleware(middleware)...)
	}
}

// UseGlobal 注册框架级全局中间件。只有运行框架显式授权的内置策略插件才应使用。
func (c *Context) UseGlobal(middleware ...Middleware) error {
	if c == nil || c.app == nil {
		return ErrGlobalMiddlewareUnavailable
	}
	if !c.env.AllowGlobalMiddleware {
		return ErrGlobalMiddlewareUnavailable
	}
	c.app.Use(coreMiddleware(middleware)...)
	return nil
}

// On 注册通用事件路由。
func (c *Context) On(rules ...Rule) *Route {
	if c == nil || c.router == nil {
		return nil
	}
	return newRoute(c, c.router.On(coreRules(rules)...))
}

// OnMessage 注册消息事件路由。
func (c *Context) OnMessage(rules ...Rule) *Route {
	if c == nil || c.router == nil {
		return nil
	}
	return newRoute(c, c.router.OnMessage(coreRules(rules)...))
}

// Command 注册命令路由。
func (c *Context) Command(names ...string) *Route {
	if c == nil || c.router == nil {
		return nil
	}
	return newRoute(c, c.router.Command(names...))
}

// Observe 注册事件旁路观察者。
func (c *Context) Observe(rules ...Rule) *Observer {
	if c == nil || c.app == nil {
		return nil
	}
	return newObserver(c, c.app.Observe(coreRules(rules)...))
}

// Go 注册生命周期托管的后台任务。
func (c *Context) Go(name string, fn TaskFunc, opts ...TaskOption) {
	if c != nil && c.app != nil {
		c.app.Go(scopedName(c.PluginID(), name), fn, opts...)
	}
}

// Every 注册生命周期托管的周期任务。
func (c *Context) Every(name string, interval time.Duration, fn TaskFunc, opts ...TaskOption) {
	if c != nil && c.app != nil {
		c.app.Every(scopedName(c.PluginID(), name), interval, fn, opts...)
	}
}

// GoWhenActionReady 注册等待动作客户端可用后执行的生命周期托管后台任务。
func (c *Context) GoWhenActionReady(name string, fn TaskFunc, opts ...TaskOption) {
	if c != nil && c.app != nil {
		c.app.GoWhenActionReady(scopedName(c.PluginID(), name), fn, opts...)
	}
}

// OnStart 注册启动钩子，适配器启动前执行。
func (c *Context) OnStart(hook Hook) {
	if c != nil && c.app != nil {
		c.app.OnStart(hook)
	}
}

// OnReady 注册就绪钩子，事件调度器准备好后、适配器启动前执行。
func (c *Context) OnReady(hook Hook) {
	if c != nil && c.app != nil {
		c.app.OnReady(hook)
	}
}

// OnShutdown 注册关闭钩子。
func (c *Context) OnShutdown(hook Hook) {
	if c != nil && c.app != nil {
		c.app.OnShutdown(hook)
	}
}

// WaitActionReady 等待动作客户端可用。
func (c *Context) WaitActionReady(ctx context.Context) error {
	if c == nil || c.app == nil {
		return core.ErrActionUnavailable
	}
	return c.app.WaitActionReady(ctx)
}

// OnAdapterState 注册动作通道状态钩子。
func (c *Context) OnAdapterState(hook AdapterStateHook) {
	if c != nil && c.app != nil {
		c.app.OnAdapterState(hook)
	}
}

// Route 是插件 ID 命名空间下的一条事件路由。
type Route struct {
	ctx   *Context
	route *core.Route
}

func newRoute(ctx *Context, route *core.Route) *Route {
	if route == nil {
		return nil
	}
	return &Route{ctx: ctx, route: route}
}

// Name 设置路由名称，名称会自动落入当前插件命名空间。
func (r *Route) Name(name string) *Route {
	if r != nil && r.route != nil {
		r.route.Name(scopedName(r.pluginName(), name))
	}
	return r
}

// Priority 设置路由优先级；数值越大越先执行，同优先级保持注册顺序。
func (r *Route) Priority(priority int) *Route {
	if r != nil && r.route != nil {
		r.route.Priority(priority)
	}
	return r
}

// Use 追加只作用于当前路由的中间件。
func (r *Route) Use(middleware ...Middleware) *Route {
	if r != nil && r.route != nil {
		r.route.Use(coreMiddleware(middleware)...)
	}
	return r
}

// Handle 设置路由处理函数。
func (r *Route) Handle(handler Handler) *Route {
	if r != nil && r.route != nil {
		r.route.Handle(coreHandler(handler))
	}
	return r
}

func (r *Route) pluginName() string {
	if r == nil || r.ctx == nil {
		return ""
	}
	return r.ctx.PluginID()
}

// Observer 是插件 ID 命名空间下的一条旁路事件观察规则。
type Observer struct {
	ctx      *Context
	observer *core.Observer
}

func newObserver(ctx *Context, observer *core.Observer) *Observer {
	if observer == nil {
		return nil
	}
	return &Observer{ctx: ctx, observer: observer}
}

// Name 设置观察者名称，名称会自动落入当前插件命名空间。
func (o *Observer) Name(name string) *Observer {
	if o != nil && o.observer != nil {
		o.observer.Name(scopedName(o.pluginName(), name))
	}
	return o
}

// Handle 设置观察者处理函数。
func (o *Observer) Handle(handler ObserverHandler) *Observer {
	if o != nil && o.observer != nil {
		o.observer.Handle(handler)
	}
	return o
}

func (o *Observer) pluginName() string {
	if o == nil || o.ctx == nil {
		return ""
	}
	return o.ctx.PluginID()
}

// EventContext 是插件事件处理函数的上下文。
type EventContext struct {
	core *core.Context
}

func newEventContext(c *core.Context) *EventContext {
	if c == nil {
		return nil
	}
	return &EventContext{core: c}
}

// Context 返回当前事件处理链使用的标准 context。
func (c *EventContext) Context() context.Context {
	if c == nil || c.core == nil || c.core.Context == nil {
		return context.Background()
	}
	return c.core.Context
}

// UnsafeCoreContext 返回底层 core.Context。普通插件不应依赖它；它只适合协议适配、诊断或迁移代码。
func (c *EventContext) UnsafeCoreContext() *core.Context {
	if c == nil {
		return nil
	}
	return c.core
}

// Event 返回当前正在处理的标准化事件。
func (c *EventContext) Event() *Event {
	if c == nil || c.core == nil {
		return nil
	}
	return c.core.Event()
}

// Match 返回当前路由的匹配详情。
func (c *EventContext) Match() Match {
	if c == nil || c.core == nil {
		return Match{}
	}
	return c.core.Match()
}

// RouteName 返回当前路由的诊断名称。
func (c *EventContext) RouteName() string {
	if c == nil || c.core == nil {
		return ""
	}
	return c.core.RouteName()
}

// Set 写入当前插件处理链内可见的局部值。
func (c *EventContext) Set(key string, value any) {
	if c != nil && c.core != nil {
		c.core.Set(key, value)
	}
}

// Get 读取当前插件处理链内的局部值。
func (c *EventContext) Get(key string) (any, bool) {
	if c == nil || c.core == nil {
		return nil, false
	}
	return c.core.Get(key)
}

// String 读取当前插件处理链内的字符串局部值。
func (c *EventContext) String(key string) string {
	if c == nil || c.core == nil {
		return ""
	}
	return c.core.String(key)
}

// Command 返回命令规则匹配到的命令名。
func (c *EventContext) Command() string {
	if c == nil || c.core == nil {
		return ""
	}
	return c.core.Command()
}

// Args 返回命令名之后的原始参数文本。
func (c *EventContext) Args() string {
	if c == nil || c.core == nil {
		return ""
	}
	return c.core.Args()
}

// Argv 返回按空白拆分后的命令参数。
func (c *EventContext) Argv() []string {
	if c == nil || c.core == nil {
		return nil
	}
	return c.core.Argv()
}

// Var 读取当前规则匹配写入的变量。
func (c *EventContext) Var(key string) (any, bool) {
	if c == nil || c.core == nil {
		return nil, false
	}
	return c.core.Var(key)
}

// VarString 读取当前规则匹配写入的字符串变量。
func (c *EventContext) VarString(key string) string {
	if c == nil || c.core == nil {
		return ""
	}
	return c.core.VarString(key)
}

// ConversationID 返回当前事件对应的会话键。
func (c *EventContext) ConversationID() string {
	if c == nil || c.core == nil {
		return ""
	}
	return c.core.ConversationID()
}

// UserID 返回当前事件的用户 ID。
func (c *EventContext) UserID() string {
	if c == nil || c.core == nil {
		return ""
	}
	return c.core.UserID()
}

// SelfID 返回当前机器人账号 ID。
func (c *EventContext) SelfID() string {
	if c == nil || c.core == nil {
		return ""
	}
	return c.core.SelfID()
}

// GroupID 返回当前事件的群 ID。
func (c *EventContext) GroupID() string {
	if c == nil || c.core == nil {
		return ""
	}
	return c.core.GroupID()
}

// GroupRole 返回当前事件发送者在群内的标准角色，例如 owner、manager 或 member。
func (c *EventContext) GroupRole() string {
	if c == nil || c.core == nil {
		return ""
	}
	return c.core.GroupRole()
}

// HasPermission 判断当前事件是否满足一项标准权限标识。
func (c *EventContext) HasPermission(permission string) bool {
	if c == nil || c.core == nil {
		return false
	}
	return c.core.HasPermission(permission)
}

// HasAnyPermission 判断当前事件是否满足任一标准权限标识。
func (c *EventContext) HasAnyPermission(permissions ...string) bool {
	if c == nil || c.core == nil {
		return false
	}
	return c.core.HasAnyPermission(permissions...)
}

// IsPrivate 判断当前事件是否为私聊消息。
func (c *EventContext) IsPrivate() bool {
	return c != nil && c.core != nil && c.core.IsPrivate()
}

// IsGroup 判断当前事件是否为群消息。
func (c *EventContext) IsGroup() bool {
	return c != nil && c.core != nil && c.core.IsGroup()
}

// RawEvent 返回适配器保留的原始协议事件对象。
func (c *EventContext) RawEvent() any {
	if c == nil || c.core == nil {
		return nil
	}
	return c.core.RawEvent()
}

// Text 返回事件中的文本内容。
func (c *EventContext) Text() string {
	if c == nil || c.core == nil {
		return ""
	}
	return c.core.Text()
}

// Target 返回当前事件的自然回复目标。
func (c *EventContext) Target() ReplyTarget {
	if c == nil || c.core == nil {
		return ReplyTarget{}
	}
	return c.core.Target()
}

// Stop 停止当前事件继续传播到后续路由。
func (c *EventContext) Stop() {
	if c != nil && c.core != nil {
		c.core.Stop()
	}
}

// Stopped 报告当前事件是否已被要求停止传播。
func (c *EventContext) Stopped() bool {
	return c != nil && c.core != nil && c.core.Stopped()
}

// Pass 跳过当前路由且不视为失败。
func (c *EventContext) Pass() error {
	if c == nil || c.core == nil {
		return ErrPass
	}
	return c.core.Pass()
}

// StopError 返回可用于处理函数的停止错误，同时标记当前上下文已停止。
func (c *EventContext) StopError() error {
	if c == nil || c.core == nil {
		return ErrStop
	}
	return c.core.StopError()
}

// Reply 使用事件的自然目标发送消息。
func (c *EventContext) Reply(chain message.Chain) (MessageReceipt, error) {
	if c == nil || c.core == nil {
		return MessageReceipt{}, ErrReplyTargetUnavailable
	}
	return c.core.Reply(chain)
}

// ReplyText 使用事件的自然目标发送纯文本回复。
func (c *EventContext) ReplyText(text string) (MessageReceipt, error) {
	if c == nil || c.core == nil {
		return MessageReceipt{}, ErrReplyTargetUnavailable
	}
	return c.core.ReplyText(text)
}

// Handler 处理已经匹配成功的插件事件。
type Handler func(*EventContext) error

// Middleware 包装插件处理函数。
type Middleware func(Handler) Handler

// Rule 判断插件路由是否应处理当前事件。
type Rule interface {
	Match(context.Context, *EventContext) (Match, bool)
}

// RuleFunc 将普通函数适配为 Rule。
type RuleFunc func(context.Context, *EventContext) (Match, bool)

// Match 调用底层函数完成规则匹配。
func (f RuleFunc) Match(ctx context.Context, c *EventContext) (Match, bool) {
	return f(ctx, c)
}

func coreHandler(handler Handler) core.Handler {
	if handler == nil {
		return nil
	}
	return func(c *core.Context) error {
		return handler(newEventContext(c))
	}
}

func coreMiddleware(middleware []Middleware) []core.Middleware {
	out := make([]core.Middleware, 0, len(middleware))
	for _, item := range middleware {
		if item == nil {
			continue
		}
		item := item
		out = append(out, func(next core.Handler) core.Handler {
			wrapped := item(func(c *EventContext) error {
				if c == nil {
					return next(nil)
				}
				return next(c.UnsafeCoreContext())
			})
			return coreHandler(wrapped)
		})
	}
	return out
}

func coreRules(rules []Rule) []core.Rule {
	out := make([]core.Rule, 0, len(rules))
	for _, rule := range rules {
		if rule == nil {
			continue
		}
		rule := rule
		out = append(out, core.RuleFunc(func(ctx context.Context, c *core.Context) (Match, bool) {
			return rule.Match(ctx, newEventContext(c))
		}))
	}
	return out
}

func scopedName(pluginName, name string) string {
	pluginName = strings.TrimSpace(pluginName)
	name = strings.TrimSpace(name)
	if pluginName == "" {
		if name == "" {
			return "plugin"
		}
		return name
	}
	if name == "" {
		return pluginName
	}
	if name == pluginName || strings.HasPrefix(name, pluginName+".") {
		return name
	}
	return pluginName + "." + name
}
