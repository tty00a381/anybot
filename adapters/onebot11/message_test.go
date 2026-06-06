package onebot11

import (
	"encoding/json"
	"testing"

	"github.com/tty00a381/anybot/core"
	"github.com/tty00a381/anybot/core/message"
)

func TestParseCQAndRender(t *testing.T) {
	chain := ParseCQ("hi &amp; [CQ:at,qq=123] ok")
	if got := chain.Text(); got != "hi &  ok" {
		t.Fatalf("text = %q", got)
	}
	if chain[1].Type != "at" || chain[1].Data["id"] != "123" {
		t.Fatalf("at segment = %#v", chain[1])
	}
	rendered := CQString(message.New(message.Text("hi [x]"), At(123)))
	if rendered != "hi &#91;x&#93;[CQ:at,qq=123]" {
		t.Fatalf("rendered = %q", rendered)
	}
}

func TestSegmentBuilders(t *testing.T) {
	node := CustomNode(42, "bot", message.New(Text("hi"), At(10000)))
	wire := toWireChain(message.New(node))
	content := wire[0].Data["content"].(message.Chain)
	if content[1].Data["qq"] != "10000" {
		t.Fatalf("node content = %#v", content)
	}
	if JSON(`{"a":1}`).Type != "json" || XML("<a/>").Type != "xml" || Share("u", "t", "c", "i").Type != "share" {
		t.Fatal("common segment builders returned unexpected types")
	}
}

func TestEventUnmarshalAndNormalize(t *testing.T) {
	var event Event
	err := json.Unmarshal([]byte(`{
		"time": 1,
		"self_id": "10000",
		"post_type": "message",
		"message_type": "group",
		"message_id": "88",
		"group_id": "123",
		"user_id": "456",
		"sender": {"user_id": "456", "nickname": "alice"},
		"message": "hello [CQ:at,qq=10000]",
		"x_napcat": true
	}`), &event)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := event.Extra["x_napcat"]; !ok {
		t.Fatal("extra field not preserved")
	}
	normalized := event.Normalize()
	if normalized.DetailType != "group" || normalized.GroupID != "123" || normalized.UserID != "456" {
		t.Fatalf("normalized = %#v", normalized)
	}
	if event.Sender.UserID != 456 {
		t.Fatalf("sender = %#v", event.Sender)
	}
	if normalized.Text != "hello " {
		t.Fatalf("text = %q", normalized.Text)
	}
}

func TestEventUnmarshalClearsMissingFields(t *testing.T) {
	var event Event
	if err := json.Unmarshal([]byte(`{
		"post_type":"message",
		"message_type":"private",
		"user_id":42,
		"message":"hello",
		"x_napcat":true
	}`), &event); err != nil {
		t.Fatal(err)
	}
	if len(event.Message) == 0 || len(event.Extra) == 0 {
		t.Fatalf("event=%#v", event)
	}
	if err := json.Unmarshal([]byte(`{"post_type":"notice","notice_type":"friend_recall","user_id":42}`), &event); err != nil {
		t.Fatal(err)
	}
	if len(event.Message) != 0 {
		t.Fatalf("message 未清空: %#v", event.Message)
	}
	if event.Extra != nil {
		t.Fatalf("extra 未清空: %#v", event.Extra)
	}
	if event.PostType != "notice" || event.NoticeType != "friend_recall" {
		t.Fatalf("event=%#v", event)
	}
}

func TestContextHelpers(t *testing.T) {
	raw := &Event{PostType: "message", UserID: 42}
	ctx := core.New(
		core.WithAdapter(New(&recordTransport{})),
	)
	c := core.NewTestContext(ctx, raw.Normalize())
	client, ok := ClientFrom(c)
	if !ok || client == nil {
		t.Fatal("client not found")
	}
	event, ok := EventFrom(c)
	if !ok || event.UserID != 42 {
		t.Fatalf("event=%#v ok=%v", event, ok)
	}
}
