package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	defaultDialogueName     = "default"
	defaultDialoguePriority = 1000

	dialogueRecordKey = "anybot.sdk.dialogue.record"
	dialogueErrorKey  = "anybot.sdk.dialogue.error"
)

// DialogueScope 描述多轮对话状态绑定的会话维度。
type DialogueScope int

const (
	// DialogueScopeConversation 将状态绑定到当前自然会话，默认用于一对一的多轮聊天。
	DialogueScopeConversation DialogueScope = iota
	// DialogueScopeUser 将状态绑定到用户维度，适合跨群或跨频道延续的私有偏好流程。
	DialogueScopeUser
	// DialogueScopeGroup 将状态绑定到群或频道维度，适合多人共同推进的群聊流程。
	DialogueScopeGroup
)

// DialogueOption 配置 SDK 多轮对话。
type DialogueOption func(*dialogueOptions)

type dialogueOptions struct {
	ttl      time.Duration
	priority int
	rules    []Rule
	scope    DialogueScope
}

// DialogueWithTTL 设置对话状态的过期时间；零值表示不过期。
func DialogueWithTTL(ttl time.Duration) DialogueOption {
	return func(options *dialogueOptions) {
		options.ttl = ttl
	}
}

// DialogueWithPriority 设置恢复对话的路由优先级；默认优先于普通命令和消息路由。
func DialogueWithPriority(priority int) DialogueOption {
	return func(options *dialogueOptions) {
		options.priority = priority
	}
}

// DialogueWithRules 为恢复中的对话追加约束规则，例如 ToMe 或 NotFromSelf。
func DialogueWithRules(rules ...Rule) DialogueOption {
	return func(options *dialogueOptions) {
		options.rules = append(options.rules, rules...)
	}
}

// DialogueWithScope 设置对话状态绑定的会话维度。
func DialogueWithScope(scope DialogueScope) DialogueOption {
	return func(options *dialogueOptions) {
		options.scope = scope
	}
}

// DialogueHandler 处理一个已恢复的对话步骤。
type DialogueHandler func(*DialogueTurn) error

// Dialogue 是 SDK 提供的多轮对话控制器。
type Dialogue struct {
	ctx      *Context
	name     string
	ttl      time.Duration
	priority int
	rules    []Rule
	scope    DialogueScope
	route    *Route

	mu       sync.RWMutex
	handlers map[string]DialogueHandler
}

// Dialogue 创建插件命名空间内的多轮对话控制器，并注册用于恢复对话的高优先级消息路由。
func (c *Context) Dialogue(name string, opts ...DialogueOption) *Dialogue {
	options := dialogueOptions{
		priority: defaultDialoguePriority,
		scope:    DialogueScopeConversation,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = defaultDialogueName
	}
	dialogue := &Dialogue{
		ctx:      c,
		name:     name,
		ttl:      options.ttl,
		priority: options.priority,
		rules:    append([]Rule(nil), options.rules...),
		scope:    options.scope,
		handlers: map[string]DialogueHandler{},
	}
	if c != nil {
		rules := append([]Rule{dialogue.activeRule()}, dialogue.rules...)
		dialogue.route = c.OnMessage(rules...)
		if dialogue.route != nil {
			dialogue.route.
				Name(scopedName(c.manifest.Name, "dialogue:"+dialogue.name)).
				Priority(dialogue.priority).
				Handle(dialogue.handle)
		}
	}
	return dialogue
}

// Name 返回对话控制器名称。
func (d *Dialogue) Name() string {
	if d == nil {
		return ""
	}
	return d.name
}

// Route 返回恢复对话所使用的路由，便于高级插件进一步命名或调优。
func (d *Dialogue) Route() *Route {
	if d == nil {
		return nil
	}
	return d.route
}

// Step 注册一个对话步骤处理函数。
func (d *Dialogue) Step(name string, handler DialogueHandler) *Dialogue {
	if d == nil {
		return d
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return d
	}
	d.mu.Lock()
	d.handlers[name] = handler
	d.mu.Unlock()
	return d
}

// Begin 将当前事件所在会话推进到指定对话步骤。
func (d *Dialogue) Begin(c *EventContext, step string, data any) error {
	if err := d.saveNewState(c, step, data); err != nil {
		return err
	}
	c.Stop()
	return nil
}

// BeginText 开始对话并回复一条提示文本。
func (d *Dialogue) BeginText(c *EventContext, step string, data any, prompt string) error {
	if err := d.Begin(c, step, data); err != nil {
		return err
	}
	_, err := c.ReplyText(prompt)
	return err
}

