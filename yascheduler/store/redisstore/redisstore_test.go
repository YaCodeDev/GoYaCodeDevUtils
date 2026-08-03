package redisstore_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/store"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/store/memstore"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/store/redisstore"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/store/storetest"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

const (
	testExecutorType = protocol.ExecutorType("worker")
	testFunctionName = protocol.FunctionName("report")
	testInstanceID   = protocol.InstanceID("exec-1")
)

func newTestStore(t *testing.T, config redisstore.Config) (sut *redisstore.Store) {
	t.Helper()

	server := miniredis.RunT(t)

	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	return redisstore.NewStore(client, config)
}

func testJobUUID(seed store.JobKey) (id protocol.JobUUID) {
	seedLength := len(seed)
	if seedLength > len(id)-1 {
		seedLength = len(id) - 1
	}

	id[0] = byte(seedLength)
	copy(id[1:], seed[:seedLength])

	return id
}

func newTestJob(idSeed store.JobKey, key store.JobKey) (job *store.Job) {
	return &store.Job{
		ID:           testJobUUID(idSeed),
		Key:          key,
		ExecutorType: testExecutorType,
		Function:     protocol.FunctionSpec{Name: testFunctionName},
		Enabled:      true,
	}
}

func requireNoTestError(t *testing.T, err yaerrors.Error, intent string) {
	t.Helper()

	if err != nil {
		t.Fatalf("%s: %v", intent, err)
	}
}

func TestConformance(t *testing.T) {
	t.Parallel()

	storetest.TestStore(t, func(t *testing.T) store.Store {
		t.Helper()

		return newTestStore(t, redisstore.Config{})
	})
}

func TestConformanceResultCaps(t *testing.T) {
	t.Parallel()

	storetest.TestResultCaps(t, func(t *testing.T, caps storetest.Caps) store.Store {
		t.Helper()

		return newTestStore(t, redisstore.Config{
			MaxResults:            caps.MaxResults,
			MaxResultsPerInstance: caps.MaxResultsPerInstance,
		})
	})
}

