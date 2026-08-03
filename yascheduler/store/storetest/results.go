package storetest

import (
	"context"
	"testing"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/store"
)

// TestResultRepository runs the pending-result conformance subtests that
// need no storage caps against stores the factory builds: storage and
// replacement, deletion, instance listings, send accounting, expiry, and
// counting. TestResultCaps covers the caps through a BoundedFactory.
func TestResultRepository(t *testing.T, factory Factory) {
	t.Helper()

	t.Run(
		"when a result is stored / then the instance holds it",
		func(t *testing.T) {
			t.Parallel()

			const resultKey = store.JobKey("result-store")

			sut := factory(t)

			id, stored := storeResult(t, sut, resultKey, suiteInstanceID)

			if !stored {
				t.Fatal("a fresh result should be stored")
			}

			held := resultsForInstance(t, sut, suiteInstanceID, unlimited)

			if len(held) != 1 {
				t.Fatalf("the instance should hold one result: got %d", len(held))
			}

			if held[0].JobUUID != id || !held[0].Success {
				t.Errorf("the held result should keep its fields: got %v", held[0])
			}

			if held[0].Attempts != 0 {
				t.Errorf("a fresh result should start unsent: got %d", held[0].Attempts)
			}
		},
	)

	t.Run(
		"when a nil result is stored / then the store reports its own error",
		func(t *testing.T) {
			t.Parallel()

			sut := factory(t)

			stored, err := sut.StoreResult(context.Background(), nil)
			requireSentinel(t, err, store.ErrNilResult, "a nil pending result must not be stored")

			if stored {
				t.Error("a nil pending result should not report storage")
			}
		},
	)

	t.Run(
		"when a held job's result is stored again / then counters and creation time survive",
		func(t *testing.T) {
			t.Parallel()

			const (
				resultKey = store.JobKey("result-replace")
				firstSend = store.ResultAttempts(1)
			)

			sut := factory(t)
			sentAt := baseTime.Add(time.Minute)

			id, stored := storeResult(t, sut, resultKey, suiteInstanceID)

			if !stored {
				t.Fatal("the first result should be stored")
			}

			markResultSent(t, sut, id, sentAt)

			before := resultsForInstance(t, sut, suiteInstanceID, unlimited)

			if len(before) != 1 {
				t.Fatalf("the instance should hold one result: got %d", len(before))
			}

			replaced, err := sut.StoreResult(context.Background(), &store.PendingResult{
				JobUUID:    id,
				InstanceID: suiteInstanceID,
				Success:    false,
				HasValue:   true,
			})
			requireNoError(t, err, "re-storing a held job should not fail")

			if !replaced {
				t.Fatal("re-storing a held job should be accepted")
			}

			after := resultsForInstance(t, sut, suiteInstanceID, unlimited)

			if len(after) != 1 {
				t.Fatalf("the replacement should not add a record: got %d", len(after))
			}

			if after[0].Success {
				t.Errorf("the replacement should store the new success flag: got %v", after[0])
			}

			if !after[0].HasValue {
				t.Errorf("the replacement should store the new value flag: got %v", after[0])
			}

			if after[0].Attempts != firstSend {
				t.Errorf(
					"the replacement should keep the send counter: got %d, want %d",
					after[0].Attempts,
					firstSend,
				)
			}

			if !after[0].CreatedAt.Equal(before[0].CreatedAt) {
				t.Errorf(
					"the replacement should keep the creation time: got %v",
					after[0].CreatedAt,
				)
			}

			if !after[0].LastSentAt.Equal(sentAt) {
				t.Errorf(
					"the replacement should keep the send instant: got %v",
					after[0].LastSentAt,
				)
			}
		},
	)

	t.Run(
		"when a held result moves to another instance / then only the new instance holds it",
		func(t *testing.T) {
			t.Parallel()

			const resultKey = store.JobKey("result-move")

			sut := factory(t)

			if _, stored := storeResult(t, sut, resultKey, suiteInstanceID); !stored {
				t.Fatal("the first result should be stored")
			}

			id, moved := storeResult(t, sut, resultKey, otherInstanceID)

			if !moved {
				t.Fatal("re-storing under another instance should be accepted")
			}

			if old := resultsForInstance(t, sut, suiteInstanceID, unlimited); len(old) != 0 {
				t.Errorf("the old instance should release the result: got %v", old)
			}

			held := resultsForInstance(t, sut, otherInstanceID, unlimited)

			if len(held) != 1 || held[0].JobUUID != id {
				t.Errorf("the new instance should hold the result: got %v", held)
			}
		},
	)

	t.Run(
		"when a middle result is deleted / then the surviving order holds",
		func(t *testing.T) {
			t.Parallel()

			const wantSurvivors = 2

			sut := factory(t)

			first, _ := storeResult(t, sut, "result-delete-a", suiteInstanceID)
			second, _ := storeResult(t, sut, "result-delete-b", suiteInstanceID)
			third, _ := storeResult(t, sut, "result-delete-c", suiteInstanceID)

			if !deleteResult(t, sut, second) {
				t.Fatal("a held result should report deletion")
			}

			if deleteResult(t, sut, second) {
				t.Error("a replayed deletion should report nothing deleted")
			}

			held := resultsForInstance(t, sut, suiteInstanceID, unlimited)

			if len(held) != wantSurvivors {
				t.Fatalf("two results should survive the deletion: got %d", len(held))
			}

			if held[0].JobUUID != first || held[1].JobUUID != third {
				t.Errorf("deletion should keep the remaining order: got %v", held)
			}
		},
	)

	t.Run(
		"when results are stored for one instance / then they return in storage order",
		func(t *testing.T) {
			t.Parallel()

			const (
				takeTwo  = store.BatchLimit(2)
				wantHeld = 3
			)

			sut := factory(t)

			first, _ := storeResult(t, sut, "result-order-a", suiteInstanceID)
			second, _ := storeResult(t, sut, "result-order-b", suiteInstanceID)
			third, _ := storeResult(t, sut, "result-order-c", suiteInstanceID)
			foreign, _ := storeResult(t, sut, "result-order-d", otherInstanceID)

			held := resultsForInstance(t, sut, suiteInstanceID, unlimited)

			if len(held) != wantHeld {
				t.Fatalf("the instance should hold three results: got %d", len(held))
			}

			if held[0].JobUUID != first ||
				held[1].JobUUID != second ||
				held[2].JobUUID != third {
				t.Errorf("results should return in storage order: got %v", held)
			}

			limited := resultsForInstance(t, sut, suiteInstanceID, takeTwo)

			if len(limited) != int(takeTwo) {
				t.Fatalf("the limit should cap the result list: got %d", len(limited))
			}

			if limited[0].JobUUID != first || limited[1].JobUUID != second {
				t.Errorf("the limit should keep the earliest results: got %v", limited)
			}

			other := resultsForInstance(t, sut, otherInstanceID, unlimited)

			if len(other) != 1 || other[0].JobUUID != foreign {
				t.Errorf("an instance should only see its own results: got %v", other)
			}
		},
	)

	t.Run(
		"when a result is marked sent / then its counters advance",
		func(t *testing.T) {
			t.Parallel()

			const (
				resultKey   = store.JobKey("result-sent")
				missingSeed = store.JobKey("result-sent-missing")
				firstSend   = store.ResultAttempts(1)
			)

			sut := factory(t)
			sentAt := baseTime.Add(time.Minute)

			id, _ := storeResult(t, sut, resultKey, suiteInstanceID)

			markResultSent(t, sut, id, sentAt)

			held := resultsForInstance(t, sut, suiteInstanceID, unlimited)

			if len(held) != 1 {
				t.Fatalf("the instance should hold one result: got %d", len(held))
			}

			if held[0].Attempts != firstSend {
				t.Errorf("a send should count one attempt: got %d", held[0].Attempts)
			}

			if !held[0].LastSentAt.Equal(sentAt) {
				t.Errorf("a send should record its instant: got %v", held[0].LastSentAt)
			}

			markErr := sut.MarkResultSent(context.Background(), jobUUID(missingSeed), sentAt)
			requireSentinel(
				t,
				markErr,
				store.ErrResultNotFound,
				"marking an unheld result sent must fail",
			)
		},
	)

	t.Run(
		"when results are compared to a cutoff / then only older ones return in order",
		func(t *testing.T) {
			t.Parallel()

			const (
				takeOne     = store.BatchLimit(1)
				wantExpired = 2
			)

			sut := factory(t)

			oldest, _ := storeResult(t, sut, "result-expiry-a", suiteInstanceID)
			newest, _ := storeResult(t, sut, "result-expiry-b", otherInstanceID)

			oldestHeld := resultsForInstance(t, sut, suiteInstanceID, unlimited)
			newestHeld := resultsForInstance(t, sut, otherInstanceID, unlimited)

			if len(oldestHeld) != 1 || len(newestHeld) != 1 {
				t.Fatal("both instances should hold their results")
			}

			cutoff := oldestHeld[0].CreatedAt
			if newestHeld[0].CreatedAt.After(cutoff) {
				cutoff = newestHeld[0].CreatedAt
			}

			cutoff = cutoff.Add(time.Second)

			expired := expiredResults(t, sut, cutoff, unlimited)

			if len(expired) != wantExpired {
				t.Fatalf("both stored results should expire: got %d", len(expired))
			}

			if expired[0].JobUUID != oldest || expired[1].JobUUID != newest {
				t.Errorf("expired results should order by storage time: got %v", expired)
			}

			limited := expiredResults(t, sut, cutoff, takeOne)

			if len(limited) != int(takeOne) {
				t.Fatalf("the limit should cap the expired list: got %d", len(limited))
			}

			if limited[0].JobUUID != oldest {
				t.Errorf("the limit should keep the oldest result: got %v", limited)
			}
		},
	)

	t.Run(
		"when no result is older than the cutoff / then nothing expires",
		func(t *testing.T) {
			t.Parallel()

			const resultKey = store.JobKey("result-fresh")

			sut := factory(t)

			storeResult(t, sut, resultKey, suiteInstanceID)

			held := resultsForInstance(t, sut, suiteInstanceID, unlimited)

			if len(held) != 1 {
				t.Fatalf("the instance should hold one result: got %d", len(held))
			}

			if expired := expiredResults(t, sut, held[0].CreatedAt, unlimited); len(expired) != 0 {
				t.Errorf("a result stored at the cutoff should not expire: got %v", expired)
			}
		},
	)

	t.Run(
		"when results are counted / then the count follows stores and deletes",
		func(t *testing.T) {
			t.Parallel()

			const (
				afterStores = store.OccurrenceCount(2)
				afterDelete = store.OccurrenceCount(1)
			)

			sut := factory(t)

			first, _ := storeResult(t, sut, "result-count-a", suiteInstanceID)
			storeResult(t, sut, "result-count-b", otherInstanceID)

			if count := countResults(t, sut); count != afterStores {
				t.Errorf("both stored results should count: got %d", count)
			}

			if !deleteResult(t, sut, first) {
				t.Fatal("the held result should report deletion")
			}

			if count := countResults(t, sut); count != afterDelete {
				t.Errorf("a deleted result should leave the count: got %d", count)
			}
		},
	)
}

