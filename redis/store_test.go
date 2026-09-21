package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/shanjunmei/go-session"
)

func TestRedisStoreContract(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	store := NewStore(client)
	ctx := context.Background()

	s, err := store.NewSession("sess-1", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Create(ctx, s); err != nil {
		t.Fatal(err)
	}

	s.Set("k", "v")
	if v, ok := s.Get("k"); !ok || v != "v" {
		t.Fatalf("set/get failed: got=%v ok=%v", v, ok)
	}

	// reload from Redis
	s2, err := store.Get(ctx, "sess-1")
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := s2.Get("k"); v != "v" {
		t.Fatalf("persisted value missing after reload: %v", v)
	}

	if err := store.Delete(ctx, "sess-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(ctx, "sess-1"); err != session.ErrSessionNotFound {
		t.Fatalf("expected ErrSessionNotFound, got %v", err)
	}
}

// TestRedisStoreCountExcludesExpiredKeys 回归测试：验证 Count 不把
// "TTL 已到期、但尚未被 Redis 后台淘汰、仍残留在 keyspace" 的会话计入活动数。
//
// 复现手法（确定性，不依赖真实墙钟/后台淘汰）：用 miniredis 直接写入一个
// session:* key 并将其 TTL 设为负值。miniredis 的 SetTTL 不会触发 checkTTL 删除，
// 因此该 key 仍存在于 keyspace、SCAN 会返回它；旧实现直接累加 SCAN 结果会误计，
// 修复实现通过 PTTL 过滤掉它。
func TestRedisStoreCountExcludesExpiredKeys(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	store := NewStore(client)
	ctx := context.Background()

	// 一个活动会话（1 小时 TTL）。
	active, err := store.NewSession("sess-active", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Create(ctx, active); err != nil {
		t.Fatal(err)
	}

	// 一个已过期、但仍残留在 keyspace 的 key（不经过 checkTTL 删除）。
	if err := client.Set(ctx, "session:sess-expired", "x", 0).Err(); err != nil {
		t.Fatal(err)
	}
	mr.SetTTL("session:sess-expired", -5*time.Millisecond)

	got, err := store.Count(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if want := 1; got != want {
		t.Fatalf("Count returned %d, want %d (expired session must be excluded)", got, want)
	}

	// 额外确认 SCAN 确实能看到这个过期 key，证明旧实现会误计。
	scanned, err := client.Keys(ctx, "session:*").Result()
	if err != nil {
		t.Fatal(err)
	}
	if len(scanned) != 2 {
		t.Fatalf("precondition: SCAN/KEYS should see 2 keys (incl. expired), got %d", len(scanned))
	}
}
