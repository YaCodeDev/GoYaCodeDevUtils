---
name: goyacodedevutils-yaimagesupport
description: Centralizes image.RegisterFormat decoder registration (JPEG/PNG/GIF/BMP/TIFF/WebP/ICO/XPM/PNM/SVG, plus optional CGO HEIF/HEIF-like) behind Init/InitCGO, with a bounded, overridable SVG decoder. Use instead of hand-rolling per-service blank imports and a local oksvg/rasterx SVG decoder.
---

# yaimagesupport Skill

Import path: `github.com/YaCodeDev/GoYaCodeDevUtils/yaimagesupport`.

Centralizes `image.RegisterFormat` decoder registration so services stop hand-rolling their own blank-import
lists and their own `oksvg`/`rasterx` SVG decode loop.

## Key API

- `Init()` — registers the pure-Go decoders (stdlib JPEG/PNG/GIF, `golang.org/x/image` BMP/TIFF/WebP,
  `github.com/fyne-io/image` ICO/XPM, `github.com/spakin/netpbm` PBM/PGM/PPM/PAM) plus this package's own SVG
  decoder. Safe to call more than once (`sync.Once` guarded).
- `InitCGO() yaerrors.Error` — calls `Init` and, when built with `CGO_ENABLED=1 -tags yaimagesupport_native`,
  additionally registers `github.com/jdeng/goheif` (HEIC/HEIF/AVIF-like ISO BMFF, format name `heic`). Without
  both build conditions it returns `ErrCGOSupportUnavailable` wrapped at HTTP 501; nil on success.
- `SetMaxSVGDimension(maxDimension int)` / `SetMaxSVGPayloadSize(maxPayloadSize int64)` — process-wide,
  concurrency-safe overrides for the SVG decoder's caps. A non-positive value resets to the default.
- `DefaultMaxSVGDimension = 4096` (px, ~64MiB worst-case RGBA), `DefaultMaxSVGPayloadSize = 10 << 20` (10MiB
  raw payload read before parsing).
- `ErrSVGPayloadTooLarge`, `ErrSVGDimensionTooLarge`, `ErrCGOSupportUnavailable` — sentinels, matchable via
  `errors.Is` against whatever `yaerrors.Error` the decode call returned.

## Usage Notes

- Call `Init()` (or `InitCGO()`) once from service startup, before any `image.Decode`/`image.DecodeConfig`
  call; registration is global via `image.RegisterFormat`, same as any stdlib-style decoder package.
- The registry detects format by magic bytes, not filename/extension — a payload named `.jxl` that is
  actually PNG still decodes as PNG.
- SVG decoding enforces both caps unconditionally: payload size before parsing (`io.LimitReader`) and
  rasterized width/height derived from the SVG's `viewBox` (rejected before allocating an oversized
  `image.RGBA`, not after). Both cap violations return a `yaerrors.Error` wrapping the matching sentinel.
- `InitCGO`'s native path vendors libde265/dav1d source (no system libde265/dav1d needed) but requires a C
  and C++ compiler on the build host; a `.github/workflows/yaimagesupport-native-prebuild.yml` job in this
  repo publishes a prebuilt `pkgdir` bundle to ArtifactKeeper `PreBuiltLibraries` so consumers can skip
  compiling the native decoder themselves (`-pkgdir` flag on `go build`).
- No `fx.go` — nothing here is constructible; `Init`/`InitCGO` are plain startup-hook functions, called
  directly from a service's own startup path (or its own `fx.Invoke`), not provided as an Fx module.
- Depends on `yaerrors` for every returned error (`err113` applies here like every other package in this
  repo — no bare `errors.New`/`fmt.Errorf` outside `errors.go`).
