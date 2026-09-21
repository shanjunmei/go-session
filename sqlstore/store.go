package sqlstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/shanjunmei/go-session"
	"github.com/shanjunmei/go-session/sqlstore/dialect"
)

// sqlStore is the core SQL-backed session store. It depends only on the
// dialect.Dialect interface and never embeds dialect-specific SQL, so concrete
// dialects can live in their own sub-packages.
type sqlStore struct {
	db      *sql.DB
	dialect dialect.Dialect
}

// NewStore builds a SQL-backed session store over an already-opened *sql.DB
// and a Dialect. Pick the dialect from a sub-package, e.g.
//
//	store, _ := sqlstore.NewStore(db, sqlite.Dialect)   // modernc.org/sqlite
//	store, _ := sqlstore.NewStore(db, postgres.Dialect) // pgx/stdlib
//
// A one-time CREATE TABLE IF NOT EXISTS is executed at construction time.
func NewStore(db *sql.DB, d dialect.Dialect) (session.SessionStore, error) {
	if _, err := db.ExecContext(context.Background(), d.CreateTableSQL()); err != nil {
		return nil, fmt.Errorf("create sessions table: %w", err)
	}
	return &sqlStore{db: db, dialect: d}, nil
}

func (s *sqlStore) NewSession(id string, expires time.Duration) (session.Session, error) {
	gs := newSQLSession(id, expires)
	gs.store = s
	return gs, nil
}

func (s *sqlStore) Create(ctx context.Context, sess session.Session) error {
	gs, ok := sess.(*sqlSession)
	if !ok {
		return errors.New("session is not a sqlSession")
	}
	id, dataJSON, expiresAt, lastAccessed, err := gs.snapshot()
	if err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, s.dialect.UpsertSQL(), id, dataJSON, expiresAt, lastAccessed); err != nil {
		return session.ErrSessionCreateErr
	}
	gs.clearDirty()
	return nil
}

func (s *sqlStore) Get(ctx context.Context, sessionID string) (session.Session, error) {
	var (
		id, data      string
		expiresAt, la time.Time
	)
	err := s.db.QueryRowContext(ctx, s.dialect.SelectSQL(), sessionID).
		Scan(&id, &data, &expiresAt, &la)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, session.ErrSessionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}
	if time.Now().After(expiresAt) {
		_, _ = s.db.ExecContext(ctx, s.dialect.DeleteSQL(), sessionID)
		return nil, session.ErrSessionExpired
	}
	// Refresh last_accessed with a single statement (no full data round-trip).
	if _, err := s.db.ExecContext(ctx, s.dialect.TouchSQL(), sessionID); err != nil {
		_ = err // best-effort
	}
	// Keep the in-memory value consistent with what TouchSQL wrote to the DB.
	la = time.Now()
	gs, err := loadSQLSession(id, data, expiresAt, la)
	if err != nil {
		return nil, fmt.Errorf("load session: %w", err)
	}
	gs.store = s
	return gs, nil
}

func (s *sqlStore) Update(ctx context.Context, sess session.Session) error {
	gs, ok := sess.(*sqlSession)
	if !ok {
		return errors.New("session is not a sqlSession")
	}
	gs.mu.RLock()
	expired := time.Now().After(gs.expiresAt)
	dirty := gs.dirty
	gs.mu.RUnlock()
	if !dirty {
		return nil
	}
	if expired {
		if _, err := s.db.ExecContext(ctx, s.dialect.DeleteSQL(), gs.id); err != nil {
			return fmt.Errorf("delete expired session: %w", err)
		}
		gs.clearDirty()
		return session.ErrSessionExpired
	}
	id, dataJSON, expiresAt, lastAccessed, err := gs.snapshot()
	if err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, s.dialect.UpsertSQL(), id, dataJSON, expiresAt, lastAccessed); err != nil {
		return fmt.Errorf("update session: %w", err)
	}
	gs.clearDirty()
	return nil
}

func (s *sqlStore) Delete(ctx context.Context, sessionID string) error {
	res, err := s.db.ExecContext(ctx, s.dialect.DeleteSQL(), sessionID)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return session.ErrSessionNotFound
	}
	return nil
}

// Count returns the number of active (not yet expired) sessions.
func (s *sqlStore) Count(ctx context.Context) (int, error) {
	var n int
	if err := s.db.QueryRowContext(ctx, s.dialect.CountSQL(), time.Now()).Scan(&n); err != nil {
		return 0, fmt.Errorf("count sessions: %w", err)
	}
	return n, nil
}

func (s *sqlStore) GC(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, s.dialect.GCSQL(), time.Now()); err != nil {
		return fmt.Errorf("gc sessions: %w", err)
	}
	return nil
}

// Close closes the underlying *sql.DB. The store takes ownership of the
// connection lifecycle; if you need to share the DB elsewhere, wrap it.
func (s *sqlStore) Close() error {
	return s.db.Close()
}
