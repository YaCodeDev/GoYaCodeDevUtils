package yagzip_test

import (
	"bytes"
	"math/rand"
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yagzip"
	"github.com/stretchr/testify/require"
)

func TestFlow_BasicCases(t *testing.T) {
	vectors := [][]byte{
		{},
		[]byte("a"),
		[]byte("Hello, RZK!"),
		bytes.Repeat([]byte("x"), 128),
		bytes.Repeat([]byte{0x00}, 1024),
		bytes.Repeat([]byte{0xEE, 0xFF, 0x00, 0x01}, 257),
	}

	gzip := yagzip.NewGzip()
	for i, in := range vectors {
		z, err := gzip.Zip(in)
		require.NoErrorf(t, err, "case %d: Zip failed", i)

		out, err := gzip.Unzip(z)
		require.NoErrorf(t, err, "case %d: Unzip failed", i)

		require.Equalf(t, in, out, "case %d: mismatch", i)
	}
}

func TestFlow_LargeCase(t *testing.T) {
	sizes := []int{1 << 10, 64 << 10, 256 << 10}
	rng := rand.New(rand.NewSource(42))

	for _, n := range sizes {
		in := make([]byte, n)
		_, err := rng.Read(in)
		require.NoErrorf(t, err, "n=%d: rng read failed", n)

		gzip := yagzip.NewGzip()
		z, err := gzip.Zip(in)
		require.NoErrorf(t, err, "n=%d: Zip failed", n)

		out, err := gzip.Unzip(z)
		require.NoErrorf(t, err, "n=%d: Unzip failed", n)

		require.Equalf(t, in, out, "n=%d: mismatch after round-trip", n)
	}
}
