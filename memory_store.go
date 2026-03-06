package session

import (
	"context"
	"sync"
	"time"
)

// -------------------------- 默认实现：内存SessionStore --------------------------
// memoryStore SessionStore接口的内存实现（持久化到内存，可扩展为文件/Redis/MySQL）
type memoryStore struct {
	sessions map[string]Session
	mu       sync.RWMutex
	config   StoreConfig
}

// NewMemoryStore 创建内存存储实例
func NewMemoryStore(cfg StoreConfig) SessionStore {
	return &memoryStore{
		sessions: make(map[string]Session),
		config:   cfg,
	}
}

// 实现SessionStore接口
func (m *memoryStore) Create(ctx context.Context, s Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.sessions[s.ID()]; ok {
		return ErrSessionCreateErr
	}
	m.sessions[s.ID()] = s
	return nil
}

func (m *memoryStore) Get(ctx context.Context, sessionID string) (Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[sessionID]
	if !ok {
		return nil, ErrSessionNotFound
	}
	if s.IsExpired() {
		return nil, ErrSessionExpired
	}
	s.UpdateAccessedAt()
	return s, nil
}

func (m *memoryStore) Update(ctx context.Context, s Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s.IsExpired() {
		delete(m.sessions, s.ID())
		return ErrSessionExpired
	}
	m.sessions[s.ID()] = s
	return nil
}

func (m *memoryStore) Delete(ctx context.Context, sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.sessions[sessionID]; !ok {
		return ErrSessionNotFound
	}
	delete(m.sessions, sessionID)
	return nil
}

func (m *memoryStore) GC(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for id, s := range m.sessions {
		if now.After(s.ExpiresAt()) {
			delete(m.sessions, id)
		}
	}
	return nil
}

func (m *memoryStore) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions = make(map[string]Session)
	return nil
}
