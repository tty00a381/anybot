package onebot11

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/tty00a381/anybot"
	"github.com/tty00a381/anybot/message"
)

// Client 是 OneBot v11 的类型化动作客户端。
type Client struct {
	transport     Transport
	actionTimeout time.Duration
}

// ClientFrom 从 AnyBot 上下文提取 OneBot v11 客户端。
func ClientFrom(c *anybot.Context) (*Client, bool) {
	if c == nil {
		return nil, false
	}
	client, ok := c.Client().(*Client)
	return client, ok
}

// MustClient 从 AnyBot 上下文提取 OneBot v11 客户端；类型不匹配时 panic。
func MustClient(c *anybot.Context) *Client {
	client, ok := ClientFrom(c)
	if !ok {
		panic("onebot11: context action client is not *onebot11.Client")
	}
	return client
}

// CallRaw 调用 OneBot v11 动作，并返回协议无关的原始响应封套。
func (c *Client) CallRaw(ctx context.Context, action string, params any) (*anybot.ActionResponse, error) {
	ctx, cancel := c.withActionTimeout(ctx)
	defer cancel()
	resp, err := c.transport.CallRaw(ctx, action, params)
	if err != nil {
		return nil, err
	}
	return resp.actionResponse(), nil
}

// Call 调用 OneBot v11 动作，并在成功时把 data 解码到 out。
func (c *Client) Call(ctx context.Context, action string, params any, out any) error {
	ctx, cancel := c.withActionTimeout(ctx)
	defer cancel()
	resp, err := c.transport.CallRaw(ctx, action, params)
	if err != nil {
		return err
	}
	if resp == nil {
		return fmt.Errorf("onebot11: action %s returned nil response", action)
	}
	if !resp.OK() {
		return &anybot.ActionError{
			Action:  action,
			Status:  resp.Status,
			RetCode: resp.RetCode,
			Message: resp.Message,
			Wording: resp.Wording,
		}
	}
	return resp.Decode(out)
}

func (c *Client) withActionTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	if c.actionTimeout <= 0 {
		return ctx, func() {}
	}
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, c.actionTimeout)
}

// Send 根据 ReplyTarget 选择私聊或群聊动作发送消息。
func (c *Client) Send(ctx context.Context, target anybot.ReplyTarget, chain message.Chain) (anybot.MessageReceipt, error) {
	var out struct {
		MessageID flexibleInt64 `json:"message_id"`
	}
	wire := toWireChain(chain)
	switch {
	case target.GroupID != "":
		err := c.Call(ctx, "send_group_msg", map[string]any{
			"group_id": idParam(target.GroupID),
			"message":  wire,
		}, &out)
		return anybot.MessageReceipt{ID: idString(out.MessageID.int64())}, err
	case target.UserID != "":
		err := c.Call(ctx, "send_private_msg", map[string]any{
			"user_id": idParam(target.UserID),
			"message": wire,
		}, &out)
		return anybot.MessageReceipt{ID: idString(out.MessageID.int64())}, err
	default:
		return anybot.MessageReceipt{}, fmt.Errorf("onebot11: no supported reply target")
	}
}

// SendPrivateMessage 发送私聊消息，并返回消息 ID。
func (c *Client) SendPrivateMessage(ctx context.Context, userID any, chain message.Chain) (string, error) {
	receipt, err := c.Send(ctx, anybot.ReplyTarget{UserID: fmt.Sprint(userID)}, chain)
	return receipt.ID, err
}

// SendGroupMessage 发送群消息，并返回消息 ID。
func (c *Client) SendGroupMessage(ctx context.Context, groupID any, chain message.Chain) (string, error) {
	receipt, err := c.Send(ctx, anybot.ReplyTarget{GroupID: fmt.Sprint(groupID)}, chain)
	return receipt.ID, err
}

// SendMessage 调用 send_msg 发送消息，适合需要显式指定 message_type 的场景。
func (c *Client) SendMessage(ctx context.Context, messageType string, userID, groupID any, chain message.Chain) (string, error) {
	var out struct {
		MessageID flexibleInt64 `json:"message_id"`
	}
	err := c.Call(ctx, "send_msg", map[string]any{
		"message_type": messageType,
		"user_id":      optionalID(userID),
		"group_id":     optionalID(groupID),
		"message":      toWireChain(chain),
	}, &out)
	return idString(out.MessageID.int64()), err
}

