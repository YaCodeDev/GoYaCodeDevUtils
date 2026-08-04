package redisstore_test

import (
	"context"
	"errors"
	"net/http"
	"strconv"
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

const (
	evalCommand    = "eval"
	evalShaCommand = "evalsha"

	unlimitedRaces = -1
)

type scriptRaceHook struct {
	remaining int
	mutate    func()
}

func (h *scriptRaceHook) DialHook(next redis.DialHook) redis.DialHook {
	return next
}

func (h *scriptRaceHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		isScript := cmd.Name() == evalCommand || cmd.Name() == evalShaCommand
		if isScript && h.mutate != nil && h.remaining != 0 {
			if h.remaining > 0 {
				h.remaining--
			}

			h.mutate()
		}

		return next(ctx, cmd)
	}
}

func (h *scriptRaceHook) ProcessPipelineHook(
	next redis.ProcessPipelineHook,
) redis.ProcessPipelineHook {
	return next
}

type raceHarness struct {
	setup *redisstore.Store
	sut   *redisstore.Store
	hook  *scriptRaceHook
}

func newRaceHarness(t *testing.T) (harness *raceHarness) {
	t.Helper()

	server := miniredis.RunT(t)

	plain := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = plain.Close() })

	hook := &scriptRaceHook{}

	hooked := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = hooked.Close() })
	hooked.AddHook(hook)

	return &raceHarness{
		setup: redisstore.NewStore(plain, redisstore.Config{}),
		sut:   redisstore.NewStore(hooked, redisstore.Config{}),
		hook:  hook,
	}
}

func requireHookFired(t *testing.T, hook *scriptRaceHook) {
	t.Helper()

	if hook.remaining != 0 {
		t.Fatal("the race hook should intercept a script run")
	}
}

func TestUpsertJobKeyMappingRace(t *testing.T) {
	t.Parallel()

	t.Run(
		"when the key mapping changes between read and script / then the upsert retries onto the new job",
		func(t *testing.T) {
			t.Parallel()

			const (
				jobKey      = store.JobKey("race-upsert")
				staleIDSeed = store.JobKey("race-upsert-stale")
				freshIDSeed = store.JobKey("race-upsert-fresh")
				racerIDSeed = store.JobKey("race-upsert-racer")

				replacedVersion = store.Version(2)
			)

			harness := newRaceHarness(t)

			stale, err := harness.setup.UpsertJob(
				context.Background(),
				newTestJob(staleIDSeed, jobKey),
			)
			requireNoTestError(t, err, "the initial job should store")

			harness.hook.remaining = 1
			harness.hook.mutate = func() {
				deleted, raceErr := harness.setup.DeleteJob(context.Background(), stale.ID)
				requireNoTestError(t, raceErr, "the racing delete should not fail")

				if !deleted {
					t.Fatal("the racing delete should remove the initial job")
				}

				_, raceErr = harness.setup.UpsertJob(
					context.Background(),
					newTestJob(freshIDSeed, jobKey),
				)
				requireNoTestError(t, raceErr, "the racing upsert should not fail")
			}

			upserted, err := harness.sut.UpsertJob(
				context.Background(),
				newTestJob(racerIDSeed, jobKey),
			)
			requireNoTestError(t, err, "the raced upsert should retry and succeed")
			requireHookFired(t, harness.hook)

			if upserted.ID != testJobUUID(freshIDSeed) {
				t.Errorf(
					"the retried upsert should land on the new job: got %s, want %s",
					upserted.ID,
					testJobUUID(freshIDSeed),
				)
			}

			if upserted.Version != replacedVersion {
				t.Errorf(
					"the retried upsert should bump the new job: got %d, want %d",
					upserted.Version,
					replacedVersion,
				)
			}
		},
	)
}

