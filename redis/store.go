package redis

import (
	"context"
	"errors"
	"time"

	"github.com/shanjunmei/go-session"

	"github.com/redis/go-redis/v9"
)

type redisStore struct {
	client *redis.Client
}

// NewStore 创建基于Redis的会话存储
// client: 已初始化的 *redis.Client
func NewStore(client *redis.Client) session.SessionStore {
	return &redisStore{
		client: client,
	}
}

// NewSession 创建新的会话实例（仅内存，尚未存入Redis）
func (r *redisStore) NewSession(id string, expires time.Duration) (session.Session, error) {
	s := newRedisSession(id, expires)
	s.setClient(r.client)
	return s, nil
}

// Create 将会话存入Redis
func (r *redisStore) Create(ctx context.Context, s session.Session) error {
	rs, ok := s.(*redisSession)
	if !ok {
		return errors.New("session is not a redisSession")
	}
	// 检查是否已存在
	exists, err := r.client.Exists(ctx, rs.key()).Result()
	if err != nil {
		return err
	}
	if exists > 0 {
		return session.ErrSessionCreateErr
	}
	// 初始化Hash字段
	now := time.Now()
	expiresAt := s.ExpiresAt()
	if expiresAt.IsZero() {
		// 如果未设置过期时间，使用默认24小时
		expiresAt = now.Add(24 * time.Hour)
	}
	pipe := r.client.Pipeline()
	pipe.HSet(ctx, rs.key(), "data", "{}")
	pipe.HSet(ctx, rs.key(), "last_accessed", now.Unix())
	pipe.ExpireAt(ctx, rs.key(), expiresAt)
	_, err = pipe.Exec(ctx)
	return err
}

// Get 从Redis加载会话
func (r *redisStore) Get(ctx context.Context, sessionId string) (session.Session, error) {
	key := "session:" + sessionId
	// 检查key是否存在
	exists, err := r.client.Exists(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	if exists == 0 {
		return nil, session.ErrSessionNotFound
	}
	// 检查是否过期（通过TTL）
	ttl, err := r.client.TTL(ctx, key).Result()
	if err != nil || ttl <= 0 {
		// 过期或出错，删除key并返回过期错误
		r.client.Del(ctx, key)
		return nil, session.ErrSessionExpired
	}
	// 创建会话对象
	s := &redisSession{
		id:     sessionId,
		client: r.client,
	}
	// 更新最后访问时间（可选）
	s.UpdateAccessedAt()
	return s, nil
}

// Update 更新Redis中的会话
// 由于所有修改操作已直接写Redis，此方法无需额外操作，但为符合接口保留。
func (r *redisStore) Update(ctx context.Context, s session.Session) error {
	// 可选：重新设置过期时间
	// 这里简单返回nil
	return nil
}

// Delete 删除Redis中的会话
func (r *redisStore) Delete(ctx context.Context, sessionId string) error {
	key := "session:" + sessionId
	result, err := r.client.Del(ctx, key).Result()
	if err != nil {
		return err
	}
	if result == 0 {
		return session.ErrSessionNotFound
	}
	return nil
}

// Count 返回当前活动（未过期）会话数（监控/指标）。
//
// 实现说明：Redis 的 SCAN 只遍历 keyspace 字典，既不触发惰性过期检查，
// 也不会过滤掉 TTL 已到期、但尚未被后台主动淘汰（activeExpireCycle）的物理 key。
// 因此直接累加 SCAN 结果会把已过期的会话也算入，导致统计偏高，且与
// interfaces.go 中 "当前活动会话数" 的契约以及 memory/sqlstore 的语义不一致。
// 这里对每个 SCAN 批次用 Pipeline 批量查询 PTTL，仅统计 TTL 仍有效的 key，
// 以得到与其他存储一致的活动会话数。
func (r *redisStore) Count(ctx context.Context) (int, error) {
	var n int
	var cursor uint64
	for {
		keys, next, err := r.client.Scan(ctx, cursor, "session:*", 256).Result()
		if err != nil {
			return 0, err
		}
		if len(keys) > 0 {
			// 批量查询 TTL，过滤已过期但尚未被 Redis 淘汰的 key。
			cmds, err := r.client.Pipelined(ctx, func(pipe redis.Pipeliner) error {
				for _, k := range keys {
					pipe.PTTL(ctx, k)
				}
				return nil
			})
			if err != nil {
				return 0, err
			}
			for _, cmdr := range cmds {
				dur, err := cmdr.(*redis.DurationCmd).Result()
				if err != nil {
					return 0, err
				}
				// PTTL 语义：>=0 表示仍有剩余时间；-1 表示无过期（持久，视为活动）；
				// -2 或其余负值表示已过期/不存在，不计入活动会话。
				if dur >= 0 || dur == -1 {
					n++
				}
			}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return n, nil
}

// GC Redis自动处理过期，无需操作
func (r *redisStore) GC(ctx context.Context) error {
	return nil
}

// Close 关闭Redis客户端连接
func (r *redisStore) Close() error {
	return r.client.Close()
}