// DeleteMessage 撤回指定消息。
func (c *Client) DeleteMessage(ctx context.Context, messageID any) error {
	return c.Call(ctx, "delete_msg", map[string]any{"message_id": optionalID(messageID)}, nil)
}

// GetMessage 获取指定消息详情。
func (c *Client) GetMessage(ctx context.Context, messageID any) (MessageInfo, error) {
	var out MessageInfo
	err := c.Call(ctx, "get_msg", map[string]any{"message_id": optionalID(messageID)}, &out)
	return out, err
}

// GetForwardMessage 获取合并转发消息内容。
func (c *Client) GetForwardMessage(ctx context.Context, id any) (message.Chain, error) {
	var out struct {
		Messages message.Chain `json:"messages"`
	}
	err := c.Call(ctx, "get_forward_msg", map[string]any{"id": fmt.Sprint(id)}, &out)
	return out.Messages, err
}

// SendLike 向好友发送赞；times 不大于 0 时按 1 次处理。
func (c *Client) SendLike(ctx context.Context, userID any, times int) error {
	if times <= 0 {
		times = 1
	}
	return c.Call(ctx, "send_like", map[string]any{
		"user_id": optionalID(userID),
		"times":   times,
	}, nil)
}

// GetLoginInfo 获取当前登录账号信息。
func (c *Client) GetLoginInfo(ctx context.Context) (LoginInfo, error) {
	var out LoginInfo
	err := c.Call(ctx, "get_login_info", nil, &out)
	return out, err
}

// GetStrangerInfo 获取陌生人信息。
func (c *Client) GetStrangerInfo(ctx context.Context, userID any, noCache bool) (StrangerInfo, error) {
	var out StrangerInfo
	err := c.Call(ctx, "get_stranger_info", map[string]any{
		"user_id":  optionalID(userID),
		"no_cache": noCache,
	}, &out)
	return out, err
}

// GetFriendList 获取好友列表。
func (c *Client) GetFriendList(ctx context.Context) ([]FriendInfo, error) {
	var out []FriendInfo
	err := c.Call(ctx, "get_friend_list", nil, &out)
	return out, err
}

// GetGroupList 获取群列表。
func (c *Client) GetGroupList(ctx context.Context) ([]GroupInfo, error) {
	var out []GroupInfo
	err := c.Call(ctx, "get_group_list", nil, &out)
	return out, err
}

// GetGroupInfo 获取群信息。
func (c *Client) GetGroupInfo(ctx context.Context, groupID any, noCache bool) (GroupInfo, error) {
	var out GroupInfo
	err := c.Call(ctx, "get_group_info", map[string]any{
		"group_id": optionalID(groupID),
		"no_cache": noCache,
	}, &out)
	return out, err
}

// GetGroupMemberInfo 获取指定群成员信息。
func (c *Client) GetGroupMemberInfo(ctx context.Context, groupID, userID any, noCache bool) (GroupMemberInfo, error) {
	var out GroupMemberInfo
	err := c.Call(ctx, "get_group_member_info", map[string]any{
		"group_id": optionalID(groupID),
		"user_id":  optionalID(userID),
		"no_cache": noCache,
	}, &out)
	return out, err
}

// GetGroupMemberList 获取指定群的成员列表。
func (c *Client) GetGroupMemberList(ctx context.Context, groupID any, noCache bool) ([]GroupMemberInfo, error) {
	var out []GroupMemberInfo
	err := c.Call(ctx, "get_group_member_list", map[string]any{
		"group_id": optionalID(groupID),
		"no_cache": noCache,
	}, &out)
	return out, err
}

// GetGroupHonorInfo 获取群荣誉信息，kind 对应 OneBot v11 的 type 参数。
func (c *Client) GetGroupHonorInfo(ctx context.Context, groupID any, kind string) (GroupHonorInfo, error) {
	var out GroupHonorInfo
	err := c.Call(ctx, "get_group_honor_info", map[string]any{
		"group_id": optionalID(groupID),
		"type":     kind,
	}, &out)
	return out, err
}

