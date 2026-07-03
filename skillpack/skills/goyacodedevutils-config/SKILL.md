---
name: goyacodedevutils-config
description: Load typed Go config structs from environment variables, .env, and .yatools/<name>.json overlays via reflection. Use for any app/tool config loading instead of hand-rolled os.Getenv calls.
---

# config Skill

Import path: `github.com/YaCodeDev/GoYaCodeDevUtils/config`.

Loads configuration from environment variables (with `.env` and optional `.yatools/<name>.json` overlays)
directly into typed Go structs via reflection. Use instead of hand-rolled `os.Getenv` parsing.

## Key API

- `LoadConfigStructFromEnv[T](instance *T, log)` — loads env into `instance`; panics/`Fatalf` on error.
- `LoadConfigStructFromEnvHandlingError[T](instance *T, log) yaerrors.Error` — same, returns an error instead of panicking.
- `LoadConfigStructFromEnvWithYaTools[T](name string, instance *T, log) yaerrors.Error` — also seeds env from `.yatools/<name>.json` (project dir, then home dir) before loading.
- `LoadDotEnv() yaerrors.Error` — parses `.env` in the working directory; never overrides an already-set env var.
- `GetEnv`/`GetEnvArray`/`GetEnvMap[T]` (and `*WithCustomType` variants) — single-value reads outside a struct.
- `LoadYaToolsConfig` / `LoadYaToolsConfigFromDir` / `WriteYaToolsConfig` / `WriteYaToolsConfigToDir` / `WriteYaToolsHomeConfig` / `SeedEnvFromYaToolsConfig` / `YaToolsConfigPath` / `YaToolsHomeConfigPath` — the `.yatools/<name>.json` read/write/seed primitives.
- `const DefaultTagName = "default"`.
- `ErrConfigStructMustBeStruct`, `ErrValueIsRequired`, `ErrInvalidDotEnvFileFormat`.

## Usage Notes

- Struct field names convert to `SCREAMING_SNAKE_CASE` env keys; nested structs get a `PARENT_CHILD` prefix (e.g. `OpenAI.APIKey` → `OPEN_AI_API_KEY`).
- Use a `default:"..."` tag for fallback values (parsed the same way as a real env value).
- A field with no default tag and no env var set is required — missing it fails/panics the load; pre-populate fields you want as defaults on the instance before calling if you don't want to use tags.
- Depends on `valueparser` for parsing, `yaerrors` for errors, and `yalogger` (a nil logger auto-defaults to a base logrus logger).
- Precedence with `LoadConfigStructFromEnvWithYaTools`: real env vars set before the process starts win, then `.env`, then the project's `.yatools/<name>.json`, then the home `.yatools/<name>.json`, then struct defaults.
