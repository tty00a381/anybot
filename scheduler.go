package anybot

import "sync"

type keyedLocks struct {
	mu    sync.Mutex
	locks map[string]*keyedLock
}

type keyedLock struct {
	mu   sync.Mutex
	refs int
}

func newKeyedLocks() *keyedLocks {
	return &keyedLocks{locks: map[string]*keyedLock{}}
}

func (l *keyedLocks) lock(key string) func() {
	l.mu.Lock()
	item := l.locks[key]
	if item == nil {
		item = &keyedLock{}
		l.locks[key] = item
	}
	item.refs++
	l.mu.Unlock()

	item.mu.Lock()
	return func() {
		item.mu.Unlock()

		l.mu.Lock()
		item.refs--
		if item.refs == 0 {
			delete(l.locks, key)
		}
		l.mu.Unlock()
	}
}
