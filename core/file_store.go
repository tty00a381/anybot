package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// FileStore 是 goroutine 安全的文件持久化 Store，适合插件运行框架保存轻量状态。
type FileStore struct {
	mu    sync.Mutex
	path  string
	items map[string]fileStoreItem
	now   func() time.Time
}

type fileStoreDocument struct {
	Version int                      `json:"version"`
	Items   map[string]fileStoreItem `json:"items"`
}

type fileStoreItem struct {
	Value     []byte     `json:"value"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// NewFileStore 打开或创建一个 JSON 文件存储。文件不存在时会在第一次写入时创建。
func NewFileStore(path string) (*FileStore, error) {
	if path == "" {
		return nil, fmt.Errorf("store path is required")
	}
	store := &FileStore{
		path:  path,
		items: map[string]fileStoreItem{},
		now:   time.Now,
	}
	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

// Path 返回当前存储文件路径。
func (s *FileStore) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}

// Get 读取指定键的值；键不存在或已过期时返回 ok=false。
func (s *FileStore) Get(ctx context.Context, key string) ([]byte, bool, error) {
	if err := checkStoreContext(ctx); err != nil {
		return nil, false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[key]
	if !ok {
		return nil, false, nil
	}
	if item.expired(s.now()) {
		before := cloneFileStoreItems(s.items)
		delete(s.items, key)
		if err := s.saveLocked(ctx); err != nil {
			s.items = before
			return nil, false, err
		}
		return nil, false, nil
	}
	return append([]byte(nil), item.Value...), true, nil
}

// Set 写入指定键的值。ttl 大于 0 时，值会在到期后失效。
func (s *FileStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if err := checkStoreContext(ctx); err != nil {
		return err
	}
	item := fileStoreItem{Value: append([]byte(nil), value...)}
	if ttl > 0 {
		expiresAt := s.now().Add(ttl)
		item.ExpiresAt = &expiresAt
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	before := cloneFileStoreItems(s.items)
	s.items[key] = item
	s.sweepExpiredLocked(s.now())
	if err := s.saveLocked(ctx); err != nil {
		s.items = before
		return err
	}
	return nil
}

// Delete 删除指定键。
func (s *FileStore) Delete(ctx context.Context, key string) error {
	if err := checkStoreContext(ctx); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.items[key]; !ok {
		return nil
	}
	before := cloneFileStoreItems(s.items)
	delete(s.items, key)
	if err := s.saveLocked(ctx); err != nil {
		s.items = before
		return err
	}
	return nil
}

// Sweep 立即清理所有已过期的文件存储项。
func (s *FileStore) Sweep(ctx context.Context) error {
	if err := checkStoreContext(ctx); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	before := cloneFileStoreItems(s.items)
	if !s.sweepExpiredLocked(s.now()) {
		return nil
	}
	if err := s.saveLocked(ctx); err != nil {
		s.items = before
		return err
	}
	return nil
}

func (s *FileStore) load() error {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return fmt.Errorf("%s: store file is empty", s.path)
	}
	var doc fileStoreDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("%s: %w", s.path, err)
	}
	if doc.Version != 1 {
		return fmt.Errorf("%s: unsupported store version %d", s.path, doc.Version)
	}
	if doc.Items == nil {
		doc.Items = map[string]fileStoreItem{}
	}
	s.items = doc.Items
	return nil
}

func (s *FileStore) saveLocked(ctx context.Context) error {
	if err := checkStoreContext(ctx); err != nil {
		return err
	}
	doc := fileStoreDocument{Version: 1, Items: s.items}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return writeFileAtomic(s.path, data, 0o600)
}

func (s *FileStore) sweepExpiredLocked(now time.Time) bool {
	var changed bool
	for key, item := range s.items {
		if item.expired(now) {
			delete(s.items, key)
			changed = true
		}
	}
	return changed
}

func cloneFileStoreItems(items map[string]fileStoreItem) map[string]fileStoreItem {
	out := make(map[string]fileStoreItem, len(items))
	for key, item := range items {
		item.Value = append([]byte(nil), item.Value...)
		if item.ExpiresAt != nil {
			expiresAt := *item.ExpiresAt
			item.ExpiresAt = &expiresAt
		}
		out[key] = item
	}
	return out
}

func (item fileStoreItem) expired(now time.Time) bool {
	return item.ExpiresAt != nil && now.After(*item.ExpiresAt)
}

func checkStoreContext(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	removeTmp := true
	defer func() {
		if removeTmp {
			_ = os.Remove(tmpPath)
		}
	}()
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	removeTmp = false
	// After rename the write is committed from this process' point of view. A
	// directory fsync failure only weakens crash durability, so keep memory in
	// step with the file instead of reporting a post-commit failure upstream.
	_ = syncDir(dir)
	return nil
}

func syncDir(dir string) error {
	file, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer file.Close()
	return file.Sync()
}
