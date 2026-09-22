# go-session

> 轻量级、存储无关的 Go HTTP 会话管理库。
> 中文 | [English](README.en.md)

模块路径：`github.com/shanjunmei/go-session`

---

## 特性

- **可插拔存储后端**：内置内存（memory）、Redis、SQL（SQLite / PostgreSQL，通过 dialect 适配）三种实现，统一遵循 `session.SessionStore` 接口。
- **HTTP 集成**：`session.SessionManager` 基于 `net/http` 提供 Cookie 会话管理（`GetSession` / `DestroySession`），并内置后台 GC 定时清理。
- **过期语义统一**：每个会话拥有 TTL 过期时间，访问时惰性校验，并由后台 ticker 主动回收；`Count` 在所有后端均返回**活动（未过期）会话数**。
- **并发安全**：各存储后端对读写路径加锁或使用单连接客户端，可在并发 HTTP handler 中安全使用。

## 安装

核心模块（含 `memory` 后端）仅需 Go 1.21 及以上：

```bash
go get github.com/shanjunmei/go-session
```

`redis` 与 `sqlstore` 后端是**独立嵌套模块**，按需单独引入，并各自带 Go 版本下限：

```bash
go get github.com/shanjunmei/go-session/redis      # 要求 Go 1.24+
go get github.com/shanjunmei/go-session/sqlstore   # 要求 Go 1.25+（纯 Go SQLite）
```

> 未使用重后端的消费者不会被更高的 Go 工具链要求拖累。

## 快速开始

### 内存存储（memory）

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

	sess, err := m.GetSession(w, r, true) // 第三个参数 create=true：缺失时自动创建
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

### Redis 存储（redis）

会话以 Hash 形式存于 Redis，键名为 `session:<id>`，并通过 `ExpireAt` 设置过期时间。

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

### SQL 存储（sqlstore，支持 SQLite / PostgreSQL）

SQL 后端通过 `dialect.Dialect` 接口隔离数据库差异，核心存储不依赖任何具体数据库驱动；连接与参数绑定由存储层负责。

```go
import (
	"database/sql"

	_ "modernc.org/sqlite" // 纯 Go SQLite 驱动

	"github.com/shanjunmei/go-session"
	"github.com/shanjunmei/go-session/sqlstore"
	"github.com/shanjunmei/go-session/sqlstore/sqlite"
)

db, err := sql.Open("sqlite", "sessions.db")
if err != nil {
	panic(err)
}
store, err := sqlstore.NewStore(db, sqlite.Dialect) // PostgreSQL 使用 postgres.Dialect
if err != nil {
	panic(err)
}
m := session.NewManager(store)
```

`NewStore` 在构造时会执行一次 `CREATE TABLE IF NOT EXISTS sessions`（字段：`id`、`data`、`expires_at`、`last_accessed`）。

## API 概览

### `session.Session`

| 方法 | 说明 |
| --- | --- |
| `ID() string` | 会话唯一 ID |
| `Get(key string) (any, bool)` | 读取属性 |
| `Set(key string, value any)` | 写入属性（Redis / SQL 后端会同步持久化） |
| `Del(key string)` | 删除属性 |
| `Clear()` | 清空所有属性 |
| `ExpiresAt() time.Time` | 过期时间 |
| `SetExpires(d time.Duration)` | 设置过期时长 |
| `IsExpired() bool` | 是否已过期 |
| `LastAccessedAt() time.Time` | 最后访问时间 |
| `UpdateAccessedAt()` | 刷新最后访问时间 |

### `session.SessionStore`

| 方法 | 说明 |
| --- | --- |
| `NewSession(id string, expires time.Duration) (Session, error)` | 创建会话实例（尚未持久化） |
| `Create(ctx, s) error` | 持久化会话 |
| `Get(ctx, sessionId) (Session, error)` | 加载会话（过期返回 `ErrSessionExpired`） |
| `Update(ctx, s) error` | 更新会话 |
| `Delete(ctx, sessionId) error` | 删除会话 |
| `Count(ctx) (int, error)` | 当前**活动会话数**（监控/指标） |
| `GC(ctx) error` | 清理过期会话 |
| `Close() error` | 关闭存储 |

### `session.SessionManager`

| 方法 | 说明 |
| --- | --- |
| `GetSession(w, r, create bool) (Session, error)` | 按 Cookie 解析/创建会话 |
| `DestroySession(w, r) error` | 销毁会话并清除 Cookie |
| `Start() error` | 启动后台 GC ticker |
| `Stop() error` | 停止 GC 并关闭存储 |

## 配置

通过 `session.NewManager(store, opts...)` 的 `Option` 调整 Cookie 与存储参数：

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

默认值：`Cookie.Name="sessionId"`、`Store.Expires=24h`、`Store.GCInterval=5m`、`Store.SessionIDLen=16`。

## 关于 `Count`

`Count` 的契约语义是**当前活动（未过期）会话数**，用于监控/指标。三个后端的实现保持一致：

- `memory`：遍历会话映射，排除 `IsExpired()` 为真的会话。
- `sqlstore`：`SELECT COUNT(*) FROM sessions WHERE expires_at > ?`，由数据库按过期时间过滤。
- `redis`：以 `SCAN session:*` 分批迭代，对每个 key 批量查询 `PTTL` 并仅计入 TTL 仍有效（或持久无过期）的 key，从而排除 TTL 已到期、但 Redis 后台尚未淘汰的物理 key。

## GitHub Pages

本仓库的 `docs/` 目录包含一份双语（中文 / English）、双主题（亮 / 暗）的单页站点。在仓库 **Settings → Pages → Source** 选择从分支 `main` 的 `/docs` 目录发布即可。
