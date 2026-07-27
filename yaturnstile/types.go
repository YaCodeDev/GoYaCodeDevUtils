package yaturnstile

import (
	"errors"
	"net/http"
	"net/url"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
)

// SecretKey is the server-side Turnstile secret a Verifier authenticates to
// siteverify with. It is required unless Config.Disabled turns verification off.
type SecretKey string

// LogString redacts SecretKey from log output.
func (SecretKey) LogString() string {
	return redactedValue
}

// Configured reports whether a secret key was supplied at all.
func (key SecretKey) Configured() bool {
	return key != ""
}

// Endpoint is the siteverify URL a Verifier posts to. It exists as a field so a
// test can point a Verifier at an httptest server, and so a deployment behind an
// egress proxy can redirect the call.
type Endpoint string

// Validate reports whether endpoint is a non-empty absolute URL.
func (endpoint Endpoint) Validate() yaerrors.Error {
	if endpoint == "" {
		return yaerrors.FromError(
			http.StatusBadRequest,
			ErrConfigEndpointRequired,
			logTag+" config",
		)
	}

	parsed, err := url.Parse(string(endpoint))
	if err != nil {
		return yaerrors.FromError(
			http.StatusBadRequest,
			errors.Join(err, ErrConfigEndpointInvalid),
			logTag+" config",
		)
	}

	if parsed.Scheme == "" || parsed.Host == "" {
		return yaerrors.FromError(
			http.StatusBadRequest,
			ErrConfigEndpointInvalid,
			logTag+" config",
		)
	}

	return nil
}

// Disabled turns verification off, so Verify passes every token without calling
// Cloudflare. It exists so local runs, CI, and tests need no Cloudflare
// credentials, and it must be set deliberately: a blank SecretKey alone does not
// disable verification, it fails Config.Validate.
type Disabled bool

// Token is the single-use widget response a client submits for verification.
type Token string

// LogString redacts Token from log output.
func (Token) LogString() string {
	return redactedValue
}

// RemoteIP is the client address forwarded to siteverify as an extra check. It
// is optional: an empty value omits the field from the request.
//
// Cloudflare compares it against the address the token was minted from, so a
// caller deriving it from a proxy header must trust that header — an
// attacker-controlled value weakens the check rather than strengthening it.
type RemoteIP string

// Config configures a Verifier. It is loadable via
// config.LoadConfigStructFromEnv (TURNSTILE_SECRET_KEY, TURNSTILE_ENDPOINT,
// TURNSTILE_DISABLED under whatever key path the caller nests it at).
//
// Verification fails closed: a Config that names no SecretKey and does not set
// Disabled is rejected by Validate, and a Verifier built from one refuses every
// token instead of silently letting traffic through.
type Config struct {
	SecretKey SecretKey `default:""`
	Endpoint  Endpoint  `default:"https://challenges.cloudflare.com/turnstile/v0/siteverify"`
	Disabled  Disabled  `default:"false"`
}

// Validate checks the endpoint, and requires a secret key unless verification is
// explicitly disabled.
func (config *Config) Validate() yaerrors.Error {
	if err := config.Endpoint.Validate(); err != nil {
		return err.Wrap(logTag + " invalid config")
	}

	if config.Disabled {
		return nil
	}

	if !config.SecretKey.Configured() {
		return yaerrors.FromError(
			http.StatusBadRequest,
			ErrConfigSecretKeyRequired,
			logTag+" invalid config",
		)
	}

	return nil
}

// siteverifyResponse is Cloudflare's documented siteverify answer. Only the
// fields this package acts on are decoded.
//
//nolint:tagliatelle // fields mirror Cloudflare's own kebab-case wire format, not this org's convention
type siteverifyResponse struct {
	Success    bool     `json:"success"`
	ErrorCodes []string `json:"error-codes"`
}
