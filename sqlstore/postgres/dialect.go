// Package postgres provides the PostgreSQL dialect for go-session/sqlstore.
// Use it together with the pure-Go pgx/stdlib driver.
package postgres

import "github.com/shanjunmei/go-session/sqlstore/dialect"

type postgresDialect struct{}

func (postgresDialect) CreateTableSQL() string {
	return `CREATE TABLE IF NOT EXISTS sessions (
		id VARCHAR(64) PRIMARY KEY,
		data TEXT NOT NULL,
		expires_at TIMESTAMP NOT NULL,
		last_accessed TIMESTAMP NOT NULL
	)`
}
func (postgresDialect) UpsertSQL() string {
	return `INSERT INTO sessions (id, data, expires_at, last_accessed) VALUES ($1, $2, $3, $4)
		ON CONFLICT (id) DO UPDATE SET data = EXCLUDED.data, expires_at = EXCLUDED.expires_at, last_accessed = EXCLUDED.last_accessed`
}
func (postgresDialect) SelectSQL() string {
	return `SELECT id, data, expires_at, last_accessed FROM sessions WHERE id = $1`
}
func (postgresDialect) DeleteSQL() string { return `DELETE FROM sessions WHERE id = $1` }
func (postgresDialect) TouchSQL() string {
	return `UPDATE sessions SET last_accessed = NOW() WHERE id = $1`
}
func (postgresDialect) GCSQL() string { return `DELETE FROM sessions WHERE expires_at < $1` }
func (postgresDialect) CountSQL() string { return `SELECT COUNT(*) FROM sessions WHERE expires_at > $1` }

// Dialect is the PostgreSQL dialect. Pass it to sqlstore.NewStore.
var Dialect dialect.Dialect = postgresDialect{}
