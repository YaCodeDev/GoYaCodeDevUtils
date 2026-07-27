package yatotp

import (
	//nolint:gosec // RFC 6238 mandates HMAC-SHA1 for authenticator-app interop; HMAC does not rely on collision resistance
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base32"
	"hash"
	"net/http"
	"strings"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
)

// Secret is a user's base32-encoded TOTP shared secret, in the unpadded RFC 3548
// alphabet authenticator apps expect. It is a credential: store it encrypted at
// rest and never render it outside the enrolment step.
type Secret string

// LogString redacts Secret from log output.
func (Secret) LogString() string {
	return redactedValue
}

// Validate reports whether secret decodes as base32 and carries enough entropy.
func (secret Secret) Validate() yaerrors.Error {
	if secret == "" {
		return yaerrors.FromError(http.StatusBadRequest, ErrSecretRequired, logTag+" secret")
	}

	decoded, err := secret.decode()
	if err != nil {
		return err.Wrap(logTag + " secret")
	}

	if len(decoded) < minSecretBytes {
		return yaerrors.FromError(http.StatusBadRequest, ErrSecretTooShort, logTag+" secret")
	}

	return nil
}

func (secret Secret) decode() ([]byte, yaerrors.Error) {
	normalized := strings.ToUpper(strings.TrimSpace(string(secret)))
	normalized = strings.ReplaceAll(normalized, " ", "")
	normalized = strings.TrimRight(normalized, "=")

	decoded, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(normalized)
	if err != nil {
		return nil, yaerrors.FromError(http.StatusBadRequest, ErrSecretInvalid, logTag+" secret")
	}

	return decoded, nil
}

// Code is a user-supplied one-time code. It is a short-lived credential, so it
// redacts from logs like the secret it is derived from.
type Code string

// LogString redacts Code from log output.
func (Code) LogString() string {
	return redactedValue
}

// RecoveryCode is a single-use fallback credential for a user who lost their
// authenticator. Store only a hash of it, never the value itself.
type RecoveryCode string

// LogString redacts RecoveryCode from log output.
func (RecoveryCode) LogString() string {
	return redactedValue
}

// RecoveryCodeCount is how many recovery codes NewRecoveryCodes mints.
type RecoveryCodeCount uint8

// Counter is the RFC 6238 time step a code was derived from. A verifier persists
// the last accepted Counter per user and refuses to accept it twice, so a code
// observed in transit cannot be replayed inside its own time window.
type Counter uint64

// Algorithm names the HMAC hash backing code derivation.
//
// AlgorithmSHA1 is the default because it is what authenticator apps implement.
// RFC 6238 uses SHA-1 inside HMAC, where its collision weakness does not apply;
// this is not a signature or a content hash.
type Algorithm string

// Validate reports whether algorithm is one this package implements.
func (algorithm Algorithm) Validate() yaerrors.Error {
	switch algorithm {
	case AlgorithmSHA1, AlgorithmSHA256, AlgorithmSHA512:
		return nil
	default:
		return yaerrors.FromError(
			http.StatusBadRequest,
			ErrConfigAlgorithm,
			logTag+" config",
		)
	}
}

func (algorithm Algorithm) hasher() func() hash.Hash {
	switch algorithm {
	case AlgorithmSHA256:
		return sha256.New
	case AlgorithmSHA512:
		return sha512.New
	case AlgorithmSHA1:
		return sha1.New
	default:
		return sha1.New
	}
}

// Digits is how many decimal digits a generated code carries.
type Digits uint8

// Validate reports whether digits is within the range authenticator apps accept.
func (digits Digits) Validate() yaerrors.Error {
	if digits < MinDigits || digits > MaxDigits {
		return yaerrors.FromError(http.StatusBadRequest, ErrConfigDigits, logTag+" config")
	}

	return nil
}

// Period is the RFC 6238 time step, in seconds.
type Period uint32

// Validate reports whether period is non-zero.
func (period Period) Validate() yaerrors.Error {
	if period == 0 {
		return yaerrors.FromError(http.StatusBadRequest, ErrConfigPeriod, logTag+" config")
	}

	return nil
}

// Skew is how many extra time steps either side of the current one Validate
// accepts, absorbing clock drift between a user's device and the server.
type Skew uint8

// Validate reports whether skew stays inside MaxSkew.
func (skew Skew) Validate() yaerrors.Error {
	if skew > MaxSkew {
		return yaerrors.FromError(http.StatusBadRequest, ErrConfigSkew, logTag+" config")
	}

	return nil
}

// Issuer is the service name an authenticator app shows beside an account.
type Issuer string

// Validate reports whether issuer is free of the colon otpauth URIs use as a
// label separator.
func (issuer Issuer) Validate() yaerrors.Error {
	if strings.Contains(string(issuer), ":") {
		return yaerrors.FromError(
			http.StatusBadRequest,
			ErrConfigIssuerInvalid,
			logTag+" config",
		)
	}

	return nil
}

// AccountName identifies the enrolled user inside an authenticator app, usually
// their email address.
type AccountName string

// Validate reports whether the account name is present and free of the colon
// otpauth URIs use as a label separator.
func (account AccountName) Validate() yaerrors.Error {
	if account == "" {
		return yaerrors.FromError(
			http.StatusBadRequest,
			ErrAccountNameRequired,
			logTag+" account name",
		)
	}

	if strings.Contains(string(account), ":") {
		return yaerrors.FromError(
			http.StatusBadRequest,
			ErrAccountNameInvalid,
			logTag+" account name",
		)
	}

	return nil
}

// ProvisioningURI is an otpauth:// enrolment URI, normally rendered as a QR
// code. It embeds the shared secret, so it is as sensitive as the secret itself.
type ProvisioningURI string

// LogString redacts ProvisioningURI from log output.
func (ProvisioningURI) LogString() string {
	return redactedValue
}

// Config configures an Authenticator. It is loadable via
// config.LoadConfigStructFromEnv (TOTP_ISSUER, TOTP_ALGORITHM, TOTP_DIGITS,
// TOTP_PERIOD, TOTP_SKEW under whatever key path the caller nests it at).
//
// The defaults are the interoperable ones: SHA1, six digits, a thirty-second
// period, and one step of skew. Changing algorithm or digits narrows which
// authenticator apps can enrol.
type Config struct {
	Issuer    Issuer    `default:""`
	Algorithm Algorithm `default:"SHA1"`
	Digits    Digits    `default:"6"`
	Period    Period    `default:"30"`
	Skew      Skew      `default:"1"`
}

// Validate cascades validation across every field.
func (config *Config) Validate() yaerrors.Error {
	if err := config.Issuer.Validate(); err != nil {
		return err.Wrap(logTag + " invalid config")
	}

	if err := config.Algorithm.Validate(); err != nil {
		return err.Wrap(logTag + " invalid config")
	}

	if err := config.Digits.Validate(); err != nil {
		return err.Wrap(logTag + " invalid config")
	}

	if err := config.Period.Validate(); err != nil {
		return err.Wrap(logTag + " invalid config")
	}

	if err := config.Skew.Validate(); err != nil {
		return err.Wrap(logTag + " invalid config")
	}

	return nil
}
