package session

import (
	"context"
	"errors"
	"net/http"
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