func TestTimePrecision(t *testing.T) {
	t.Parallel()

	t.Run(
		"when an execution is created with a nanosecond instant / then the instant round-trips exactly",
		func(t *testing.T) {
			t.Parallel()

			const jobKey = store.JobKey("precision-create")

			scheduledAt := time.Date(2023, time.November, 14, 22, 13, 20, 123456789, time.UTC)

			sut := newTestStore(t, redisstore.Config{})

			job, err := sut.UpsertJob(context.Background(), newTestJob(jobKey, jobKey))
			requireNoTestError(t, err, "job creation should not fail")

			execution, fresh, err := sut.CreateExecution(
				context.Background(),
				job.ID,
				scheduledAt,
				store.StateScheduled,
				false,
			)
			requireNoTestError(t, err, "execution creation should not fail")

			if !fresh {
				t.Fatal("a fresh occurrence should create an execution")
			}

			fetched, err := sut.GetExecution(context.Background(), execution.ID)
			requireNoTestError(t, err, "execution fetch should not fail")

			if !fetched.ScheduledAt.Equal(scheduledAt) {
				t.Errorf(
					"the schedule instant should round-trip: got %v, want %v",
					fetched.ScheduledAt,
					scheduledAt,
				)
			}

			if fetched.ScheduledAt.UnixNano() != scheduledAt.UnixNano() {
				t.Errorf(
					"the schedule instant should keep nanoseconds: got %d, want %d",
					fetched.ScheduledAt.UnixNano(),
					scheduledAt.UnixNano(),
				)
			}
		},
	)

	t.Run(
		"when the next attempt time is updated with a nanosecond instant / then the instant round-trips exactly",
		func(t *testing.T) {
			t.Parallel()

			const jobKey = store.JobKey("precision-update")

			scheduledAt := time.Date(2023, time.November, 14, 22, 13, 20, 0, time.UTC)
			nextAttemptAt := time.Date(2023, time.November, 14, 22, 14, 21, 987654321, time.UTC)

			sut := newTestStore(t, redisstore.Config{})

			job, err := sut.UpsertJob(context.Background(), newTestJob(jobKey, jobKey))
			requireNoTestError(t, err, "job creation should not fail")

			execution, _, err := sut.CreateExecution(
				context.Background(),
				job.ID,
				scheduledAt,
				store.StateScheduled,
				false,
			)
			requireNoTestError(t, err, "execution creation should not fail")

			updated, err := sut.UpdateExecution(
				context.Background(),
				execution.ID,
				execution.Version,
				store.ExecutionUpdate{NextAttemptAt: &nextAttemptAt},
			)
			requireNoTestError(t, err, "execution update should not fail")

			if updated.NextAttemptAt.UnixNano() != nextAttemptAt.UnixNano() {
				t.Fatalf(
					"the updated instant should keep nanoseconds: got %d, want %d",
					updated.NextAttemptAt.UnixNano(),
					nextAttemptAt.UnixNano(),
				)
			}

			fetched, err := sut.GetExecution(context.Background(), execution.ID)
			requireNoTestError(t, err, "execution fetch should not fail")

			if fetched.NextAttemptAt.UnixNano() != nextAttemptAt.UnixNano() {
				t.Errorf(
					"the stored instant should keep nanoseconds: got %d, want %d",
					fetched.NextAttemptAt.UnixNano(),
					nextAttemptAt.UnixNano(),
				)
			}
		},
	)

	t.Run(
		"when the clock carries nanoseconds / then the job creation time round-trips exactly",
		func(t *testing.T) {
			t.Parallel()

			const jobKey = store.JobKey("precision-clock")

			clockAt := time.Date(2023, time.November, 14, 22, 13, 20, 111222333, time.UTC)

			sut := newTestStore(t, redisstore.Config{})
			sut.SetClock(func() time.Time { return clockAt })

			created, err := sut.UpsertJob(context.Background(), newTestJob(jobKey, jobKey))
			requireNoTestError(t, err, "job creation should not fail")

			if created.CreatedAt.UnixNano() != clockAt.UnixNano() {
				t.Fatalf(
					"the returned creation time should keep nanoseconds: got %d, want %d",
					created.CreatedAt.UnixNano(),
					clockAt.UnixNano(),
				)
			}

			fetched, err := sut.GetJob(context.Background(), created.ID)
			requireNoTestError(t, err, "job fetch should not fail")

			if fetched.CreatedAt.UnixNano() != clockAt.UnixNano() {
				t.Errorf(
					"the stored creation time should keep nanoseconds: got %d, want %d",
					fetched.CreatedAt.UnixNano(),
					clockAt.UnixNano(),
				)
			}
		},
	)

	t.Run(
		"when a fresh result is stored / then the last sent instant stays zero",
		func(t *testing.T) {
			t.Parallel()

			const resultSeed = store.JobKey("precision-result")

			sut := newTestStore(t, redisstore.Config{})

			stored, err := sut.StoreResult(context.Background(), &store.PendingResult{
				JobUUID:    testJobUUID(resultSeed),
				InstanceID: testInstanceID,
				Success:    true,
			})
			requireNoTestError(t, err, "result storage should not fail")

			if !stored {
				t.Fatal("a fresh result should be stored")
			}

			held, err := sut.ResultsForInstance(context.Background(), testInstanceID, 0)
			requireNoTestError(t, err, "instance result lookup should not fail")

			if len(held) != 1 {
				t.Fatalf("the instance should hold one result: got %d", len(held))
			}

			if !held[0].LastSentAt.IsZero() {
				t.Errorf(
					"an unsent result should keep the zero instant: got %v",
					held[0].LastSentAt,
				)
			}
		},
	)
}

