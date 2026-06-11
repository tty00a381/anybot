package onebot11

import (
	"encoding/json"
	"fmt"

	"github.com/tty00a381/anybot/core"
	"github.com/tty00a381/anybot/core/message"
)

// Sender 描述 OneBot v11 消息发送者信息。
type Sender struct {
	UserID   int64  `json:"user_id,omitempty"`
	Nickname string `json:"nickname,omitempty"`
	Card     string `json:"card,omitempty"`
	Sex      string `json:"sex,omitempty"`
	Age      int    `json:"age,omitempty"`
	Area     string `json:"area,omitempty"`
	Level    string `json:"level,omitempty"`
	Role     string `json:"role,omitempty"`
	Title    string `json:"title,omitempty"`
}

// UnmarshalJSON 兼容数字或字符串形式的 user_id。
func (s *Sender) UnmarshalJSON(data []byte) error {
	var decoded struct {
		UserID   flexibleInt64 `json:"user_id"`
		Nickname string        `json:"nickname"`
		Card     string        `json:"card"`
		Sex      string        `json:"sex"`
		Age      int           `json:"age"`
		Area     string        `json:"area"`
		Level    string        `json:"level"`
		Role     string        `json:"role"`
		Title    string        `json:"title"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	s.UserID = decoded.UserID.int64()
	s.Nickname = decoded.Nickname
	s.Card = decoded.Card
	s.Sex = decoded.Sex
	s.Age = decoded.Age
	s.Area = decoded.Area
	s.Level = decoded.Level
	s.Role = decoded.Role
	s.Title = decoded.Title
	return nil
}

// Event 是宽松的 OneBot v11 事件模型，已知字段会被标准化，未知字段保留在 Extra。
type Event struct {
	Time       int64  `json:"time,omitempty"`
	SelfID     int64  `json:"self_id,omitempty"`
	PostType   string `json:"post_type,omitempty"`
	SubType    string `json:"sub_type,omitempty"`
	RawMessage string `json:"raw_message,omitempty"`

	MessageType string        `json:"message_type,omitempty"`
	MessageID   int64         `json:"message_id,omitempty"`
	UserID      int64         `json:"user_id,omitempty"`
	GroupID     int64         `json:"group_id,omitempty"`
	Message     message.Chain `json:"message,omitempty"`
	Sender      Sender        `json:"sender,omitempty"`

	NoticeType    string `json:"notice_type,omitempty"`
	RequestType   string `json:"request_type,omitempty"`
	MetaEventType string `json:"meta_event_type,omitempty"`

	OperatorID int64  `json:"operator_id,omitempty"`
	TargetID   int64  `json:"target_id,omitempty"`
	Comment    string `json:"comment,omitempty"`
	Flag       string `json:"flag,omitempty"`

	Extra map[string]json.RawMessage `json:"-"`
	Raw   json.RawMessage            `json:"-"`
}

// EventFrom 从 AnyBot 上下文提取 OneBot v11 原始事件。
//
// c 可以是 *core.Context，也可以是 SDK 的 *sdk.EventContext 这类暴露
// UnsafeCoreContext() *core.Context 的上下文。
func EventFrom(c any) (*Event, bool) {
	coreCtx := coreContextFrom(c)
	if coreCtx == nil {
		return nil, false
	}
	event, ok := coreCtx.RawEvent().(*Event)
	return event, ok
}

type unsafeCoreContexter interface {
	UnsafeCoreContext() *core.Context
}

func coreContextFrom(c any) *core.Context {
	switch ctx := c.(type) {
	case nil:
		return nil
	case *core.Context:
		return ctx
	case unsafeCoreContexter:
		return ctx.UnsafeCoreContext()
	default:
		return nil
	}
}

// UnmarshalJSON 保留未知字段，并同时接受数组消息与 CQ 字符串消息。
func (e *Event) UnmarshalJSON(data []byte) error {
	var decoded struct {
		Time          flexibleInt64   `json:"time"`
		SelfID        flexibleInt64   `json:"self_id"`
		PostType      string          `json:"post_type"`
		SubType       string          `json:"sub_type"`
		RawMessage    string          `json:"raw_message"`
		MessageType   string          `json:"message_type"`
		MessageID     flexibleInt64   `json:"message_id"`
		UserID        flexibleInt64   `json:"user_id"`
		GroupID       flexibleInt64   `json:"group_id"`
		Message       json.RawMessage `json:"message"`
		Sender        Sender          `json:"sender"`
		NoticeType    string          `json:"notice_type"`
		RequestType   string          `json:"request_type"`
		MetaEventType string          `json:"meta_event_type"`
		OperatorID    flexibleInt64   `json:"operator_id"`
		TargetID      flexibleInt64   `json:"target_id"`
		Comment       string          `json:"comment"`
		Flag          string          `json:"flag"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	e.Time = decoded.Time.int64()
	e.SelfID = decoded.SelfID.int64()
	e.PostType = decoded.PostType
	e.SubType = decoded.SubType
	e.RawMessage = decoded.RawMessage
	e.MessageType = decoded.MessageType
	e.MessageID = decoded.MessageID.int64()
	e.UserID = decoded.UserID.int64()
	e.GroupID = decoded.GroupID.int64()
	e.Sender = decoded.Sender
	e.NoticeType = decoded.NoticeType
	e.RequestType = decoded.RequestType
	e.MetaEventType = decoded.MetaEventType
	e.OperatorID = decoded.OperatorID.int64()
	e.TargetID = decoded.TargetID.int64()
	e.Comment = decoded.Comment
	e.Flag = decoded.Flag
	e.Message = nil
	if len(decoded.Message) > 0 {
		msg, err := parseMessage(decoded.Message)
		if err != nil {
			return err
		}
		e.Message = msg
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	for _, key := range []string{
		"time", "self_id", "post_type", "sub_type", "raw_message",
		"message_type", "message_id", "user_id", "group_id", "message", "sender",
		"notice_type", "request_type", "meta_event_type",
		"operator_id", "target_id", "comment", "flag",
	} {
		delete(raw, key)
	}
	e.Raw = append(e.Raw[:0], data...)
	e.Extra = nil
	if len(raw) > 0 {
		e.Extra = raw
	}
	return nil
}

// Normalize 将 OneBot v11 事件转换为 AnyBot 标准事件封套。
func (e *Event) Normalize() *core.Event {
	if e == nil {
		return nil
	}
	detail := e.SubType
	switch e.PostType {
	case "message":
		detail = e.MessageType
	case "notice":
		detail = e.NoticeType
	case "request":
		detail = e.RequestType
	case "meta_event":
		detail = e.MetaEventType
	}
	id := ""
	if e.MessageID != 0 {
		id = fmt.Sprint(e.MessageID)
	}
	text := e.Message.Text()
	if text == "" {
		text = e.RawMessage
	}
	return &core.Event{
		ID:         id,
		Protocol:   Protocol,
		SelfID:     idString(e.SelfID),
		Type:       e.PostType,
		DetailType: detail,
		SubType:    e.SubType,
		UserID:     idString(e.UserID),
		GroupID:    idString(e.GroupID),
		GroupRole:  normalizeGroupRole(e.Sender.Role),
		Text:       text,
		Message:    e.Message,
		Raw:        append(json.RawMessage(nil), e.Raw...),
		Data:       e,
	}
}

func normalizeGroupRole(role string) string {
	switch role {
	case "owner":
		return "owner"
	case "admin", "manager":
		return "manager"
	case "member":
		return "member"
	default:
		return ""
	}
}

func idString(id int64) string {
	if id == 0 {
		return ""
	}
	return fmt.Sprint(id)
}
