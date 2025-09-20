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

type IRateLimit interface {
	Check(
		ctx context.Context,
		id uint64,
		group string,
	) (bool, yaerrors.Error)
	Refresh(
		ctx context.Context,
		id uint64,
		group string,
	) yaerrors.Error
	Increment(
		ctx context.Context,
		id uint64,
		group string,
	) yaerrors.Error
	Get(
		ctx context.Context,
		id uint64,
		group string,
	) (*Storage, yaerrors.Error)
}

type Storage struct {
	Limit        uint8
	FirstRequest int64
}

type RateLimit[Cache yacache.Container] struct {
	Cache yacache.Cache[Cache]
	Limit uint8
	Rate  time.Duration
}

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

func (r *RateLimit[Cache]) Check(
	ctx context.Context,
	id uint64,
	group string,
) (bool, yaerrors.Error) {
	storage, err := r.Get(ctx, id, group)
	if err != nil {
		return false, err.Wrap("failed to check storage")
	}

	if storage.Limit+1 >= r.Limit {
		return true, nil
	}

	return false, nil
}

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
	}

	if time.Now().Add(-r.Rate).Before(time.Unix(storage.FirstRequest, 0)) {
		if err := r.Cache.Set(
			ctx,
			formatKey(id, group),
			fmt.Sprintf("%d,%d", storage.Limit+1, storage.FirstRequest),
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

func (r *RateLimit[Cache]) Refresh(
	ctx context.Context,
	id uint64,
	group string,
) yaerrors.Error {
	if err := r.Cache.Set(ctx, formatKey(id, group), fmt.Sprintf("%d,%d", 1, time.Now().Unix()), 0); err != nil {
		return err.Wrap("failed to set refreshed storage")
	}

	return nil
}

func (r *RateLimit[Cache]) Get(
	ctx context.Context,
	id uint64,
	group string,
) (*Storage, yaerrors.Error) {
	value, yaerr := r.Cache.Get(ctx, formatKey(id, group))
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

func formatKey(id uint64, group string) string {
	return fmt.Sprintf("rate-limit-%d-%s", id, group)
}