// End 清除当前事件所在会话中的对话状态。
func (d *Dialogue) End(c *EventContext) error {
	if d == nil {
		return errors.New("anybot: dialogue is nil")
	}
	session, err := d.session(c)
	if err != nil {
		return err
	}
	return session.Delete(c.Context, d.storeKey())
}

// Active 返回当前事件所在会话中的对话状态。
func (d *Dialogue) Active(c *EventContext) (DialogueSnapshot, bool, error) {
	record, ok, err := d.load(c)
	if err != nil || !ok {
		return DialogueSnapshot{}, ok, err
	}
	return record.snapshot(d.name), true, nil
}

func (d *Dialogue) activeRule() Rule {
	return RuleFunc(func(ctx context.Context, c *EventContext) (Match, bool) {
		record, ok, err := d.load(c)
		if err != nil {
			return Match{Score: 100, Reason: "dialogue:" + d.name + ":error", Vars: map[string]any{
				dialogueErrorKey: err,
			}}, true
		}
		if !ok {
			return Match{}, false
		}
		return Match{Score: 100, Reason: "dialogue:" + d.name, Vars: map[string]any{
			dialogueRecordKey: record,
		}}, true
	})
}

func (d *Dialogue) handle(c *EventContext) error {
	if value, ok := c.Get(dialogueErrorKey); ok {
		if err, ok := value.(error); ok {
			c.Stop()
			return err
		}
	}
	record, ok := dialogueRecordFromContext(c)
	if !ok {
		loaded, active, err := d.load(c)
		if err != nil {
			return err
		}
		if !active {
			return ErrPass
		}
		record = loaded
	}
	handler := d.handler(record.Step)
	if handler == nil {
		c.Stop()
		return fmt.Errorf("dialogue %s step %q has no handler", d.name, record.Step)
	}
	turn := &DialogueTurn{
		EventContext: c,
		dialogue:     d,
		record:       record,
	}
	err := handler(turn)
	switch {
	case errors.Is(err, ErrPass):
		return err
	case err != nil:
		c.Stop()
		return err
	default:
		if !c.Stopped() {
			c.Stop()
		}
		return nil
	}
}

func (d *Dialogue) handler(step string) DialogueHandler {
	d.mu.RLock()
	handler := d.handlers[step]
	d.mu.RUnlock()
	return handler
}

func (d *Dialogue) hasStep(step string) bool {
	d.mu.RLock()
	_, ok := d.handlers[step]
	d.mu.RUnlock()
	return ok
}

func (d *Dialogue) saveNewState(c *EventContext, step string, data any) error {
	if d == nil {
		return errors.New("anybot: dialogue is nil")
	}
	step = strings.TrimSpace(step)
	if step == "" {
		return fmt.Errorf("dialogue %s step is required", d.name)
	}
	if !d.hasStep(step) {
		return fmt.Errorf("dialogue %s step %q is not registered", d.name, step)
	}
	record, err := newDialogueRecord(step, data)
	if err != nil {
		return fmt.Errorf("dialogue %s state: %w", d.name, err)
	}
	return d.save(c, record)
}

func (d *Dialogue) load(c *EventContext) (dialogueRecord, bool, error) {
	if d == nil {
		return dialogueRecord{}, false, errors.New("anybot: dialogue is nil")
	}
	session, err := d.session(c)
	if err != nil {
		return dialogueRecord{}, false, err
	}
	var record dialogueRecord
	ok, err := session.LoadJSON(c.Context, d.storeKey(), &record)
	if err != nil || !ok {
		return dialogueRecord{}, ok, err
	}
	if strings.TrimSpace(record.Step) == "" {
		return dialogueRecord{}, false, fmt.Errorf("dialogue %s has empty step", d.name)
	}
	return record, true, nil
}

func (d *Dialogue) save(c *EventContext, record dialogueRecord) error {
	session, err := d.session(c)
	if err != nil {
		return err
	}
	return session.SaveJSON(c.Context, d.storeKey(), record, d.ttl)
}

func (d *Dialogue) session(c *EventContext) (*Session, error) {
	if d == nil || d.ctx == nil {
		return nil, errors.New("anybot: dialogue context is nil")
	}
	if c == nil {
		return nil, errors.New("anybot: dialogue event context is nil")
	}
	switch d.scope {
	case DialogueScopeUser:
		return d.ctx.UserSession(c), nil
	case DialogueScopeGroup:
		return d.ctx.GroupSession(c), nil
	default:
		return d.ctx.Session(c), nil
	}
}

