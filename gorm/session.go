package gorm

import (
	"context"
	"encoding/json"
	"go-session"
	"log/slog"
	"sync"
	"time"
)

// gormSession 实现 session.Session 接口，数据存储在内存中，由 gormStore 负责持久化
type gormSession struct {
	id           string
	data         map[string]any
	expiresAt    time.Time
	lastAccessed time.Time
	mu           sync.RWMutex
	dirty        bool
	store        session.SessionStore
}

// newGormSession 创建新的会话实例
func newGormSession(id string, expires time.Duration) *gormSession {
	now := time.Now()
	return &gormSession{
		id:           id,
		data:         make(map[string]any),
		expiresAt:    now.Add(expires),
		lastAccessed: now,
		dirty:        true, // 新创建的会话需要保存
	}
}

// loadGormSession 从数据库模型加载会话
func loadGormSession(model *SessionModel) (*gormSession, error) {
	var data map[string]any
	if model.Data != "" {
		if err := json.Unmarshal([]byte(model.Data), &data); err != nil {
			return nil, err
		}
	} else {
		data = make(map[string]any)
	}
	return &gormSession{
		id:           model.ID,
		data:         data,
		expiresAt:    model.ExpiresAt,
		lastAccessed: model.LastAccessed,
		dirty:        false,
	}, nil
}

// ID 返回会话ID
func (s *gormSession) ID() string {
	return s.id
}

// Get 获取属性
func (s *gormSession) Get(key string) (any, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.data[key]
	return val, ok
}

// Set 设置属性
func (s *gormSession) Set(key string, value any) {
	s.mu.Lock()
	//defer s.mu.Unlock()
	s.data[key] = value
	s.lastAccessed = time.Now()
	s.dirty = true
	s.mu.Unlock()
	err := s.store.Update(context.Background(), s)
	if err != nil {
		slog.Warn("failed to update session after set", "session_id", s.id, "error", err)
	}

}

// Del 删除属性
func (s *gormSession) Del(key string) {
	s.mu.Lock()
	//defer s.mu.Unlock()
	delete(s.data, key)
	s.lastAccessed = time.Now()
	s.dirty = true
	s.mu.Unlock()
	err := s.store.Update(context.Background(), s)
	if err != nil {
		slog.Warn("failed to update session after del", "session_id", s.id, "error", err)
	}
}

// Clear 清空属性
func (s *gormSession) Clear() {
	s.mu.Lock()
	//defer s.mu.Unlock()
	s.data = make(map[string]any)
	s.lastAccessed = time.Now()
	s.dirty = true
	s.mu.Unlock()
	err := s.store.Update(context.Background(), s)
	if err != nil {
		slog.Warn("failed to update session after clear", "session_id", s.id, "error", err)
	}
}

// ExpiresAt 返回过期时间
func (s *gormSession) ExpiresAt() time.Time {
	return s.expiresAt
}

// SetExpires 设置过期时长
func (s *gormSession) SetExpires(d time.Duration) {
	s.mu.Lock()
	//defer s.mu.Unlock()
	s.expiresAt = time.Now().Add(d)
	s.dirty = true
	s.mu.Unlock()
	err := s.store.Update(context.Background(), s)
	if err != nil {
		slog.Warn("failed to update session after set expires", "session_id", s.id, "error", err)
	}
}

// IsExpired 是否过期
func (s *gormSession) IsExpired() bool {
	return time.Now().After(s.expiresAt)
}

// LastAccessedAt 最后访问时间
func (s *gormSession) LastAccessedAt() time.Time {
	return s.lastAccessed
}

// UpdateAccessedAt 更新最后访问时间
func (s *gormSession) UpdateAccessedAt() {
	s.mu.Lock()
	//defer s.mu.Unlock()
	s.lastAccessed = time.Now()
	s.dirty = true
	s.mu.Unlock()
	err := s.store.Update(context.Background(), s)
	if err != nil {
		slog.Warn("failed to update session after update accessed time", "session_id", s.id, "error", err)
	}

}

// isDirty 返回是否有未保存修改
func (s *gormSession) isDirty() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.dirty
}

// clearDirty 清除脏标记
func (s *gormSession) clearDirty() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dirty = false
}

// toModel 转换为数据库模型
func (s *gormSession) toModel() (*SessionModel, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	dataBytes, err := json.Marshal(s.data)
	if err != nil {
		return nil, err
	}
	return &SessionModel{
		ID:           s.id,
		Data:         string(dataBytes),
		ExpiresAt:    s.expiresAt,
		LastAccessed: s.lastAccessed,
	}, nil
}
