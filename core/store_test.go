package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const storeTestProtocol Protocol = "test"

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

func TestFileStorePersistsValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "store.json")
	store, err := NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Set(context.Background(), "binding", []byte(`{"uuid":"abc"}`), 0); err != nil {
		t.Fatal(err)
	}
	reopened, err := NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	data, ok, err := reopened.Get(context.Background(), "binding")
	if err != nil || !ok || string(data) != `{"uuid":"abc"}` {
		t.Fatalf("ok=%v err=%v data=%s", ok, err, data)
	}
}

func TestFileStorePersistsTTL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "store.json")
	now := time.Unix(100, 0)
	store, err := NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	store.now = func() time.Time { return now }
	if err := store.Set(context.Background(), "code", []byte("123456"), time.Second); err != nil {
		t.Fatal(err)
	}
	reopened, err := NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Second)
	reopened.now = func() time.Time { return now }
	if _, ok, err := reopened.Get(context.Background(), "code"); err != nil || ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestFileStoreCopiesValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "store.json")
	store, err := NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	value := []byte("abc")
	if err := store.Set(context.Background(), "k", value, 0); err != nil {
		t.Fatal(err)
	}
	value[0] = 'x'
	out, ok, err := store.Get(context.Background(), "k")
	if err != nil || !ok || string(out) != "abc" {
		t.Fatalf("ok=%v err=%v out=%q", ok, err, out)
	}
	out[0] = 'y'
	out, ok, err = store.Get(context.Background(), "k")
	if err != nil || !ok || string(out) != "abc" {
		t.Fatalf("ok=%v err=%v out=%q", ok, err, out)
	}
}

func TestFileStoreRejectsInvalidDocument(t *testing.T) {
	path := filepath.Join(t.TempDir(), "store.json")
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewFileStore(path); err == nil {
		t.Fatal("invalid store should be rejected")
	}
}

func TestFileStoreHonorsContext(t *testing.T) {
	store, err := NewFileStore(filepath.Join(t.TempDir(), "store.json"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := store.Set(ctx, "k", []byte("v"), 0); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
}

func TestFileStoreRollsBackMemoryOnSaveError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "store.json")
	store, err := NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Set(context.Background(), "old", []byte("value"), 0); err != nil {
		t.Fatal(err)
	}
	store.path = t.TempDir()
	if err := store.Set(context.Background(), "new", []byte("value"), 0); err == nil {
		t.Fatal("set should fail when store path is a directory")
	}
	if _, ok := store.items["new"]; ok {
		t.Fatal("failed set should not remain in memory")
	}
	if string(store.items["old"].Value) != "value" {
		t.Fatalf("old value changed: %#v", store.items["old"])
	}
}

func TestContextSessionScopes(t *testing.T) {
	app := New()
	c := NewTestContext(app, &Event{
		Protocol: storeTestProtocol,
		Type:     "message",
		UserID:   "42",
		GroupID:  "100",
	})
	if c.Session().Key() != "test:group:100:user:42" {
		t.Fatalf("conversation key = %q", c.Session().Key())
	}
	if c.UserSession().Key() != "test:user:42" {
		t.Fatalf("user key = %q", c.UserSession().Key())
	}
	if c.GroupSession().Key() != "test:group:100" {
		t.Fatalf("group key = %q", c.GroupSession().Key())
	}
	if c.SessionBy("custom").Key() != "custom" {
		t.Fatalf("custom key = %q", c.SessionBy("custom").Key())
	}
}
