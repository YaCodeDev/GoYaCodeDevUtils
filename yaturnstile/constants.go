package yaturnstile

import "time"

const (
	// DefaultEndpoint is Cloudflare's public Turnstile siteverify endpoint, and the
	// value Config carries unless a deployment overrides it.
	DefaultEndpoint Endpoint = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

	// DefaultRequestTimeout bounds a single siteverify round trip.
	DefaultRequestTimeout = 10 * time.Second

	// DefaultMaxResponseBytes caps how much of a siteverify answer is read, so a
	// compromised or misrouted endpoint cannot stream an unbounded body at the
	// decoder.
	DefaultMaxResponseBytes int64 = 1 << 16
)

const (
	logTag = "[TURNSTILE]"

	redactedValue = "[REDACTED]"

	formFieldSecret   = "secret"
	formFieldResponse = "response"
	formFieldRemoteIP = "remoteip"

	contentTypeHeader = "Content-Type"
	contentTypeForm   = "application/x-www-form-urlencoded"
)
