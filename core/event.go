package core

import (
	"encoding/json"
	"fmt"

	"github.com/tty00a381/anybot/core/message"
)

// Protocol 标识事件和动作所属的协议，具体取值由适配器包定义。
type Protocol string

// Event 是 AnyBot 的协议无关事件封套，保留常用标准字段和原始协议数据。
type Event struct {
	ID         string
	Protocol   Protocol
	SelfID     string
	Type       string
	DetailType string
	SubType    string

	UserID    string
	GroupID   string
	GuildID   string
	ChannelID string

	Text    string
	Message message.Chain

	Raw  json.RawMessage
	Data any
}

// Clone 返回事件副本，适合交给异步观察者或后台任务保存。
func (e *Event) Clone() *Event {
	if e == nil {
		return nil
	}
	out := *e
	out.Raw = append(json.RawMessage(nil), e.Raw...)
	out.Message = e.Message.Clone()
	return &out
}

// DecodeRaw 将原始协议事件 JSON 解码到 out；没有原始数据时直接返回 nil。
func (e *Event) DecodeRaw(out any) error {
	if e == nil || len(e.Raw) == 0 {
		return nil
	}
	return json.Unmarshal(e.Raw, out)
}

// ConversationID 返回适合会话存储的稳定键，优先按频道、群聊、私聊逐级区分。
func (e *Event) ConversationID() string {
	if e == nil {
		return ""
	}
	switch {
	case e.GuildID != "" || e.ChannelID != "":
		return fmt.Sprintf("%s:guild:%s:channel:%s:user:%s", e.Protocol, e.GuildID, e.ChannelID, e.UserID)
	case e.GroupID != "":
		return fmt.Sprintf("%s:group:%s:user:%s", e.Protocol, e.GroupID, e.UserID)
	case e.UserID != "":
		return fmt.Sprintf("%s:private:%s", e.Protocol, e.UserID)
	default:
		return fmt.Sprintf("%s:self:%s:%s:%s", e.Protocol, e.SelfID, e.Type, e.DetailType)
	}
}

// UserSessionID 返回按用户维度隔离的存储键。
func (e *Event) UserSessionID() string {
	if e == nil || e.UserID == "" {
		return ""
	}
	return fmt.Sprintf("%s:user:%s", e.Protocol, e.UserID)
}

// GroupSessionID 返回按群或频道维度隔离的存储键。
func (e *Event) GroupSessionID() string {
	if e == nil {
		return ""
	}
	switch {
	case e.GuildID != "" || e.ChannelID != "":
		return fmt.Sprintf("%s:guild:%s:channel:%s", e.Protocol, e.GuildID, e.ChannelID)
	case e.GroupID != "":
		return fmt.Sprintf("%s:group:%s", e.Protocol, e.GroupID)
	default:
		return ""
	}
}

// Target 返回该事件的自然回复目标，供 Context.Reply 和适配器发送消息使用。
func (e *Event) Target() ReplyTarget {
	if e == nil {
		return ReplyTarget{}
	}
	return ReplyTarget{
		Protocol:  e.Protocol,
		SelfID:    e.SelfID,
		UserID:    e.UserID,
		GroupID:   e.GroupID,
		GuildID:   e.GuildID,
		ChannelID: e.ChannelID,
		EventID:   e.ID,
	}
}

// IsMessage 判断该事件是否为消息事件。
func (e *Event) IsMessage() bool {
	return e != nil && e.Type == "message"
}
