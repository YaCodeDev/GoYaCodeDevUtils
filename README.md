# GoYaCodeDevUtils

This repository contains a collection of Go utilities used across YaCodeDev projects.

The utilities are designed to be reusable and modular, making it easy to integrate them into different projects.

## Agent skill pack

`skillpack/` is the source for a `.yaaipkg` skill pack: a catalog skill (`goyacodedevutils-catalog`) plus
one usage skill per utility package (config, errors, logging, caching, rate limiting, backoff, hashing,
gzip, RSA, feature flags, FSM, thread-safe collections, locales, and the Telegram bot stack), so an AI
coding agent can find and use the right package instead of hand-rolling equivalent functionality. It has no
agent file — it installs and updates as a skill-only package.

Install it into a project with [`yaagentmanager`](https://github.com/YaCodeDev/YaCodeDevTools/tree/main/yaagentmanager):

```bash
yaagentmanager install goyacodedevutils
```

Build and publish a new version by pushing a `vX.Y.Z` tag (e.g. `v0.2.0`), or `vX.Y.Z-rc.N` for a
release candidate (e.g. `v0.2.0-rc.1`); `.github/workflows/skill-build.yaml` packs `skillpack/` and
publishes it to artifactkeeper. To build it locally instead:

```bash
yaagentmanager pack skillpack --version 0.1.0
```

## Development

The repo is normally checked out under a parent Go workspace, so local gates force `GOWORK=off`.
Ignored `*.dev` directories are not part of the supported package surface and are excluded from `make test`.

```bash
make format
make lint
make test
make test-race
```

### Versioning

Release tags are plain semver, never a `skill-v`-style prefix: `vX.Y.Z` for a release, `vX.Y.Z-rc.N`
for a release candidate/beta. Start at `0.0.1`. Bump **minor** for every shipped major feature (first
feature -> `0.1.0`, second -> `0.2.0`, ...). Bump **patch** whenever a build/deploy is needed, at most
once per commit. Bump **major** manually, rarely. An `-rc.N` suffix's `N` starts at `1` for a version's
first candidate and increments per additional candidate before the final release tag drops the suffix.

An rc/beta version installs only "on demand", never as `@latest`: pin it explicitly with
`yaagentmanager install goyacodedevutils@0.2.0-rc.1` (or `update -version 0.2.0-rc.1`).
