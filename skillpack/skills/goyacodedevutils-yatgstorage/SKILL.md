---
name: goyacodedevutils-yatgstorage
description: Redis-backed persistence for gotd/td's updates.Manager state (pts/qts/seq/date via RedisJSON) plus channel/user access-hash bookkeeping, and a GORM/AES-encrypted session-storage layer. Use for any yatgclient/yatgbot state or session persistence.
---

# yatgstorage Skill

Import path: `github.com/YaCodeDev/GoYaCodeDevUtils/yatgstorage`.

Redis-backed persistence for gotd/td's `updates.Manager` state (pts/qts/seq/date via RedisJSON) plus
channel/user access-hash bookkeeping, and a separate GORM/AES-encrypted session-storage layer for the
client's auth key.

## Key API

- `Store` interface — `Ping`, `GetState`/`SetState`/`SetPts`/`SetQts`/`SetDate`/`SetSeq`/`SetDateSeq`, `SetChannelPts`/`GetChannelPts`/`ForEachChannels`, `SetChannelAccessHash`/`GetChannelAccessHash`, `AccessHashSaveHandler`, `SetUserAccessHash`/`GetUserAccessHash`, `TelegramStorageCompatible() updates.StateStorage`, `TelegramAccessHasherCompatible() updates.ChannelAccessHasher`.
- `Storage` struct + `NewStorage(cache yacache.Cache[*redis.Client], log yalogger.Logger) *Storage`.
- `HandlerFunc func(ctx, tg.UpdatesClass) error` — adapts a plain func to `telegram.UpdateHandler`.
- `SessionStore` interface — `LoadSession`, `StoreSession`, `TelegramSessionStorageCompatible()`.
- `SessionStorage` struct + `NewSessionStorage(entityID, secret) *SessionStorage` (in-memory repo) / `NewSessionStorageWithCustomRepo(entityID, secret, repo EntitySessionStorageRepo) *SessionStorage`.
- `EntitySessionStorageRepo` interface — `UpdateAuthKey`, `FetchAuthKey`.
- `GormRepo` + `NewGormSessionStorage(poolDB *gorm.DB) (*GormRepo, yaerrors.Error)` (auto-migrates the `YaTgClientSession` table); `MemoryRepo` + `NewMemorySessionStorage(entityID) *MemoryRepo`.
- `AES` struct + `NewAES(key string) AES` — `Encrypt`/`Decrypt` (AES-256-CTR); `DeriveAESKey(data string) []byte` (SHA-256).
- `YaTgClientSession` GORM model; `const BasePathRedisJSON = "$"`, `FieldEncryptedAuthKey`, `LoggerEntityID`, etc.

## Usage Notes

- Uses ReJSON v2, so `Storage` requires a Redis build with the RedisJSON module (via `cache.Raw()` exposing a `*redis.Client` with `JSONGet`/`JSONSet`/`JSONMSet`).
- The internal `writeStateWithRecovery` auto-repairs missing/wrong-type RedisJSON roots (lazily creates `{}` at `"$"`) and retries once on recoverable errors (e.g. "no such key", "wrongtype").
- Session storage (AES + GORM) is a separate concern from pts/qts state storage; the AES key is derived via SHA-256 of the secret string (`DeriveAESKey`), giving AES-256. Depends on `yacache`, `yaerrors`, `yalogger`, `yathreadsafeset`; used by `yatgbot` and `yatgclient` (`NewUpdateManagerWithYaStorage`).
- Fx: `StoreModule` provides `Store` (needs `yacache.Cache[*redis.Client]` + `yalogger.Logger`); `SessionRepoModule` provides `EntitySessionStorageRepo` from a `*gorm.DB` (`fx.go`).
