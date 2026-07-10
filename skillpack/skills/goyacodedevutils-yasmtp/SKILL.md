---
name: goyacodedevutils-yasmtp
description: Send email through a plain SMTP relay (STARTTLS + PLAIN auth) with connection reuse, bounded yabackoff retry, and optional html/template rendering. Use instead of hand-rolling net/smtp calls.
---

# yasmtp Skill

Import path: `github.com/YaCodeDev/GoYaCodeDevUtils/yasmtp`.

Sends `Message` values through a single, lazily-dialed, mutex-guarded SMTP connection (STARTTLS +
`PLAIN` auth). A failed send closes and clears the connection so the next call redials. Every send is
retried with a `yabackoff` exponential backoff up to `DefaultMaxAttempts`. Header values (`To`,
`Subject`, `From`) are sanitized (CRLF stripped) before the message is built, preventing header
injection.

## Key API

- `Config` — `{ Host Host, Port Port, Username Username, Password Password, From From; TLSConfig
  *tls.Config }`. Every scalar field is a named string/uint16 type, not a bare primitive. `Host` and
  `From` have no universal default and are required; `Port` defaults to `587`; `Username`/`Password` are
  optional (empty skips auth). `TLSConfig` nil derives `{ServerName: Host, MinVersion: TLS12}` from the
  system root CAs — set it only to trust a private CA or a self-signed relay.
- `(*Config) Validate() yaerrors.Error` — cascades `Host`/`Port`/`From` validation (non-empty, non-zero,
  parseable address). Not called automatically by `NewMailer`; call it explicitly after loading `Config`.
- `Config` is loadable via `config.LoadConfigStructFromEnv[yasmtp.Config](&cfg, log)` — env keys derive
  from field names (`..._HOST`, `..._PORT`, `..._USERNAME`, `..._PASSWORD`, `..._FROM` under whatever key
  path the caller nests it at).
- `Password.LogString() string` returns `"[REDACTED]"` — safe to log a `Config`/`Password` value directly.
- `Message` — `{ To []Recipient, Subject Subject, Text Body, HTML Body }`. At least one of `Text`/`HTML`
  must be set; when both are set the message is sent `multipart/alternative`.
- `(Message) Validate() yaerrors.Error` — non-empty `To` with every `Recipient` a parseable address, plus
  a non-empty body. `Send` calls this internally.
- `NewMailer(config *Config, log yalogger.Logger) *Mailer` — config is copied once at construction.
- `(*Mailer) Send(ctx context.Context, message Message) yaerrors.Error`
- `(*Mailer) SendTemplate(ctx context.Context, to []Recipient, subject Subject, tmpl *template.Template, data any) yaerrors.Error` — renders `tmpl` into an HTML body and sends it.
- `(*Mailer) Close() yaerrors.Error` — releases the pooled connection; safe to call even if never connected.
- `const DefaultMaxAttempts = 3`, `DefaultRetryInitialInterval = 500ms`, `DefaultRetryMultiplier = 2.0`, `DefaultRetryMaxInterval = 5s`, `DefaultDialTimeout = 10s`.

## Usage Notes

- Depends on `yabackoff` (retry) and `yaerrors`/`yalogger` (the standard error/logging types) — no other
  GoYaCodeDevUtils package or third-party mail library.
- Connection reuse is a single pooled `*smtp.Client` guarded by a `sync.Mutex`; concurrent `Send` calls
  serialize on it rather than opening one connection per call. A `Noop()` health check runs before reuse;
  on failure or any transmit error the connection is closed and the next `Send` redials.
- `Send` fails fast (no dial attempt) when `Message.Validate()` rejects it (no recipients, an invalid
  recipient address, or no body).
- Retries only wrap the whole dial-through-transmit attempt; a context that is `Done()` before a retry's
  wait elapses short-circuits immediately instead of sleeping — pass an already-canceled context to skip
  retries in tests.
- Testing without a real SMTP relay: run a local `net.Listen` server implementing STARTTLS with a
  self-signed cert and point `Config.TLSConfig.RootCAs` at that cert's pool — see this package's own
  `testserver_test.go` for a reusable fake-server pattern.
- Fx: `Module` (`fx.go`) provides `*Mailer` from a supplied `*Config` (`fx.Supply(&yasmtp.Config{...})`).
