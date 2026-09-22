# go-session

> A lightweight, storage-agnostic HTTP session library for Go.
> English | [中文](README.md)

Module path: `github.com/shanjunmei/go-session`

### Features

- **Pluggable storage backends**: built-in in-memory (`memory`), Redis, and SQL (SQLite / PostgreSQL via a `dialect` adapter) implementations, all conforming to the `session.SessionStore` interface.
- **HTTP integration**: `session.SessionManager` provides cookie-based session handling over `net/http` (`GetSession` / `DestroySession`) with a background GC ticker.
- **Consistent expiry semantics**: every session carries a TTL, validated lazily on access and reclaimed by a background ticker; `Count` returns the number of **active (non-expired)** sessions across all backends.
- **Concurrency-safe**: each backend guards its read/write paths or uses a single client connection, so it is safe to use from concurrent HTTP handlers.

### Install

The core module (including the `memory` backend) requires Go 1.21 or later:

```bash
go get github.com/shanjunmei/go-session
```

The `redis` and `sqlstore` backends are **separate nested modules**, imported on demand, each with its own Go version floor:

```bash
go get github.com/shanjunmei/go-session/redis      # requires Go 1.24+
go get github.com/shanjunmei/go-session/sqlstore   # requires Go 1.25+ (pure-Go SQLite)
```

> Consumers that do not use the heavy backends are not forced onto a newer Go toolchain.

### Quick start

#### In-memory store (memory)

```go
package main

import (
	"net/http"

	"github.com/shanjunmei/go-session"
	"github.com/shanjunmei/go-session/memory"
)

func handler(w http.ResponseWriter, r *http.Request) {
	store := memory.NewStore()
	m := session.NewManager(store)
	_ = m.Start()
	defer m.Stop()

	sess, err := m.GetSession(w, r, true) // create=true: auto-create when missing
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sess.Set("user", 42)
}

func main() {
	http.HandleFunc("/", handler)
	_ = http.ListenAndServe(":8080", nil)
}
```

#### Redis store (redis)

Sessions are stored in Redis as hashes under the key `session:<id>`, with an `ExpireAt` TTL.

```go
import (
	"github.com/redis/go-redis/v9"
	"github.com/shanjunmei/go-session"
	"github.com/shanjunmei/go-session/redis"
)

client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
store := redis.NewStore(client)
m := session.NewManager(store)
```

#### SQL store (sqlstore, SQLite / PostgreSQL)

The SQL backend isolates database differences behind the `dialect.Dialect` interface; the core store never depends on a concrete driver. Connection and parameter binding are owned by the store.

```go
import (
	"database/sql"

	_ "modernc.org/sqlite" // pure-Go SQLite driver

	"github.com/shanjunmei/go-session"
	"github.com/shanjunmei/go-session/sqlstore"
	"github.com/shanjunmei/go-session/sqlstore/sqlite"
)

db, err := sql.Open("sqlite", "sessions.db")
if err != nil {
	panic(err)
}
store, err := sqlstore.NewStore(db, sqlite.Dialect) // use postgres.Dialect for PostgreSQL
if err != nil {
	panic(err)
}
m := session.NewManager(store)
```

`NewStore` runs `CREATE TABLE IF NOT EXISTS sessions` once at construction (columns: `id`, `data`, `expires_at`, `last_accessed`).

### API overview

#### `session.Session`

| Method | Description |
| --- | --- |
| `ID() string` | Unique session ID |
| `Get(key string) (any, bool)` | Read an attribute |
| `Set(key string, value any)` | Write an attribute (Redis / SQL backends persist immediately) |
| `Del(key string)` | Delete an attribute |
| `Clear()` | Clear all attributes |
| `ExpiresAt() time.Time` | Expiry time |
| `SetExpires(d time.Duration)` | Set expiry duration |
| `IsExpired() bool` | Whether the session has expired |
| `LastAccessedAt() time.Time` | Last access time |
| `UpdateAccessedAt()` | Refresh last access time |

#### `session.SessionStore`

| Method | Description |
| --- | --- |
| `NewSession(id string, expires time.Duration) (Session, error)` | Create a session instance (not yet persisted) |
| `Create(ctx, s) error` | Persist the session |
| `Get(ctx, sessionId) (Session, error)` | Load a session (returns `ErrSessionExpired` if expired) |
| `Update(ctx, s) error` | Update the session |
| `Delete(ctx, sessionId) error` | Delete the session |
| `Count(ctx) (int, error)` | Current **active session count** (monitoring/metrics) |
| `GC(ctx) error` | Reclaim expired sessions |
| `Close() error` | Close the store |

#### `session.SessionManager`

| Method | Description |
| --- | --- |
| `GetSession(w, r, create bool) (Session, error)` | Resolve/create a session from the cookie |
| `DestroySession(w, r) error` | Destroy the session and clear the cookie |
| `Start() error` | Start the background GC ticker |
| `Stop() error` | Stop GC and close the store |

### Configuration

Tune cookie and store parameters via `session.NewManager(store, opts...)` `Option`s:

```go
m := session.NewManager(store,
	session.WithName("sid"),
	session.WithPath("/"),
	session.WithDomain("example.com"),
	session.WithSecure(true),
	session.WithHttpOnly(true),
	session.WithSameSite(http.SameSiteLaxMode),
	session.WithExpires(12*time.Hour),
	session.WithGCInterval(10*time.Minute),
	session.WithSessionIDLen(32),
)
```

Defaults: `Cookie.Name="sessionId"`, `Store.Expires=24h`, `Store.GCInterval=5m`, `Store.SessionIDLen=16`.

### On `Count`

The contract of `Count` is the number of **active (non-expired)** sessions, intended for monitoring/metrics. All three backends stay consistent:

- `memory`: iterates the session map and excludes sessions where `IsExpired()` is true.
- `sqlstore`: `SELECT COUNT(*) FROM sessions WHERE expires_at > ?`, filtered by the database on expiry time.
- `redis`: iterates `session:*` in batches with `SCAN`, queries `PTTL` for each key in a pipeline, and counts only keys whose TTL is still valid (or persistent with no expiry) — thereby excluding keys whose TTL has elapsed but which Redis has not yet evicted.

### GitHub Pages

The `docs/` directory of this repository contains a bilingual (Chinese / English), dual-theme (light / dark) single-page site. Enable it from **Settings → Pages → Source**, publishing from the `/docs` directory of the `main` branch.
