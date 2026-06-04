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
	if Face(14).Type != "face" || Record("a.amr").Type != "record" || Video("a.mp4").Type != "video" {
		t.Fatal("common segment builders returned unexpected types")
	}
}
