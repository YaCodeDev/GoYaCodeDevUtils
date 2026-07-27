---
name: goyacodedevutils-yaturnstile
description: Verify Cloudflare Turnstile captcha tokens against the siteverify API, with fail-closed configuration, redacted credentials, and a bounded response read. Use instead of hand-rolling a siteverify HTTP client; pair with yaginmiddleware.Turnstile for Gin routes.
---

# yaturnstile Skill

Import path: `github.com/YaCodeDev/GoYaCodeDevUtils/yaturnstile`.

Verifies a client-submitted Cloudflare Turnstile token by POSTing it to Cloudflare's siteverify
endpoint. Transport-agnostic on purpose: the Gin adapter lives in `yaginmiddleware.Turnstile`, so a
socket handshake or a queue consumer can verify a token the same way an HTTP handler does — the same
split as `yarsa` (crypto) and `yaginmiddleware.RSASecureHeader` (Gin wrapper).

## Key API

- `Config` — `{ SecretKey SecretKey, Endpoint Endpoint, Disabled Disabled }`. Every field is a named
  type, not a bare primitive. Loadable via `config.LoadConfigStructFromEnv` (`..._SECRET_KEY`,
  `..._ENDPOINT`, `..._DISABLED`).
- `(*Config) Validate() yaerrors.Error` — endpoint must be an absolute URL; `SecretKey` is required
  **unless** `Disabled` is set. Not called by `NewVerifier`; call it at startup.
- `NewVerifier(config *Config, log yalogger.Logger) Verifier` — config copied once; a nil config or nil
  logger is tolerated. `Endpoint` empty falls back to `DefaultEndpoint`.
- `Verifier.Verify(ctx, token Token, remoteIP RemoteIP, log yalogger.Logger) (bool, yaerrors.Error)` —
  a nil `log` falls back to the constructor's logger.
- `SecretKey.Configured() bool`, `SecretKey.LogString()`, `Token.LogString()` → `"[REDACTED]"`.
- `const DefaultEndpoint`, `DefaultRequestTimeout = 10s`, `DefaultMaxResponseBytes = 64 KiB`.
- Sentinels in `errors.go`: `ErrConfigEndpointRequired`, `ErrConfigEndpointInvalid`,
  `ErrConfigSecretKeyRequired`, `ErrBuildRequest`, `ErrDoRequest`, `ErrDecodeResponse`,
  `ErrUnexpectedStatus`.
- Fx: `Module` (`fx.go`) provides `Verifier` from a supplied `*Config`.

## Security Notes

- **Fails closed on every path.** `false` means Cloudflare rejected the token; a non-nil error means the
  check could not be completed. Neither may be treated as a pass. A missing `SecretKey` with `Disabled`
  unset is a misconfiguration that returns a 500, **not** a silent bypass — that is the one behaviour to
  preserve if you ever adapt this code, since "blank secret disables captcha" is the classic way a
  production deployment ends up unprotected.
- Opting out is explicit and separate: set `Disabled` (env `..._DISABLED=true`) for local runs and CI.
- An empty token short-circuits to `false` without spending a siteverify call.
- The response body is read through an `io.LimitReader` capped at `DefaultMaxResponseBytes`, so a
  compromised or misrouted endpoint cannot stream an unbounded body at the JSON decoder.
- The HTTP client pins `MinVersion: tls.VersionTLS12` and a 10s timeout.
- `remoteIP` is compared by Cloudflare against the address the token was minted from. Deriving it from a
  proxy header only helps if that header is trusted — `gin`'s `ClientIP()` needs `SetTrustedProxies`
  configured, or an attacker-supplied value weakens the check.
- Verify checks the `success` flag only. Cloudflare also echoes `hostname` and `action`; a caller that
  pins those needs its own siteverify call.

## Usage Notes

- Depends only on `yaerrors`/`yalogger` — no third-party captcha library.
- Testing: point `Endpoint` at an `httptest.NewServer` that answers `{"success":true}` /
  `{"success":false,"error-codes":[...]}`; see this package's own `yaturnstile_test.go`.
- For a Gin route, do not call `Verify` from a handler — register `yaginmiddleware.NewTurnstile(verifier,
  log)` after an `ErrorBoundary` and let it guard the route group.
