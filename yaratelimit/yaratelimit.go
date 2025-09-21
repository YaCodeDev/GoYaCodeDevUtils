// Package yaratelimit implements a simple fixed-window rate limiter backed by
// a yacache.Cache. It stores a per-(id, group) counter alongside the unix
// timestamp of the first request in the current window.
//
// # Storage layout
//
// Each subject is addressed by a string key:
//
//	rate-limit:<id>-<group>
//
// The cache value is a compact CSV tuple:
//
//	"<count>,<first_unix_sec>"
//
// For example: "3,1726860000" means 3 requests since unix time 1726860000.
//
// # Model
//
// The limiter uses a fixed window of size Rate (time.Duration). The value
// tracks the number of hits and the unix timestamp of the first hit within
// the active window. On each Increment:
//
//   - If no record exists: Refresh() creates one with count=1, first=now.
//   - If now - first < Rate: count++ (up to Limit).
//   - If now - first >= Rate: Refresh() starts a new window with count=1.
//
// # Semantics
//
//   - Increment(ctx, id, group) -> (banned bool, err)
//
//     Increments the counter if inside the current window or refreshes the
//     window if it has expired. Returns true if the subject should be treated
//     as banned/over limit *after* this call (i.e., when the count reaches or
//     exceeds Limit).
//
//   - CheckBanned(ctx, id, group) -> (banned bool, err)
//
//     Reads the current value and returns whether the next hit would be over
//     the limit. (Useful to check prior to serving a request.)
//
//   - Refresh(ctx, id, group)
//
//     Resets the window (count=1, first=now).
//
//   - Get(ctx, id, group)
//
//     Returns the parsed Storage (count/limit used in this window and first
//     request unix timestamp).
package yaratelimit

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yacache"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
)

// IRateLimit exposes the behaviour of a fixed-window rate limiter backed by a cache.
//
// Example:
//
//	cache := yacache.NewCache(yacache.NewMemoryContainer())
//	rl := yaratelimit.NewRateLimit(cache, 5, time.Minute)
//	banned, err := rl.Increment(ctx, 42, "signin")
//	_ = banned; _ = err
type IRateLimit interface {
	// CheckBanned inspects the current window for (id, group) and returns true
	// if the *next* request should be treated as banned (i.e., would reach/exceed Limit).
	Check(
		ctx context.Context,
		id uint64,
		group string,
	) (bool, yaerrors.Error)

	// Refresh resets the window for (id, group) to count=1 at current timestamp.
	Refresh(
		ctx context.Context,
		id uint64,
		group string,
	) yaerrors.Error

	// Increment applies a hit inside the current window or refreshes if expired.
	// It returns true if the subject should be treated as banned after this hit.
	Increment(
		ctx context.Context,
		id uint64,
		group string,
	) yaerrors.Error

	// Get returns the current storage tuple for (id, group).
	Get(
		ctx context.Context,
		id uint64,
		group string,
	) (*Storage, yaerrors.Error)
}

// Storage is the parsed representation of the CSV value in the cache.
type Storage struct {
	// Limit is the count within the current window (despite the name, it stores the current usage).
	Limit uint8
	// FirstRequest is the unix timestamp of the first request within the current window.
	FirstRequest int64
}

// RateLimit is a fixed-window limiter backed by a yacache.Cache.
// The zero value is not valid; use NewRateLimit.
type RateLimit[Cache yacache.Container] struct {
	Cache yacache.Cache[Cache]
	// Limit is the max allowed hits per window.
	Limit uint8
	// Rate is the window size (duration).
	Rate time.Duration
}

// NewRateLimit wires dependencies and returns a ready-to-use limiter.
//
//   - cache: any yacache implementation (memory, redis, etc.)
//   - limit: maximum hits per window
//   - rate : window duration
//
// Example:
//
//	rl := yaratelimit.NewRateLimit(cache, 5, time.Minute)
func NewRateLimit[Cache yacache.Container](
	cache yacache.Cache[Cache],
	limit uint8,
	rate time.Duration,
) *RateLimit[Cache] {
	return &RateLimit[Cache]{
		Limit: limit,
		Rate:  rate,
		Cache: cache,
	}
}

