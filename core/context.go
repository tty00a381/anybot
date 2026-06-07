package core

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/tty00a381/anybot/core/message"
)

// Context 是路由处理函数的工作上下文，聚合事件、运行时能力、匹配结果和局部值。
type Context struct {
	context.Context

	app   *App
	event *Event
	match Match
	route *Route

	values    map[string]any
	matchVars map[string]any
	stopped   bool
}

func newContext(ctx context.Context, app *App, event *Event) *Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return &Context{
		Context: ctx,
		app:     app,
		event:   event,
		values:  map[string]any{},
	}
}

// NewTestContext 创建测试用上下文；app 为空时会使用默认配置的新 App。
func NewTestContext(app *App, event *Event) *Context {
	if app == nil {
		app = New()
	}
	return newContext(context.Background(), app, event)
}

// App 返回当前上下文所属的运行时。
func (c *Context) App() *App {
	return c.app
}

// Adapter 返回当前运行时配置的协议适配器。
func (c *Context) Adapter() Adapter {
	if c.app == nil {
		return nil
	}
	return c.app.Adapter()
}

// Event 返回当前正在处理的标准化事件。
func (c *Context) Event() *Event {
	return c.event
}

// Client 返回适配器动作客户端，可用于协议无关的消息发送。
func (c *Context) Client() ActionClient {
	if c.app == nil {
		return nil
	}
	return c.app.Client()
}

// Store 返回运行时配置的会话存储。
func (c *Context) Store() Store {
	if c.app == nil {
		return nil
	}
	return c.app.Store()
}

// Logger 返回运行时日志器；上下文未绑定运行时时返回 slog.Default。
func (c *Context) Logger() *slog.Logger {
	if c.app == nil {
		return slog.Default()
	}
	return c.app.Logger()
}

// Match 返回当前路由的匹配详情，包括评分、原因和规则写入的变量副本。
func (c *Context) Match() Match {
	out := c.match
	if len(out.Vars) > 0 {
		out.Vars = cloneVars(out.Vars)
	}
	return out
}

// Route 返回当前正在执行的路由。
func (c *Context) Route() *Route {
	return c.route
}

// RouteName 返回当前路由的诊断名称，未命名时返回空字符串。
func (c *Context) RouteName() string {
	return routeName(c.route)
}

// Set 写入当前路由处理链内可见的局部值。
func (c *Context) Set(key string, value any) {
	c.values[key] = value
}

// Get 读取当前路由处理链内的局部值。
func (c *Context) Get(key string) (any, bool) {
	value, ok := c.values[key]
	return value, ok
}

// String 读取当前路由处理链内的字符串局部值；不存在或类型不匹配时返回空字符串。
func (c *Context) String(key string) string {
	value, ok := c.Get(key)
	if !ok {
		return ""
	}
	text, _ := value.(string)
	return text
}

func (c *Context) applyMatch(route *Route, match Match) {
	c.clearMatch()
	c.route = route
	c.match = match
	c.matchVars = cloneVars(match.Vars)
}

func (c *Context) clearMatch() {
	c.route = nil
	c.match = Match{}
	c.matchVars = nil
	c.values = map[string]any{}
}

// Command 返回命令规则匹配到的命令名。
func (c *Context) Command() string {
	return c.matchString("command")
}

// Args 返回命令名之后的原始参数文本。
func (c *Context) Args() string {
	return c.matchString("args")
}

// Argv 返回按空白拆分后的命令参数，并保留简单引号与转义处理。
func (c *Context) Argv() []string {
	value, ok := c.matchVar("argv")
	if !ok {
		return nil
	}
	argv, _ := value.([]string)
	return append([]string(nil), argv...)
}

// Var 读取当前规则匹配写入的变量。
func (c *Context) Var(key string) (any, bool) {
	return c.matchVar(key)
}

// VarString 读取当前规则匹配写入的字符串变量；不存在或类型不匹配时返回空字符串。
func (c *Context) VarString(key string) string {
	return c.matchString(key)
}

func (c *Context) matchVar(key string) (any, bool) {
	value, ok := c.matchVars[key]
	return value, ok
}

func (c *Context) matchString(key string) string {
	value, ok := c.matchVar(key)
	if !ok {
		return ""
	}
	text, _ := value.(string)
	return text
}

