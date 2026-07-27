// Package yaturnstile verifies Cloudflare Turnstile tokens against the
// siteverify API.
//
// It is transport-agnostic: the Gin middleware that reads a token off a request
// and rejects it lives in yaginmiddleware, built on the Verifier defined here,
// so a non-HTTP caller (a socket handshake, a queue consumer) can verify a token
// the same way.
//
// Config is loadable via config.LoadConfigStructFromEnv and, once populated,
// checked with Config.Validate before it is handed to NewVerifier.
//
// # Quick start
//
//	cfg := yaturnstile.Config{
//	    SecretKey: "0x4AAA...",
//	    Endpoint:  yaturnstile.DefaultEndpoint,
//	}
//	if err := cfg.Validate(); err != nil {
//	    // handle error
//	}
//
//	verifier := yaturnstile.NewVerifier(&cfg, log)
//
//	verified, err := verifier.Verify(ctx, token, remoteIP, log)
//
// # Failing closed
//
// A false result and a non-nil error are different outcomes: false is a token
// Cloudflare rejected, an error is a siteverify call that could not be completed
// or understood. Both must deny the request — Verify never reports true on a
// path it could not actually check.
//
// Verification is skipped only when Config.Disabled is set deliberately, which
// keeps local runs, CI, and tests free of Cloudflare credentials. A Config that
// merely omits SecretKey is a misconfiguration: Config.Validate rejects it, and a
// Verifier built from one refuses every token rather than letting traffic
// through unchecked.
//
// # Scope
//
// Verify checks only the success flag siteverify returns. Cloudflare also echoes
// the hostname and action a token was minted for; a caller that pins those
// should read them from its own siteverify call rather than expect this package
// to enforce them.
package yaturnstile

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yalogger"
)

// Verifier checks a client-supplied Turnstile token against Cloudflare.
type Verifier interface {
	// Verify reports whether token is valid for remoteIP. A false result is a
	// rejected token, not a failure: err is non-nil only when siteverify could
	// not be reached or understood. Neither outcome may be treated as a pass.
	// Passing a nil log falls back to the logger the Verifier was constructed
	// with.
	Verify(
		ctx context.Context,
		token Token,
		remoteIP RemoteIP,
		log yalogger.Logger,
	) (verified bool, err yaerrors.Error)
}

type verifier struct {
	config Config
	client *http.Client
	log    yalogger.Logger
}

// NewVerifier builds a Verifier from a Config. config is copied once at
// construction, so the caller may reuse or discard the pointer afterward.
//
// A nil config, or one carrying neither a SecretKey nor Disabled, yields a
// Verifier that refuses every token: construction never turns a misconfiguration
// into an open door. Call Config.Validate at startup to surface that as a
// startup failure instead of a runtime rejection.
func NewVerifier(config *Config, log yalogger.Logger) Verifier {
	if log == nil {
		log = yalogger.NewBaseLogger(nil).NewLogger()
	}

	built := verifier{
		client: &http.Client{
			Timeout: DefaultRequestTimeout,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
			},
		},
		log: log,
	}

	if config != nil {
		built.config = *config
	}

	if built.config.Endpoint == "" {
		built.config.Endpoint = DefaultEndpoint
	}

	return &built
}

func (v *verifier) Verify(
	ctx context.Context,
	token Token,
	remoteIP RemoteIP,
	log yalogger.Logger,
) (verified bool, err yaerrors.Error) {
	if log == nil {
		log = v.log
	}

	if v.config.Disabled {
		log.Debug(logTag + " verification is disabled, passing token unchecked")

		return true, nil
	}

	if !v.config.SecretKey.Configured() {
		return false, yaerrors.FromError(
			http.StatusInternalServerError,
			ErrConfigSecretKeyRequired,
			logTag+" cannot verify",
		)
	}

	if token == "" {
		log.Warn(logTag + " request carried no turnstile token")

		return false, nil
	}

	request, err := v.newVerifyRequest(ctx, token, remoteIP)
	if err != nil {
		return false, err.Wrap(logTag + " cannot verify")
	}

	response, doErr := v.client.Do(request)
	if doErr != nil {
		return false, yaerrors.FromError(
			http.StatusBadGateway,
			errors.Join(doErr, ErrDoRequest),
			logTag+" cannot verify",
		)
	}

	defer func() {
		if closeErr := response.Body.Close(); closeErr != nil {
			log.Warnf("%s failed to close siteverify response body: %v", logTag, closeErr)
		}
	}()

	if response.StatusCode != http.StatusOK {
		return false, yaerrors.FromError(
			http.StatusBadGateway,
			ErrUnexpectedStatus,
			logTag+" cannot verify",
		)
	}

	var decoded siteverifyResponse

	bounded := io.LimitReader(response.Body, DefaultMaxResponseBytes)
	if decodeErr := json.NewDecoder(bounded).Decode(&decoded); decodeErr != nil {
		return false, yaerrors.FromError(
			http.StatusBadGateway,
			errors.Join(decodeErr, ErrDecodeResponse),
			logTag+" cannot verify",
		)
	}

	if !decoded.Success {
		log.Warnf("%s verification rejected: %s", logTag, strings.Join(decoded.ErrorCodes, ", "))
	}

	return decoded.Success, nil
}

func (v *verifier) newVerifyRequest(
	ctx context.Context,
	token Token,
	remoteIP RemoteIP,
) (request *http.Request, err yaerrors.Error) {
	form := url.Values{}
	form.Set(formFieldSecret, string(v.config.SecretKey))
	form.Set(formFieldResponse, string(token))

	if remoteIP != "" {
		form.Set(formFieldRemoteIP, string(remoteIP))
	}

	built, buildErr := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		string(v.config.Endpoint),
		strings.NewReader(form.Encode()),
	)
	if buildErr != nil {
		return nil, yaerrors.FromError(
			http.StatusInternalServerError,
			errors.Join(buildErr, ErrBuildRequest),
			logTag+" cannot verify",
		)
	}

	built.Header.Set(contentTypeHeader, contentTypeForm)

	return built, nil
}
