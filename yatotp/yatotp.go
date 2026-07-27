// Package yatotp implements RFC 6238 time-based one-time passwords, the second
// factor every mainstream authenticator app speaks.
//
// It covers the whole enrolment-and-verification loop: minting a shared secret,
// handing the user an otpauth:// provisioning URI to scan, deriving codes, and
// verifying a submitted code with bounded clock skew and replay rejection. It
// also mints single-use recovery codes for a user who loses their device.
//
// # Quick start
//
//	cfg := yatotp.Config{Issuer: "Example"}
//
//	authenticator := yatotp.NewAuthenticator(&cfg)
//
//	secret, err := yatotp.NewSecret()
//	uri, err := authenticator.ProvisioningURI(secret, "user@example.com")
//	// render uri as a QR code, store secret encrypted against the user
//
//	counter, verified, err := authenticator.ValidateAfter(
//	    secret, submitted, time.Now(), user.LastTOTPCounter,
//	)
//	// persist counter as user.LastTOTPCounter when verified
//
// # Replay
//
// A TOTP code stays valid for its whole time step, so a code observed in transit
// can be replayed inside that window. RFC 6238 requires a verifier to accept a
// given time step only once. ValidateAfter enforces that: it refuses any code
// whose time step is not newer than the last one accepted for that user. Validate
// skips the check and returns the matched Counter instead, for a caller that
// tracks replay itself; prefer ValidateAfter.
//
// # Storage
//
// A shared secret is a credential equivalent to the second factor itself: store
// it encrypted at rest, never log it, and never re-display it after enrolment.
// Secret, Code, RecoveryCode, and ProvisioningURI all redact through LogString.
// Recovery codes are returned in plaintext once and must be stored hashed, the
// same way a password is.
package yatotp

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
)

// Authenticator derives and verifies TOTP codes under one Config.
type Authenticator struct {
	config Config
}

// NewAuthenticator builds an Authenticator from a Config, filling any zero field
// with its documented default. config is copied once at construction, so the
// caller may reuse or discard the pointer afterward.
//
// Construction does not validate: call Config.Validate at startup to surface a
// bad algorithm or digit count as a startup failure.
func NewAuthenticator(config *Config) *Authenticator {
	built := Authenticator{}

	if config != nil {
		built.config = *config
	}

	if built.config.Algorithm == "" {
		built.config.Algorithm = AlgorithmSHA1
	}

	if built.config.Digits == 0 {
		built.config.Digits = DefaultDigits
	}

	if built.config.Period == 0 {
		built.config.Period = DefaultPeriod
	}

	return &built
}

// Counter returns the RFC 6238 time step covering at.
func (a *Authenticator) Counter(at time.Time) Counter {
	seconds := at.UTC().Unix()
	if seconds < 0 {
		return 0
	}

	return Counter(uint64(seconds) / uint64(a.config.Period))
}

// Generate derives the code valid at the given instant.
func (a *Authenticator) Generate(secret Secret, at time.Time) (Code, yaerrors.Error) {
	return a.GenerateAt(secret, a.Counter(at))
}

// GenerateAt derives the code for an explicit time step. It is the building
// block Generate and Validate share, and is exported so a caller can precompute
// a code for a specific window in a test.
func (a *Authenticator) GenerateAt(secret Secret, counter Counter) (Code, yaerrors.Error) {
	key, err := secret.decode()
	if err != nil {
		return "", err.Wrap(logTag + " cannot generate")
	}

	if len(key) < minSecretBytes {
		return "", yaerrors.FromError(
			http.StatusBadRequest,
			ErrSecretTooShort,
			logTag+" cannot generate",
		)
	}

	block := make([]byte, hotpCounterBytes)
	binary.BigEndian.PutUint64(block, uint64(counter))

	mac := hmac.New(a.config.Algorithm.hasher(), key)
	mac.Write(block)

	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & hotpOffsetMask
	truncated := binary.BigEndian.Uint32(sum[offset:offset+4]) & hotpTruncationMask

	modulus := uint32(1)
	for range a.config.Digits {
		modulus *= decimalBase
	}

	digits := strconv.FormatUint(uint64(truncated%modulus), decimalBase)
	if padding := int(a.config.Digits) - len(digits); padding > 0 {
		digits = strings.Repeat("0", padding) + digits
	}

	return Code(digits), nil
}

// Validate reports whether code is valid at the given instant, tolerating
// Config.Skew time steps either side, and returns the time step it matched.
//
// The returned Counter must be persisted and fed back as ValidateAfter's
// lastAccepted, or the code stays replayable for the rest of its window. Prefer
// ValidateAfter, which does that check itself.
func (a *Authenticator) Validate(
	secret Secret,
	code Code,
	at time.Time,
) (matched Counter, verified bool, err yaerrors.Error) {
	if shapeErr := a.checkCodeShape(code); shapeErr != nil {
		return 0, false, shapeErr
	}

	current := a.Counter(at)
	skew := Counter(a.config.Skew)

	lowest := Counter(0)
	if current > skew {
		lowest = current - skew
	}

	for candidate := lowest; candidate <= current+skew; candidate++ {
		expected, generateErr := a.GenerateAt(secret, candidate)
		if generateErr != nil {
			return 0, false, generateErr.Wrap(logTag + " cannot validate")
		}

		if subtle.ConstantTimeCompare([]byte(expected), []byte(code)) == 1 {
			matched = candidate
			verified = true
		}
	}

	return matched, verified, nil
}

