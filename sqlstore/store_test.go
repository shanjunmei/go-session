package sqlstore

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"go-session"
	"go-session/sqlstore/sqlite"

	_ "modernc.org/sqlite"
)

func newTestStore(t *testing.T) session.SessionStore {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	st, err := NewStore(db, sqlite.Dialect)
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func TestSQLStoreContract(t *testing.T) {
	store := newTestStore(t)
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

	// reload from DB
	s2, err := store.Get(ctx, "sess-1")
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := s2.Get("k"); v != "v" {
		t.Fatalf("persisted value missing after reload: %v", v)
	}

	// mutate again and reload to ensure update path works
	s2.Set("n", "42")
	s3, err := store.Get(ctx, "sess-1")
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := s3.Get("n"); v != "42" {
		t.Fatalf("update path failed: n=%v", v)
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

	// GC should not error
	if err := store.GC(ctx); err != nil {
		t.Fatal(err)
	}
}
