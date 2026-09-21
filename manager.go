package session

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

type manager struct {
	store    SessionStore
	config   Config
	ctx      context.Context
	cancel   context.CancelFunc
	gcTicker *time.Ticker
}

type Option func(*Config)

func WithCookie(cfg CookieConfig) Option {
	return func(c *Config) {
		c.Cookie = cfg
	}
}
func WithSecure(secure bool) Option {
	return func(c *Config) {
		c.Cookie.Secure = secure
	}
}
func WithHttpOnly(httpOnly bool) Option {
	return func(c *Config) {
		c.Cookie.HttpOnly = httpOnly
	}
}
func WithPath(path string) Option {
	return func(c *Config) {
		c.Cookie.Path = path
	}
}
func WithDomain(domain string) Option {
	return func(c *Config) {
		c.Cookie.Domain = domain
	}
}
func WithSameSite(sameSite http.SameSite) Option {
	return func(c *Config) {
		c.Cookie.SameSite = sameSite
	}
}
func WithName(name string) Option {
	return func(c *Config) {
		c.Cookie.Name = name
	}
}
func WithMaxAge(maxAge int) Option {
	return func(c *Config) {
		c.Cookie.MaxAge = maxAge
	}
}
func WithStoreConfig(cfg StoreConfig) Option {
	return func(c *Config) {
		c.Store = cfg
	}
}

func WithExpires(expires time.Duration) Option {
	return func(c *Config) {
		c.Store.Expires = expires
	}
}

func WithGCInterval(gcInterval time.Duration) Option {
	return func(c *Config) {
		c.Store.GCInterval = gcInterval
	}
}

func WithSessionIDLen(sessionIDLen int) Option {
	return func(c *Config) {
		c.Store.SessionIDLen = sessionIDLen
	}
}
func NewManager(store SessionStore, opts ...Option) SessionManager {
	cfg := DefaultConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.Cookie.Name == "" {
		cfg.Cookie.Name = DefaultConfig.Cookie.Name
	}
	if cfg.Store.Expires == 0 {
		cfg.Store.Expires = DefaultConfig.Store.Expires
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &manager{
		store:  store,
		config: cfg,
		ctx:    ctx,
		cancel: cancel,
	}
}

func (m *manager) GetSession(w http.ResponseWriter, r *http.Request, create bool) (Session, error) {
	var sessionId string
	cookie, err := r.Cookie(m.config.Cookie.Name)
	if err == nil {
		sessionId = cookie.Value
	}
	if sessionId != "" {
		s, err := m.store.Get(r.Context(), sessionId)
		if err == nil && !s.IsExpired() {
			return s, nil
		}
	}

	if !create {
		return nil, ErrNoSession
	}

	sessionId = generateId(m.config.Store.SessionIDLen)
	newSession, err := m.store.NewSession(sessionId, m.config.Store.Expires)
	if err != nil {
		return nil, ErrSessionCreateErr
	}
	if err := m.store.Create(r.Context(), newSession); err != nil {
		return nil, ErrSessionCreateErr
	}

	http.SetCookie(w, &http.Cookie{
		Name:     m.config.Cookie.Name,
		Value:    sessionId,
		Path:     m.config.Cookie.Path,
		Domain:   m.config.Cookie.Domain,
		HttpOnly: m.config.Cookie.HttpOnly,
		Secure:   m.config.Cookie.Secure,
		MaxAge:   m.config.Cookie.MaxAge,
		SameSite: m.config.Cookie.SameSite,
	})

	return newSession, nil
}

func (m *manager) DestroySession(w http.ResponseWriter, r *http.Request) error {
	cookie, err := r.Cookie(m.config.Cookie.Name)
	if err != nil {
		return nil
	}

	if err := m.store.Delete(r.Context(), cookie.Value); err != nil {
		return ErrSessionDeleteErr
	}

	http.SetCookie(w, &http.Cookie{
		Name:     m.config.Cookie.Name,
		Value:    "",
		Path:     m.config.Cookie.Path,
		Domain:   m.config.Cookie.Domain,
		HttpOnly: m.config.Cookie.HttpOnly,
		Secure:   m.config.Cookie.Secure,
		MaxAge:   -1,
		SameSite: m.config.Cookie.SameSite,
	})

	return nil
}

func (m *manager) Start() error {
	slog.Info("start session manager")
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
	slog.Info("stop session manager")
	m.cancel()
	return m.store.Close()
}