// TestResultCaps runs the pending-result cap conformance subtests against
// stores the bounded factory builds. Every subtest states the caps it
// needs, so the factory must honor the given Caps exactly.
func TestResultCaps(t *testing.T, factory BoundedFactory) {
	t.Helper()

	t.Run(
		"when one instance fills its cap / then further results are refused, not failed",
		func(t *testing.T) {
			t.Parallel()

			const perInstanceCap = store.OccurrenceCount(2)

			sut := factory(t, Caps{MaxResultsPerInstance: perInstanceCap})

			if _, stored := storeResult(t, sut, "result-cap-a", suiteInstanceID); !stored {
				t.Fatal("the first result should fit the cap")
			}

			if _, stored := storeResult(t, sut, "result-cap-b", suiteInstanceID); !stored {
				t.Fatal("the second result should fit the cap")
			}

			if _, overflow := storeResult(t, sut, "result-cap-c", suiteInstanceID); overflow {
				t.Error("a result past the per-instance cap should be refused")
			}

			if count := countResults(t, sut); count != perInstanceCap {
				t.Errorf("a refused result should not be stored: got %d", count)
			}
		},
	)

	t.Run(
		"when one instance is full / then another instance still stores results",
		func(t *testing.T) {
			t.Parallel()

			const (
				perInstanceCap = store.OccurrenceCount(1)
				bothInstances  = store.OccurrenceCount(2)
			)

			sut := factory(t, Caps{MaxResultsPerInstance: perInstanceCap})

			if _, stored := storeResult(t, sut, "result-split-a", suiteInstanceID); !stored {
				t.Fatal("the first instance should store its result")
			}

			if _, overflow := storeResult(t, sut, "result-split-b", suiteInstanceID); overflow {
				t.Error("the first instance should be full")
			}

			if _, stored := storeResult(t, sut, "result-split-c", otherInstanceID); !stored {
				t.Error("a second instance should have its own budget")
			}

			if count := countResults(t, sut); count != bothInstances {
				t.Errorf("both instances should hold one result each: got %d", count)
			}
		},
	)

	t.Run(
		"when a full instance re-stores a held job / then the replacement is accepted",
		func(t *testing.T) {
			t.Parallel()

			const (
				perInstanceCap = store.OccurrenceCount(1)
				heldJobKey     = store.JobKey("result-cap-replace")
			)

			sut := factory(t, Caps{MaxResultsPerInstance: perInstanceCap})

			if _, stored := storeResult(t, sut, heldJobKey, suiteInstanceID); !stored {
				t.Fatal("the first result should fit the cap")
			}

			if _, stored := storeResult(t, sut, heldJobKey, suiteInstanceID); !stored {
				t.Error("re-storing a held job should not count against the cap")
			}

			if count := countResults(t, sut); count != perInstanceCap {
				t.Errorf("a replacement should not add a record: got %d", count)
			}
		},
	)

	t.Run(
		"when the total cap is reached / then every instance is refused",
		func(t *testing.T) {
			t.Parallel()

			const totalCap = store.OccurrenceCount(1)

			sut := factory(t, Caps{MaxResults: totalCap})

			if _, stored := storeResult(t, sut, "result-total-a", suiteInstanceID); !stored {
				t.Fatal("the first result should fit the total cap")
			}

			if _, overflow := storeResult(t, sut, "result-total-b", otherInstanceID); overflow {
				t.Error("a result past the total cap should be refused")
			}

			if count := countResults(t, sut); count != totalCap {
				t.Errorf("the total cap should bound the store: got %d", count)
			}
		},
	)
}
