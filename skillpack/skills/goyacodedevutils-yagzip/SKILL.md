---
name: goyacodedevutils-yagzip
description: Gzip compress/decompress helpers for []byte payloads with a decompression size cap (zip-bomb protection). Use instead of the stdlib compress/gzip package directly.
---

# yagzip Skill

Import path: `github.com/YaCodeDev/GoYaCodeDevUtils/yagzip`.

Gzip compress/decompress helpers for `[]byte` payloads with a decompression size cap (zip-bomb protection).

## Key API

- `Gzip` struct — `{ Level, MaxDecompressedSize }`.
- `NewGzip() *Gzip` — default level + 64MiB cap.
- `NewGzipWithLevel(level int) *Gzip`.
- `NewGzipWithLevelAndMaxSize(level, maxDecompressedSize int64) *Gzip`.
- Methods: `Zip(object []byte) ([]byte, yaerrors.Error)`, `Unzip(compressed []byte) ([]byte, yaerrors.Error)`.
- `const DefaultCompression = flate.DefaultCompression`, `DefaultMaxDecompressedSize = 64 << 20` (64 MiB).
- `var ErrDecompressedPayloadTooLarge`.

## Usage Notes

- `Unzip` enforces `MaxDecompressedSize` (default 64MiB) via `io.LimitReader`; exceeding it returns `ErrDecompressedPayloadTooLarge` — tune the cap with `NewGzipWithLevelAndMaxSize` for larger/smaller expected payloads.
- Depends only on `yaerrors`; used by `yaginmiddleware`'s RSA-secure-header pipeline.
