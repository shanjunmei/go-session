package memory

import (
	"sync"
	"time"
)

type memorySession struct {
	id           string
	data         map[string]any
	expiresAt    time.Time
	lastAccessed time.Time
	mu           sync.RWMutex
}

func newMemorySession(id string, expires time.Duration) *memorySession {
	now := time.Now()
	return &memorySession{
		id:           id,
		data:         make(map[string]any),
		expiresAt:    now.Add(expires),
		lastAccessed: now,
	}
}

func (m *memorySession) ID() string {
	return m.id
}

func (m *memorySession) Get(key string) (any, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	val, ok := m.data[key]
	return val, ok
}

func (m *memorySession) Set(key string, value any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = value
	m.lastAccessed = time.Now()
}

func (m *memorySession) Del(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
	m.lastAccessed = time.Now()
}

func (m *memorySession) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data = make(map[string]any)
	m.lastAccessed = time.Now()
}

func (m *memorySession) ExpiresAt() time.Time {
	return m.expiresAt
}

func (m *memorySession) SetExpires(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.expiresAt = time.Now().Add(d)
}

func (m *memorySession) IsExpired() bool {
	return time.Now().After(m.expiresAt)
}

func (m *memorySession) LastAccessedAt() time.Time {
	return m.lastAccessed
}

func (m *memorySession) UpdateAccessedAt() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastAccessed = time.Now()
}
