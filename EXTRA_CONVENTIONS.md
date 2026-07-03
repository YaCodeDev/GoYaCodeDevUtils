# Extra Conventions — GoYaCodeDevUtils
## What this repo is

`github.com/YaCodeDev/GoYaCodeDevUtils` (go 1.25.0) is a pure Go utility library, not an Gin/MariaDB service. There are no controllers, services, usecases, DTOs or models here — the baseline's service-layering rules do not apply. Each top-level directory is one independently usable package (see `skillpack/skills/goyacodedevutils-catalog/SKILL.md` for the full package list and purpose of each). Every time a new package is added, it should be added to the catalog in that skillpack.

## Package layout

Each package keeps its own `constants.go` and, where it defines errors, its own `errors.go` with sentinel `var Err... = errors.New("...")` declarations — this already matches the baseline's "package constants live in `constants.go`" rule. Larger packages add `types.go` (named types) and `utils.go` (unexported helpers) alongside the main implementation file(s).

## Errors

Every package returns `yaerrors.Error` (`github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors`) instead of the builtin `error`, exactly like the baseline's own `yaerrors` guidance. This repo additionally enables the `err113` linter (the baseline demo disables it): a non-trivial error must originate from a named sentinel declared in a package's `errors.go`, then be raised through `yaerrors.FromError`/`FromErrorWithLog`/`FromString`/`FromStringWithLog` — an inline `errors.New(...)` or `fmt.Errorf(...)` at a non-`errors.go` call site fails lint here.

## Lint differences from the baseline demo (`.golangci.yml`)

- `tagliatelle` and `recvcheck` are **enabled** here (the baseline demo disables both): struct/JSON tags must satisfy `tagliatelle`'s casing rules, and each type's methods must consistently use one receiver kind (pointer or value), not a mix.
- `forbidigo` forbids a bare `panic(` or `unsafe.` call without a comment justifying it. `panic` is used sparingly here for unrecoverable, programmer-error conditions inside reflection-heavy code (e.g. `config/utils.go`, `yalogger/logrus_logger.go`) — never for expected/recoverable failures, which still return `yaerrors.Error` as usual.
- `_test.go` files are fully excluded from lint (`exclusions.paths` in `.golangci.yml`), so lint findings never come from test files in this repo.
- `*.dev` top-level directories (`old.dev/`, `variable.dev/`, `yascheduler.dev/`) are excluded from lint, from `git` (`.gitignore` has `*.dev*`), and from the module's supported surface — treat them as unreachable: never build, lint, reference, or extend code inside them.
- Formatters are the same four as the baseline demo: `gofmt`, `gofumpt`, `goimports`, `golines`.

## Generics and reflection

Several packages lean on Go generics (`[T any]`, type constraints) and `reflect` for generic behavior (`config`, `valueparser`, `threadsafemap`, `yathreadsafeset`, `yaautoflags`, `yaflags`). When extending these, follow the existing constraint/reflection patterns already in the package rather than introducing a parallel non-generic variant.

## License

The repository root `LICENCE.md` is titled "YaCodeDev Private License" — this is the canonical license text and name; any `license` field in a package manifest published from this repo (e.g. a `.yaaipkg` `yaaipkg.json`) should read `"YaCodeDev Private License"`.
