package yahash

import (
	"hash/fnv"
	"strconv"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/valueparser"
)

type HashableType valueparser.ParsableType

type HashFunc[I HashableType, O comparable] func(data I, args ...I) O

type Hash[I HashableType, O comparable] struct {
	hash     HashFunc[I, O]
	interval time.Duration
	secret   I
	back     int
}

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

func (h *Hash[I, O]) Hash(data I, args ...I) O {
	return h.hash(data, append(args, h.secret)...)
}

func (h *Hash[I, O]) HashWithTime(inputTime time.Time, args ...I) O {
	parsedTime, _ := valueparser.
		ParseValue[I](
		strconv.FormatInt(inputTime.Unix()/int64(h.interval/time.Second), 10)) // SAFETY: This cannot return error

	return h.hash(parsedTime, append(args, h.secret)...)
}

func (h *Hash[I, O]) ValidateWithoutTime(expected O, data I, args ...I) bool {
	return h.Hash(data, args...) == expected
}

func (h *Hash[I, O]) Validate(expected O, args ...I) bool {
	for i := 0; i <= h.back; i++ {
		date := time.Now().Add(h.interval * -time.Duration(i))
		generated := h.HashWithTime(date, args...)

		if generated == expected {
			return true
		}
	}

	return false
}

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

func FNVStringToInt64(data string, args ...string) int64 {
	hasher := fnv.New64()
	hasher.Write([]byte(data))

	for _, arg := range args {
		hasher.Write([]byte(arg))
	}

	return int64(hasher.Sum64())
}
