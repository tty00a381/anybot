// testkit 包提供插件单元测试用的轻量 AnyBot 运行时。
package testkit

import (
	"context"
	"errors"

	"github.com/tty00a381/anybot/sdk"
	"github.com/tty00a381/anybot/sdk/message"
)

const defaultProtocol sdk.Protocol = "test"

// ErrAppUnavailable 表示测试运行时不可用。
var ErrAppUnavailable = errors.New("anybot testkit: app is unavailable")

// Option 调整测试运行时。
type Option func(*options)

type options struct {
	superUsers []string
}

// WithSuperUsers 配置测试运行时的超级用户。
func WithSuperUsers(ids ...string) Option {
	return func(opts *options) {
		opts.superUsers = append([]string(nil), ids...)
	}
}

// App 是插件测试用运行时，记录插件通过 Reply 或 Send 发出的消息。
type App struct {
	runtime *sdk.App
	client  *Client
}

// NewApp 创建带记录客户端的测试运行时。
func NewApp(opts ...Option) *App {
	var cfg options
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	client := &Client{}
	runtimeOptions := []sdk.Option{sdk.WithAdapter(adapter{client: client})}
	if len(cfg.superUsers) > 0 {
		runtimeOptions = append(runtimeOptions, sdk.WithSuperUsers(cfg.superUsers...))
	}
	return &App{
		runtime: sdk.NewApp(runtimeOptions...),
		client:  client,
	}
}

// Runtime 返回底层运行时，供少数需要直接调度或检查 App 状态的测试使用。
func (app *App) Runtime() *sdk.App {
	if app == nil {
		return nil
	}
	return app.runtime
}

// InstallDefault 用插件默认配置安装插件定义。
func (app *App) InstallDefault(definitions ...sdk.Definition) error {
	return sdk.InstallDefault(app.Runtime(), definitions...)
}

// Install 安装已按测试配置构建好的插件实例。
func (app *App) Install(plugins ...sdk.Plugin) error {
	return sdk.Install(app.Runtime(), plugins...)
}

// DispatchText 投递一条消息事件。
func (app *App) DispatchText(text string, opts ...EventOption) error {
	return app.Dispatch(context.Background(), Message(text, opts...))
}

// Dispatch 投递自定义事件。
func (app *App) Dispatch(ctx context.Context, event *sdk.Event) error {
	if app == nil || app.runtime == nil {
		return ErrAppUnavailable
	}
	return app.runtime.Dispatch(ctx, event)
}

// Client 返回记录客户端。
func (app *App) Client() *Client {
	if app == nil {
		return nil
	}
	return app.client
}

// LastReplyText 返回最后一条发送消息的纯文本内容。
func (app *App) LastReplyText() string {
	if app == nil || app.client == nil {
		return ""
	}
	return app.client.LastText()
}

// Message 创建一条测试消息事件。
func Message(text string, opts ...EventOption) *sdk.Event {
	event := &sdk.Event{
		Protocol: defaultProtocol,
		SelfID:   "bot",
		Type:     "message",
		UserID:   "user",
		Text:     text,
		Message:  message.New(message.Text(text)),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(event)
		}
	}
	return event
}

// EventOption 调整测试事件。
type EventOption func(*sdk.Event)

// FromUser 设置消息发送者 ID。
func FromUser(id string) EventOption {
	return func(event *sdk.Event) {
		event.UserID = id
	}
}

// InGroup 设置群 ID，使事件成为群消息。
func InGroup(id string) EventOption {
	return func(event *sdk.Event) {
		event.GroupID = id
	}
}

// WithMessage 设置完整消息链，并同步 Text 为消息链中的纯文本内容。
func WithMessage(chain message.Chain) EventOption {
	return func(event *sdk.Event) {
		event.Message = chain.Clone()
		event.Text = event.Message.Text()
	}
}

// Mention 追加一个 at 消息段，便于测试 Mentioned 和 ToMe 规则。
func Mention(id string) EventOption {
	return func(event *sdk.Event) {
		event.Message = event.Message.Append(message.Raw("at", map[string]any{"id": id}))
	}
}

// MentionSelf 追加一个指向当前机器人账号的 at 消息段。
func MentionSelf() EventOption {
	return func(event *sdk.Event) {
		id := event.SelfID
		if id == "" {
			id = "bot"
		}
		event.Message = event.Message.Append(message.Raw("at", map[string]any{"id": id}))
	}
}

// WithSelfID 设置当前机器人账号 ID。
func WithSelfID(id string) EventOption {
	return func(event *sdk.Event) {
		event.SelfID = id
	}
}

// WithProtocol 设置协议标识。
func WithProtocol(protocol sdk.Protocol) EventOption {
	return func(event *sdk.Event) {
		event.Protocol = protocol
	}
}

type adapter struct {
	client *Client
}

func (a adapter) Protocol() sdk.Protocol { return defaultProtocol }

func (a adapter) Start(context.Context, sdk.EmitFunc) error {
	return nil
}

func (a adapter) Client() sdk.ActionClient { return a.client }

// SentMessage 是测试客户端记录到的一条发送消息。
type SentMessage struct {
	Target sdk.ReplyTarget
	Chain  message.Chain
}

// Client 是记录发送消息的测试动作客户端。
type Client struct {
	sent []SentMessage
}

// Send 记录一条发送消息。
func (c *Client) Send(_ context.Context, target sdk.ReplyTarget, chain message.Chain) (sdk.MessageReceipt, error) {
	if c != nil {
		c.sent = append(c.sent, SentMessage{Target: target, Chain: chain.Clone()})
	}
	return sdk.MessageReceipt{ID: "test"}, nil
}

// Sent 返回所有已发送消息的副本。
func (c *Client) Sent() []SentMessage {
	if c == nil {
		return nil
	}
	out := make([]SentMessage, len(c.sent))
	for i, item := range c.sent {
		out[i] = SentMessage{Target: item.Target, Chain: item.Chain.Clone()}
	}
	return out
}

// Last 返回最后一条发送消息。
func (c *Client) Last() (SentMessage, bool) {
	if c == nil || len(c.sent) == 0 {
		return SentMessage{}, false
	}
	item := c.sent[len(c.sent)-1]
	return SentMessage{Target: item.Target, Chain: item.Chain.Clone()}, true
}

// LastText 返回最后一条发送消息的纯文本内容。
func (c *Client) LastText() string {
	item, ok := c.Last()
	if !ok {
		return ""
	}
	return item.Chain.Text()
}