// SetGroupBan 设置群成员禁言时长。
func (c *Client) SetGroupBan(ctx context.Context, groupID, userID any, duration time.Duration) error {
	return c.Call(ctx, "set_group_ban", map[string]any{
		"group_id": optionalID(groupID),
		"user_id":  optionalID(userID),
		"duration": int64(duration.Seconds()),
	}, nil)
}

// SetGroupAnonymousBan 设置匿名用户禁言。
func (c *Client) SetGroupAnonymousBan(ctx context.Context, groupID any, anonymous any, anonymousFlag string, duration time.Duration) error {
	return c.Call(ctx, "set_group_anonymous_ban", map[string]any{
		"group_id":       optionalID(groupID),
		"anonymous":      anonymous,
		"anonymous_flag": anonymousFlag,
		"duration":       int64(duration.Seconds()),
	}, nil)
}

// SetGroupWholeBan 开启或关闭全群禁言。
func (c *Client) SetGroupWholeBan(ctx context.Context, groupID any, enable bool) error {
	return c.Call(ctx, "set_group_whole_ban", map[string]any{
		"group_id": optionalID(groupID),
		"enable":   enable,
	}, nil)
}

// SetGroupAnonymous 开启或关闭群匿名聊天。
func (c *Client) SetGroupAnonymous(ctx context.Context, groupID any, enable bool) error {
	return c.Call(ctx, "set_group_anonymous", map[string]any{
		"group_id": optionalID(groupID),
		"enable":   enable,
	}, nil)
}

// SetGroupKick 踢出群成员，并可拒绝其后续加群请求。
func (c *Client) SetGroupKick(ctx context.Context, groupID, userID any, rejectAddRequest bool) error {
	return c.Call(ctx, "set_group_kick", map[string]any{
		"group_id":           optionalID(groupID),
		"user_id":            optionalID(userID),
		"reject_add_request": rejectAddRequest,
	}, nil)
}

// SetGroupAdmin 设置或取消群管理员。
func (c *Client) SetGroupAdmin(ctx context.Context, groupID, userID any, enable bool) error {
	return c.Call(ctx, "set_group_admin", map[string]any{
		"group_id": optionalID(groupID),
		"user_id":  optionalID(userID),
		"enable":   enable,
	}, nil)
}

// SetGroupCard 设置群成员名片。
func (c *Client) SetGroupCard(ctx context.Context, groupID, userID any, card string) error {
	return c.Call(ctx, "set_group_card", map[string]any{
		"group_id": optionalID(groupID),
		"user_id":  optionalID(userID),
		"card":     card,
	}, nil)
}

// SetGroupName 设置群名称。
func (c *Client) SetGroupName(ctx context.Context, groupID any, name string) error {
	return c.Call(ctx, "set_group_name", map[string]any{
		"group_id":   optionalID(groupID),
		"group_name": name,
	}, nil)
}

// SetGroupLeave 退出群；dismiss 为 true 时尝试解散群。
func (c *Client) SetGroupLeave(ctx context.Context, groupID any, dismiss bool) error {
	return c.Call(ctx, "set_group_leave", map[string]any{
		"group_id":   optionalID(groupID),
		"is_dismiss": dismiss,
	}, nil)
}

// SetGroupSpecialTitle 设置群成员专属头衔。
func (c *Client) SetGroupSpecialTitle(ctx context.Context, groupID, userID any, title string, duration time.Duration) error {
	return c.Call(ctx, "set_group_special_title", map[string]any{
		"group_id":      optionalID(groupID),
		"user_id":       optionalID(userID),
		"special_title": title,
		"duration":      int64(duration.Seconds()),
	}, nil)
}

// SetFriendAddRequest 处理好友添加请求。
func (c *Client) SetFriendAddRequest(ctx context.Context, flag string, approve bool, remark string) error {
	return c.Call(ctx, "set_friend_add_request", map[string]any{
		"flag":    flag,
		"approve": approve,
		"remark":  remark,
	}, nil)
}

// SetGroupAddRequest 处理加群请求或邀请。
func (c *Client) SetGroupAddRequest(ctx context.Context, flag, subType string, approve bool, reason string) error {
	return c.Call(ctx, "set_group_add_request", map[string]any{
		"flag":     flag,
		"sub_type": subType,
		"approve":  approve,
		"reason":   reason,
	}, nil)
}

