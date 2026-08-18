package redis

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// redisSession 实现 session.Session 接口，数据存储在Redis中
type redisSession struct {
	id     string
	client *redis.Client
}

// newRedisSession 创建新的Redis会话实例（仅内存表示，尚未存入Redis）
func newRedisSession(id string, expires time.Duration) *redisSession {
	return &redisSession{
		id:     id,
		client: nil, // 后续由store注入
	}
}

// setClient 由store调用，注入Redis客户端
func (s *redisSession) setClient(client *redis.Client) {
	s.client = client
}

func (s *redisSession) ID() string {
	return s.id
}

// key 返回Redis中存储该会话的键名
func (s *redisSession) key() string {
	return "session:" + s.id
}

// Get 获取指定key的属性值
func (s *redisSession) Get(key string) (any, bool) {
	ctx := context.Background()
	dataJSON, err := s.client.HGet(ctx, s.key(), "data").Result()
	if err == redis.Nil {
		return nil, false
	} else if err != nil {
		return nil, false
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(dataJSON), &data); err != nil {
		return nil, false
	}
	val, ok := data[key]
	return val, ok
}

// Set 设置属性值，并更新最后访问时间
func (s *redisSession) Set(key string, value any) {
	ctx := context.Background()
	// 读取当前data
	dataJSON, err := s.client.HGet(ctx, s.key(), "data").Result()
	var data map[string]any
	if err == redis.Nil {
		data = make(map[string]any)
	} else if err != nil {
		return
	} else {
		if err := json.Unmarshal([]byte(dataJSON), &data); err != nil {
			data = make(map[string]any)
		}
	}
	data[key] = value
	newDataJSON, _ := json.Marshal(data)
	// 更新data和最后访问时间
	pipe := s.client.Pipeline()
	pipe.HSet(ctx, s.key(), "data", newDataJSON)
	pipe.HSet(ctx, s.key(), "last_accessed", time.Now().Unix())
	// 注意：此处未自动延长过期时间，如需延长请调用SetExpires
	_, _ = pipe.Exec(ctx)
}

// Del 删除指定属性
func (s *redisSession) Del(key string) {
	ctx := context.Background()
	dataJSON, err := s.client.HGet(ctx, s.key(), "data").Result()
	if err != nil {
		return
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(dataJSON), &data); err != nil {
		return
	}
	delete(data, key)
	newDataJSON, _ := json.Marshal(data)
	pipe := s.client.Pipeline()
	pipe.HSet(ctx, s.key(), "data", newDataJSON)
	pipe.HSet(ctx, s.key(), "last_accessed", time.Now().Unix())
	_, _ = pipe.Exec(ctx)
}

// Clear 清空所有属性
func (s *redisSession) Clear() {
	ctx := context.Background()
	pipe := s.client.Pipeline()
	pipe.HSet(ctx, s.key(), "data", "{}")
	pipe.HSet(ctx, s.key(), "last_accessed", time.Now().Unix())
	_, _ = pipe.Exec(ctx)
}

// ExpiresAt 返回会话的过期时间（通过Redis TTL计算）
func (s *redisSession) ExpiresAt() time.Time {
	ctx := context.Background()
	ttl, err := s.client.TTL(ctx, s.key()).Result()
	if err != nil || ttl < 0 {
		return time.Time{}
	}
	return time.Now().Add(ttl)
}

// SetExpires 设置会话的过期时长
func (s *redisSession) SetExpires(d time.Duration) {
	ctx := context.Background()
	s.client.Expire(ctx, s.key(), d)
}

// IsExpired 检查会话是否已过期
func (s *redisSession) IsExpired() bool {
	ctx := context.Background()
	ttl, err := s.client.TTL(ctx, s.key()).Result()
	return err != nil || ttl <= 0
}

// LastAccessedAt 返回最后访问时间（从Redis读取）
func (s *redisSession) LastAccessedAt() time.Time {
	ctx := context.Background()
	lastStr, err := s.client.HGet(ctx, s.key(), "last_accessed").Result()
	if err != nil {
		return time.Time{}
	}
	last, err := strconv.ParseInt(lastStr, 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.Unix(last, 0)
}

// UpdateAccessedAt 更新最后访问时间
func (s *redisSession) UpdateAccessedAt() {
	ctx := context.Background()
	s.client.HSet(ctx, s.key(), "last_accessed", time.Now().Unix())
}
