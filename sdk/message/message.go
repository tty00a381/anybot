// Package message 为插件 SDK 用户暴露 AnyBot 的协议中立消息模型。
package message

import coremsg "github.com/tty00a381/anybot/core/message"

// Segment 表示一个协议中立的消息段。
type Segment = coremsg.Segment

// Chain 是按顺序排列的协议中立消息段。
type Chain = coremsg.Chain

// New 根据消息段构造新的消息链。
func New(segments ...Segment) Chain { return coremsg.New(segments...) }

// Raw 创建带自定义类型和数据的消息段。
func Raw(kind string, data map[string]any) Segment { return coremsg.Raw(kind, data) }

// Text 创建纯文本消息段。
func Text(text string) Segment { return coremsg.Text(text) }

// Image 创建图片消息段。
func Image(file string) Segment { return coremsg.Image(file) }

// Video 创建视频消息段。
func Video(file string) Segment { return coremsg.Video(file) }
