---
name: goyacodedevutils-yaginmiddleware
description: Gin middleware that transparently encrypts/decrypts a typed struct carried in an HTTP header via RSA-OAEP + gzip + MessagePack + base64. Use for any Gin route that needs an encrypted request/response header payload.
---

# yaginmiddleware Skill

Import path: `github.com/YaCodeDev/GoYaCodeDevUtils/yaginmiddleware`.

Gin middleware for transparently encrypting/decrypting a typed struct carried in an HTTP header via
RSA-OAEP + gzip + MessagePack + base64.

## Key API

- `Middleware` interface — `Handle(ctx *gin.Context)`.
- `RSASecureHeader[T any]` struct — `{ RSA, HeaderName, ContextKey, ContextAbort }`.
- `NewEncodeRSA[T](headerName, contextKey, rsaPrivateKey, contextAbort) *RSASecureHeader[T]`.
- `NewEncodeRSAWithCompressionLevel[T](..., compressionLevel) *RSASecureHeader[T]`.
- Methods: `Encode(data T) (string, yaerrors.Error)`, `EncodeWithSrc(src string, data T)`, `Decode(data string) (string, *T, yaerrors.Error)`, `Handle(ctx)` (Gin middleware), `HandleRequest(ctx)` (non-middleware one-shot variant).

## Usage Notes

- Pipeline: struct → MessagePack → gzip → RSA-OAEP encrypt (public key, chunked) → base64. `Encode` uses the public half of the key, `Decode` the private half.
- `EncodeWithSrc`/`Decode` support an optional plaintext prefix (separated by an invisible Hangul filler rune) that survives the round-trip unencrypted — useful for embedding a version/client tag.
- If `ContextAbort = true` and decoding fails, `Handle` aborts the Gin request. Depends on `yaencoding`, `yaerrors`, `yagzip`, `yarsa` (all in this repo) plus `gin-gonic/gin`.
