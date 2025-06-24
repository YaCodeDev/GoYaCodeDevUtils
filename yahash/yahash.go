package yahash

import (
	"hash/fnv"
	"time"
)

func Hash64WithTime(date time.Time, args ...string) int64 {
	return Hash64(date.Format(time.DateOnly), args...)
}

func Hash64(data string, args ...string) int64 {
	hasher := fnv.New64a()
	hasher.Write([]byte(data))

	for _, arg := range args {
		hasher.Write([]byte(arg))
	}

	return int64(hasher.Sum64())
}

func ValidateHash64ByDays(expectedHash int64, daysBack int, args ...string) bool {
	for i := 0; i <= daysBack; i++ {
		date := time.Now().AddDate(0, 0, -i)
		generatedHash := Hash64WithTime(date, args...)

		if generatedHash == expectedHash {
			return true
		}
	}

	return false
}
