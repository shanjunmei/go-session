package sqlstore

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/shanjunmei/go-session"
)

type sqlSession struct {
	id           string
	data         map[string]any
	expiresAt    time.Time
	lastAccessed time.Time
	mu           sync.RWMutex
	dirty        bool
	store        session.SessionStore
}

func newSQLSession(id string, expires time.Duration) *sqlSession {
	now := time.Now()
	return &sqlSession{
		id:           id,
		data:         make(map[string]any),
		expiresAt:    now.Add(expires),
		lastAccessed: now,
		dirty:        true,
	}
}

func loadSQLSession(id, data string, expiresAt, lastAccessed time.Time) (*sqlSession, error) {
	var m map[string]any
	if data != "" {
		if err := json.Unmarshal([]byte(data), &m); err != nil {
			return nil, err
		}
	}
	if m == nil {
		m = make(map[string]any)
	}
	return &sqlSession{
		id:           id,
		data:         m,
		expiresAt:    expiresAt,
		lastAccessed: lastAccessed,
		dirty:        false,
	}, nil
}

// snapshot returns a consistent, marshalled copy of the session state.
// Must be called without holding s.mu.
func (s *sqlSession) snapshot() (id, dataJSON string, expiresAt, lastAccessed time.Time, err error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, err := json.Marshal(s.data)
	if err != nil {
		return "", "", time.Time{}, time.Time{}, fmt.Errorf("marshal session data: %w", err)
	}
	return s.id, string(b), s.expiresAt, s.lastAccessed, nil
}

func (s *sqlSession) isDirty() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.dirty
}

func (s *sqlSession) clearDirty() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dirty = false
}

func (s *sqlSession) ID() string { return s.id }

func (s *sqlSession) Get(key string) (any, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.data[key]
	return v, ok
}

func (s *sqlSession) Set(key string, value any) {
	s.mu.Lock()
	s.data[key] = value
	s.lastAccessed = time.Now()
	s.dirty = true
	s.mu.Unlock()
	s.persist()
}

func (s *sqlSession) Del(key string) {
	s.mu.Lock()
	delete(s.data, key)
	s.lastAccessed = time.Now()
	s.dirty = true
	s.mu.Unlock()
	s.persist()
}

func (s *sqlSession) Clear() {
	s.mu.Lock()
	s.data = make(map[string]any)
	s.lastAccessed = time.Now()
	s.dirty = true
	s.mu.Unlock()
	s.persist()
}

func (s *sqlSession) ExpiresAt() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.expiresAt
}

func (s *sqlSession) SetExpires(d time.Duration) {
	s.mu.Lock()
	s.expiresAt = time.Now().Add(d)
	s.lastAccessed = time.Now()
	s.dirty = true
	s.mu.Unlock()
	s.persist()
}

func (s *sqlSession) IsExpired() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return time.Now().After(s.expiresAt)
}

func (s *sqlSession) LastAccessedAt() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastAccessed
}

func (s *sqlSession) UpdateAccessedAt() {
	s.mu.Lock()
	s.lastAccessed = time.Now()
	s.dirty = true
	s.mu.Unlock()
	s.persist()
}

// persist flushes the session to its store. Held locks must be released
// before calling this; the store reads a consistent snapshot under RLock.
func (s *sqlSession) persist() {
	if s.store != nil {
		if err := s.store.Update(context.Background(), s); err != nil {
			slog.Error("session persist failed", "id", s.id, "error", err)
		}
	}
}