func (d *Dialogue) storeKey() string {
	return "dialogue:" + d.name
}

func dialogueRecordFromContext(c *EventContext) (dialogueRecord, bool) {
	value, ok := c.Get(dialogueRecordKey)
	if !ok {
		return dialogueRecord{}, false
	}
	record, ok := value.(dialogueRecord)
	return record, ok
}

type dialogueRecord struct {
	Step      string          `json:"step"`
	Data      json.RawMessage `json:"data,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

func newDialogueRecord(step string, data any) (dialogueRecord, error) {
	raw, err := marshalDialogueData(data)
	if err != nil {
		return dialogueRecord{}, err
	}
	now := time.Now().UTC()
	return dialogueRecord{
		Step:      step,
		Data:      raw,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func marshalDialogueData(data any) (json.RawMessage, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return append(json.RawMessage(nil), raw...), nil
}

func (r dialogueRecord) snapshot(name string) DialogueSnapshot {
	return DialogueSnapshot{
		Name:      name,
		Step:      r.Step,
		Data:      append(json.RawMessage(nil), r.Data...),
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
	}
}

// DialogueSnapshot 是对话状态的只读快照。
type DialogueSnapshot struct {
	Name      string
	Step      string
	Data      json.RawMessage
	CreatedAt time.Time
	UpdatedAt time.Time
}

// DialogueTurn 是一次已恢复对话步骤的处理上下文。
type DialogueTurn struct {
	*EventContext

	dialogue *Dialogue
	record   dialogueRecord
}

// Dialogue 返回当前对话控制器。
func (t *DialogueTurn) Dialogue() *Dialogue {
	if t == nil {
		return nil
	}
	return t.dialogue
}

// Step 返回当前正在处理的对话步骤名。
func (t *DialogueTurn) Step() string {
	if t == nil {
		return ""
	}
	return t.record.Step
}

// State 返回当前对话状态快照。
func (t *DialogueTurn) State() DialogueSnapshot {
	if t == nil || t.dialogue == nil {
		return DialogueSnapshot{}
	}
	return t.record.snapshot(t.dialogue.name)
}

// Load 将当前步骤携带的数据解码到 out。
func (t *DialogueTurn) Load(out any) error {
	if t == nil {
		return errors.New("anybot: dialogue turn is nil")
	}
	if len(t.record.Data) == 0 || out == nil {
		return nil
	}
	return json.Unmarshal(t.record.Data, out)
}

// Next 将当前会话推进到另一个步骤，并替换步骤数据。
func (t *DialogueTurn) Next(step string, data any) error {
	if t == nil || t.dialogue == nil {
		return errors.New("anybot: dialogue turn is nil")
	}
	record, err := t.dialogue.nextRecord(t.record, step, data)
	if err != nil {
		return err
	}
	if err := t.dialogue.save(t.EventContext, record); err != nil {
		return err
	}
	t.record = record
	return nil
}

// NextText 推进到另一个步骤并回复一条提示文本。
func (t *DialogueTurn) NextText(step string, data any, prompt string) error {
	if err := t.Next(step, data); err != nil {
		return err
	}
	_, err := t.ReplyText(prompt)
	return err
}

// End 清除当前会话中的对话状态。
func (t *DialogueTurn) End() error {
	if t == nil || t.dialogue == nil {
		return errors.New("anybot: dialogue turn is nil")
	}
	return t.dialogue.End(t.EventContext)
}

// EndText 清除对话状态并回复一条文本。
func (t *DialogueTurn) EndText(text string) error {
	if err := t.End(); err != nil {
		return err
	}
	_, err := t.ReplyText(text)
	return err
}

func (d *Dialogue) nextRecord(current dialogueRecord, step string, data any) (dialogueRecord, error) {
	step = strings.TrimSpace(step)
	if step == "" {
		return dialogueRecord{}, fmt.Errorf("dialogue %s step is required", d.name)
	}
	if !d.hasStep(step) {
		return dialogueRecord{}, fmt.Errorf("dialogue %s step %q is not registered", d.name, step)
	}
	raw, err := marshalDialogueData(data)
	if err != nil {
		return dialogueRecord{}, fmt.Errorf("dialogue %s state: %w", d.name, err)
	}
	now := time.Now().UTC()
	if current.CreatedAt.IsZero() {
		current.CreatedAt = now
	}
	return dialogueRecord{
		Step:      step,
		Data:      raw,
		CreatedAt: current.CreatedAt,
		UpdatedAt: now,
	}, nil
}
