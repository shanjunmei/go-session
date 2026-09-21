package memory

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/shanjunmei/go-session" // 根据实际模块路径替换
)

type memoryStore struct {
	sessions map[string]session.Session
	mu       sync.RWMutex
}

func NewStore() session.SessionStore {
	return &memoryStore{
		sessions: make(map[string]session.Session),
	}
}

func (m *memoryStore) NewSession(id string, expires time.Duration) (session.Session, error) {
	return newMemorySession(id, expires), nil
}

func (m *memoryStore) Create(ctx context.Context, s session.Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.sessions[s.ID()]; ok {
		return session.ErrSessionCreateErr
	}
	m.sessions[s.ID()] = s
	return nil
}

func (m *memoryStore) Get(ctx context.Context, sessionId string) (session.Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[sessionId]
	if !ok {
		return nil, session.ErrSessionNotFound
	}
	if s.IsExpired() {
		return nil, session.ErrSessionExpired
	}
	s.UpdateAccessedAt()
	return s, nil
}

func (m *memoryStore) Update(ctx context.Context, s session.Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s.IsExpired() {
		delete(m.sessions, s.ID())
		return session.ErrSessionExpired
	}
	m.sessions[s.ID()] = s
	return nil
}

func (m *memoryStore) Delete(ctx context.Context, sessionId string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.sessions[sessionId]; !ok {
		return session.ErrSessionNotFound
	}
	delete(m.sessions, sessionId)
	return nil
}

// Count 统计当前活动（未过期）会话数。
func (m *memoryStore) Count(ctx context.Context) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	n := 0
	for _, s := range m.sessions {
		if !s.IsExpired() {
			n++
		}
	}
	return n, nil
}

func (m *memoryStore) GC(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for id, s := range m.sessions {
		if now.After(s.ExpiresAt()) {
			log.Printf("session GC: delete expired session [ID: %s], expired at: %v", id, s.ExpiresAt())
			delete(m.sessions, id)
		}
	}
	return nil
}

func (m *memoryStore) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions = make(map[string]session.Session)
	return nil
}