func TestScopedKeyEncoding(t *testing.T) {
	t.Parallel()

	t.Run(
		"when executor type and key would join ambiguously / then each scoped job keeps its own identity",
		func(t *testing.T) {
			t.Parallel()

			const (
				firstIDSeed  = store.JobKey("hostile-first")
				secondIDSeed = store.JobKey("hostile-second")

				sharedJoin = "worker:alpha"
			)

			firstType := protocol.ExecutorType("worker")
			firstKey := store.JobKey("alpha:beta")
			secondType := protocol.ExecutorType(sharedJoin)
			secondKey := store.JobKey("beta")

			sut := newTestStore(t, redisstore.Config{})

			first := newTestJob(firstIDSeed, firstKey)
			first.ExecutorType = firstType

			second := newTestJob(secondIDSeed, secondKey)
			second.ExecutorType = secondType

			createdFirst, err := sut.UpsertJob(context.Background(), first)
			requireNoTestError(t, err, "the first hostile job should store")

			createdSecond, err := sut.UpsertJob(context.Background(), second)
			requireNoTestError(t, err, "the second hostile job should store")

			if createdFirst.ID == createdSecond.ID {
				t.Fatal("ambiguous scoped keys should address two distinct jobs")
			}

			fetchedFirst, err := sut.GetJobByKey(context.Background(), firstType, firstKey)
			requireNoTestError(t, err, "the first scoped lookup should not fail")

			fetchedSecond, err := sut.GetJobByKey(context.Background(), secondType, secondKey)
			requireNoTestError(t, err, "the second scoped lookup should not fail")

			if fetchedFirst.ID != createdFirst.ID || fetchedSecond.ID != createdSecond.ID {
				t.Errorf(
					"each scoped lookup should find its own job: got %s and %s",
					fetchedFirst.ID,
					fetchedSecond.ID,
				)
			}
		},
	)

	t.Run(
		"when one executor type extends another as a prefix / then deleting one leaves the other",
		func(t *testing.T) {
			t.Parallel()

			const (
				firstIDSeed  = store.JobKey("prefix-first")
				secondIDSeed = store.JobKey("prefix-second")
			)

			firstType := protocol.ExecutorType("worker")
			firstKey := store.JobKey("x")
			secondType := protocol.ExecutorType("workerx")
			secondKey := store.JobKey("")

			sut := newTestStore(t, redisstore.Config{})

			first := newTestJob(firstIDSeed, firstKey)
			first.ExecutorType = firstType

			second := newTestJob(secondIDSeed, secondKey)
			second.ExecutorType = secondType

			createdFirst, err := sut.UpsertJob(context.Background(), first)
			requireNoTestError(t, err, "the first prefix job should store")

			createdSecond, err := sut.UpsertJob(context.Background(), second)
			requireNoTestError(t, err, "the second prefix job should store")

			deleted, err := sut.DeleteJob(context.Background(), createdFirst.ID)
			requireNoTestError(t, err, "the first prefix job should delete")

			if !deleted {
				t.Fatal("the stored job delete should report true")
			}

			survivor, err := sut.GetJobByKey(context.Background(), secondType, secondKey)
			requireNoTestError(t, err, "the surviving scoped lookup should not fail")

			if survivor.ID != createdSecond.ID {
				t.Errorf(
					"the surviving job should keep its identity: got %s, want %s",
					survivor.ID,
					createdSecond.ID,
				)
			}

			_, missErr := sut.GetJobByKey(context.Background(), firstType, firstKey)
			if missErr == nil || !errors.Is(missErr, store.ErrJobNotFound) {
				t.Errorf("the deleted scoped key should miss: got %v", missErr)
			}
		},
	)
}

func TestUpdateExecutionContention(t *testing.T) {
	t.Parallel()

	t.Run(
		"when two updates race one version / then exactly one wins and the other conflicts",
		func(t *testing.T) {
			t.Parallel()

			const (
				jobKey = store.JobKey("contention")
				racers = 2
			)

			sut := newTestStore(t, redisstore.Config{})

			job, err := sut.UpsertJob(context.Background(), newTestJob(jobKey, jobKey))
			requireNoTestError(t, err, "job creation should not fail")

			scheduledAt := time.Date(2023, time.November, 14, 22, 13, 20, 0, time.UTC)

			execution, _, err := sut.CreateExecution(
				context.Background(),
				job.ID,
				scheduledAt,
				store.StateScheduled,
				false,
			)
			requireNoTestError(t, err, "execution creation should not fail")

			ready := store.StateReady
			start := make(chan struct{})
			outcomes := make(chan yaerrors.Error, racers)

			var group sync.WaitGroup

			for racer := 0; racer < racers; racer++ {
				group.Add(1)

				go func() {
					defer group.Done()
					<-start

					_, updateErr := sut.UpdateExecution(
						context.Background(),
						execution.ID,
						execution.Version,
						store.ExecutionUpdate{State: &ready},
					)
					outcomes <- updateErr
				}()
			}

			close(start)
			group.Wait()
			close(outcomes)

			var wins, conflicts int

			for outcome := range outcomes {
				switch {
				case outcome == nil:
					wins++
				case errors.Is(outcome, store.ErrVersionConflict):
					conflicts++
				default:
					t.Fatalf("a racing update should win or conflict: got %v", outcome)
				}
			}

			if wins != 1 || conflicts != 1 {
				t.Errorf(
					"exactly one racer should win: got %d wins and %d conflicts",
					wins,
					conflicts,
				)
			}
		},
	)
}

