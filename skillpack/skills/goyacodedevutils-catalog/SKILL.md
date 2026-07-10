---
name: goyacodedevutils-catalog
description: Index of every GoYaCodeDevUtils utility package (config, errors, logging, caching, rate limiting, backoff, hashing, gzip, RSA, feature flags, FSM, thread-safe collections, locales, Telegram bot stack) with a pointer to each package's own skill. Use before hand-rolling infrastructure code GoYaCodeDevUtils may already provide.
---

# GoYaCodeDevUtils Catalog

Use this skill first whenever you need infrastructure/utility functionality: config loading, errors,
logging, caching, rate limiting, retries, hashing, compression, encryption, feature flags, state machines,
thread-safe collections, i18n, or a Telegram bot. Using an existing GoYaCodeDevUtils package is mandatory
wherever one covers the need; do not hand-roll an equivalent.

Import path prefix: `github.com/YaCodeDev/GoYaCodeDevUtils/<package>`.

## Foundation

- `yaerrors` — structured error type with an HTTP-style code and wrap-chain traceback; the standard error return type across every package here. Skill: `goyacodedevutils-yaerrors`.
- `yalogger` — structured logrus-backed `Logger` interface threaded through nearly every package. Skill: `goyacodedevutils-yalogger`.
- `config` — loads env vars (plus `.env` and `.yatools/<name>.json` overlays) directly into typed structs via reflection. Skill: `goyacodedevutils-config`.
- `valueparser` — generic string-to-typed-value parsing (scalars, arrays, maps, custom `Unmarshalable`/`TextUnmarshaler` types); powers `config`. Skill: `goyacodedevutils-valueparser`.

## Data structures & caching

- `threadsafemap` — generic mutex-protected `map[K]V`. Skill: `goyacodedevutils-threadsafemap`.
- `yathreadsafeset` — generic mutex-protected set with union/difference/intersect. Skill: `goyacodedevutils-yathreadsafeset`.
- `yacache` — pluggable key-value cache (in-memory or Redis backend) with a hash-oriented API. Skill: `goyacodedevutils-yacache`.
- `yafsm` — finite-state-machine storage on top of `yacache`, keyed per-entity. Skill: `goyacodedevutils-yafsm`.
- `yaratelimit` — fixed-window rate limiter on top of `yacache`. Skill: `goyacodedevutils-yaratelimit`.

## Bit flags & retries

- `yaautoflags` — packs a struct's bool fields into/out of a single integer `Flags` field. Skill: `goyacodedevutils-yaautoflags`.
- `yaflags` — packs/unpacks raw bit-index lists into/from an unsigned integer. Skill: `goyacodedevutils-yaflags`.
- `yabackoff` — exponential back-off strategy for retry loops. Skill: `goyacodedevutils-yabackoff`.

## Encoding, hashing & security

- `yaencoding` — Gob/MessagePack encode-decode plus base64 helpers. Skill: `goyacodedevutils-yaencoding`.
- `yagzip` — gzip compress/decompress with a decompression-size cap. Skill: `goyacodedevutils-yagzip`.
- `yahash` — salted, time-windowed hashing (e.g. short-lived tokens) around any hash function. Skill: `goyacodedevutils-yahash`.
- `yarsa` — deterministic RSA key generation, flexible key parsing, chunked RSA-OAEP encrypt/decrypt. Skill: `goyacodedevutils-yarsa`.
- `yaginmiddleware` — Gin middleware that encrypts a typed struct into an HTTP header (RSA-OAEP + gzip + MessagePack + base64), built on `yarsa`/`yagzip`/`yaencoding`. Skill: `goyacodedevutils-yaginmiddleware`.
- `yasmtp` — SMTP mailer (STARTTLS + PLAIN auth) with connection reuse, `yabackoff` retry, and optional `html/template` rendering. Skill: `goyacodedevutils-yasmtp`.

## Localization

- `yalocales` — loads JSON locale files into a lookup/format tree with strict cross-language key consistency, plus Go codegen. Skill: `goyacodedevutils-yalocales`.

## Telegram bot stack

- `yatgbot` — high-level Telegram bot framework (router, filters, middleware, FSM-aware routing, per-user locale, message queue); composes most of the packages above. Skill: `goyacodedevutils-yatgbot`.
- `yatgclient` — gotd/td client wrapper: background-connect-with-retry, bot auth, updates-manager wiring, SOCKS5/MTProto proxy helpers. Skill: `goyacodedevutils-yatgclient`.
- `yatgstorage` — Redis-backed `updates.Manager` state storage plus AES/GORM session storage for the client auth key. Skill: `goyacodedevutils-yatgstorage`.
- `yatgmessageencoding` — converts a custom Markdown-like syntax to/from Telegram rich-text entities (UTF-16LE offsets). Skill: `goyacodedevutils-yatgmessageencoding`.

## Fx wiring

Optional, additive `go.uber.org/fx` modules exist in each package's own `fx.go` for `yalogger`, `yagzip`,
`yabackoff`, `yatgmessageencoding`, `yalocales`, `yacache`, `yatgstorage`, `yatgclient`, `yatgbot`, and
`yasmtp` —
consuming services can wire these via Fx instead of manual construction. Packages left out are generic
over a caller-specific type parameter (`threadsafemap`, `yathreadsafeset`, `yahash`, `yaratelimit`,
`yafsm`, `yaginmiddleware`, `config`) or are pure helper-function packages with nothing constructible
(`yaerrors`, `yaencoding`, `yaflags`, `yaautoflags`, `yarsa`, `valueparser`) — see each package's own
skill for its module name(s).

## Choosing a package

- Need a typed config from env vars? `config` (with optional `.yatools/<name>.json` overlay support).
- Need to return or propagate an error with an HTTP-style code? `yaerrors` — never the builtin `error` for anything non-trivial.
- Need a cache, and maybe rate limiting or a state machine on top of it? `yacache`, then `yaratelimit`/`yafsm`.
- Need retries with backoff? `yabackoff`, not a hand-rolled sleep loop.
- Need to send an encrypted struct over an HTTP header? `yaginmiddleware` (already wires `yarsa` + `yagzip` + `yaencoding`).
- Building a Telegram bot? Start from `yatgbot`; only reach for `yatgclient`/`yatgstorage`/`yatgmessageencoding` directly for lower-level control.
- Need to send an email (verification code, notification)? `yasmtp` — not a hand-rolled `net/smtp` call.
- Something not listed here but still infrastructure-shaped (env config, caching, retries, hashing, encryption, i18n, bit flags, thread-safe collections)? Re-check this catalog before adding a new dependency or hand-rolling it.
