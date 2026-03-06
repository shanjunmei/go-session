package session

import "time"

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
