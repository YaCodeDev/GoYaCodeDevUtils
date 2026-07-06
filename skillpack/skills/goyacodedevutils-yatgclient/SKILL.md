---
name: goyacodedevutils-yatgclient
description: Thin wrapper around gotd/td's telegram.Client adding background-connect-with-retry, bot-token authorization, updates.Manager wiring, and SOCKS5/MTProto proxy helpers. Use for direct low-level Telegram client control instead of raw gotd/td.
---

# yatgclient Skill

Import path: `github.com/YaCodeDev/GoYaCodeDevUtils/yatgclient`.

Thin wrapper around `gotd/td`'s `telegram.Client` adding background-connect-with-retry, bot-token
authorization, `updates.Manager` wiring to `yatgstorage`, and SOCKS5/MTProto proxy helpers.

## Key API

- `Client` struct (embeds `*telegram.Client`) + `ClientOptions{ AppID, AppHash, EntityID, TelegramOptions, ChunkSize, BackgroundConnectConfig }`; `NewClient(options *ClientOptions, log) *Client`.
- Methods: `BackgroundConnect(ctx) yaerrors.Error` (blocks until first connect, then retries forever in the background with exponential backoff), `BotAuthorization(ctx, botToken) yaerrors.Error`, `RunUpdatesManager(ctx, gaps *updates.Manager, options updates.AuthOptions, channel *chan EntityError) <-chan EntityError`, `UploadMediaPhoto`/`UploadMediaDocument`/`UploadFile(ctx, io.Reader)`.
- `EntityError` struct — `{ Err yaerrors.Error, EntityID int64 }`.
- `NewUpdateManagerWithYaStorage(entityID, handler, storage yatgstorage.Store) *updates.Manager`.
- `MTProto` struct — `{ Host, Port, Secret }` + `NewMTProtoWithParseURL(url, log) (*MTProto, yaerrors.Error)`, methods `String`/`GetFullAddress`/`ParseURL`/`GetResolver`/`GetInputClientProxy`.
- `SOCKS5` struct — `{ Host, Port, Username, Password }` + `NewSOCKS5WithParseURL(url, log) (*SOCKS5, yaerrors.Error)`, methods `String`/`GetFullAddress`/`GetAuth`/`ParseURL`/`GetContextDialer`/`GetResolver`.
- `const KiloByte`, `DefaultChunkSize = 512KiB`.

## Usage Notes

- `BackgroundConnect` uses `yabackoff.Exponential` internally for reconnects (1s initial, 2x multiplier, 2min max, 5min reset-after by default, via `BackgroundConnectConfig`).
- `RunUpdatesManager` also auto-retries the `updates.Manager` loop forever with backoff; both loops stop cleanly when `ctx` is cancelled.
- Directly depends on `yabackoff`, `yaerrors`, `yalogger`, `yatgstorage` (for `NewUpdateManagerWithYaStorage`); used by `yatgbot` — prefer `yatgbot` unless you need this level of direct client control.
- Fx: `Module` (`fx.go`) provides `*Client` and registers an `fx.Lifecycle` hook that background-connects on start and cancels on stop.
