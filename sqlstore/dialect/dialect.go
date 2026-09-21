// Package dialect defines the SQL dialect contract used by the sqlstore
// session store. Concrete dialects live in sub-packages (e.g.
// go-session/sqlstore/postgres, go-session/sqlstore/sqlite) so the core store
// depends only on this interface and never on a specific database.
package dialect

// Dialect abstracts the SQL differences between supported databases.
// Implementations only return the correct DDL/DML strings; the store owns the
// connection, parameter binding and transaction handling.
type Dialect interface {
	// CreateTableSQL creates the sessions table if it does not already exist.
	CreateTableSQL() string
	// UpsertSQL inserts or updates a session. Args: id, data, expires_at, last_accessed.
	UpsertSQL() string
	// SelectSQL loads a session by id. Arg: id.
	SelectSQL() string
	// DeleteSQL removes a session by id. Arg: id.
	DeleteSQL() string
	// TouchSQL refreshes last_accessed for a session by id. Arg: id.
	TouchSQL() string
	// GCSQL deletes rows whose expires_at is before the argument. Arg: now.
	GCSQL() string
	// CountSQL counts active sessions (expires_at after the argument). Arg: now.
	// Dialects MUST use their own placeholder style (? for sqlite, $1 for postgres).
	CountSQL() string
}
