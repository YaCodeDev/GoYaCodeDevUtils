package yahash_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yahash"
	"github.com/stretchr/testify/assert"
)

var testDataForHash = []string{"yadatetestlolkek", "polliizz", "yanevlad_"}

func TestHash64_Deterministic(t *testing.T) {
	yadata := "yadata"

	hash1 := yahash.Hash64(yadata, testDataForHash...)
	hash2 := yahash.Hash64(yadata, testDataForHash...)

	assert.Equal(t, hash1, hash2, fmt.Sprintf("Hash64 not deterministic: got %d and %d", hash1, hash2))
}

func TestHash64WithTime_Deterministic(t *testing.T) {
	hash1 := yahash.Hash64WithTime(time.Now(), testDataForHash...)
	hash2 := yahash.Hash64WithTime(time.Now(), testDataForHash...)

	assert.Equal(t, hash1, hash2, fmt.Sprintf("Hash64WithTime not deterministic: got %d and %d", hash1, hash2))
}

func TestHash64_Matches_Hash64WithTime(t *testing.T) {
	hash64 := yahash.Hash64(time.Now().Format(time.DateOnly), testDataForHash...)
	hash64WithTime := yahash.Hash64WithTime(time.Now(), testDataForHash...)

	assert.Equal(t, hash64, hash64WithTime,
		fmt.Sprintf("Hash64 doesn't match to Hash64WithTime. hash64: %d, hash64WithTime: %d", hash64, hash64WithTime))
}

func TestValidateHash64ByDays_Today(t *testing.T) {
	t.Parallel()

	t.Run("Today", func(t *testing.T) {
		todayHash := yahash.Hash64(time.Now().Format(time.DateOnly), testDataForHash...)
		daysBack := 1

		assert.True(t, yahash.ValidateHash64ByDays(todayHash, daysBack, testDataForHash...),
			"Failed to validate correct hash")
	})

	t.Run("Yesterday", func(t *testing.T) {
		hashYesterday := yahash.Hash64WithTime(time.Now().AddDate(0, 0, -1), testDataForHash...)

		t.Run("True", func(t *testing.T) {
			daysBack := 1

			assert.True(t, yahash.ValidateHash64ByDays(hashYesterday, daysBack, testDataForHash...),
				"Got `False` by valid hash64")
		})

		t.Run("False", func(t *testing.T) {
			daysBack := 0

			assert.False(t, yahash.ValidateHash64ByDays(hashYesterday, daysBack, testDataForHash...),
				"Got `True` by invalid hash64")
		})
	})

	t.Run("Tomorrow Day", func(t *testing.T) {
		hash := yahash.Hash64WithTime(time.Now().AddDate(0, 0, 1), testDataForHash...)
		daysBack := 1

		assert.False(t, yahash.ValidateHash64ByDays(hash, daysBack, testDataForHash...),
			"Got `True` by invalid hash64 because tomorrow day")
	})

	t.Run("Invalid Date", func(t *testing.T) {
		hash := yahash.Hash64WithTime(time.Now().AddDate(0, 0, -3), testDataForHash...)

		assert.False(t, yahash.ValidateHash64ByDays(hash, 1, testDataForHash...),
			"Got `True` by invalid hash64 because old date")
	})
}