func TestStoreResultInstanceListRace(t *testing.T) {
	t.Parallel()

	t.Run(
		"when the holding list changes between read and script / then the store retries onto the new list",
		func(t *testing.T) {
			t.Parallel()

			const (
				resultSeed    = store.JobKey("race-store-result")
				firstInstance = protocol.InstanceID("inst-first")
				movedInstance = protocol.InstanceID("inst-moved")
				finalInstance = protocol.InstanceID("inst-final")
			)

			harness := newRaceHarness(t)
			jobUUID := testJobUUID(resultSeed)

			stored, err := harness.setup.StoreResult(context.Background(), &store.PendingResult{
				JobUUID:    jobUUID,
				InstanceID: firstInstance,
				Success:    true,
			})
			requireNoTestError(t, err, "the initial result should store")

			if !stored {
				t.Fatal("the initial result should be accepted")
			}

			harness.hook.remaining = 1
			harness.hook.mutate = func() {
				moved, raceErr := harness.setup.StoreResult(
					context.Background(),
					&store.PendingResult{
						JobUUID:    jobUUID,
						InstanceID: movedInstance,
						Success:    true,
					},
				)
				requireNoTestError(t, raceErr, "the racing re-store should not fail")

				if !moved {
					t.Fatal("the racing re-store should be accepted")
				}
			}

			accepted, err := harness.sut.StoreResult(context.Background(), &store.PendingResult{
				JobUUID:    jobUUID,
				InstanceID: finalInstance,
				Success:    true,
			})
			requireNoTestError(t, err, "the raced store should retry and succeed")
			requireHookFired(t, harness.hook)

			if !accepted {
				t.Fatal("the raced store should be accepted")
			}

			held, err := harness.setup.ResultsForInstance(context.Background(), finalInstance, 0)
			requireNoTestError(t, err, "the final instance lookup should not fail")

			if len(held) != 1 || held[0].JobUUID != jobUUID {
				t.Fatalf("the final instance should hold the result: got %d", len(held))
			}

			for _, orphaned := range []protocol.InstanceID{firstInstance, movedInstance} {
				stranded, listErr := harness.setup.ResultsForInstance(
					context.Background(),
					orphaned,
					0,
				)
				requireNoTestError(t, listErr, "the orphaned instance lookup should not fail")

				if len(stranded) != 0 {
					t.Errorf(
						"the outgrown list %q should be empty: got %d",
						orphaned,
						len(stranded),
					)
				}
			}
		},
	)
}

func TestDeleteResultInstanceListRace(t *testing.T) {
	t.Parallel()

	t.Run(
		"when the holding list changes between read and script / then the delete retries and removes the result",
		func(t *testing.T) {
			t.Parallel()

			const (
				resultSeed    = store.JobKey("race-delete-result")
				firstInstance = protocol.InstanceID("inst-first")
				movedInstance = protocol.InstanceID("inst-moved")
			)

			harness := newRaceHarness(t)
			jobUUID := testJobUUID(resultSeed)

			stored, err := harness.setup.StoreResult(context.Background(), &store.PendingResult{
				JobUUID:    jobUUID,
				InstanceID: firstInstance,
				Success:    true,
			})
			requireNoTestError(t, err, "the initial result should store")

			if !stored {
				t.Fatal("the initial result should be accepted")
			}

			harness.hook.remaining = 1
			harness.hook.mutate = func() {
				moved, raceErr := harness.setup.StoreResult(
					context.Background(),
					&store.PendingResult{
						JobUUID:    jobUUID,
						InstanceID: movedInstance,
						Success:    true,
					},
				)
				requireNoTestError(t, raceErr, "the racing re-store should not fail")

				if !moved {
					t.Fatal("the racing re-store should be accepted")
				}
			}

			deleted, err := harness.sut.DeleteResult(context.Background(), jobUUID)
			requireNoTestError(t, err, "the raced delete should retry and succeed")
			requireHookFired(t, harness.hook)

			if !deleted {
				t.Fatal("the raced delete should report true")
			}

			count, err := harness.setup.CountResults(context.Background())
			requireNoTestError(t, err, "the result count should not fail")

			if count != 0 {
				t.Errorf("no result should survive the delete: got %d", count)
			}

			for _, orphaned := range []protocol.InstanceID{firstInstance, movedInstance} {
				stranded, listErr := harness.setup.ResultsForInstance(
					context.Background(),
					orphaned,
					0,
				)
				requireNoTestError(t, listErr, "the orphaned instance lookup should not fail")

				if len(stranded) != 0 {
					t.Errorf(
						"the outgrown list %q should be empty: got %d",
						orphaned,
						len(stranded),
					)
				}
			}
		},
	)

	t.Run(
		"when the holding list keeps changing on every try / then the delete gives up with a conflict",
		func(t *testing.T) {
			t.Parallel()

			const (
				resultSeed    = store.JobKey("race-delete-exhaust")
				firstInstance = protocol.InstanceID("inst-first")
			)

			harness := newRaceHarness(t)
			jobUUID := testJobUUID(resultSeed)

			stored, err := harness.setup.StoreResult(context.Background(), &store.PendingResult{
				JobUUID:    jobUUID,
				InstanceID: firstInstance,
				Success:    true,
			})
			requireNoTestError(t, err, "the initial result should store")

			if !stored {
				t.Fatal("the initial result should be accepted")
			}

			round := 0
			harness.hook.remaining = unlimitedRaces
			harness.hook.mutate = func() {
				round++

				moved, raceErr := harness.setup.StoreResult(
					context.Background(),
					&store.PendingResult{
						JobUUID:    jobUUID,
						InstanceID: protocol.InstanceID("inst-" + strconv.Itoa(round)),
						Success:    true,
					},
				)
				requireNoTestError(t, raceErr, "the racing re-store should not fail")

				if !moved {
					t.Fatal("the racing re-store should be accepted")
				}
			}

			_, conflictErr := harness.sut.DeleteResult(context.Background(), jobUUID)

			if round == 0 {
				t.Fatal("the race hook should intercept a script run")
			}

			if conflictErr == nil || !errors.Is(conflictErr, redisstore.ErrConcurrentUpdate) {
				t.Fatalf("the exhausted delete should conflict: got %v", conflictErr)
			}

			if conflictErr.Code() != http.StatusConflict {
				t.Errorf(
					"the exhausted delete should carry a conflict code: got %d, want %d",
					conflictErr.Code(),
					http.StatusConflict,
				)
			}
		},
	)
}

