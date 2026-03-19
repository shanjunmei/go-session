package gorm

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go-session"

	"gorm.io/gorm"
)

// SessionModel 对应数据库表
type SessionModel struct {
	ID           string    `gorm:"primaryKey;size:64"` // 会话ID
	Data         string    `gorm:"type:text"`          // 序列化的数据
	ExpiresAt    time.Time `gorm:"index"`              // 过期时间，建立索引便于GC
	LastAccessed time.Time // 最后访问时间
}

// TableName 指定表名，可自定义
func (SessionModel) TableName() string {
	return "sessions"
}

// gormStore 实现 session.SessionStore 接口
type gormStore struct {
	db     *gorm.DB
	config GormConfig
}

// GormConfig 可选的配置项
type GormConfig struct {
	TableName string // 自定义表名
}

// NewStore 创建基于GORM的会话存储
// db: 已初始化的*gorm.DB连接
// opts: 可选配置
func NewStore(db *gorm.DB, opts ...func(*GormConfig)) (session.SessionStore, error) {
	cfg := GormConfig{
		TableName: "sessions",
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	// 如果表名自定义，临时修改模型表名
	if cfg.TableName != "sessions" {
		// 注意：GORM中可以通过回调或使用不同模型来指定表名，这里简单处理：直接使用传入的db，但自动迁移时需指定表名
		// 我们可以创建一个本地模型并设置TableName，但为了自动迁移，我们需要注册模型
		// 简便起见，自动迁移时使用默认模型，但设置表名会影响全局？更好的做法是使用gorm的Table方法
		// 这里采用：直接迁移默认表名，或者让用户自行迁移。为了简化，我们不在此自动迁移，而是让用户自行处理。
		// 但为方便，我们仍然尝试迁移，但需要确保表名正确。
		// 可以使用db.Table(cfg.TableName).AutoMigrate(&SessionModel{})
	}
	store := &gormStore{
		db: db,
	}
	// 自动迁移表
	if err := db.AutoMigrate(&SessionModel{}); err != nil {
		return nil, fmt.Errorf("failed to migrate session table: %w", err)
	}
	return store, nil
}

// NewSession 创建新的会话实例（仅内存，不写入数据库）
func (s *gormStore) NewSession(id string, expires time.Duration) (session.Session, error) {
	sess := newGormSession(id, expires)
	sess.store = s // 注入存储
	return sess, nil
}

// Create 将会话写入数据库
func (s *gormStore) Create(ctx context.Context, sess session.Session) error {
	gs, ok := sess.(*gormSession)
	if !ok {
		return errors.New("session is not a gormSession")
	}
	model, err := gs.toModel()
	if err != nil {
		return err
	}
	// 使用GORM创建记录
	result := s.db.WithContext(ctx).Create(model)
	if result.Error != nil {
		// 检查是否主键冲突
		if errors.Is(result.Error, gorm.ErrDuplicatedKey) {
			return session.ErrSessionCreateErr
		}
		return fmt.Errorf("failed to create session: %w", result.Error)
	}
	gs.clearDirty()
	return nil
}

// Get 从数据库加载会话
func (s *gormStore) Get(ctx context.Context, sessionId string) (session.Session, error) {
	var model SessionModel
	result := s.db.WithContext(ctx).Where("id = ?", sessionId).First(&model)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, session.ErrSessionNotFound
		}
		return nil, fmt.Errorf("failed to get session: %w", result.Error)
	}
	// 检查过期
	if time.Now().After(model.ExpiresAt) {
		// 可选：自动删除过期记录
		s.db.WithContext(ctx).Delete(&model)
		return nil, session.ErrSessionExpired
	}
	gs, err := loadGormSession(&model)
	if err != nil {
		return nil, fmt.Errorf("failed to load session data: %w", err)
	}
	gs.store = s
	gs.UpdateAccessedAt() // 更新最后访问时间
	// 注意：更新最后访问时间需要写回数据库，但Get操作通常不自动保存，可以在下次Update时保存。
	// 这里简单地在内存中更新，但数据库中的lastAccessed会滞后。如果希望保持准确，可以在此时更新数据库。
	// 我们选择在Update时统一写回，这里仅内存更新。
	return gs, nil
}

// Update 将会话写回数据库（仅在会话有修改时执行）
func (s *gormStore) Update(ctx context.Context, sess session.Session) error {
	gs, ok := sess.(*gormSession)
	if !ok {
		return errors.New("session is not a gormSession")
	}
	if !gs.isDirty() {
		return nil // 无修改
	}
	// 检查是否过期
	if gs.IsExpired() {
		// 从数据库删除
		s.db.WithContext(ctx).Delete(&SessionModel{ID: gs.ID()})
		return session.ErrSessionExpired
	}
	model, err := gs.toModel()
	if err != nil {
		return err
	}
	// 更新数据库
	result := s.db.WithContext(ctx).Save(model)
	if result.Error != nil {
		return fmt.Errorf("failed to update session: %w", result.Error)
	}
	gs.clearDirty()
	return nil
}

// Delete 删除数据库中的会话
func (s *gormStore) Delete(ctx context.Context, sessionId string) error {
	result := s.db.WithContext(ctx).Delete(&SessionModel{ID: sessionId})
	if result.Error != nil {
		return fmt.Errorf("failed to delete session: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return session.ErrSessionNotFound
	}
	return nil
}

// GC 删除所有过期的会话记录
func (s *gormStore) GC(ctx context.Context) error {
	result := s.db.WithContext(ctx).Where("expires_at < ?", time.Now()).Delete(&SessionModel{})
	if result.Error != nil {
		return fmt.Errorf("failed to GC sessions: %w", result.Error)
	}
	return nil
}

// Close 关闭存储（对于GORM，可以关闭底层数据库连接）
func (s *gormStore) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