// ValidateAfter behaves like Validate but rejects any code whose time step is
// not newer than lastAccepted, so a code cannot be presented twice inside its
// own window. Pass a zero lastAccepted for a user who has not verified yet, and
// persist the returned Counter on success.
func (a *Authenticator) ValidateAfter(
	secret Secret,
	code Code,
	at time.Time,
	lastAccepted Counter,
) (matched Counter, verified bool, err yaerrors.Error) {
	matched, verified, err = a.Validate(secret, code, at)
	if err != nil || !verified {
		return matched, verified, err
	}

	if matched <= lastAccepted {
		return matched, false, nil
	}

	return matched, true, nil
}

// ProvisioningURI builds the otpauth:// URI an authenticator app scans to
// enrol. It embeds the shared secret and is therefore as sensitive as the secret
// itself: serve it once, over TLS, to the authenticated owner only.
func (a *Authenticator) ProvisioningURI(
	secret Secret,
	account AccountName,
) (ProvisioningURI, yaerrors.Error) {
	if err := secret.Validate(); err != nil {
		return "", err.Wrap(logTag + " cannot build provisioning uri")
	}

	if err := account.Validate(); err != nil {
		return "", err.Wrap(logTag + " cannot build provisioning uri")
	}

	if err := a.config.Issuer.Validate(); err != nil {
		return "", err.Wrap(logTag + " cannot build provisioning uri")
	}

	label := string(account)
	if a.config.Issuer != "" {
		label = string(a.config.Issuer) + ":" + string(account)
	}

	query := url.Values{}
	query.Set(queryFieldSecret, strings.TrimRight(strings.ToUpper(string(secret)), "="))
	query.Set(queryFieldAlgorithm, string(a.config.Algorithm))
	query.Set(queryFieldDigits, strconv.FormatUint(uint64(a.config.Digits), decimalBase))
	query.Set(queryFieldPeriod, strconv.FormatUint(uint64(a.config.Period), decimalBase))

	if a.config.Issuer != "" {
		query.Set(queryFieldIssuer, string(a.config.Issuer))
	}

	uri := url.URL{
		Scheme:   uriScheme,
		Host:     uriHost,
		Path:     "/" + label,
		RawQuery: query.Encode(),
	}

	return ProvisioningURI(uri.String()), nil
}

func (a *Authenticator) checkCodeShape(code Code) yaerrors.Error {
	if code == "" {
		return yaerrors.FromError(http.StatusBadRequest, ErrCodeRequired, logTag+" code")
	}

	if len(code) != int(a.config.Digits) {
		return yaerrors.FromError(http.StatusBadRequest, ErrCodeLength, logTag+" code")
	}

	for _, symbol := range code {
		if symbol < '0' || symbol > '9' {
			return yaerrors.FromError(http.StatusBadRequest, ErrCodeNotNumeric, logTag+" code")
		}
	}

	return nil
}

// NewSecret mints a DefaultSecretBytes shared secret from crypto/rand, encoded
// as unpadded base32.
func NewSecret() (Secret, yaerrors.Error) {
	return NewSecretWithSize(DefaultSecretBytes)
}

// NewSecretWithSize behaves like NewSecret with an explicit byte length. Sizes
// below the RFC 4226 minimum are rejected rather than silently accepted.
func NewSecretWithSize(size int) (Secret, yaerrors.Error) {
	if size < minSecretBytes {
		return "", yaerrors.FromError(
			http.StatusBadRequest,
			ErrSecretTooShort,
			logTag+" cannot mint secret",
		)
	}

	raw := make([]byte, size)
	if _, err := rand.Read(raw); err != nil {
		return "", yaerrors.FromError(
			http.StatusInternalServerError,
			errors.Join(err, ErrGenerateSecret),
			logTag+" cannot mint secret",
		)
	}

	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw)

	return Secret(encoded), nil
}

// NewRecoveryCodes mints count single-use recovery codes from crypto/rand, each
// an unpadded base32 string. They are returned in plaintext exactly once: show
// them to the user, store only hashes, and compare a submitted code with
// RecoveryCode.Matches against the plaintext you never keep.
func NewRecoveryCodes(count RecoveryCodeCount) ([]RecoveryCode, yaerrors.Error) {
	if count == 0 || int(count) > recoveryCodeMaxSize {
		return nil, yaerrors.FromError(
			http.StatusBadRequest,
			ErrRecoveryCodeCount,
			logTag+" cannot mint recovery codes",
		)
	}

	codes := make([]RecoveryCode, 0, count)

	for range count {
		raw := make([]byte, recoveryCodeBytes)
		if _, err := rand.Read(raw); err != nil {
			return nil, yaerrors.FromError(
				http.StatusInternalServerError,
				errors.Join(err, ErrGenerateRecovery),
				logTag+" cannot mint recovery codes",
			)
		}

		encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw)
		codes = append(codes, RecoveryCode(encoded))
	}

	return codes, nil
}

// Matches compares two recovery codes in constant time, so a mismatch reveals
// nothing about how many leading characters were right.
func (code RecoveryCode) Matches(other RecoveryCode) bool {
	return subtle.ConstantTimeCompare([]byte(code), []byte(other)) == 1
}
