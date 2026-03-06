package session

import (
	"context"
	"net/http"
	"time"
)

// -------------------------- 默认实现：SessionManager --------------------------
// manager SessionManager接口的默认实现
type manager struct {
	store    SessionStore
	config   Config
	gcTicker *time.Ticker
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewManager 创建会话管理器实例
func NewManager(store SessionStore, cfg Config) SessionManager {
	if cfg.Cookie.Name == "" {
		cfg.Cookie = DefaultConfig.Cookie
	}
	if cfg.Store.Expires == 0 {
		cfg.Store = DefaultConfig.Store
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &manager{
		store:  store,
		config: cfg,
		ctx:    ctx,
		cancel: cancel,
	}
}

// 实现SessionManager接口
func (m *manager) GetSession(w http.ResponseWriter, r *http.Request) (Session, error) {
	// 1. 从Cookie读取会话ID
	var sessionID string
	cookie, err := r.Cookie(m.config.Cookie.Name)
	if err == nil {
		sessionID = cookie.Value
	}

	// 2. 从存储获取会话
	if sessionID != "" {
		s, err := m.store.Get(r.Context(), sessionID)
		if err == nil && !s.IsExpired() {
			return s, nil
		}
	}

	// 3. 创建新会话
	sessionID = NewSessionId(m.config.Store.SessionIDLen)
	newSession := NewMemorySession(sessionID, m.config.Store.Expires)
	if err := m.store.Create(r.Context(), newSession); err != nil {
		return nil, ErrSessionCreateErr
	}

	// 4. 设置Cookie
	http.SetCookie(w, &http.Cookie{
		Name:     m.config.Cookie.Name,
		Value:    sessionID,
		Path:     m.config.Cookie.Path,
		Domain:   m.config.Cookie.Domain,
		HttpOnly: m.config.Cookie.HttpOnly,
		Secure:   m.config.Cookie.Secure,
		MaxAge:   m.config.Cookie.MaxAge,
	})

	return newSession, nil
}

func (m *manager) DestroySession(w http.ResponseWriter, r *http.Request) error {
	// 1. 读取Cookie
	cookie, err := r.Cookie(m.config.Cookie.Name)
	if err != nil {
		return nil
	}

	// 2. 删除存储中的会话
	if err := m.store.Delete(r.Context(), cookie.Value); err != nil {
		return ErrSessionDeleteErr
	}

	// 3. 清空Cookie
	http.SetCookie(w, &http.Cookie{
		Name:     m.config.Cookie.Name,
		Value:    "",
		Path:     m.config.Cookie.Path,
		HttpOnly: m.config.Cookie.HttpOnly,
		Secure:   m.config.Cookie.Secure,
		MaxAge:   -1, // 立即过期
	})

	return nil
}

func (m *manager) Start() error {
	// 启动GC定时任务
	m.gcTicker = time.NewTicker(m.config.Store.GCInterval)
	go func() {
		for {
			select {
			case <-m.ctx.Done():
				m.gcTicker.Stop()
				return
			case <-m.gcTicker.C:
				_ = m.store.GC(context.Background())
			}
		}
	}()
	return nil
}

func (m *manager) Stop() error {
	m.cancel()             // 停止GC
	return m.store.Close() // 关闭存储
}
