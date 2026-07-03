---
name: goyacodedevutils-yaencoding
description: Serialize/deserialize arbitrary Go values using Gob or MessagePack, plus base64 string<->bytes helpers. Use instead of hand-rolling encoding/gob or msgpack calls directly.
---

# yaencoding Skill

Import path: `github.com/YaCodeDev/GoYaCodeDevUtils/yaencoding`.

Serializes/deserializes arbitrary Go values using Gob or MessagePack, plus base64 string/bytes helpers.

## Key API

- `EncodeGob(v any) ([]byte, yaerrors.Error)`, `DecodeGob[T any](data []byte) (*T, yaerrors.Error)`.
- `EncodeMessagePack(value any) ([]byte, yaerrors.Error)`, `DecodeMessagePack[T any](bytes []byte) (*T, yaerrors.Error)`.
- `ToString(data []byte) string` — base64 standard encode.
- `ToBytes(data string) ([]byte, yaerrors.Error)` — base64 standard decode.

## Usage Notes

- `EncodeGob`/`EncodeMessagePack` return raw `[]byte`, **not** base64 text, despite what the doc comment implies — call `ToString`/`ToBytes` separately if you need a text-safe transport (as `yaginmiddleware` does).
- Depends only on `yaerrors` + `vmihailenco/msgpack`; used by `yaginmiddleware`'s RSA-secure-header pipeline.
