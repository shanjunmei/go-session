package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"sync"
	"time"
)

// -------------------------- 通用错误定义 --------------------------
var (
	ErrSessionExpired   = errors.New("session: expired")          // 会话已过期
	ErrSessionNotFound  = errors.New("session: not found")        // 会话不存在
	ErrSessionCreateErr = errors.New("session: failed to create") // 会话创建失败
	ErrSessionUpdateErr = errors.New("session: failed to update") // 会话更新失败
	ErrSessionDeleteErr = errors.New("session: failed to delete") // 会话删除失败
)

// -------------------------- 核心接口1：Session（实体能力） --------------------------
// Session 定义会话实体的核心能力，与存储无关
type Session interface {
	// 基础标识
	ID() string // 会话唯一ID
	// 属性操作
	Get(key string) (interface{}, bool) // 获取属性
	Set(key string, value interface{})  // 设置属性
	Del(key string)                     // 删除属性
	Clear()                             // 清空所有属性
	// 过期管理
	ExpiresAt() time.Time       // 过期时间
	SetExpires(d time.Duration) // 设置过期时长
	IsExpired() bool            // 是否过期
	// 生命周期
	LastAccessedAt() time.Time // 最后访问时间
	UpdateAccessedAt()         // 更新最后访问时间
}

// -------------------------- 核心接口2：SessionStore（持久化层） --------------------------
// SessionStore 抽象会话持久化层，定义存储无关的CRUD+GC接口
type SessionStore interface {
	// 会话CRUD
	Create(ctx context.Context, s Session) error                // 创建会话（持久化）
	Get(ctx context.Context, sessionID string) (Session, error) // 获取会话
	Update(ctx context.Context, s Session) error                // 更新会话
	Delete(ctx context.Context, sessionID string) error         // 删除会话
	// 过期清理
	GC(ctx context.Context) error // 清理过期会话
	// 存储生命周期
	Close() error // 关闭存储（释放连接/资源）
}

// -------------------------- 核心接口3：SessionManager（会话管理） --------------------------
// SessionManager 统筹会话全生命周期，对接HTTP协议
type SessionManager interface {
	// HTTP会话操作
	GetSession(w http.ResponseWriter, r *http.Request) (Session, error) // 获取/创建会话
	DestroySession(w http.ResponseWriter, r *http.Request) error        // 销毁会话
	// 管理生命周期
	Start() error // 启动管理器（初始化存储/启动GC）
	Stop() error  // 停止管理器（关闭存储/停止GC）
}

// -------------------------- 配置结构体 --------------------------
// Config 会话管理器配置
type Config struct {
	Cookie CookieConfig // Cookie配置
	Store  StoreConfig  // 存储配置
}

// CookieConfig Cookie相关配置
type CookieConfig struct {
	Name     string // Cookie名称
	Path     string // 路径
	Domain   string // 域名
	HttpOnly bool   // 防止XSS
	Secure   bool   // 仅HTTPS传输
	MaxAge   int    // Cookie有效期（秒）
}

// StoreConfig 存储相关配置
type StoreConfig struct {
	Expires      time.Duration // 会话默认过期时长
	GCInterval   time.Duration // GC执行间隔
	SessionIDLen int           // 会话ID长度（字节）
}

// 默认配置
var DefaultConfig = Config{
	Cookie: CookieConfig{
		Name:     "GSESSIONID",
		Path:     "/",
		HttpOnly: true,
		Secure:   false,
		MaxAge:   86400, // 24小时
	},
	Store: StoreConfig{
		Expires:      24 * time.Hour,
		GCInterval:   5 * time.Minute,
		SessionIDLen: 16, // 16字节→32位十六进制ID
	},
}

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
	sessionID = generateSessionID(m.config.Store.SessionIDLen)
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

// -------------------------- 工具函数：生成会话ID --------------------------
func generateSessionID(length int) string {
	b := make([]byte, length)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