// GetCookies 获取指定域名的 Cookies。
func (c *Client) GetCookies(ctx context.Context, domain string) (string, error) {
	var out struct {
		Cookies string `json:"cookies"`
	}
	err := c.Call(ctx, "get_cookies", map[string]any{"domain": domain}, &out)
	return out.Cookies, err
}

// GetCSRFToken 获取 CSRF token。
func (c *Client) GetCSRFToken(ctx context.Context) (int64, error) {
	var out struct {
		Token int64 `json:"token"`
	}
	err := c.Call(ctx, "get_csrf_token", nil, &out)
	return out.Token, err
}

// GetCredentials 获取指定域名的 Cookies 与 CSRF token。
func (c *Client) GetCredentials(ctx context.Context, domain string) (Credentials, error) {
	var out Credentials
	err := c.Call(ctx, "get_credentials", map[string]any{"domain": domain}, &out)
	return out, err
}

// GetRecord 获取语音文件信息，并可请求协议端转换格式。
func (c *Client) GetRecord(ctx context.Context, file, outFormat string) (FileInfo, error) {
	var out FileInfo
	err := c.Call(ctx, "get_record", map[string]any{
		"file":       file,
		"out_format": outFormat,
	}, &out)
	return out, err
}

// GetImage 获取图片文件信息。
func (c *Client) GetImage(ctx context.Context, file string) (FileInfo, error) {
	var out FileInfo
	err := c.Call(ctx, "get_image", map[string]any{"file": file}, &out)
	return out, err
}

// CanSendImage 检查当前协议端是否可以发送图片。
func (c *Client) CanSendImage(ctx context.Context) (bool, error) {
	var out struct {
		Yes bool `json:"yes"`
	}
	err := c.Call(ctx, "can_send_image", nil, &out)
	return out.Yes, err
}

// CanSendRecord 检查当前协议端是否可以发送语音。
func (c *Client) CanSendRecord(ctx context.Context) (bool, error) {
	var out struct {
		Yes bool `json:"yes"`
	}
	err := c.Call(ctx, "can_send_record", nil, &out)
	return out.Yes, err
}

// GetStatus 获取协议端运行状态。
func (c *Client) GetStatus(ctx context.Context) (StatusInfo, error) {
	var out StatusInfo
	err := c.Call(ctx, "get_status", nil, &out)
	return out, err
}

// GetVersionInfo 获取协议端版本信息。
func (c *Client) GetVersionInfo(ctx context.Context) (VersionInfo, error) {
	var out VersionInfo
	err := c.Call(ctx, "get_version_info", nil, &out)
	return out, err
}

// SetRestart 请求协议端延迟重启。
func (c *Client) SetRestart(ctx context.Context, delay time.Duration) error {
	return c.Call(ctx, "set_restart", map[string]any{"delay": int64(delay.Milliseconds())}, nil)
}

// CleanCache 请求协议端清理缓存。
func (c *Client) CleanCache(ctx context.Context) error {
	return c.Call(ctx, "clean_cache", nil, nil)
}

// UploadPrivateFile 上传私聊文件。
func (c *Client) UploadPrivateFile(ctx context.Context, userID any, file, name string) error {
	return c.Call(ctx, "upload_private_file", map[string]any{
		"user_id": optionalID(userID),
		"file":    file,
		"name":    name,
	}, nil)
}

// UploadGroupFile 上传群文件。
func (c *Client) UploadGroupFile(ctx context.Context, groupID any, file, name, folder string) error {
	return c.Call(ctx, "upload_group_file", map[string]any{
		"group_id": optionalID(groupID),
		"file":     file,
		"name":     name,
		"folder":   folder,
	}, nil)
}

// DeleteGroupFile 删除群文件。
func (c *Client) DeleteGroupFile(ctx context.Context, groupID any, fileID, busid any) error {
	return c.Call(ctx, "delete_group_file", map[string]any{
		"group_id": optionalID(groupID),
		"file_id":  fmt.Sprint(fileID),
		"busid":    optionalID(busid),
	}, nil)
}

