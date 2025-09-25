package yagzip_test

import (
	"bytes"
	"math/rand"
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yagzip"
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

	for i, in := range vectors {
		z, err := yagzip.Zip(in)
		if err != nil {
			t.Fatalf("case %d: Zip failed: %v", i, err)
		}

		out, err := yagzip.Unzip(z)
		if err != nil {
			t.Fatalf("case %d: Unzip failed: %v", i, err)
		}

		if !bytes.Equal(in, out) {
			t.Fatalf("case %d: mismatch\nin:  %v\nout: %v", i, in, out)
		}
	}
}

func TestFlow_LargeCase(t *testing.T) {
	sizes := []int{1 << 10, 64 << 10, 256 << 10}
	rng := rand.New(rand.NewSource(42))

	for _, n := range sizes {
		in := make([]byte, n)
		if _, err := rng.Read(in); err != nil {
			t.Fatalf("rng read failed: %v", err)
		}

		z, err := yagzip.Zip(in)
		if err != nil {
			t.Fatalf("n=%d: Zip failed: %v", n, err)
		}

		out, err := yagzip.Unzip(z)
		if err != nil {
			t.Fatalf("n=%d: Unzip failed: %v", n, err)
		}

		if !bytes.Equal(in, out) {
			t.Fatalf("n=%d: mismatch after round-trip", n)
		}
	}
}

func TestUnzip_InvalidInput(t *testing.T) {
	bad := []byte("not-a-gzip-stream")

	if _, err := yagzip.Unzip(bad); err == nil {
		t.Fatalf("expected error for invalid gzip input, got nil")
	}
}
