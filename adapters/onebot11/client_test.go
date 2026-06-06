package onebot11

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/tty00a381/anybot/core"
	"github.com/tty00a381/anybot/core/message"
)

func TestClientSendGroupMessage(t *testing.T) {
	transport := &recordTransport{response: &Response{
		Status:  "ok",
		RetCode: 0,
		Data:    json.RawMessage(`{"message_id":"321"}`),
	}}
	client := (&Adapter{transport: transport, client: &Client{transport: transport}}).Client()
	receipt, err := client.Send(context.Background(), core.ReplyTarget{GroupID: "123"}, message.New(message.Text("hi"), At(456)))
	if err != nil {
		t.Fatal(err)
	}
	if receipt.ID != "321" {
		t.Fatalf("receipt = %#v", receipt)
	}
	if transport.action != "send_group_msg" {
		t.Fatalf("action = %q", transport.action)
	}
	params := transport.params.(map[string]any)
	chain := params["message"].(message.Chain)
	if chain[1].Data["qq"] != "456" {
		t.Fatalf("wire chain = %#v", chain)
	}
}

func TestClientActionError(t *testing.T) {
	transport := &recordTransport{response: &Response{Status: "failed", RetCode: 1400, Wording: "bad"}}
	client := &Client{transport: transport}
	err := client.Call(context.Background(), "bad_action", nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestClientCallRejectsNilResponse(t *testing.T) {
	client := &Client{transport: &recordTransport{}}
	err := client.Call(context.Background(), "bad_action", nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if got := err.Error(); got != "onebot11: action bad_action returned nil response" {
		t.Fatalf("err = %q", got)
	}
}

func TestResponseAcceptsStringRetCode(t *testing.T) {
	var response Response
	if err := json.Unmarshal([]byte(`{"status":"ok","retcode":"0","data":{"message_id":"7"}}`), &response); err != nil {
		t.Fatal(err)
	}
	if !response.OK() {
		t.Fatalf("response=%#v", response)
	}
	var out struct {
		MessageID flexibleInt64 `json:"message_id"`
	}
	if err := response.Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.MessageID.int64() != 7 {
		t.Fatalf("out=%#v", out)
	}
	if string(response.Raw) == "" {
		t.Fatal("raw response should be preserved")
	}
}

func TestResponseUnmarshalClearsData(t *testing.T) {
	var response Response
	if err := json.Unmarshal([]byte(`{"status":"ok","retcode":0,"data":{"message_id":7}}`), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Data) == 0 {
		t.Fatal("data should be present")
	}
	if err := json.Unmarshal([]byte(`{"status":"failed","retcode":"1","wording":"bad"}`), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Data) != 0 || response.Wording != "bad" || response.RetCode != 1 {
		t.Fatalf("response=%#v", response)
	}
}

func TestTypedActions(t *testing.T) {
	transport := &recordTransport{response: &Response{
		Status:  "ok",
		RetCode: 0,
		Data:    json.RawMessage(`{"user_id":10000,"nickname":"bot"}`),
	}}
	client := &Client{transport: transport}
	info, err := client.GetLoginInfo(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.UserID != 10000 || info.Nickname != "bot" || transport.action != "get_login_info" {
		t.Fatalf("info=%#v action=%s", info, transport.action)
	}

	transport.response = &Response{Status: "ok", RetCode: 0}
	err = client.SetGroupBan(context.Background(), 1, 2, 3_000_000_000)
	if err != nil {
		t.Fatal(err)
	}
	params := transport.params.(map[string]any)
	if transport.action != "set_group_ban" || params["duration"] != int64(3) {
		t.Fatalf("action=%s params=%#v", transport.action, params)
	}
}

func TestMoreTypedActions(t *testing.T) {
	transport := &recordTransport{response: &Response{Status: "ok", RetCode: 0}}
	client := &Client{transport: transport}

	if err := client.SetGroupName(context.Background(), "100", "新群名"); err != nil {
		t.Fatal(err)
	}
	params := transport.params.(map[string]any)
	if transport.action != "set_group_name" || params["group_name"] != "新群名" {
		t.Fatalf("action=%s params=%#v", transport.action, params)
	}

	if err := client.UploadGroupFile(context.Background(), 100, "/tmp/a.txt", "a.txt", "/"); err != nil {
		t.Fatal(err)
	}
	params = transport.params.(map[string]any)
	if transport.action != "upload_group_file" || params["name"] != "a.txt" {
		t.Fatalf("action=%s params=%#v", transport.action, params)
	}

	transport.response = &Response{Status: "ok", RetCode: 0, Data: json.RawMessage(`{"app_name":"NapCat","app_version":"1.0.0","protocol_version":"11"}`)}
	info, err := client.GetVersionInfo(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.AppName != "NapCat" || transport.action != "get_version_info" {
		t.Fatalf("info=%#v action=%s", info, transport.action)
	}
}

func TestMessageInfoParsesCQMessage(t *testing.T) {
	var info MessageInfo
	err := json.Unmarshal([]byte(`{
		"time":"1",
		"message_type":"group",
		"message_id":"2",
		"real_id":"3",
		"sender":{"user_id":"42","nickname":"alice"},
		"message":"hi [CQ:at,qq=42]",
		"raw_message":"hi [CQ:at,qq=42]"
	}`), &info)
	if err != nil {
		t.Fatal(err)
	}
	if info.Message.Text() != "hi " || info.Message[1].Data["id"] != "42" {
		t.Fatalf("message = %#v", info.Message)
	}
	if info.MessageID != 2 || info.RealID != 3 || info.Sender.UserID != 42 {
		t.Fatalf("info = %#v", info)
	}
	if err := json.Unmarshal([]byte(`{"message_id":"4","raw_message":"撤回提示"}`), &info); err != nil {
		t.Fatal(err)
	}
	if len(info.Message) != 0 || info.MessageID != 4 || info.RawMessage != "撤回提示" {
		t.Fatalf("info = %#v", info)
	}
}

func TestInfoStructsAcceptStringIDs(t *testing.T) {
	var login LoginInfo
	if err := json.Unmarshal([]byte(`{"user_id":"10000","nickname":"bot"}`), &login); err != nil {
		t.Fatal(err)
	}
	if login.UserID != 10000 {
		t.Fatalf("login=%#v", login)
	}

	var friend FriendInfo
	if err := json.Unmarshal([]byte(`{"user_id":"42","nickname":"alice","remark":"a"}`), &friend); err != nil {
		t.Fatal(err)
	}
	if friend.UserID != 42 {
		t.Fatalf("friend=%#v", friend)
	}

	var group GroupInfo
	if err := json.Unmarshal([]byte(`{"group_id":"100","group_name":"g","member_count":1,"max_member_count":2}`), &group); err != nil {
		t.Fatal(err)
	}
	if group.GroupID != 100 {
		t.Fatalf("group=%#v", group)
	}

	var member GroupMemberInfo
	if err := json.Unmarshal([]byte(`{"group_id":"100","user_id":"42","nickname":"alice","join_time":"10","last_sent_time":"11","title_expire_time":"12"}`), &member); err != nil {
		t.Fatal(err)
	}
	if member.GroupID != 100 || member.UserID != 42 || member.JoinTime != 10 || member.LastSentTime != 11 || member.TitleExpireTime != 12 {
		t.Fatalf("member=%#v", member)
	}

	var honor GroupHonorInfo
	if err := json.Unmarshal([]byte(`{"group_id":"100","current_talkative":{"user_id":"42","nickname":"alice"}}`), &honor); err != nil {
		t.Fatal(err)
	}
	if honor.GroupID != 100 || honor.CurrentTalkative.UserID != 42 {
		t.Fatalf("honor=%#v", honor)
	}

	var files GroupFiles
	if err := json.Unmarshal([]byte(`{"files":[{"group_id":"100","file_id":"f","busid":"1","file_size":"2","upload_time":"3","dead_time":"4","modify_time":"5","download_times":"6","uploader":"42"}],"folders":[{"group_id":"100","folder_id":"d","create_time":"7","creator":"42","total_file_count":"8"}]}`), &files); err != nil {
		t.Fatal(err)
	}
	if files.Files[0].Uploader != 42 || files.Folders[0].TotalFileCount != 8 {
		t.Fatalf("files=%#v", files)
	}
}

func TestClientActionTimeoutKeepsExistingDeadline(t *testing.T) {
	transport := &recordTransport{response: &Response{Status: "ok", RetCode: 0}}
	client := &Client{transport: transport, actionTimeout: time.Minute}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := client.Call(ctx, "get_status", nil, nil); err != nil {
		t.Fatal(err)
	}
	if _, ok := transport.ctx.Deadline(); !ok {
		t.Fatal("deadline was lost")
	}
}

type recordTransport struct {
	action   string
	params   any
	response *Response
	ctx      context.Context
}

func (t *recordTransport) Start(context.Context, func(context.Context, *Event) error) error {
	return nil
}

func (t *recordTransport) CallRaw(ctx context.Context, action string, params any) (*Response, error) {
	t.ctx = ctx
	t.action = action
	t.params = params
	return t.response, nil
}
