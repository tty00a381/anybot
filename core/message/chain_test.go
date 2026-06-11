package message

import "testing"

func TestChainTextAndAppend(t *testing.T) {
	chain := New(Text("hello"), Image("file://a.png")).Append(Text(" world"))
	if got := chain.Text(); got != "hello world" {
		t.Fatalf("Text() = %q", got)
	}
	if chain.IsZero() {
		t.Fatal("chain should not be zero")
	}
	if Raw("custom", map[string]any{"x": "y"}).Type != "custom" || Video("a.mp4").Type != "video" {
		t.Fatal("common segment builders returned unexpected types")
	}
}
