package napcat

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/tty00a381/anybot/adapters/onebot11"
)

// API 包装 NapCat 扩展动作，并保留原始调用能力。
type API struct {
	client Client
}

// Client 是 NapCat 扩展动作所需的 OneBot v11 调用能力。
type Client interface {
	Call(context.Context, string, any, any) error
	CallRaw(context.Context, string, any) (*onebot11.Response, error)
}

// New 创建 NapCat 扩展 API 客户端。
func New(client Client) *API {
	return &API{client: client}
}

// Call 调用 NapCat 动作，并将响应 data 解码到 out。
func (api *API) Call(ctx context.Context, action string, params any, out any) error {
	return api.client.Call(ctx, action, params, out)
}

// CallRaw 调用 NapCat 动作，并返回 OneBot v11 原始响应封套。
func (api *API) CallRaw(ctx context.Context, action string, params any) (*onebot11.Response, error) {
	return api.client.CallRaw(ctx, action, params)
}

// MessageSummary 是 NapCat 历史消息接口返回的紧凑消息结构。
type MessageSummary struct {
	MessageID  int64  `json:"message_id"`
	RealID     int64  `json:"real_id,omitempty"`
	Time       int64  `json:"time,omitempty"`
	UserID     int64  `json:"user_id,omitempty"`
	GroupID    int64  `json:"group_id,omitempty"`
	RawMessage string `json:"raw_message,omitempty"`
}

// UnmarshalJSON 兼容 NapCat 返回的数字或字符串形式 ID。
func (m *MessageSummary) UnmarshalJSON(data []byte) error {
	var decoded struct {
		MessageID  flexibleInt64 `json:"message_id"`
		RealID     flexibleInt64 `json:"real_id"`
		Time       flexibleInt64 `json:"time"`
		UserID     flexibleInt64 `json:"user_id"`
		GroupID    flexibleInt64 `json:"group_id"`
		RawMessage string        `json:"raw_message"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	m.MessageID = decoded.MessageID.int64()
	m.RealID = decoded.RealID.int64()
	m.Time = decoded.Time.int64()
	m.UserID = decoded.UserID.int64()
	m.GroupID = decoded.GroupID.int64()
	m.RawMessage = decoded.RawMessage
	return nil
}

// GetGroupMessageHistory 调用 NapCat get_group_msg_history 扩展。
func (api *API) GetGroupMessageHistory(ctx context.Context, groupID any, messageSeq any, count int) ([]MessageSummary, error) {
	var out struct {
		Messages []MessageSummary `json:"messages"`
	}
	err := api.Call(ctx, "get_group_msg_history", map[string]any{
		"group_id":    id(groupID),
		"message_seq": id(messageSeq),
		"count":       count,
	}, &out)
	return out.Messages, err
}

// GetFriendMessageHistory 调用 NapCat get_friend_msg_history 扩展。
func (api *API) GetFriendMessageHistory(ctx context.Context, userID any, messageSeq any, count int) ([]MessageSummary, error) {
	var out struct {
		Messages []MessageSummary `json:"messages"`
	}
	err := api.Call(ctx, "get_friend_msg_history", map[string]any{
		"user_id":     id(userID),
		"message_seq": id(messageSeq),
		"count":       count,
	}, &out)
	return out.Messages, err
}

// SetMessageEmojiLike 调用 NapCat set_msg_emoji_like 扩展。
func (api *API) SetMessageEmojiLike(ctx context.Context, messageID any, emojiID any) error {
	return api.Call(ctx, "set_msg_emoji_like", map[string]any{
		"message_id": id(messageID),
		"emoji_id":   id(emojiID),
	}, nil)
}

// FileInfo 描述 NapCat 文件下载或查询结果。
type FileInfo struct {
	File string `json:"file"`
	URL  string `json:"url,omitempty"`
	Name string `json:"name,omitempty"`
	Size int64  `json:"size,omitempty"`
}

// UnmarshalJSON 兼容 NapCat 返回的数字或字符串形式文件大小。
func (f *FileInfo) UnmarshalJSON(data []byte) error {
	var decoded struct {
		File string        `json:"file"`
		URL  string        `json:"url"`
		Name string        `json:"name"`
		Size flexibleInt64 `json:"size"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	f.File = decoded.File
	f.URL = decoded.URL
	f.Name = decoded.Name
	f.Size = decoded.Size.int64()
	return nil
}

// DownloadFile 调用 NapCat download_file 扩展。
func (api *API) DownloadFile(ctx context.Context, url string, threadCount int, headers []string) (FileInfo, error) {
	var out FileInfo
	err := api.Call(ctx, "download_file", map[string]any{
		"url":          url,
		"thread_count": threadCount,
		"headers":      headers,
	}, &out)
	return out, err
}

// GetFile 调用 NapCat get_file 扩展。
func (api *API) GetFile(ctx context.Context, fileID any) (FileInfo, error) {
	var out FileInfo
	err := api.Call(ctx, "get_file", map[string]any{"file_id": fmt.Sprint(fileID)}, &out)
	return out, err
}

func id(value any) any {
	switch v := value.(type) {
	case nil:
		return nil
	case string:
		return v
	default:
		return v
	}
}

type flexibleInt64 int64

func (id *flexibleInt64) UnmarshalJSON(data []byte) error {
	text := strings.TrimSpace(string(data))
	if text == "" || text == "null" {
		return nil
	}
	if strings.HasPrefix(text, `"`) {
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		text = strings.TrimSpace(value)
		if text == "" {
			return nil
		}
	}
	parsed, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return err
	}
	*id = flexibleInt64(parsed)
	return nil
}

func (id flexibleInt64) int64() int64 {
	return int64(id)
}
