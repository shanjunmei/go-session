package session

import (
	"sync"
	"time"
)

// -------------------------- 默认实现：内存Session --------------------------
// memorySession Session接口的内存实现
type memorySession struct {
	id           string
	data         map[string]interface{}
	expiresAt    time.Time
	lastAccessed time.Time
	mu           sync.RWMutex
}

// NewMemorySession 创建内存Session实例
func NewMemorySession(sessionID string, expires time.Duration) Session {
	now := time.Now()
	return &memorySession{
		id:           sessionID,
		data:         make(map[string]interface{}),
		expiresAt:    now.Add(expires),
		lastAccessed: now,
	}
}

// 实现Session接口
func (m *memorySession) ID() string {
	return m.id
}

func (m *memorySession) Get(key string) (interface{}, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	val, ok := m.data[key]
	return val, ok
}

func (m *memorySession) Set(key string, value interface{}) {
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
	m.data = make(map[string]interface{})
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
