---
name: goyacodedevutils-yaginmiddleware
description: Gin middleware collection — an encrypted-header codec (RSA-OAEP + gzip + MessagePack + base64), a centralized HTTP error boundary, static-secret and HS256-JWT bearer auth, and a non-production debug-CORS handler. Use for any Gin route needing an encrypted header payload, centralized error responses, bearer auth, or dev-only CORS.
---

# yaginmiddleware Skill

Import path: `github.com/YaCodeDev/GoYaCodeDevUtils/yaginmiddleware`.

Every middleware in this package implements `Middleware interface { Handle(ctx *gin.Context) }` and is
registered via `router.Use(x.Handle)`.

## RSASecureHeader — encrypted struct header codec

- `RSASecureHeader[T any]` struct — `{ RSA, HeaderName, ContextKey, ContextAbort }`.
- `NewEncodeRSA[T](headerName, contextKey, rsaPrivateKey, contextAbort) *RSASecureHeader[T]`.
- `NewEncodeRSAWithCompressionLevel[T](..., compressionLevel) *RSASecureHeader[T]`.
- Methods: `Encode(data T) (string, yaerrors.Error)`, `EncodeWithSrc(src string, data T)`, `Decode(data string) (string, *T, yaerrors.Error)`, `Handle(ctx)` (Gin middleware), `HandleRequest(ctx)` (non-middleware one-shot variant).
- Pipeline: struct → MessagePack → gzip → RSA-OAEP encrypt (public key, chunked) → base64. `Encode` uses the public half of the key, `Decode` the private half.
- `EncodeWithSrc`/`Decode` support an optional plaintext prefix (separated by an invisible Hangul filler rune) that survives the round-trip unencrypted — useful for embedding a version/client tag.
- If `ContextAbort = true` and decoding fails, `Handle` aborts the Gin request. Depends on `yaencoding`, `yaerrors`, `yagzip`, `yarsa` plus `gin-gonic/gin`.

## ErrorBoundary — centralized HTTP error responses

- `NewErrorBoundary(log yalogger.Logger) *ErrorBoundary` — default `{"error": message}` response shape.
- `NewErrorBoundaryWithResponse(log, response ErrorResponseFunc) *ErrorBoundary` — `type ErrorResponseFunc func(status int, message string) any`, for services whose existing API contract uses a different response shape (e.g. `{"status":.., "message":..}`).
- Downstream code records an error via `ctx.Error(err)`; `ErrorBoundary.Handle` reads `ctx.Errors` after `ctx.Next()`: single `yaerrors.Error` → its `Code()`/`UnwrapLastError()` become the response, logged Warn (4xx/503) or Error otherwise; single non-`yaerrors.Error` → generic 500; more than one recorded error → flat `418 "Backend developer is a teapot"` (an existing org-wide convention for the "should not happen" case, not new).
- Must be registered *before* `StaticBearerAuth`/`JWTBearerAuth` (or any middleware that aborts via `ctx.Error`+`ctx.Abort`) in the `Use()` chain — those middlewares don't write a response themselves.

## StaticBearerAuth — shared-secret bearer auth

- `NewStaticBearerAuth(secret string) *StaticBearerAuth`.
- Compares the `Authorization` (falling back to `AccessToken`) header, minus a `Bearer ` prefix, against `secret` in constant time (`crypto/subtle`).
- **Fails closed**: empty `secret`, missing header, or mismatch all reject with 401 — an unconfigured secret never means "no auth required".

## JWTBearerAuth / ValidateJWT / GenerateJWT — HS256 bearer auth

- `JWTClaims{ Sub uint64; Role uint8; Status uint8; jwt.RegisteredClaims }` — this org's standard claim shape.
- `GenerateJWT(subject uint64, role, status uint8, life time.Duration, secret []byte) (string, yaerrors.Error)`.
- `ValidateJWT(tokenString string, secret []byte) (JWTClaims, yaerrors.Error)` — plain function (HS256, one-minute leeway), reusable outside Gin (e.g. a Socket.IO handshake).
- `NewJWTBearerAuth(secret []byte, minRole uint8) *JWTBearerAuth` / `NewJWTBearerAuthWithContextKey(secret, minRole, contextKey)` — Gin wrapper requiring `claims.Role >= minRole`, storing claims under `DefaultJWTContextKey` (or a custom key).
- Covers the common "validate token + minimum role" case only. Services with extra claim-based rules (status handling, side effects) should call `ValidateJWT` directly from their own middleware instead of wrapping `JWTBearerAuth`.

## DebugCORS — non-production CORS

- `NewDebugCORS() *DebugCORS` — reflects the request `Origin` back as an allow-all CORS policy, answers `OPTIONS` preflights with 204. Non-production use only.