// CheckBanned inspects the current window and returns true if the next call to
// Increment should be considered banned. Returns false if the subject has not
// reached the threshold yet or if no storage exists.
//
// Example:
//
//	banned, err := rl.CheckBanned(ctx, userID, "signup")
//	if err != nil { /* handle */ }
//	if banned { /* throttle */ }
func (r *RateLimit[Cache]) CheckBanned(
	ctx context.Context,
	id uint64,
	group string,
) (bool, yaerrors.Error) {
	storage, err := r.Get(ctx, id, group)
	if err != nil {
		return false, err.Wrap("failed to check storage")
	}

	// If the next increment would cross the limit, treat as banned.
	if storage.Limit+1 >= r.Limit {
		return true, nil
	}

	return false, nil
}

// Increment records a hit for (id, group).
// If the window is still active, it increments the counter.
// If the window expired, it Refreshes the window (count=1).
// Returns true if the subject is now banned (count >= Limit).
//
// Example:
//
//	banned, err := rl.Increment(ctx, userID, "api:v1")
//	if banned { /* reject */ }
func (r *RateLimit[Cache]) Increment(
	ctx context.Context,
	id uint64,
	group string,
) (bool, yaerrors.Error) {
	storage, err := r.Get(ctx, id, group)
	if err != nil {
		if err := r.Refresh(ctx, id, group); err != nil {
			return false, err.Wrap("failed to refresh")
		}

		return false, nil
	}

	if storage.Limit >= r.Limit {
		return true, nil
	}

	if time.Now().Add(-r.Rate).Before(time.Unix(storage.FirstRequest, 0)) {
		if err := r.Cache.Set(
			ctx,
			FormatKey(id, group),
			FormatValue(storage.Limit+1, storage.FirstRequest),
			0,
		); err != nil {
			return false, err.Wrap("failed to increment storage")
		}
	} else {
		if err := r.Refresh(ctx, id, group); err != nil {
			return false, err.Wrap("failed to refresh")
		}

		return false, nil
	}

	if storage.Limit+1 >= r.Limit {
		return true, nil
	}

	return false, nil
}

// Refresh resets the window for (id, group) to count=1 at the current timestamp.
//
// Example:
//
//	_ = rl.Refresh(ctx, 42, "password_reset")
func (r *RateLimit[Cache]) Refresh(
	ctx context.Context,
	id uint64,
	group string,
) yaerrors.Error {
	if err := r.Cache.Set(ctx, FormatKey(id, group), fmt.Sprintf("%d,%d", 1, time.Now().Unix()), 0); err != nil {
		return err.Wrap("failed to set refreshed storage")
	}

	return nil
}

// Get fetches and parses the cache record for (id, group).
//
// Example:
//
//	st, err := rl.Get(ctx, 42, "sms")
//	if err != nil { /* handle */ }
//	fmt.Println(st.Limit, st.FirstRequest)
func (r *RateLimit[Cache]) Get(
	ctx context.Context,
	id uint64,
	group string,
) (*Storage, yaerrors.Error) {
	value, yaerr := r.Cache.Get(ctx, FormatKey(id, group))
	if yaerr != nil {
		return nil, yaerr.Wrap("failed to get storage")
	}

	const separate = 2

	values := strings.Split(value, ",")
	if len(values) != separate {
		return nil, yaerrors.FromString(
			http.StatusInternalServerError,
			"not compare storage",
		)
	}

	limit, err := strconv.ParseUint(values[0], 10, 8)
	if err != nil {
		return nil, yaerrors.FromError(
			http.StatusInternalServerError,
			err,
			"couldn't validate limit",
		)
	}

	firstRequest, err := strconv.ParseInt(values[1], 10, 64)
	if err != nil {
		return nil, yaerrors.FromError(
			http.StatusInternalServerError,
			err,
			"couldn't validate unix time",
		)
	}

	return &Storage{
		Limit:        uint8(limit),
		FirstRequest: firstRequest,
	}, nil
}

// FormatKey constructs the cache key for (id, group).
//
// Example:
//
//	k := yaratelimit.FormatKey(100, "signup") // "rate-limit-100-signup"
func FormatKey(id uint64, group string) string {
	return fmt.Sprintf("rate-limit-%d-%s", id, group)
}

// FormatValue serializes a (count, first_unix) tuple to cache string.
//
// Example:
//
//	v := yaratelimit.FormatValue(2, 1726860000) // "2,1726860000"
func FormatValue(limit uint8, firstRequest int64) string {
	return fmt.Sprintf("%d,%d", limit, firstRequest)
}
