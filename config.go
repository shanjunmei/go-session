package session

import (
	"net/http"
	"time"
)

type Config struct {
	Cookie CookieConfig
	Store  StoreConfig
}

type CookieConfig struct {
	Name     string
	Path     string
	Domain   string
	HttpOnly bool
	Secure   bool
	MaxAge   int
	SameSite http.SameSite
}

type StoreConfig struct {
	Expires      time.Duration // 会话默认过期时长
	GCInterval   time.Duration // GC执行间隔
	SessionIDLen int           // 会话ID长度（字节）
}

var DefaultConfig = Config{
	Cookie: CookieConfig{
		Name:     "sessionId",
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
	},
	Store: StoreConfig{
		Expires:      24 * time.Hour,
		GCInterval:   5 * time.Minute,
		SessionIDLen: 16,
	},
}