// CreateGroupFileFolder 创建群文件夹。
func (c *Client) CreateGroupFileFolder(ctx context.Context, groupID any, name string, parentID any) error {
	return c.Call(ctx, "create_group_file_folder", map[string]any{
		"group_id":  optionalID(groupID),
		"name":      name,
		"parent_id": optionalString(parentID),
	}, nil)
}

// DeleteGroupFolder 删除群文件夹。
func (c *Client) DeleteGroupFolder(ctx context.Context, groupID, folderID any) error {
	return c.Call(ctx, "delete_group_folder", map[string]any{
		"group_id":  optionalID(groupID),
		"folder_id": fmt.Sprint(folderID),
	}, nil)
}

// GetGroupFileSystemInfo 获取群文件系统容量与数量信息。
func (c *Client) GetGroupFileSystemInfo(ctx context.Context, groupID any) (GroupFileSystemInfo, error) {
	var out GroupFileSystemInfo
	err := c.Call(ctx, "get_group_file_system_info", map[string]any{"group_id": optionalID(groupID)}, &out)
	return out, err
}

// GetGroupRootFiles 获取群文件根目录内容。
func (c *Client) GetGroupRootFiles(ctx context.Context, groupID any) (GroupFiles, error) {
	var out GroupFiles
	err := c.Call(ctx, "get_group_root_files", map[string]any{"group_id": optionalID(groupID)}, &out)
	return out, err
}

// GetGroupFilesByFolder 获取指定群文件夹内容。
func (c *Client) GetGroupFilesByFolder(ctx context.Context, groupID, folderID any) (GroupFiles, error) {
	var out GroupFiles
	err := c.Call(ctx, "get_group_files_by_folder", map[string]any{
		"group_id":  optionalID(groupID),
		"folder_id": fmt.Sprint(folderID),
	}, &out)
	return out, err
}

// GetGroupFileURL 获取群文件下载链接。
func (c *Client) GetGroupFileURL(ctx context.Context, groupID any, fileID, busid any) (string, error) {
	var out struct {
		URL string `json:"url"`
	}
	err := c.Call(ctx, "get_group_file_url", map[string]any{
		"group_id": optionalID(groupID),
		"file_id":  fmt.Sprint(fileID),
		"busid":    optionalID(busid),
	}, &out)
	return out.URL, err
}

// MessageInfo 描述 get_msg 返回的消息详情。
type MessageInfo struct {
	Time        int64         `json:"time"`
	MessageType string        `json:"message_type"`
	MessageID   int64         `json:"message_id"`
	RealID      int64         `json:"real_id,omitempty"`
	Sender      Sender        `json:"sender"`
	Message     message.Chain `json:"message"`
	RawMessage  string        `json:"raw_message"`
}

