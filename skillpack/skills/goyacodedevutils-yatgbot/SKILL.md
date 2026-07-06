---
name: goyacodedevutils-yatgbot
description: High-level Telegram bot framework on top of gotd/td - router/dispatcher with filters and middleware, FSM-aware routing, per-user localization, sequential-update scheduling, and a message-send queue. Use as the starting point for any Telegram bot.
---

# yatgbot Skill

Import path: `github.com/YaCodeDev/GoYaCodeDevUtils/yatgbot`.

High-level Telegram bot framework on top of `gotd/td` — router/dispatcher with filters and middleware,
FSM-aware routing, per-user localization, sequential-update scheduling, and a message-send queue. Wires
together `yatgclient`, `yatgstorage`, `yafsm`, `yalocales`, and `yacache`.

## Key API

- `Options` struct — all `InitYaTgBot` inputs: `AppID`/`AppHash`/`BotToken`/`PoolDB`/`Cache`/`MainRouter`/`ParseMode`/`Log`/`Sync`/`Features`/`EmbeddedLocales`/`MessageQueueRatePerSecond`/`ForgetUpdatesOnStart`.
- `InitYaTgBot(ctx, *Options) (Dispatcher, yaerrors.Error)` — one-call bootstrap: parses the bot token, sets up FSM storage, localizer, GORM+Redis session/state storage, the gotd client, connects, authorizes, and starts `updates.Manager`.
- `Dispatcher` struct — `{ FSMStore, Log, BotUser, MessageDispatcher, Localizer, Client, UpdatesErrors <-chan yatgclient.EntityError, MainRouter, Features }` + `Bind(tgDispatcher, sync bool)`.
- `RouterGroup` struct + `NewRouterGroup()`; `router.OnMessage`/`OnCallback`/`OnEditMessage`/`OnDeleteMessage`/`OnNewChannelMessage`/`OnEditChannelMessage`/`OnDeleteChannelMessages`/`OnMessageReactions`/`OnChannelParticipant`/`OnPrecheckoutQuery`/`OnInlineQuery(handler, filters...)`; `RouterGroup.IncludeRouter(subs...)`, `AddMiddleware(mw...)`.
- `Filter` type + `StateIs`/`TextEq`/`TextRegex`/`CallbackEq`/`CallbackPrefix`/`MessageServiceFilter`/`MessageServiceActionFilter[T]`/`OneOfFilter`/`AllOfFilter`.
- `HandlerData` struct — per-handler deps: `Entities`, `Client`, `Update`, `UserID`, `Peer`, `StateStorage`, `Log`, `Dispatcher`, `Localizer`, `JobResults`.
- `FeatureFlags` (uint8) + `const FeatureSequentialUpdates`.
- `ExtractMessageFromUpdate`/`ExtractMessageServiceFromUpdate(upd) (*tg.Message/*tg.MessageService, bool)`.

## Usage Notes

- `FeatureSequentialUpdates` ensures async updates sharing a user/chat key are processed in arrival order via an internal scheduler; `Options.Sync = true` forces fully synchronous (blocking) dispatch instead.
- Filters run root-router-first, then local/route filters; a handler can signal "not my type" via `ErrRouteMismatch` to fall through (used internally by `wrapHandler`'s type assertion).
- Heavily composes other packages: `yacache` (FSM + state storage), `yafsm` (per-user state), `yalocales` (per-user language via `user.LangCode`), `yalogger`, `yatgclient` (connection/auth), `yatgmessageencoding` (`ParseMode`), `yatgstorage` (session + `updates.Manager` state). Has an internal `messagequeue` subpackage not covered by its own skill.
- Fx: `Module` (`fx.go`) provides `*Dispatcher`, deferring `InitYaTgBot` to an `fx.Lifecycle` `OnStart` hook (needs a live context); `OnStop` tears the connection down.
