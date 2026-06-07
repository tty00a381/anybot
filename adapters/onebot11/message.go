package onebot11

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/tty00a381/anybot/core/message"
)

// Message 是 OneBot v11 消息链类型别名。
type Message = message.Chain

// Text 创建 OneBot v11 文本消息段。
func Text(text string) message.Segment { return message.Text(text) }

// At 创建 OneBot v11 提及消息段。
func At(qq any) message.Segment { return message.Raw("at", map[string]any{"id": fmt.Sprint(qq)}) }

// Reply 创建 OneBot v11 回复消息段。
func Reply(id any) message.Segment { return message.Raw("reply", map[string]any{"id": fmt.Sprint(id)}) }

// Image 创建 OneBot v11 图片消息段。
func Image(file string) message.Segment { return message.Image(file) }

// Face 创建 OneBot v11 表情消息段。
func Face(id any) message.Segment { return message.Raw("face", map[string]any{"id": fmt.Sprint(id)}) }

// Record 创建 OneBot v11 语音消息段。
func Record(file string) message.Segment { return message.Raw("record", map[string]any{"file": file}) }

// Video 创建 OneBot v11 视频消息段。
func Video(file string) message.Segment { return message.Video(file) }

// JSON 创建 OneBot v11 JSON 消息段。
func JSON(data string) message.Segment { return message.Raw("json", map[string]any{"data": data}) }

// XML 创建 OneBot v11 XML 消息段。
func XML(data string) message.Segment { return message.Raw("xml", map[string]any{"data": data}) }

// Share 创建 OneBot v11 分享消息段。
func Share(url, title, content, image string) message.Segment {
	return message.Raw("share", map[string]any{"url": url, "title": title, "content": content, "image": image})
}

// Music 创建 OneBot v11 音乐消息段。
func Music(kind string, id any) message.Segment {
	return message.Raw("music", map[string]any{"type": kind, "id": fmt.Sprint(id)})
}

// CustomMusic 创建 OneBot v11 自定义音乐消息段。
func CustomMusic(url, audio, title, content, image string) message.Segment {
	return message.Raw("music", map[string]any{
		"type":    "custom",
		"url":     url,
		"audio":   audio,
		"title":   title,
		"content": content,
		"image":   image,
	})
}

// Node 创建合并转发引用节点。
func Node(id any) message.Segment { return message.Raw("node", map[string]any{"id": fmt.Sprint(id)}) }

// CustomNode 创建合并转发自定义节点。
func CustomNode(userID any, nickname string, content message.Chain) message.Segment {
	return message.Raw("node", map[string]any{"user_id": fmt.Sprint(userID), "nickname": nickname, "content": content})
}

// Custom 创建自定义 OneBot v11 消息段。
func Custom(kind string, data map[string]any) message.Segment { return message.Raw(kind, data) }

func parseMessage(raw json.RawMessage) (message.Chain, error) {
	raw = trimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	if raw[0] == '"' {
		var text string
		if err := json.Unmarshal(raw, &text); err != nil {
			return nil, err
		}
		return ParseCQ(text), nil
	}
	var chain message.Chain
	if err := json.Unmarshal(raw, &chain); err != nil {
		return nil, err
	}
	return fromWireChain(chain), nil
}

func fromWireChain(chain message.Chain) message.Chain {
	out := make(message.Chain, 0, len(chain))
	for _, seg := range chain {
		data := cloneData(seg.Data)
		if seg.Type == "at" {
			if qq, ok := data["qq"]; ok {
				data["id"] = fmt.Sprint(qq)
				delete(data, "qq")
			}
		}
		out = append(out, message.Segment{Type: seg.Type, Data: data})
	}
	return out
}

