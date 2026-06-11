package core

import (
	"context"

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

// ActionClient 定义协议无关的消息发送能力。
type ActionClient interface {
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
