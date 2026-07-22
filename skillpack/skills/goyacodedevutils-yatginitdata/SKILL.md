---
name: goyacodedevutils-yatginitdata
description: Parses and validates Telegram Mini App `initData` login payloads (HMAC-SHA256 per Telegram's documented algorithm). Use instead of a third-party initData validator.
---

# yatginitdata Skill

Import path: `github.com/YaCodeDev/GoYaCodeDevUtils/yatginitdata`.

Parses and validates Telegram Mini App `initData` (`window.Telegram.WebApp.initData`) login payloads.
Cross-checked against Telegram's official documentation and the
`github.com/telegram-mini-apps/init-data-golang` v1.5.0 reference implementation this package replaces —
use it instead of pulling in that (or any other) third-party initData library.

## Key API

- `Data` struct — `AuthDateRaw`, `CanSendAfterRaw`, `Chat`, `ChatType`, `ChatInstance`, `Hash`, `QueryID`, `Receiver`, `StartParam`, `User` (all typed per Telegram's parameter list). Methods `AuthDate() time.Time`, `CanSendAfter() time.Time`.
- `User` / `Chat` structs mirror Telegram's own field set (`ID`, `FirstName`, `LastName`, `Username`, `IsPremium`, etc. / `ID`, `Type`, `Title`, ...).
- `Parse(initData string) (Data, yaerrors.Error)` — decodes the raw query string into `Data`. Does **not** check the signature.
- `Validate(initData, botToken string, maxAge time.Duration) yaerrors.Error` — recomputes the HMAC-SHA256 signature and compares it to the payload's `hash` field. `maxAge > 0` also rejects a missing/stale `auth_date`; `maxAge <= 0` skips the expiry check.
- `Sign(fields map[string]string, botToken string, authDate time.Time) string` — builds a signed initData query string for tests/local tooling that need a valid payload without a real Telegram client.

## Usage Notes

- Always call `Validate` on data received from a client before trusting it; `Parse` alone does not verify authenticity.
- Algorithm: data-check-string = every field except `hash`, sorted alphabetically as `key=value`, joined with `\n`; secret key = `HMAC-SHA256("WebAppData", botToken)`; hash = hex(`HMAC-SHA256(secretKey, dataCheckString)`).
- `botToken` is the same bot token used elsewhere for `yatgclient`/`yatgbot` — this package needs no network access and no `gotd/td` dependency.
- Depends only on `yaerrors` plus the stdlib (`crypto/hmac`, `crypto/sha256`, `net/url`, `encoding/json`).
