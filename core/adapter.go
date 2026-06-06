package core

import (
	"context"
	"encoding/json"

	"github.com/tty00a381/anybot/core/message"
)

// EmitFunc 是适配器向 AnyBot 运行时投递标准化事件的函数。
type EmitFunc func(context.Context, *Event) error

// Adapter 把具体聊天协议接入 AnyBot 运行时。
type Adapter interface {
	Protocol() Protocol
	Start(context.Context, EmitFunc) error
	Client() ActionClient
}

// ActionClient 定义协议动作调用和消息发送能力。
type ActionClient interface {
	Call(context.Context, string, any, any) error
	CallRaw(context.Context, string, any) (*ActionResponse, error)
	Send(context.Context, ReplyTarget, message.Chain) (MessageReceipt, error)
}

// ReplyTarget 描述一次消息回复的自然目标。
type ReplyTarget struct {
	Protocol  Protocol
	SelfID    string
	UserID    string
	GroupID   string
	GuildID   string
	ChannelID string
	EventID   string
}

// MessageReceipt 是协议无关的消息发送回执。
type MessageReceipt struct {
	ID string
}

// ActionResponse 是协议无关的动作响应封套，保留标准字段和原始 data。
type ActionResponse struct {
	Status  string
	RetCode int
	Message string
	Wording string
	Echo    string
	Data    json.RawMessage
	Raw     json.RawMessage
}

// OK 判断动作响应是否表示成功。
func (r *ActionResponse) OK() bool {
	return r != nil && (r.Status == "" || r.Status == "ok") && r.RetCode == 0
}

// Decode 将动作响应的 data 字段解码到 out；空 data 会被视为成功空值。
func (r *ActionResponse) Decode(out any) error {
	if out == nil || r == nil || len(r.Data) == 0 || string(r.Data) == "null" {
		return nil
	}
	return json.Unmarshal(r.Data, out)
}
