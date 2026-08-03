package storetest

import (
	"context"
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/store"
)

// TestJobRepository runs the job conformance subtests against stores the
// factory builds: upsert identity, executor-scoped keys, lookups, deletes,
// the enabled flag, skipped-occurrence counters, and enabled listing.
func TestJobRepository(t *testing.T, factory Factory) {
	t.Helper()

	t.Run(
		"when a new key is upserted / then a job is created at version one",
		func(t *testing.T) {
			t.Parallel()

			const jobKey = store.JobKey("job-create")

			sut := factory(t)

			job := createJob(t, sut, jobKey)

			if job.ID != jobUUID(jobKey) {
				t.Errorf("a created job should keep the client id: got %s", job.ID)
			}

			if job.Version != firstVersion {
				t.Errorf(
					"a created job should start at version %d, got %d",
					firstVersion,
					job.Version,
				)
			}

			if job.CreatedAt.IsZero() {
				t.Error("a created job should record its creation time")
			}
		},
	)

	t.Run(
		"when a nil job is upserted / then the store reports its own error",
		func(t *testing.T) {
			t.Parallel()

			sut := factory(t)

			result, err := sut.UpsertJob(context.Background(), nil)
			requireSentinel(t, err, store.ErrNilJob, "a nil job must not be stored")

			if result != nil {
				t.Error("a refused upsert should not return a job")
			}
		},
	)

	t.Run(
		"when the job id is the zero uuid / then the upsert is refused",
		func(t *testing.T) {
			t.Parallel()

			const jobKey = store.JobKey("job-zero-uuid")

			sut := factory(t)

			job := newJob(jobKey)
			job.ID = jobUUID("")

			_, err := sut.UpsertJob(context.Background(), job)
			requireSentinel(t, err, store.ErrZeroJobUUID, "a zero job uuid must not be stored")
		},
	)

	t.Run(
		"when the same key is upserted again / then identity and counters survive",
		func(t *testing.T) {
			t.Parallel()

			const (
				jobKey       = store.JobKey("job-reupsert")
				otherIDSeed  = store.JobKey("job-reupsert-other")
				skippedCount = store.OccurrenceCount(7)
			)

			sut := factory(t)

			first := createJob(t, sut, jobKey)

			err := sut.AddSkippedOccurrences(context.Background(), first.ID, skippedCount)
			requireNoError(t, err, "recording skipped occurrences should not fail")

			replacement := newJob(jobKey)
			replacement.ID = jobUUID(otherIDSeed)
			replacement.Enabled = false

			second := upsertJob(t, sut, replacement)

			if second.ID != first.ID {
				t.Errorf("re-upsert should keep the job id: got %s, want %s", second.ID, first.ID)
			}

			if second.Version != secondVersion {
				t.Errorf(
					"re-upsert should bump the version to %d, got %d",
					secondVersion,
					second.Version,
				)
			}

			if !second.CreatedAt.Equal(first.CreatedAt) {
				t.Errorf("re-upsert should preserve creation time: got %v", second.CreatedAt)
			}

			if second.SkippedOccurrences != skippedCount {
				t.Errorf(
					"re-upsert should preserve skipped occurrences: got %d, want %d",
					second.SkippedOccurrences,
					skippedCount,
				)
			}

			if second.Enabled {
				t.Error("re-upsert should store the replacement fields")
			}
		},
	)

	t.Run(
		"when the same key is upserted under two executor types / then two jobs coexist",
		func(t *testing.T) {
			t.Parallel()

			const (
				jobKey      = store.JobKey("job-shared-key")
				otherIDSeed = store.JobKey("job-shared-key-other")
			)

			sut := factory(t)

			first := createJob(t, sut, jobKey)

			foreign := newJob(otherIDSeed)
			foreign.Key = jobKey
			foreign.ExecutorType = otherExecutorType

			second := upsertJob(t, sut, foreign)

			if second.ID == first.ID {
				t.Error("another executor type should create its own job")
			}

			if second.Version != firstVersion {
				t.Errorf(
					"the second job should start at version %d, got %d",
					firstVersion,
					second.Version,
				)
			}

			if kept := getJobByKey(t, sut, suiteExecutorType, jobKey); kept.ID != first.ID {
				t.Errorf(
					"the first executor type should keep its job: got %s, want %s",
					kept.ID,
					first.ID,
				)
			}

			if kept := getJobByKey(t, sut, otherExecutorType, jobKey); kept.ID != second.ID {
				t.Errorf(
					"the second executor type should keep its job: got %s, want %s",
					kept.ID,
					second.ID,
				)
			}
		},
	)

	t.Run(
		"when a stored job is fetched by id / then the job is returned",
		func(t *testing.T) {
			t.Parallel()

			const jobKey = store.JobKey("job-get-hit")

			sut := factory(t)

			created := createJob(t, sut, jobKey)

			fetched := getJob(t, sut, created.ID)

			if fetched.ID != created.ID || fetched.Key != jobKey {
				t.Errorf("the fetch should return the stored job: got %s", fetched.ID)
			}
		},
	)

	t.Run(
		"when an unknown job id is fetched / then the lookup misses",
		func(t *testing.T) {
			t.Parallel()

			const unknownSeed = store.JobKey("job-get-miss")

			sut := factory(t)

			_, err := sut.GetJob(context.Background(), jobUUID(unknownSeed))
			requireSentinel(t, err, store.ErrJobNotFound, "an unknown job id must miss")
		},
	)

	t.Run(
		"when a stored key is fetched under its executor type / then the job is returned",
		func(t *testing.T) {
			t.Parallel()

			const jobKey = store.JobKey("job-key-hit")

			sut := factory(t)

			created := createJob(t, sut, jobKey)

			if fetched := getJobByKey(t, sut, suiteExecutorType, jobKey); fetched.ID != created.ID {
				t.Errorf(
					"the key fetch should return the stored job: got %s, want %s",
					fetched.ID,
					created.ID,
				)
			}
		},
	)

	t.Run(
		"when a stored key is fetched under another executor type / then the lookup misses",
		func(t *testing.T) {
			t.Parallel()

			const jobKey = store.JobKey("job-key-miss")

			sut := factory(t)

			createJob(t, sut, jobKey)

			_, err := sut.GetJobByKey(context.Background(), otherExecutorType, jobKey)
			requireSentinel(
				t,
				err,
				store.ErrJobNotFound,
				"a lookup under the wrong executor type must miss",
			)
		},
	)

	t.Run(
		"when a stored job is deleted / then the job row and its key are gone",
		func(t *testing.T) {
			t.Parallel()

			const jobKey = store.JobKey("job-delete-hit")

			sut := factory(t)

			created := createJob(t, sut, jobKey)

			if !deleteJob(t, sut, created.ID) {
				t.Fatal("a stored job delete should report true")
			}

			_, fetchErr := sut.GetJob(context.Background(), created.ID)
			requireSentinel(t, fetchErr, store.ErrJobNotFound, "the deleted job should be gone")

			_, keyErr := sut.GetJobByKey(context.Background(), suiteExecutorType, jobKey)
			requireSentinel(
				t,
				keyErr,
				store.ErrJobNotFound,
				"the deleted job's key should be freed",
			)
		},
	)

	t.Run(
		"when a job is deleted twice / then the second delete reports false",
		func(t *testing.T) {
			t.Parallel()

			const jobKey = store.JobKey("job-delete-twice")

			sut := factory(t)

			created := createJob(t, sut, jobKey)

			if !deleteJob(t, sut, created.ID) {
				t.Fatal("the first delete should report true")
			}

			if deleteJob(t, sut, created.ID) {
				t.Error("a replayed delete should report false")
			}
		},
	)

	t.Run(
		"when the same key exists under another executor type / then only the matching entry is freed",
		func(t *testing.T) {
			t.Parallel()

			const (
				jobKey      = store.JobKey("job-delete-scoped")
				otherIDSeed = store.JobKey("job-delete-scoped-other")
			)

			sut := factory(t)

			created := createJob(t, sut, jobKey)

			foreign := newJob(otherIDSeed)
			foreign.Key = jobKey
			foreign.ExecutorType = otherExecutorType

			other := upsertJob(t, sut, foreign)

			if !deleteJob(t, sut, created.ID) {
				t.Fatal("the delete should report true")
			}

			if kept := getJobByKey(t, sut, otherExecutorType, jobKey); kept.ID != other.ID {
				t.Errorf(
					"the surviving entry should be the other-type job: got %s, want %s",
					kept.ID,
					other.ID,
				)
			}

			_, missErr := sut.GetJobByKey(context.Background(), suiteExecutorType, jobKey)
			requireSentinel(t, missErr, store.ErrJobNotFound, "the matching entry should be freed")
		},
	)

	t.Run(
		"when a deleted key is upserted again / then a fresh job materializes",
		func(t *testing.T) {
			t.Parallel()

			const (
				jobKey    = store.JobKey("job-delete-reupsert")
				freshSeed = store.JobKey("job-delete-reupsert-fresh")
			)

			sut := factory(t)

			created := createJob(t, sut, jobKey)

			if !deleteJob(t, sut, created.ID) {
				t.Fatal("the delete should report true")
			}

			replacement := newJob(jobKey)
			replacement.ID = jobUUID(freshSeed)

			fresh := upsertJob(t, sut, replacement)

			if fresh.ID != jobUUID(freshSeed) {
				t.Errorf(
					"the re-upsert should keep its own identity: got %s, want %s",
					fresh.ID,
					jobUUID(freshSeed),
				)
			}

			if fresh.Version != firstVersion {
				t.Errorf(
					"the re-upsert should materialize a fresh job at version %d: got %d",
					firstVersion,
					fresh.Version,
				)
			}
		},
	)

	t.Run(
		"when the enabled flag is flipped / then the stored job follows it",
		func(t *testing.T) {
			t.Parallel()

			const jobKey = store.JobKey("job-enabled-flip")

			sut := factory(t)

			created := createJob(t, sut, jobKey)

			err := sut.SetJobEnabled(context.Background(), created.ID, false)
			requireNoError(t, err, "disabling a stored job should not fail")

			if disabled := getJob(t, sut, created.ID); disabled.Enabled {
				t.Error("a disabled job should store the flag")
			}

			err = sut.SetJobEnabled(context.Background(), created.ID, true)
			requireNoError(t, err, "re-enabling a stored job should not fail")

			if enabled := getJob(t, sut, created.ID); !enabled.Enabled {
				t.Error("a re-enabled job should store the flag")
			}
		},
	)

	t.Run(
		"when an unknown job's enabled flag is flipped / then the update misses",
		func(t *testing.T) {
			t.Parallel()

			const unknownSeed = store.JobKey("job-enabled-miss")

			sut := factory(t)

			err := sut.SetJobEnabled(context.Background(), jobUUID(unknownSeed), true)
			requireSentinel(t, err, store.ErrJobNotFound, "an unknown job enable must miss")
		},
	)

	t.Run(
		"when skipped occurrences are added twice / then the counter accumulates",
		func(t *testing.T) {
			t.Parallel()

			const (
				jobKey      = store.JobKey("job-skip-count")
				firstBatch  = store.OccurrenceCount(2)
				secondBatch = store.OccurrenceCount(3)
				wantSkipped = firstBatch + secondBatch
			)

			sut := factory(t)

			created := createJob(t, sut, jobKey)

			err := sut.AddSkippedOccurrences(context.Background(), created.ID, firstBatch)
			requireNoError(t, err, "the first skip batch should not fail")

			err = sut.AddSkippedOccurrences(context.Background(), created.ID, secondBatch)
			requireNoError(t, err, "the second skip batch should not fail")

			if counted := getJob(t, sut, created.ID); counted.SkippedOccurrences != wantSkipped {
				t.Errorf(
					"skip batches should accumulate to %d: got %d",
					wantSkipped,
					counted.SkippedOccurrences,
				)
			}
		},
	)

	t.Run(
		"when skipped occurrences are added to an unknown job / then the update misses",
		func(t *testing.T) {
			t.Parallel()

			const (
				unknownSeed = store.JobKey("job-skip-miss")
				oneSkip     = store.OccurrenceCount(1)
			)

			sut := factory(t)

			err := sut.AddSkippedOccurrences(context.Background(), jobUUID(unknownSeed), oneSkip)
			requireSentinel(t, err, store.ErrJobNotFound, "an unknown job skip must miss")
		},
	)

	t.Run(
		"when jobs are listed / then only enabled jobs return in identifier order",
		func(t *testing.T) {
			t.Parallel()

			const (
				laterKey    = store.JobKey("job-list-b")
				earlierKey  = store.JobKey("job-list-a")
				disabledKey = store.JobKey("job-list-c")
				wantListed  = 2
			)

			sut := factory(t)

			later := createJob(t, sut, laterKey)
			earlier := createJob(t, sut, earlierKey)

			disabled := newJob(disabledKey)
			disabled.Enabled = false

			upsertJob(t, sut, disabled)

			listed := listEnabledJobs(t, sut)

			if len(listed) != wantListed {
				t.Fatalf("only enabled jobs should list: got %d", len(listed))
			}

			if listed[0].ID != earlier.ID || listed[1].ID != later.ID {
				t.Errorf("enabled jobs should order by identifier: got %v", listed)
			}
		},
	)
}