func TestPreallocatedIdentifiers(t *testing.T) {
	t.Parallel()

	t.Run(
		"when an occurrence is replayed / then the burned identifier leaves a gap and no execution is lost",
		func(t *testing.T) {
			t.Parallel()

			const (
				jobKey = store.JobKey("prealloc-execution")

				firstExecutionID = protocol.ExecutionID(1)
				nextExecutionID  = protocol.ExecutionID(3)
			)

			scheduledFirst := time.Date(2023, time.November, 14, 22, 13, 20, 0, time.UTC)
			scheduledNext := scheduledFirst.Add(time.Minute)

			sut := newTestStore(t, redisstore.Config{})

			job, err := sut.UpsertJob(context.Background(), newTestJob(jobKey, jobKey))
			requireNoTestError(t, err, "job creation should not fail")

			first, fresh, err := sut.CreateExecution(
				context.Background(),
				job.ID,
				scheduledFirst,
				store.StateScheduled,
				false,
			)
			requireNoTestError(t, err, "the first execution should create")

			if !fresh || first.ID != firstExecutionID {
				t.Fatalf(
					"the first execution should take the first identifier: got %d, want %d",
					first.ID,
					firstExecutionID,
				)
			}

			replayed, refreshed, err := sut.CreateExecution(
				context.Background(),
				job.ID,
				scheduledFirst,
				store.StateScheduled,
				false,
			)
			requireNoTestError(t, err, "the replayed occurrence should not fail")

			if refreshed || replayed.ID != firstExecutionID {
				t.Fatalf(
					"the replayed occurrence should return the stored execution: got %d, want %d",
					replayed.ID,
					firstExecutionID,
				)
			}

			next, freshNext, err := sut.CreateExecution(
				context.Background(),
				job.ID,
				scheduledNext,
				store.StateScheduled,
				false,
			)
			requireNoTestError(t, err, "the next execution should create")

			if !freshNext || next.ID != nextExecutionID {
				t.Errorf(
					"the replay should burn one identifier: got %d, want %d",
					next.ID,
					nextExecutionID,
				)
			}
		},
	)

	t.Run(
		"when an attempt is refused / then the burned identifier leaves a gap and no attempt is lost",
		func(t *testing.T) {
			t.Parallel()

			const (
				jobKey = store.JobKey("prealloc-attempt")

				missingExecutionID = protocol.ExecutionID(1000000)
				firstAttemptNumber = store.AttemptNumber(1)
				storedAttemptID    = protocol.AttemptID(2)
			)

			scheduledAt := time.Date(2023, time.November, 14, 22, 13, 20, 0, time.UTC)

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

			_, missErr := sut.CreateAttempt(
				context.Background(),
				missingExecutionID,
				firstAttemptNumber,
				testInstanceID,
			)
			if missErr == nil || !errors.Is(missErr, store.ErrExecutionNotFound) {
				t.Fatalf("an attempt on a missing execution should refuse: got %v", missErr)
			}

			attempt, err := sut.CreateAttempt(
				context.Background(),
				execution.ID,
				firstAttemptNumber,
				testInstanceID,
			)
			requireNoTestError(t, err, "the stored attempt should create")

			if attempt.ID != storedAttemptID {
				t.Errorf(
					"the refusal should burn one identifier: got %d, want %d",
					attempt.ID,
					storedAttemptID,
				)
			}
		},
	)
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
