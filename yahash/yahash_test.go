package yahash_test

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yahash"
	"github.com/stretchr/testify/assert"
)

var (
	testDataForHash = []string{"yadatetestlolkek", "polliizz", "yanevlad_"}
	secret          = "yanesupertestsecret"
	testHash        = yahash.NewHash(yahash.FNVStringToInt64, secret, time.Hour, 5)
)

func TestHash64_DeterministicWorks(t *testing.T) {
	data := "yadata"

	hash1 := testHash.Hash(data, testDataForHash...)
	hash2 := testHash.Hash(data, testDataForHash...)

	assert.Equal(
		t,
		hash1,
		hash2,
		fmt.Sprintf("Hash not deterministic: got %d and %d", hash1, hash2),
	)
}

func TestHash64WithTime_DeterministicWorks(t *testing.T) {
	now := time.Now()

	hash1 := testHash.HashWithTime(now, testDataForHash...)
	hash2 := testHash.HashWithTime(now, testDataForHash...)

	assert.Equal(
		t,
		hash1,
		hash2,
		fmt.Sprintf("Hash64WithTime not deterministic: got %d and %d", hash1, hash2),
	)
}

func TestHash_Matches_HashWithTime(t *testing.T) {
	now := time.Now()

	hash := testHash.Hash(
		strconv.FormatInt(now.Unix()/int64(time.Hour/time.Second), 10), testDataForHash...,
	)
	hashWithTime := testHash.HashWithTime(now, testDataForHash...)

	assert.Equal(
		t,
		hash,
		hashWithTime,
		fmt.Sprintf(
			"Hash64 doesn't match to Hash64WithTime. hash64: %d, hash64WithTime: %d",
			hash,
			hashWithTime,
		),
	)
}

func TestValidateHash_Works(t *testing.T) {
	t.Parallel()

	t.Run("[Validate] Works", func(t *testing.T) {
		hash := yahash.NewHash(yahash.FNVStringToInt64, secret, time.Second, 5)

		t.Run("True", func(t *testing.T) {
			expected := hash.HashWithTime(time.Now().Add(-time.Second*4), testDataForHash...)

			assert.True(t, hash.Validate(expected, testDataForHash...),
				"Got `True` by valid hash with correct date")
		})

		t.Run("False", func(t *testing.T) {
			expected := hash.HashWithTime(time.Now().Add(-time.Second*7), testDataForHash...)

			assert.False(t, hash.Validate(expected, testDataForHash...),
				"Got `True` by invalid hash with non correct date")
		})
	})

	t.Run("[ValidateWithoutTime] Works", func(t *testing.T) {
		data := "brizzinck"

		expected := testHash.Hash(data, testDataForHash...)

		t.Run("True", func(t *testing.T) {
			assert.True(t, testHash.ValidateWithoutTime(expected, data, testDataForHash...),
				"Got `False` by valid hash without time")
		})

		t.Run("False", func(t *testing.T) {
			assert.False(t, testHash.ValidateWithoutTime(expected, data+"s", testDataForHash...),
				"Got `True` by invalid hash without time")
		})
	})

	t.Run("[ValidateWithCustomBackStepCount]", func(t *testing.T) {
		t.Run("True", func(t *testing.T) {
			expected := testHash.HashWithTime(time.Now().Add(-time.Hour*6), testDataForHash...)

			assert.True(
				t,
				testHash.ValidateWithCustomBackStepCount(expected, 7, testDataForHash...),
				"Got `False` by valid hash with correct date",
			)
		})

		t.Run("False", func(t *testing.T) {
			expected := testHash.HashWithTime(time.Now().Add(-time.Hour*16), testDataForHash...)

			assert.False(
				t,
				testHash.ValidateWithCustomBackStepCount(expected, 10, testDataForHash...),
				"Got `True` by invalid hash with non correct date",
			)
		})
	})
}
