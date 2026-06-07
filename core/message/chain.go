// message 包定义 AnyBot 的协议中立消息链。
package message

import (
	"encoding/json"
	"strings"
)

// Segment 表示一个协议中立的消息段。
type Segment struct {
	Type string         `json:"type"`
	Data map[string]any `json:"data,omitempty"`
}

// Chain 是按顺序排列的协议中立消息段。
type Chain []Segment

// New 根据消息段构造新的消息链。
func New(segments ...Segment) Chain {
	return append(Chain(nil), segments...)
}

// Raw 创建带自定义类型和数据的消息段，供适配器或协议扩展使用。
func Raw(kind string, data map[string]any) Segment {
	return Segment{Type: kind, Data: cloneMap(data)}
}

// Text 创建纯文本消息段。
func Text(text string) Segment {
	return Segment{Type: "text", Data: map[string]any{"text": text}}
}

// Image 创建图片消息段，file 通常为 URL、file URI 或协议端可识别的路径。
func Image(file string) Segment {
	return Segment{Type: "image", Data: map[string]any{"file": file}}
}

// Video 创建视频消息段。
func Video(file string) Segment {
	return Segment{Type: "video", Data: map[string]any{"file": file}}
}

// Append 返回追加消息段后的新消息链，不修改原链。
func (c Chain) Append(segments ...Segment) Chain {
	out := append(Chain(nil), c...)
	return append(out, segments...)
}

// Clone 返回消息链副本，避免异步观察者和处理链共享可变段数据。
func (c Chain) Clone() Chain {
	if c == nil {
		return nil
	}
	out := make(Chain, len(c))
	for i, seg := range c {
		out[i] = Segment{Type: seg.Type, Data: cloneMap(seg.Data)}
	}
	return out
}

// Text 返回所有文本消息段按顺序拼接后的内容。
func (c Chain) Text() string {
	var b strings.Builder
	for _, seg := range c {
		if seg.Type != "text" || seg.Data == nil {
			continue
		}
		if text, ok := seg.Data["text"].(string); ok {
			b.WriteString(text)
		}
	}
	return b.String()
}

// IsZero 判断消息链是否为空。
func (c Chain) IsZero() bool {
	return len(c) == 0
}

// MarshalJSON 将 nil 消息链编码为空数组，便于适配器直接作为消息参数传递。
func (c Chain) MarshalJSON() ([]byte, error) {
	if c == nil {
		return []byte("[]"), nil
	}
	type segment Segment
	out := make([]segment, len(c))
	for i, seg := range c {
		out[i] = segment(seg)
	}
	return json.Marshal(out)
}

func cloneMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