func TestConfigDefaults(t *testing.T) {
	t.Parallel()

	t.Run(
		"when the config is zero / then the caps match the memory store defaults",
		func(t *testing.T) {
			t.Parallel()

			if redisstore.DefaultMaxResults != memstore.DefaultMaxResults {
				t.Errorf(
					"the total cap default should match memstore: got %d, want %d",
					redisstore.DefaultMaxResults,
					memstore.DefaultMaxResults,
				)
			}

			if redisstore.DefaultMaxResultsPerInstance != memstore.DefaultMaxResultsPerInstance {
				t.Errorf(
					"the per-instance cap default should match memstore: got %d, want %d",
					redisstore.DefaultMaxResultsPerInstance,
					memstore.DefaultMaxResultsPerInstance,
				)
			}
		},
	)

	t.Run(
		"when the config is zero / then the default per-instance cap bounds storage",
		func(t *testing.T) {
			t.Parallel()

			const overflowSeed = store.JobKey("default-cap-overflow")

			sut := newTestStore(t, redisstore.Config{})

			for filled := store.OccurrenceCount(0); filled < redisstore.DefaultMaxResultsPerInstance; filled++ {
				stored, err := sut.StoreResult(context.Background(), &store.PendingResult{
					JobUUID:    fillJobUUID(filled),
					InstanceID: testInstanceID,
					Success:    true,
				})
				requireNoTestError(t, err, "a result inside the default cap should not fail")

				if !stored {
					t.Fatalf("a result inside the default cap should store: refused at %d", filled)
				}
			}

			overflow, err := sut.StoreResult(context.Background(), &store.PendingResult{
				JobUUID:    testJobUUID(overflowSeed),
				InstanceID: testInstanceID,
				Success:    true,
			})
			requireNoTestError(t, err, "an overflowing result should not fail")

			if overflow {
				t.Error("a result past the default per-instance cap should be refused")
			}

			count, err := sut.CountResults(context.Background())
			requireNoTestError(t, err, "the result count should not fail")

			if count != redisstore.DefaultMaxResultsPerInstance {
				t.Errorf(
					"the default cap should bound the store: got %d, want %d",
					count,
					redisstore.DefaultMaxResultsPerInstance,
				)
			}
		},
	)
}

func fillJobUUID(ordinal store.OccurrenceCount) (id protocol.JobUUID) {
	id[0] = 1
	id[1] = byte(ordinal)
	id[2] = byte(ordinal >> 8)

	return id
}

func TestKeyPrefixIsolation(t *testing.T) {
	t.Parallel()

	t.Run(
		"when two stores share a backend under different prefixes / then neither sees the other",
		func(t *testing.T) {
			t.Parallel()

			const (
				jobKey       = store.JobKey("prefix-isolation")
				firstPrefix  = redisstore.KeyPrefix("scheduler-a")
				secondPrefix = redisstore.KeyPrefix("scheduler-b")
			)

			server := miniredis.RunT(t)

			client := redis.NewClient(&redis.Options{Addr: server.Addr()})
			t.Cleanup(func() { _ = client.Close() })

			first := redisstore.NewStore(client, redisstore.Config{KeyPrefix: firstPrefix})
			second := redisstore.NewStore(client, redisstore.Config{KeyPrefix: secondPrefix})

			created, err := first.UpsertJob(context.Background(), newTestJob(jobKey, jobKey))
			requireNoTestError(t, err, "job creation should not fail")

			_, missErr := second.GetJob(context.Background(), created.ID)
			if missErr == nil || !errors.Is(missErr, store.ErrJobNotFound) {
				t.Errorf("the other prefix should not see the job: got %v", missErr)
			}

			fetched, err := first.GetJob(context.Background(), created.ID)
			requireNoTestError(t, err, "the owning prefix should fetch the job")

			if fetched.ID != created.ID {
				t.Errorf(
					"the owning prefix should keep the job: got %s, want %s",
					fetched.ID,
					created.ID,
				)
			}
		},
	)
}
