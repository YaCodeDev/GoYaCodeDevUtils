package memstore_test

import (
	"context"
	"testing"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/store"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/store/memstore"
)

const (
	benchExecutions    = 10000
	benchDueLimit      = 256
	benchBaseUnix      = 1700000000
	benchStepSeconds   = 1
	benchJobKeyLiteral = "bench-job"
)

func BenchmarkDueExecutions(b *testing.B) {
	ctx := context.Background()
	memStore := memstore.NewStore(memstore.Config{})
	base := time.Unix(benchBaseUnix, 0).UTC()

	memStore.SetClock(func() time.Time { return base })

	job, err := memStore.UpsertJob(ctx, &store.Job{
		ID:      jobUUID(benchJobKeyLiteral),
		Key:     benchJobKeyLiteral,
		Enabled: true,
	})
	if err != nil {
		b.Fatal(err)
	}

	for index := range benchExecutions {
		scheduledAt := base.Add(
			-time.Duration(index*benchStepSeconds) * time.Second,
		)
		if _, _, createErr := memStore.CreateExecution(
			ctx,
			job.ID,
			scheduledAt,
			store.StateScheduled,
			false,
		); createErr != nil {
			b.Fatal(createErr)
		}
	}

	b.ReportAllocs()

	for b.Loop() {
		due, dueErr := memStore.DueExecutions(ctx, base, benchDueLimit)
		if dueErr != nil {
			b.Fatal(dueErr)
		}

		if len(due) != benchDueLimit {
			b.Fatalf("due = %d, want %d", len(due), benchDueLimit)
		}
	}
}
