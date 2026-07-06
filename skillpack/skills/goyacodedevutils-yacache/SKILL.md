---
name: goyacodedevutils-yacache
description: Generic pluggable key-value cache abstraction with interchangeable in-memory and Redis backends, hash-oriented API. Use for any caching/session-store need instead of hand-rolling a map or a bare redis.Client call.
---

# yacache Skill

Import path: `github.com/YaCodeDev/GoYaCodeDevUtils/yacache`.

Generic pluggable key-value cache abstraction with interchangeable in-memory and Redis backends, a
hash-oriented API (`HSetEX`/`HGet`/etc.), plus a simple `Set`/`Get`/`GetDel`.

## Key API

- `Cache[T Container]` interface — `Raw() T`, `HSetEX`, `HGet`, `HGetAll`, `HGetDelSingle`, `HLen`, `HExist`, `HDelSingle`, `Set`, `Get`, `MGet`, `GetDel`, `Exists`, `Del`, `Ping`, `Close`.
- `Container` interface — `*redis.Client | MemoryContainer`.
- `NewCache[T Container](container T) Cache[T]` — type-switches to the matching backend; returns nil for an unsupported type.
- `Memory` struct + `NewMemory(data, tickToClean) *Memory` (starts a background TTL-sweeper goroutine); `MemoryContainer` struct + `NewMemoryContainer() MemoryContainer`.
- `Redis` struct + `NewRedis(*redis.Client) *Redis` (auto-detects DragonflyDB vs real Redis for the `HSETEX` variant).
- `NewRedisClient(host, port, password, db, log) *redis.Client` — dials and pings; `Fatalf` on failure.

## Usage Notes

- Memory backend is thread-safe (`sync.RWMutex`); TTL is enforced by a background goroutine that is weak-pointer based and auto-stops when the `Memory` is GC'd, or call `Close()` to stop it deterministically.
- Redis backend TTL relies on `HSETEX` (Redis 7+) or DragonflyDB's variant, auto-detected via `INFO server` at construction.
- All errors are `yaerrors.Error`; depends on `yaerrors` + `yalogger`. Used as a building block by `yafsm`, `yaratelimit`, `yatgstorage`, and `yatgbot` — prefer building on `yacache` rather than a raw `redis.Client` when you need caching, sessions, rate limiting, or state.
- Fx: `MemoryModule` provides `Cache[MemoryContainer]`; `RedisModule` provides `Cache[*redis.Client]` (needs a `yalogger.Logger` in the graph) — pick one, not both (`fx.go`).
