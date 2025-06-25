// Package yahash provides a small, generic helper around hashing functions that
// makes it trivial to combine arbitrary user‑supplied data with a secret (salt)
// **and** a rolling time component.
//
// ## Typical use‑case
//
//   - Build short‑lived tokens or "request signatures" that must be recomputed on
//     the server side and validated within an allowed time window.
//
//   - Generate cache‑keys that expire automatically when the configured
//     `interval` elapses.
//
//   - Quickly protect a webhook or URL with a deterministic but time‑bounded
//     hash without the overhead of full‑blown JWT or HMAC libraries.
//
// The API is intentionally minimal: you bring **any** hashing algorithm (as a
// `HashFunc`) and the helper takes care of salting it with the secret and with
// a truncated Unix‑timestamp.
//
// # Example (basic, secret‑only)
//
// The simplest scenario hashes an arbitrary string together with a secret:
//
//	package main
//
//	import (
//	    "fmt"
//	    "time"
//
//	    "github.com/YaCodeDev/GoYaCodeDevUtils/yahash"
//	)
//
//	func main() {
//	    hasher := yahash.NewHash[yahash.HashableType, int64](
//	        yahash.FNVStringToInt64,
//	        "my‑super‑secret", // salt
//	        time.Minute,       // irrelevant here, no time component
//	        0,                 // no backwards validation window
//	    )
//
//	    h := hasher.Hash("payload")
//	    fmt.Println(h)
//	}
//
// # Example (time‑bound validation)
//
//	secret := "yanesupertestsecret"
//	data   := []string{"yadatetestlolkek", "polliizz", "yanevlad_"}
//
//	// Create a token valid for five one‑hour periods back.
//	hasher := yahash.NewHash(yahash.FNVStringToInt64, secret, time.Hour, 5)
//
//	// The client computes a hash for "now".
//	expected := hasher.HashWithTime(time.Now(), data...)
//
//	// The server receives `expected` and validates it – it will compare against
//	// the current period and the previous five.
//	if ok := hasher.Validate(expected, data...); !ok {
//	    // reject request
//	}
//
// ----------------------------------------------------------------------------------
package yahash

import (
	"hash/fnv"
	"strconv"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/valueparser"
)

// HashableType describes the set of types that can be parsed from / converted to
// a string by the *valueparser* package **and** that you intend to feed into the
// supplied hashing function. In practice it will usually resolve to `string`,
// `int64`, or another scalar type supported by your parser.
type HashableType valueparser.ParsableType

// HashFunc is the signature every hashing algorithm must satisfy in order to be
// used with `Hash`.
//
//   - *I* – the input type (usually `string`).
//   - *O* – the output type **must** be `comparable` so that we can check equality
//     when validating.
//
// A hash function receives the main *data* plus zero or more *args* that are
// already salted with the secret (see implementation) – so it can simply write
// them into its internal state.
//
// For example, `FNVStringToInt64` below fulfils this contract.
//
// # Example
//
//	custom := func(s string, args ...string) uint32 {
//	    h := fnv.New32()
//	    h.Write([]byte(s))
//	    for _, a := range args { h.Write([]byte(a)) }
//	    return h.Sum32()
//	}
//
//	_ = yahash.NewHash(custom, "secret", time.Second, 3)
//
// (note that `uint32` is *comparable*, so it is allowed).
type HashFunc[I HashableType, O comparable] func(data I, args ...I) O

// Hash bundles a hashing function together with:
//
//   - *secret*  – an extra argument automatically appended to every call so the
//     output cannot be reproduced without knowing it (simple salting).
//
//   - *interval* – the size of a time‑window;
//
//   - *back* – how many *previous* windows are accepted during validation
//
// In other words, the triple *interval / back / secret* defines the security
// model while `HashFunc` defines the actual mathematical transform.
//
// The zero value is **not** usable; always construct via `NewHash`.
type Hash[I HashableType, O comparable] struct {
	hash     HashFunc[I, O]
	interval time.Duration // ≥ 1s after constructor check
	secret   I
	back     int
}

