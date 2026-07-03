---
name: goyacodedevutils-yarsa
description: RSA utilities - deterministic key generation, flexible private-key parsing (PEM/DER/base64), and chunked RSA-OAEP encrypt/decrypt for arbitrary-length data. Use instead of hand-rolling crypto/rsa key handling.
---

# yarsa Skill

Import path: `github.com/YaCodeDev/GoYaCodeDevUtils/yarsa`.

RSA utilities: deterministic key generation (reproducible from a seed), flexible private-key parsing
(PEM/DER/base64 variants), and chunked RSA-OAEP encrypt/decrypt for arbitrary-length data.

## Key API

- `KeyOpts` struct — `{ Bits, Exponent, Seed []byte }`.
- `GenerateDeterministicRSAPrivateKey(opts KeyOpts) (*rsa.PrivateKey, yaerrors.Error)` — the same seed + bits + exponent always yields an identical key.
- `ParsePrivateKey(s string) (*rsa.PrivateKey, yaerrors.Error)` — accepts PEM (PKCS#1/PKCS#8), base64-of-PEM, or base64-of-raw-DER.
- `DeterministicReader` struct + `NewDeterministicReader(seed []byte) *DeterministicReader` — HMAC-SHA256-based deterministic byte stream, an `io.Reader`.
- `StripCRLF(s string) string`.
- `Encrypt(plaintext []byte, public *rsa.PublicKey) ([]byte, yaerrors.Error)` — RSA-OAEP (SHA-256), auto-chunks plaintext (190 bytes/chunk for 2048-bit keys).
- `Decrypt(ciphertext []byte, private *rsa.PrivateKey) ([]byte, yaerrors.Error)` — reverses the chunking (ciphertext length must be a multiple of the key's block size).

## Usage Notes

- `GenerateDeterministicRSAPrivateKey` requires a high-entropy `Seed` — a weak seed fully compromises the key. `Bits` must be even and `>= 512`.
- `DeterministicReader` and `GenerateDeterministicRSA*` are **not** concurrency-safe — one instance per goroutine.
- `Encrypt`/`Decrypt` don't handle transport encoding (no base64) — apply that at the edges yourself (see `yaginmiddleware` for an example combining it with `yaencoding.ToString`). Depends only on `yaerrors`.
