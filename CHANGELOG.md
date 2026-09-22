# Changelog / 更新日志

所有重要变更记录于此文件。格式参考 [Keep a Changelog](https://keepachangelog.com/)，版本号遵循 [SemVer](https://semver.org/lang/zh-CN/)。

All notable changes are recorded in this file. Format follows [Keep a Changelog](https://keepachangelog.com/); versions follow [SemVer](https://semver.org/).

---

## [v0.1.0] - 2026-09-22 · 首个版本 / First Release

### 新增 / Added

- 会话管理核心：`Session` 数据结构与 `SessionStore` / `SessionManager` 接口，支持创建、读取、更新、删除、续期（刷新 TTL）与统计。
  Core session management: the `Session` struct plus `SessionStore` / `SessionManager` interfaces, supporting create, get, update, delete, refresh (TTL renewal) and count.
- 三种存储后端实现：
  - 内存（`memory`）：进程内存储，自动过滤过期会话。
    In-memory (`memory`): process-local storage that auto-excludes expired sessions.
  - Redis：基于 `SCAN` 的游标遍历，适用于分布式部署。
    Redis: cursor-based `SCAN` iteration, suitable for distributed deployments.
  - SQL（`sqlstore`）：支持 SQLite 与 PostgreSQL 方言，基于 `database/sql` 实现，无 ORM 依赖。
    SQL (`sqlstore`): SQLite and PostgreSQL dialects via `database/sql`, with no ORM dependency.
- `Count` 返回当前**活动（未过期）**会话数，用于监控 / 指标，三个后端语义一致。
  `Count` returns the number of **active (not-yet-expired)** sessions for monitoring / metrics, with consistent semantics across all three backends.

### 发布前修复 / Pre-release fixes

- Redis `Count` 此前用 `SCAN "session:*"` 累加全部 key，会把 TTL 已过但未被后台主动淘汰的物理 key 计入；现改为每批次通过 pipeline 批量查询 `PTTL`，仅计入仍有剩余时间（`>= 0`）或持久化（无过期）的 key。
  Redis `Count` previously summed every key from `SCAN "session:*"`, including keys whose TTL had expired but were not yet evicted by the server; it now pipelines `PTTL` per batch and counts only keys with remaining TTL (`>= 0`) or no expiry.
- `sqlstore.Get` 修复内存中 `last_accessed` 与数据库值不一致的问题。
  `sqlstore.Get` fixed an inconsistency between the in-memory `last_accessed` value and the database value.

### 文档 / Docs

- 新增双语（中文 / English）`README.md`，覆盖安装、三种 store 用法、`Session` / `SessionStore` / `SessionManager` API 概览与 `Count` 活动会话语义。
  Added a bilingual (中文 / English) `README.md` covering installation, the three store usages, the `Session` / `SessionStore` / `SessionManager` API overview, and `Count` active-session semantics.
- 新增自包含的双语 / 双主题 GitHub Pages 单页（`docs/index.html`），支持语言与亮 / 暗主题切换，作为 Pages 源（不另起独立项目）。
  Added a self-contained bilingual / dual-theme GitHub Pages single page (`docs/index.html`) with language and light / dark theme toggles, used as the Pages source without forming a separate project.
