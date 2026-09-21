package sqlstore

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/shanjunmei/go-session"
	"github.com/shanjunmei/go-session/sqlstore/sqlite"

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

// TestSQLStoreGetRefreshesLastAccessed verifies that after Get touches the DB's
// last_accessed (TouchSQL), the returned in-memory session reflects the refreshed
// value rather than the stale value read by the preceding SELECT. Regression for
// the "内存与数据库 last_accessed 不一致" bug.
func TestSQLStoreGetRefreshesLastAccessed(t *testing.T) {
	store := newTestStore(t)
	ss := store.(*sqlStore) // need raw DB access for deterministic assertions
	ctx := context.Background()

	s, err := store.NewSession("sess-la", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Create(ctx, s); err != nil {
		t.Fatal(err)
	}

	// Stale the row so a stale read is distinguishable from a fresh touch.
	stale := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	if _, err := ss.db.ExecContext(ctx, "UPDATE sessions SET last_accessed = ? WHERE id = ?", stale, "sess-la"); err != nil {
		t.Fatal(err)
	}

	got, err := store.Get(ctx, "sess-la")
	if err != nil {
		t.Fatal(err)
	}

	// In-memory value must NOT be the stale 2000 value: TouchSQL refreshed the
	// DB, and the fix makes the in-memory value match the current time.
	la := got.LastAccessedAt()
	if la.Equal(stale) {
		t.Fatalf("in-memory last_accessed still stale (%v); Get did not refresh memory", la)
	}

	// And it must agree with what is actually in the DB now.
	var dbLA time.Time
	if err := ss.db.QueryRowContext(ctx, "SELECT last_accessed FROM sessions WHERE id = ?", "sess-la").Scan(&dbLA); err != nil {
		t.Fatal(err)
	}
	if diff := la.Sub(dbLA); diff > time.Second || diff < -time.Second {
		t.Fatalf("in-memory last_accessed %v disagrees with DB %v (diff %v)", la, dbLA, diff)
	}
}
