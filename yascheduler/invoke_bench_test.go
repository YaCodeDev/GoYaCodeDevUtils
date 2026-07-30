package yascheduler

import (
	"context"
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaencoding"
)

type benchArgs struct {
	A int
	B int
}

type benchResult struct {
	Sum int
}

func BenchmarkPreparedInvoker(b *testing.B) {
	invoke := prepareInvoker(
		func(_ context.Context, args benchArgs) (benchResult, error) {
			return benchResult{Sum: args.A + args.B}, nil
		},
	)

	raw, err := yaencoding.EncodeMessagePack(benchArgs{A: 2, B: 3})
	if err != nil {
		b.Fatal(err)
	}

	ctx := context.Background()

	b.ReportAllocs()

	for b.Loop() {
		if _, wireErr := invoke(ctx, raw); wireErr != nil {
			b.Fatal(wireErr.Message)
		}
	}
}

func BenchmarkRegistryLookup(b *testing.B) {
	registry := NewRegistry()

	if err := RegisterFunction(
		registry,
		"bench",
		"v1",
		func(_ context.Context, args benchArgs) (benchResult, error) {
			return benchResult{Sum: args.A + args.B}, nil
		},
	); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()

	for b.Loop() {
		if _, found := registry.lookup("bench", "v1"); !found {
			b.Fatal("missing")
		}
	}
}
