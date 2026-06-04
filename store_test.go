package anybot

import (
	"context"
	"testing"
	"time"
)

func TestMemoryStoreTTLAndSessionJSON(t *testing.T) {
	store := NewMemoryStore()
	now := time.Unix(100, 0)
	store.now = func() time.Time { return now }

	session := NewSession(store, "conv")
	if err := session.SaveJSON(context.Background(), "state", map[string]string{"step": "ask"}, time.Second); err != nil {
		t.Fatal(err)
	}
	var out map[string]string
	ok, err := session.LoadJSON(context.Background(), "state", &out)
	if err != nil || !ok || out["step"] != "ask" {
		t.Fatalf("ok=%v err=%v out=%#v", ok, err, out)
	}
	now = now.Add(2 * time.Second)
	ok, err = session.LoadJSON(context.Background(), "state", &out)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expired session value still exists")
	}
}

func TestMemoryStoreSweepsExpiredItems(t *testing.T) {
	store := NewMemoryStore()
	store.sweepEvery = time.Second
	now := time.Unix(100, 0)
	store.now = func() time.Time { return now }

	if err := store.Set(context.Background(), "old", []byte("x"), time.Second); err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Second)
	if err := store.Set(context.Background(), "fresh", []byte("y"), 0); err != nil {
		t.Fatal(err)
	}
	store.mu.RLock()
	_, oldExists := store.items["old"]
	_, freshExists := store.items["fresh"]
	store.mu.RUnlock()
	if oldExists || !freshExists {
		t.Fatalf("old=%v fresh=%v", oldExists, freshExists)
	}
}

func TestMemoryStoreSweep(t *testing.T) {
	store := NewMemoryStore()
	now := time.Unix(100, 0)
	store.now = func() time.Time { return now }
	if err := store.Set(context.Background(), "old", []byte("x"), time.Second); err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Second)
	if err := store.Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := store.Get(context.Background(), "old"); err != nil || ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
}

func TestContextSessionScopes(t *testing.T) {
	app := New()
	c := NewTestContext(app, &Event{
		Protocol: ProtocolOneBot11,
		Type:     "message",
		UserID:   "42",
		GroupID:  "100",
	})
	if c.Session().Key() != "onebot11:group:100:user:42" {
		t.Fatalf("conversation key = %q", c.Session().Key())
	}
	if c.UserSession().Key() != "onebot11:user:42" {
		t.Fatalf("user key = %q", c.UserSession().Key())
	}
	if c.GroupSession().Key() != "onebot11:group:100" {
		t.Fatalf("group key = %q", c.GroupSession().Key())
	}
	if c.SessionBy("custom").Key() != "custom" {
		t.Fatalf("custom key = %q", c.SessionBy("custom").Key())
	}
}
