package yaringbuffer_test

import (
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaringbuffer"
)

const benchEntries = 16

func BenchmarkNext(b *testing.B) {
	ring := yaringbuffer.New[int, int](benchEntries)
	for i := range benchEntries {
		ring.Upsert(i, i)
	}

	b.ReportAllocs()

	for b.Loop() {
		if _, found := ring.Next(); !found {
			b.Fatal("empty ring")
		}
	}
}

func BenchmarkNextMatch(b *testing.B) {
	ring := yaringbuffer.New[int, int](benchEntries)
	for i := range benchEntries {
		ring.Upsert(i, i)
	}

	b.ReportAllocs()

	for b.Loop() {
		if _, found := ring.NextMatch(func(v int) bool { return v%2 == 0 }); !found {
			b.Fatal("no match")
		}
	}
}

func BenchmarkUpsertRemoveChurn(b *testing.B) {
	ring := yaringbuffer.New[int, int](benchEntries)
	for i := range benchEntries {
		ring.Upsert(i, i)
	}

	b.ReportAllocs()

	for i := 0; b.Loop(); i++ {
		key := benchEntries + i
		ring.Upsert(key, key)
		ring.Remove(key)
	}
}