// UnmarshalJSON 同时接受数组消息和 CQ 字符串消息。
func (m *MessageInfo) UnmarshalJSON(data []byte) error {
	var decoded struct {
		Time        flexibleInt64   `json:"time"`
		MessageType string          `json:"message_type"`
		MessageID   flexibleInt64   `json:"message_id"`
		RealID      flexibleInt64   `json:"real_id"`
		Sender      Sender          `json:"sender"`
		Message     json.RawMessage `json:"message"`
		RawMessage  string          `json:"raw_message"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	m.Time = decoded.Time.int64()
	m.MessageType = decoded.MessageType
	m.MessageID = decoded.MessageID.int64()
	m.RealID = decoded.RealID.int64()
	m.Sender = decoded.Sender
	m.RawMessage = decoded.RawMessage
	m.Message = nil
	if len(decoded.Message) > 0 {
		chain, err := parseMessage(decoded.Message)
		if err != nil {
			return err
		}
		m.Message = chain
	}
	return nil
}

// LoginInfo 描述当前登录账号。
type LoginInfo struct {
	UserID   int64  `json:"user_id"`
	Nickname string `json:"nickname"`
}

func (i *LoginInfo) UnmarshalJSON(data []byte) error {
	var decoded struct {
		UserID   flexibleInt64 `json:"user_id"`
		Nickname string        `json:"nickname"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	i.UserID = decoded.UserID.int64()
	i.Nickname = decoded.Nickname
	return nil
}

// StrangerInfo 描述陌生人资料。
type StrangerInfo struct {
	UserID    int64  `json:"user_id"`
	Nickname  string `json:"nickname"`
	Sex       string `json:"sex"`
	Age       int    `json:"age"`
	QID       string `json:"qid,omitempty"`
	Level     int    `json:"level,omitempty"`
	LoginDays int    `json:"login_days,omitempty"`
}

func (i *StrangerInfo) UnmarshalJSON(data []byte) error {
	var decoded struct {
		UserID    flexibleInt64 `json:"user_id"`
		Nickname  string        `json:"nickname"`
		Sex       string        `json:"sex"`
		Age       int           `json:"age"`
		QID       string        `json:"qid"`
		Level     int           `json:"level"`
		LoginDays int           `json:"login_days"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	i.UserID = decoded.UserID.int64()
	i.Nickname = decoded.Nickname
	i.Sex = decoded.Sex
	i.Age = decoded.Age
	i.QID = decoded.QID
	i.Level = decoded.Level
	i.LoginDays = decoded.LoginDays
	return nil
}

// FriendInfo 描述好友资料。
type FriendInfo struct {
	UserID   int64  `json:"user_id"`
	Nickname string `json:"nickname"`
	Remark   string `json:"remark"`
}

func (i *FriendInfo) UnmarshalJSON(data []byte) error {
	var decoded struct {
		UserID   flexibleInt64 `json:"user_id"`
		Nickname string        `json:"nickname"`
		Remark   string        `json:"remark"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	i.UserID = decoded.UserID.int64()
	i.Nickname = decoded.Nickname
	i.Remark = decoded.Remark
	return nil
}

// GroupInfo 描述群资料。
type GroupInfo struct {
	GroupID        int64  `json:"group_id"`
	GroupName      string `json:"group_name"`
	MemberCount    int    `json:"member_count"`
	MaxMemberCount int    `json:"max_member_count"`
}

func (i *GroupInfo) UnmarshalJSON(data []byte) error {
	var decoded struct {
		GroupID        flexibleInt64 `json:"group_id"`
		GroupName      string        `json:"group_name"`
		MemberCount    int           `json:"member_count"`
		MaxMemberCount int           `json:"max_member_count"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	i.GroupID = decoded.GroupID.int64()
	i.GroupName = decoded.GroupName
	i.MemberCount = decoded.MemberCount
	i.MaxMemberCount = decoded.MaxMemberCount
	return nil
}

// GroupMemberInfo 描述群成员资料。
type GroupMemberInfo struct {
	GroupID         int64  `json:"group_id"`
	UserID          int64  `json:"user_id"`
	Nickname        string `json:"nickname"`
	Card            string `json:"card"`
	Sex             string `json:"sex,omitempty"`
	Age             int    `json:"age,omitempty"`
	Area            string `json:"area,omitempty"`
	JoinTime        int64  `json:"join_time,omitempty"`
	LastSentTime    int64  `json:"last_sent_time,omitempty"`
	Role            string `json:"role"`
	Unfriendly      bool   `json:"unfriendly,omitempty"`
	Title           string `json:"title"`
	TitleExpireTime int64  `json:"title_expire_time,omitempty"`
	CardChangeable  bool   `json:"card_changeable,omitempty"`
}

func (i *GroupMemberInfo) UnmarshalJSON(data []byte) error {
	var decoded struct {
		GroupID         flexibleInt64 `json:"group_id"`
		UserID          flexibleInt64 `json:"user_id"`
		Nickname        string        `json:"nickname"`
		Card            string        `json:"card"`
		Sex             string        `json:"sex"`
		Age             int           `json:"age"`
		Area            string        `json:"area"`
		JoinTime        flexibleInt64 `json:"join_time"`
		LastSentTime    flexibleInt64 `json:"last_sent_time"`
		Role            string        `json:"role"`
		Unfriendly      bool          `json:"unfriendly"`
		Title           string        `json:"title"`
		TitleExpireTime flexibleInt64 `json:"title_expire_time"`
		CardChangeable  bool          `json:"card_changeable"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	i.GroupID = decoded.GroupID.int64()
	i.UserID = decoded.UserID.int64()
	i.Nickname = decoded.Nickname
	i.Card = decoded.Card
	i.Sex = decoded.Sex
	i.Age = decoded.Age
	i.Area = decoded.Area
	i.JoinTime = decoded.JoinTime.int64()
	i.LastSentTime = decoded.LastSentTime.int64()
	i.Role = decoded.Role
	i.Unfriendly = decoded.Unfriendly
	i.Title = decoded.Title
	i.TitleExpireTime = decoded.TitleExpireTime.int64()
	i.CardChangeable = decoded.CardChangeable
	return nil
}

// HonorItem 描述群荣誉列表中的成员条目。
type HonorItem struct {
	UserID      int64  `json:"user_id"`
	Nickname    string `json:"nickname"`
	Avatar      string `json:"avatar"`
	Description string `json:"description"`
}

func (i *HonorItem) UnmarshalJSON(data []byte) error {
	var decoded struct {
		UserID      flexibleInt64 `json:"user_id"`
		Nickname    string        `json:"nickname"`
		Avatar      string        `json:"avatar"`
		Description string        `json:"description"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	i.UserID = decoded.UserID.int64()
	i.Nickname = decoded.Nickname
	i.Avatar = decoded.Avatar
	i.Description = decoded.Description
	return nil
}

// GroupHonorInfo 描述群荣誉信息。
type GroupHonorInfo struct {
	GroupID          int64       `json:"group_id"`
	CurrentTalkative HonorItem   `json:"current_talkative"`
	TalkativeList    []HonorItem `json:"talkative_list"`
	PerformerList    []HonorItem `json:"performer_list"`
	LegendList       []HonorItem `json:"legend_list"`
	StrongNewbieList []HonorItem `json:"strong_newbie_list"`
	EmotionList      []HonorItem `json:"emotion_list"`
}

func (i *GroupHonorInfo) UnmarshalJSON(data []byte) error {
	var decoded struct {
		GroupID          flexibleInt64 `json:"group_id"`
		CurrentTalkative HonorItem     `json:"current_talkative"`
		TalkativeList    []HonorItem   `json:"talkative_list"`
		PerformerList    []HonorItem   `json:"performer_list"`
		LegendList       []HonorItem   `json:"legend_list"`
		StrongNewbieList []HonorItem   `json:"strong_newbie_list"`
		EmotionList      []HonorItem   `json:"emotion_list"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	i.GroupID = decoded.GroupID.int64()
	i.CurrentTalkative = decoded.CurrentTalkative
	i.TalkativeList = decoded.TalkativeList
	i.PerformerList = decoded.PerformerList
	i.LegendList = decoded.LegendList
	i.StrongNewbieList = decoded.StrongNewbieList
	i.EmotionList = decoded.EmotionList
	return nil
}

// Credentials 描述 Cookies 与 CSRF token。
type Credentials struct {
	Cookies   string `json:"cookies"`
	CSRFToken int64  `json:"csrf_token"`
}

func (i *Credentials) UnmarshalJSON(data []byte) error {
	var decoded struct {
		Cookies   string        `json:"cookies"`
		CSRFToken flexibleInt64 `json:"csrf_token"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	i.Cookies = decoded.Cookies
	i.CSRFToken = decoded.CSRFToken.int64()
	return nil
}

// FileInfo 描述协议端返回的本地文件信息。
type FileInfo struct {
	File string `json:"file"`
	URL  string `json:"url,omitempty"`
}

// StatusInfo 描述协议端运行状态。
type StatusInfo struct {
	Online bool           `json:"online"`
	Good   bool           `json:"good"`
	Extra  map[string]any `json:"-"`
}

// VersionInfo 描述协议端版本信息。
type VersionInfo struct {
	AppName         string `json:"app_name"`
	AppVersion      string `json:"app_version"`
	ProtocolVersion string `json:"protocol_version"`
}

// GroupFileSystemInfo 描述群文件系统容量和文件数量。
type GroupFileSystemInfo struct {
	FileCount  int64 `json:"file_count"`
	LimitCount int64 `json:"limit_count"`
	UsedSpace  int64 `json:"used_space"`
	TotalSpace int64 `json:"total_space"`
}

func (i *GroupFileSystemInfo) UnmarshalJSON(data []byte) error {
	var decoded struct {
		FileCount  flexibleInt64 `json:"file_count"`
		LimitCount flexibleInt64 `json:"limit_count"`
		UsedSpace  flexibleInt64 `json:"used_space"`
		TotalSpace flexibleInt64 `json:"total_space"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	i.FileCount = decoded.FileCount.int64()
	i.LimitCount = decoded.LimitCount.int64()
	i.UsedSpace = decoded.UsedSpace.int64()
	i.TotalSpace = decoded.TotalSpace.int64()
	return nil
}

// GroupFile 描述群文件条目。
type GroupFile struct {
	GroupID       int64  `json:"group_id"`
	FileID        string `json:"file_id"`
	FileName      string `json:"file_name"`
	BusID         int64  `json:"busid"`
	FileSize      int64  `json:"file_size"`
	UploadTime    int64  `json:"upload_time"`
	DeadTime      int64  `json:"dead_time"`
	ModifyTime    int64  `json:"modify_time"`
	DownloadTimes int64  `json:"download_times"`
	Uploader      int64  `json:"uploader"`
	UploaderName  string `json:"uploader_name"`
}

func (i *GroupFile) UnmarshalJSON(data []byte) error {
	var decoded struct {
		GroupID       flexibleInt64 `json:"group_id"`
		FileID        string        `json:"file_id"`
		FileName      string        `json:"file_name"`
		BusID         flexibleInt64 `json:"busid"`
		FileSize      flexibleInt64 `json:"file_size"`
		UploadTime    flexibleInt64 `json:"upload_time"`
		DeadTime      flexibleInt64 `json:"dead_time"`
		ModifyTime    flexibleInt64 `json:"modify_time"`
		DownloadTimes flexibleInt64 `json:"download_times"`
		Uploader      flexibleInt64 `json:"uploader"`
		UploaderName  string        `json:"uploader_name"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	i.GroupID = decoded.GroupID.int64()
	i.FileID = decoded.FileID
	i.FileName = decoded.FileName
	i.BusID = decoded.BusID.int64()
	i.FileSize = decoded.FileSize.int64()
	i.UploadTime = decoded.UploadTime.int64()
	i.DeadTime = decoded.DeadTime.int64()
	i.ModifyTime = decoded.ModifyTime.int64()
	i.DownloadTimes = decoded.DownloadTimes.int64()
	i.Uploader = decoded.Uploader.int64()
	i.UploaderName = decoded.UploaderName
	return nil
}

// GroupFolder 描述群文件夹条目。
type GroupFolder struct {
	GroupID        int64  `json:"group_id"`
	FolderID       string `json:"folder_id"`
	FolderName     string `json:"folder_name"`
	CreateTime     int64  `json:"create_time"`
	Creator        int64  `json:"creator"`
	CreatorName    string `json:"creator_name"`
	TotalFileCount int64  `json:"total_file_count"`
}

func (i *GroupFolder) UnmarshalJSON(data []byte) error {
	var decoded struct {
		GroupID        flexibleInt64 `json:"group_id"`
		FolderID       string        `json:"folder_id"`
		FolderName     string        `json:"folder_name"`
		CreateTime     flexibleInt64 `json:"create_time"`
		Creator        flexibleInt64 `json:"creator"`
		CreatorName    string        `json:"creator_name"`
		TotalFileCount flexibleInt64 `json:"total_file_count"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	i.GroupID = decoded.GroupID.int64()
	i.FolderID = decoded.FolderID
	i.FolderName = decoded.FolderName
	i.CreateTime = decoded.CreateTime.int64()
	i.Creator = decoded.Creator.int64()
	i.CreatorName = decoded.CreatorName
	i.TotalFileCount = decoded.TotalFileCount.int64()
	return nil
}

// GroupFiles 描述群文件目录内容。
type GroupFiles struct {
	Files   []GroupFile   `json:"files"`
	Folders []GroupFolder `json:"folders"`
}

func optionalID(id any) any {
	if id == nil {
		return nil
	}
	return idParam(fmt.Sprint(id))
}

func idParam(id string) any {
	if parsed, err := strconv.ParseInt(id, 10, 64); err == nil {
		return parsed
	}
	return id
}

func optionalString(value any) any {
	if value == nil {
		return nil
	}
	return fmt.Sprint(value)
}
