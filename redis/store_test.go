package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"go-session"
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
