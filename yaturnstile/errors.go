package yaturnstile

import "errors"

var (
	ErrConfigEndpointRequired  = errors.New("[TURNSTILE] config endpoint is required")
	ErrConfigEndpointInvalid   = errors.New("[TURNSTILE] config endpoint is not an absolute url")
	ErrConfigSecretKeyRequired = errors.New(
		"[TURNSTILE] config secret key is required unless verification is disabled",
	)
	ErrBuildRequest     = errors.New("[TURNSTILE] failed to build siteverify request")
	ErrDoRequest        = errors.New("[TURNSTILE] failed to send siteverify request")
	ErrDecodeResponse   = errors.New("[TURNSTILE] failed to decode siteverify response")
	ErrUnexpectedStatus = errors.New("[TURNSTILE] siteverify answered an unexpected status")
)
