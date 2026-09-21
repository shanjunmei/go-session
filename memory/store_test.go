package memory

import (
	"context"
	"testing"
	"time"

	"github.com/shanjunmei/go-session"
)

func TestMemoryStoreContract(t *testing.T) {
	store := NewStore()
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

	// reload from store
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

	// expired session
	exp, _ := store.NewSession("exp", -time.Second)
	_ = store.Create(ctx, exp)
	if _, err := store.Get(ctx, "exp"); err != session.ErrSessionExpired {
		t.Fatalf("expected ErrSessionExpired, got %v", err)
	}
}