func cloneVars(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

// ConversationID 返回当前事件对应的会话键，可直接用于会话存储。
func (c *Context) ConversationID() string {
	if c.event == nil {
		return ""
	}
	return c.event.ConversationID()
}

// UserID 返回当前事件的用户 ID，事件为空时返回空字符串。
func (c *Context) UserID() string {
	if c.event == nil {
		return ""
	}
	return c.event.UserID
}

// SelfID 返回当前机器人账号 ID，事件为空时返回空字符串。
func (c *Context) SelfID() string {
	if c.event == nil {
		return ""
	}
	return c.event.SelfID
}

// GroupID 返回当前事件的群 ID，非群事件返回空字符串。
func (c *Context) GroupID() string {
	if c.event == nil {
		return ""
	}
	return c.event.GroupID
}

// IsPrivate 判断当前事件是否为私聊消息。
func (c *Context) IsPrivate() bool {
	event := c.Event()
	return event != nil && event.Type == "message" && event.UserID != "" && event.GroupID == ""
}

// IsGroup 判断当前事件是否为群消息。
func (c *Context) IsGroup() bool {
	event := c.Event()
	return event != nil && event.Type == "message" && event.GroupID != ""
}

// RawEvent 返回适配器保留的原始协议事件对象，便于在需要时访问协议专属字段。
func (c *Context) RawEvent() any {
	if c.event == nil {
		return nil
	}
	return c.event.Data
}

// Text 返回事件中的文本内容，优先使用标准化文本，再回退到消息链文本。
func (c *Context) Text() string {
	if c.event == nil {
		return ""
	}
	if c.event.Text != "" {
		return c.event.Text
	}
	return c.event.Message.Text()
}

// Target 返回当前事件的自然回复目标。
func (c *Context) Target() ReplyTarget {
	if c.event == nil {
		return ReplyTarget{}
	}
	return c.event.Target()
}

// Stop 停止当前事件继续传播到后续路由。
func (c *Context) Stop() {
	c.stopped = true
}

// Stopped 报告当前事件是否已被要求停止传播。
func (c *Context) Stopped() bool {
	return c.stopped
}

// Pass 跳过当前路由且不视为失败，后续路由仍会继续匹配。
func (c *Context) Pass() error {
	return ErrPass
}

// StopError 返回可用于处理函数的停止错误，同时标记当前上下文已停止。
func (c *Context) StopError() error {
	c.Stop()
	return ErrStop
}

// Session 返回当前会话维度的存储视图。
func (c *Context) Session() *Session {
	key := "session"
	if c.event != nil {
		key = c.event.ConversationID()
	}
	return c.SessionBy(key)
}

// UserSession 返回当前用户维度的存储视图。
func (c *Context) UserSession() *Session {
	key := "session:user"
	if c.event != nil && c.event.UserSessionID() != "" {
		key = c.event.UserSessionID()
	}
	return c.SessionBy(key)
}

// GroupSession 返回当前群或频道维度的存储视图。
func (c *Context) GroupSession() *Session {
	key := "session:group"
	if c.event != nil && c.event.GroupSessionID() != "" {
		key = c.event.GroupSessionID()
	}
	return c.SessionBy(key)
}

// SessionBy 返回指定键空间下的存储视图；key 为空时使用默认会话键。
func (c *Context) SessionBy(key string) *Session {
	if key == "" {
		key = "session"
	}
	return NewSession(c.Store(), key)
}

// Reply 使用事件的自然目标发送消息。
func (c *Context) Reply(chain message.Chain) (MessageReceipt, error) {
	if c.event == nil {
		return MessageReceipt{}, errors.New("anybot: cannot reply without event")
	}
	client := c.Client()
	if client == nil {
		return MessageReceipt{}, errors.New("anybot: no action client configured")
	}
	target := c.event.Target()
	if target.UserID == "" && target.GroupID == "" && target.ChannelID == "" {
		return MessageReceipt{}, fmt.Errorf("anybot: cannot reply to event type %q", c.event.Type)
	}
	return client.Send(c.Context, target, chain)
}

// ReplyText 使用事件的自然目标发送纯文本回复。
func (c *Context) ReplyText(text string) (MessageReceipt, error) {
	return c.Reply(message.New(message.Text(text)))
}
