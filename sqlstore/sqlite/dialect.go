// Package sqlite provides the SQLite dialect for go-session/sqlstore.
// Use it together with the pure-Go modernc.org/sqlite driver.
package sqlite

import "github.com/shanjunmei/go-session/sqlstore/dialect"

type sqliteDialect struct{}

func (sqliteDialect) CreateTableSQL() string {
	return `CREATE TABLE IF NOT EXISTS sessions (
		id TEXT PRIMARY KEY,
		data TEXT NOT NULL,
		expires_at DATETIME NOT NULL,
		last_accessed DATETIME NOT NULL
	)`
}
func (sqliteDialect) UpsertSQL() string {
	return `INSERT INTO sessions (id, data, expires_at, last_accessed) VALUES (?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET data = excluded.data, expires_at = excluded.expires_at, last_accessed = excluded.last_accessed`
}
func (sqliteDialect) SelectSQL() string {
	return `SELECT id, data, expires_at, last_accessed FROM sessions WHERE id = ?`
}
func (sqliteDialect) DeleteSQL() string { return `DELETE FROM sessions WHERE id = ?` }
func (sqliteDialect) TouchSQL() string {
	return `UPDATE sessions SET last_accessed = CURRENT_TIMESTAMP WHERE id = ?`
}
func (sqliteDialect) GCSQL() string { return `DELETE FROM sessions WHERE expires_at < ?` }
func (sqliteDialect) CountSQL() string { return `SELECT COUNT(*) FROM sessions WHERE expires_at > ?` }

// Dialect is the SQLite dialect. Pass it to sqlstore.NewStore.
var Dialect dialect.Dialect = sqliteDialect{}