func toWireChain(chain message.Chain) message.Chain {
	out := make(message.Chain, 0, len(chain))
	for _, seg := range chain {
		data := cloneData(seg.Data)
		switch seg.Type {
		case "at":
			if id, ok := data["id"]; ok {
				data["qq"] = fmt.Sprint(id)
				delete(data, "id")
			}
		case "node":
			if content, ok := data["content"].(message.Chain); ok {
				data["content"] = toWireChain(content)
			}
		}
		out = append(out, message.Segment{Type: seg.Type, Data: data})
	}
	return out
}

// ParseCQ 将 OneBot CQ 码文本解析为消息链。
func ParseCQ(input string) message.Chain {
	if input == "" {
		return nil
	}
	var chain message.Chain
	for len(input) > 0 {
		start := strings.Index(input, "[CQ:")
		if start < 0 {
			chain = appendText(chain, unescapeCQText(input))
			break
		}
		if start > 0 {
			chain = appendText(chain, unescapeCQText(input[:start]))
			input = input[start:]
		}
		end := strings.IndexByte(input, ']')
		if end < 0 {
			chain = appendText(chain, unescapeCQText(input))
			break
		}
		body := input[len("[CQ:"):end]
		if seg, ok := parseCQSegment(body); ok {
			chain = append(chain, seg)
		} else {
			chain = appendText(chain, unescapeCQText(input[:end+1]))
		}
		input = input[end+1:]
	}
	return chain
}

// CQString 将消息链渲染为 OneBot CQ 码文本。
func CQString(chain message.Chain) string {
	var b strings.Builder
	for _, seg := range toWireChain(chain) {
		if seg.Type == "text" {
			if text, ok := seg.Data["text"].(string); ok {
				b.WriteString(escapeCQText(text))
				continue
			}
		}
		b.WriteString("[CQ:")
		b.WriteString(seg.Type)
		if len(seg.Data) > 0 {
			keys := make([]string, 0, len(seg.Data))
			for key := range seg.Data {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				b.WriteByte(',')
				b.WriteString(key)
				b.WriteByte('=')
				b.WriteString(escapeCQParam(fmt.Sprint(seg.Data[key])))
			}
		}
		b.WriteByte(']')
	}
	return b.String()
}

func appendText(chain message.Chain, text string) message.Chain {
	if text == "" {
		return chain
	}
	if len(chain) > 0 && chain[len(chain)-1].Type == "text" {
		if chain[len(chain)-1].Data == nil {
			chain[len(chain)-1].Data = map[string]any{}
		}
		chain[len(chain)-1].Data["text"] = fmt.Sprint(chain[len(chain)-1].Data["text"]) + text
		return chain
	}
	return append(chain, message.Text(text))
}

func parseCQSegment(body string) (message.Segment, bool) {
	parts := strings.Split(body, ",")
	if len(parts) == 0 || parts[0] == "" {
		return message.Segment{}, false
	}
	seg := message.Segment{Type: parts[0]}
	if len(parts) == 1 {
		return seg, true
	}
	seg.Data = make(map[string]any, len(parts)-1)
	for _, part := range parts[1:] {
		key, value, ok := strings.Cut(part, "=")
		if !ok || key == "" {
			continue
		}
		if seg.Type == "at" && key == "qq" {
			key = "id"
		}
		seg.Data[key] = unescapeCQParam(value)
	}
	return seg, true
}

func escapeCQText(s string) string {
	return strings.NewReplacer("&", "&amp;", "[", "&#91;", "]", "&#93;").Replace(s)
}

func escapeCQParam(s string) string {
	return strings.NewReplacer("&", "&amp;", "[", "&#91;", "]", "&#93;", ",", "&#44;").Replace(s)
}

func unescapeCQText(s string) string {
	return strings.NewReplacer("&#91;", "[", "&#93;", "]", "&amp;", "&").Replace(s)
}

func unescapeCQParam(s string) string {
	return strings.NewReplacer("&#44;", ",", "&#91;", "[", "&#93;", "]", "&amp;", "&").Replace(s)
}

func trimSpace(raw json.RawMessage) json.RawMessage {
	return json.RawMessage(strings.TrimSpace(string(raw)))
}

func cloneData(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
