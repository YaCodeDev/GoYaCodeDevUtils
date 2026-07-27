---
name: goyacodedevutils-yatotp
description: RFC 6238 time-based one-time passwords for two-factor auth — secret minting, otpauth provisioning URIs, code derivation, skew-tolerant verification with replay rejection, and single-use recovery codes. Use instead of a third-party TOTP library.
---

# yatotp Skill

Import path: `github.com/YaCodeDev/GoYaCodeDevUtils/yatotp`.

Implements the whole second-factor loop every authenticator app speaks: mint a shared secret, hand the
user an `otpauth://` URI to scan, derive codes, verify a submitted code with bounded clock skew, and
mint single-use recovery codes for a lost device.

Correctness is pinned to the RFC 6238 Appendix B test vectors for SHA1, SHA256, and SHA512 in
`yatotp_test.go`.

## Key API

- `Config` — `{ Issuer, Algorithm, Digits, Period, Skew }`, all named types. Loadable via
  `config.LoadConfigStructFromEnv` (`..._ISSUER`, `..._ALGORITHM`, `..._DIGITS`, `..._PERIOD`,
  `..._SKEW`). Defaults: `SHA1`, 6 digits, 30s period, skew 1.
- `(*Config) Validate() yaerrors.Error` — cascades every field; call at startup.
- `NewAuthenticator(config *Config) *Authenticator` — fills zero fields with defaults; a nil config is
  tolerated. Does not validate.
- `(*Authenticator) Counter(at time.Time) Counter` — the RFC 6238 time step covering `at`; clamps to 0
  before the epoch.
- `(*Authenticator) Generate(secret Secret, at time.Time) (Code, yaerrors.Error)`
- `(*Authenticator) GenerateAt(secret Secret, counter Counter) (Code, yaerrors.Error)`
- `(*Authenticator) Validate(secret, code, at) (matched Counter, verified bool, err yaerrors.Error)`
- `(*Authenticator) ValidateAfter(secret, code, at, lastAccepted Counter) (Counter, bool, yaerrors.Error)`
- `(*Authenticator) ProvisioningURI(secret Secret, account AccountName) (ProvisioningURI, yaerrors.Error)`
- `NewSecret() (Secret, yaerrors.Error)` / `NewSecretWithSize(size int)` — `crypto/rand`, unpadded
  base32, 20 bytes by default (RFC 4226's recommendation), 16-byte floor.
- `NewRecoveryCodes(count RecoveryCodeCount) ([]RecoveryCode, yaerrors.Error)`,
  `(RecoveryCode) Matches(other RecoveryCode) bool` — constant-time.
- `Secret`, `Code`, `RecoveryCode`, `ProvisioningURI` all `LogString()` → `"[REDACTED]"`.
- `const AlgorithmSHA1|SHA256|SHA512`, `DefaultPeriod`, `DefaultDigits`, `DefaultSkew`,
  `DefaultSecretBytes`, `DefaultRecoveryCodeCount`, `MinDigits`, `MaxDigits`, `MaxSkew`.
- Fx: `Module` (`fx.go`) provides `*Authenticator` from a supplied `*Config`.

## Security Notes

- **Use `ValidateAfter`, not `Validate`.** A TOTP code stays valid for its whole time step, so an
  observed code is replayable inside that window; RFC 6238 requires accepting a time step only once.
  `ValidateAfter` refuses any step not newer than the last accepted one. Persist the returned `Counter`
  per user (a `LastTOTPCounter` column) and feed it back. `Validate` exists for callers tracking replay
  themselves and hands back the matched `Counter` to make that possible.
- Code comparison is `crypto/subtle.ConstantTimeCompare`, and the skew loop does not early-exit, so
  neither the value nor the matching window leaks through timing.
- `Skew` is capped at `MaxSkew` (10): a wide skew directly lengthens the window a stolen code stays
  usable in, so a config typo cannot open it arbitrarily.
- SHA1 here is HMAC-SHA1 as RFC 6238 mandates for authenticator-app interop — not a collision-sensitive
  use. The `crypto/sha1` import carries an explanatory `//nolint:gosec`. Switching to SHA256/SHA512
  narrows which apps can enrol.
- Secrets are credentials equal to the factor itself: store encrypted at rest, never log, never
  re-display after enrolment. `ProvisioningURI` embeds the secret, so serve it once, over TLS, to the
  authenticated owner only.
- Recovery codes are returned in plaintext exactly once — store hashes, the way you store a password.
- Nothing here rate-limits code submission. A 6-digit code brute-forces in ~10^6 tries; pair the verify
  endpoint with `yaratelimit` and an attempt counter.

## Usage Notes

- Depends only on `yaerrors` and the standard library.
- Enrolment order: `NewSecret` → store encrypted → `ProvisioningURI` → render QR → require one
  successful `ValidateAfter` before marking 2FA active, so a user cannot lock themselves out with an
  unscanned secret.
- `Secret.Validate` normalizes case, spaces, and `=` padding before decoding, so a user pasting a
  formatted secret still works.
