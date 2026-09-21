package session

import (
	"context"
	"net/http"
	"time"
)

// Session 定义会话实体的核心能力，与存储无关
type Session interface {
	ID() string                 // 会话唯一ID
	Get(key string) (any, bool) // 获取属性
	Set(key string, value any)  // 设置属性
	Del(key string)             // 删除属性
	Clear()                     // 清空所有属性
	ExpiresAt() time.Time       // 过期时间
	SetExpires(d time.Duration) // 设置过期时长
	IsExpired() bool            // 是否过期
	LastAccessedAt() time.Time  // 最后访问时间
	UpdateAccessedAt()          // 更新最后访问时间
}

// SessionStore 抽象会话持久化层
type SessionStore interface {
	NewSession(id string, expires time.Duration) (Session, error) // 创建新的会话实例
	Create(ctx context.Context, s Session) error                  // 持久化会话
	Get(ctx context.Context, sessionId string) (Session, error)   // 获取会话
	Update(ctx context.Context, s Session) error                  // 更新会话
	Delete(ctx context.Context, sessionId string) error           // 删除会话
	Count(ctx context.Context) (int, error)                        // 当前活动会话数（监控/指标）
	GC(ctx context.Context) error                                 // 清理过期会话
	Close() error                                                 // 关闭存储
}

// SessionManager 统筹会话全生命周期，对接HTTP协议
type SessionManager interface {
	GetSession(w http.ResponseWriter, r *http.Request, create bool) (Session, error)
	DestroySession(w http.ResponseWriter, r *http.Request) error
	Start() error
	Stop() error
}
