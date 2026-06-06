package core

import (
	"context"
	"encoding/json"
	"sync"
	"time"
)

// Store 定义会话数据的读写接口，值以字节形式存储并可带 TTL。
type Store interface {
	Get(context.Context, string) ([]byte, bool, error)
	Set(context.Context, string, []byte, time.Duration) error
	Delete(context.Context, string) error
}

// MemoryStore 是 goroutine 安全的进程内 Store，适合短期会话状态。
type MemoryStore struct {
	mu          sync.RWMutex
	items       map[string]memoryItem
	now         func() time.Time
	sweepEvery  time.Duration
	lastSweptAt time.Time
}

type memoryItem struct {
	value     []byte
	expiresAt time.Time
}

// NewMemoryStore 创建默认内存存储。
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		items:      map[string]memoryItem{},
		now:        time.Now,
		sweepEvery: time.Minute,
	}
}

// Get 读取指定键的值；键不存在或已过期时返回 ok=false。
func (s *MemoryStore) Get(ctx context.Context, key string) ([]byte, bool, error) {
	select {
	case <-ctx.Done():
		return nil, false, ctx.Err()
	default:
	}
	s.mu.RLock()
	item, ok := s.items[key]
	s.mu.RUnlock()
	if !ok {
		return nil, false, nil
	}
	if !item.expiresAt.IsZero() && s.now().After(item.expiresAt) {
		_ = s.Delete(context.Background(), key)
		return nil, false, nil
	}
	return append([]byte(nil), item.value...), true, nil
}

// Set 写入指定键的值。ttl 大于 0 时，值会在到期后失效。
func (s *MemoryStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	now := s.now()
	item := memoryItem{value: append([]byte(nil), value...)}
	if ttl > 0 {
		item.expiresAt = now.Add(ttl)
	}
	s.mu.Lock()
	s.items[key] = item
	s.sweepExpiredLocked(now, false)
	s.mu.Unlock()
	return nil
}

// Delete 删除指定键。
func (s *MemoryStore) Delete(ctx context.Context, key string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	s.mu.Lock()
	delete(s.items, key)
	s.mu.Unlock()
	return nil
}

// Sweep 立即清理所有已过期的内存项。
func (s *MemoryStore) Sweep(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	s.mu.Lock()
	s.sweepExpiredLocked(s.now(), true)
	s.mu.Unlock()
	return nil
}

func (s *MemoryStore) sweepExpiredLocked(now time.Time, force bool) {
	if !force {
		if s.sweepEvery <= 0 {
			return
		}
		if !s.lastSweptAt.IsZero() && now.Sub(s.lastSweptAt) < s.sweepEvery {
			return
		}
	}
	for key, item := range s.items {
		if !item.expiresAt.IsZero() && now.After(item.expiresAt) {
			delete(s.items, key)
		}
	}
	s.lastSweptAt = now
}

// Session 是基于固定键前缀的 Store 视图。
type Session struct {
	store Store
	key   string
}

// NewSession 创建指定键前缀下的会话视图。
func NewSession(store Store, key string) *Session {
	return &Session{store: store, key: key}
}

// Key 返回当前会话视图使用的键前缀。
func (s *Session) Key() string {
	return s.key
}

// Get 读取会话视图中的原始字节值。
func (s *Session) Get(ctx context.Context, name string) ([]byte, bool, error) {
	return s.store.Get(ctx, s.key+":"+name)
}

// Set 写入会话视图中的原始字节值。
func (s *Session) Set(ctx context.Context, name string, value []byte, ttl time.Duration) error {
	return s.store.Set(ctx, s.key+":"+name, value, ttl)
}

// Delete 删除会话视图中的值。
func (s *Session) Delete(ctx context.Context, name string) error {
	return s.store.Delete(ctx, s.key+":"+name)
}

// LoadJSON 读取会话值并按 JSON 解码到 out。
func (s *Session) LoadJSON(ctx context.Context, name string, out any) (bool, error) {
	data, ok, err := s.Get(ctx, name)
	if err != nil || !ok {
		return ok, err
	}
	return true, json.Unmarshal(data, out)
}

// SaveJSON 将 value 编码为 JSON 后写入会话视图。
func (s *Session) SaveJSON(ctx context.Context, name string, value any, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return s.Set(ctx, name, data, ttl)
}
