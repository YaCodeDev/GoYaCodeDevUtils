---
name: goyacodedevutils-yaerrors
description: Structured error type with an HTTP-style status code and wrap-chain traceback. The standard error return type across GoYaCodeDevUtils and YaCodeDev services — use instead of the builtin error/fmt.Errorf for anything non-trivial.
---

# yaerrors Skill

Import path: `github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors`.

Custom structured error type carrying an HTTP-style status code and a wrap-chain traceback. Used as the
standard error return type across this entire ecosystem instead of the builtin `error`.

## Key API

- `Error` interface — embeds `error`; adds `Wrap(msg) Error`, `WrapWithLog(msg, log) Error`, `Code() int`, `Error() string`, `Unwrap() error`, `UnwrapLastError() string`.
- `FromError(code int, cause error, wrap string) Error` — wrap an existing error with a code and message.
- `FromErrorWithLog(code, cause, wrap, log) Error` — same, and also logs.
- `FromString(code int, msg string) Error` — construct from a plain message, no underlying cause.
- `FromStringWithLog(code, msg, log) Error` — same, and also logs.
- `var ErrTeapot` — safety fallback (`code = http.StatusTeapot`) substituted when a nil `*yaError` is called.

## Usage Notes

- Call `Wrap(msg)` at every layer as the error propagates up; each wrap prepends context to the traceback (`msg -> msg -> code | original`).
- Methods are nil-safe: calling them on a nil `*yaError` returns `ErrTeapot` instead of panicking — a bug marker, not a crash.
- `*WithLog` constructors/methods always log at `Error` level. For a 4xx or otherwise expected/transient error, prefer plain `FromError`/`FromString`/`Wrap` plus a separate classify-then-log step, so client/validation errors don't page maintenance at `Error` severity.
- No dependency on other repo packages except `yalogger` (only for the `*WithLog` variants); nearly every other package in this repo returns `yaerrors.Error`.