// NewHash returns an initialised Hash helper.
//
//	• `interval` shorter than one second is automatically promoted to exactly one
//	  second – sub-second windows rarely make sense and can break when system
//	  clocks are not precise.
//
//	• `back` accepts *N* previous windows for validation
//
// # Panics
//
// The function does **not** panic; all parameters are sanitised.
//
// # Example
//
//	hasher := yahash.NewHash(yahash.FNVStringToInt64, "secret", time.Minute, 5)
//	hash := hasher.HashWithTime(time.Now())
//	if !hasher.Validate(hash) { /* reject */ }
func NewHash[I HashableType, O comparable](
	hash HashFunc[I, O],
	secret I,
	interval time.Duration,
	back int,
) Hash[I, O] {
	if interval < time.Second {
		interval = time.Second
	}

	return Hash[I, O]{
		hash:     hash,
		secret:   secret,
		interval: interval,
		back:     back,
	}
}

// Hash hashes *data* together with optional extra arguments **and** the secret.
//
// The secret is always appended as the last argument so that callers do not have
// to remember to pass it explicitly – a common pitfall when the hashing happens
// in several places.
//
// # Example
//
//	hash := hasher.Hash("yadata", "ya_args1", "ya_args2")
func (h *Hash[I, O]) Hash(data I, args ...I) O {
	return h.hash(data, append(args, h.secret)...)
}

// HashWithTime is identical to `Hash` but replaces *data* with a
// Unix‑timestamp.  This effectively rolls the secret every
// *interval* and makes tokens time‑bound.
//
// # Example
//
//	// within request handler:
//	hash := hasher.HashWithTime(time.Now(), userID)
func (h *Hash[I, O]) HashWithTime(inputTime time.Time, args ...I) O {
	parsedTime, _ := valueparser.
		ParseValue[I](
		strconv.FormatInt(inputTime.Unix()/int64(h.interval/time.Second), 10)) // SAFETY: This cannot return error

	return h.hash(parsedTime, append(args, h.secret)...)
}

// ValidateWithoutTime recomputes a hash **without** the time component and
// compares it to *expected*.
//
// This is useful when you only need salting (secret) but still want a unified
// API together with the time‑aware helpers.
//
// # Example
//
//	if !hasher.ValidateWithoutTime(expected, payload) {
//	    // tampered
//	}
func (h *Hash[I, O]) ValidateWithoutTime(expected O, data I, args ...I) bool {
	return h.Hash(data, args...) == expected
}

// Validate recomputes the hash for the **current** time‑window and for *back*
// previous ones (inclusive) and returns whether any of them match *expected*.
//
// # Example
//
//	expected := hasher.HashWithTime(time.Now().Add(-2*time.Hour), "ya_args")
//
//	if ok := hasher.Validate(expected, "ya_args"); !ok {
//	    // expired
//	}
func (h *Hash[I, O]) Validate(expected O, args ...I) bool {
	return h.ValidateCustomBack(expected, h.back, args...)
}

// ValidateCustomBack behaves like `Validate` but lets the caller specify a
// custom *back* window on a per‑call basis.
//
// This is handy when the acceptable drift is not known at construction time or
// when different endpoints require different policies.
func (h *Hash[I, O]) ValidateCustomBack(expected O, back int, args ...I) bool {
	for i := 0; i <= back; i++ {
		date := time.Now().Add(h.interval * -time.Duration(i))
		generated := h.HashWithTime(date, args...)

		if generated == expected {
			return true
		}
	}

	return false
}

// FNVStringToInt64 is a ready‑to‑use 64‑bit FNV‑1a implementation compatible
// with the `HashFunc` signature.
//
// It concatenates *data* and *args* (already salted) and returns the 64‑bit
// digest as a signed integer (`int64`).
//
// # Example
//
//	hasher := yahash.NewHash(yahash.FNVStringToInt64, "secret", time.Minute, 3)
//	code := hasher.HashWithTime(time.Now(), "ya_args")
//	fmt.Println(code)
func FNVStringToInt64(data string, args ...string) int64 {
	hasher := fnv.New64()
	hasher.Write([]byte(data))

	for _, arg := range args {
		hasher.Write([]byte(arg))
	}

	return int64(hasher.Sum64())
}
