package message

import "testing"

func TestSDKMessageBuilders(t *testing.T) {
	chain := New(Text("hello"), Raw("mention", map[string]any{"id": "42"}), Image("file://a.png"))
	if got := chain.Text(); got != "hello" {
		t.Fatalf("Text() = %q", got)
	}
	if chain[1].Type != "mention" || chain[2].Type != "image" {
		t.Fatalf("chain = %#v", chain)
	}
	clone := chain.Clone()
	clone[0].Data["text"] = "mutated"
	if got := chain.Text(); got != "hello" {
		t.Fatalf("original chain mutated through clone: %q", got)
	}
}
