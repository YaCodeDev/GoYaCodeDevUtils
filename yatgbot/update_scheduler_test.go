package yatgbot

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/gotd/td/tg"
)

func TestAsyncUpdateSchedulerBlocksSharedKeysButNotIndependentOnes(t *testing.T) {
	t.Parallel()

	scheduler := newAsyncUpdateScheduler()

	firstStarted := make(chan struct{})
	independentStarted := make(chan struct{})
	sharedStarted := make(chan struct{})
	releaseFirst := make(chan struct{})

	scheduler.Enqueue([]string{"user:1", "chat:1"}, func() {
		close(firstStarted)
		<-releaseFirst
	})

	scheduler.Enqueue([]string{"user:1"}, func() {
		close(sharedStarted)
	})

	scheduler.Enqueue([]string{"user:2", "chat:2"}, func() {
		close(independentStarted)
	})

	waitForSignal(t, firstStarted, "first job to start")
	waitForSignal(t, independentStarted, "independent job to start while shared key is busy")
	assertNoSignal(t, sharedStarted, 100*time.Millisecond, "shared-key job started too early")

	close(releaseFirst)

	waitForSignal(t, sharedStarted, "shared-key job to start after the first one finishes")
}

func TestAsyncUpdateSchedulerKeepsPerKeyFIFOAcrossCombinedScopes(t *testing.T) {
	t.Parallel()

	scheduler := newAsyncUpdateScheduler()

	firstStarted := make(chan struct{})
	secondStarted := make(chan struct{})
	thirdStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	releaseSecond := make(chan struct{})

	scheduler.Enqueue([]string{"user:1"}, func() {
		close(firstStarted)
		<-releaseFirst
	})

	scheduler.Enqueue([]string{"user:1", "chat:1"}, func() {
		close(secondStarted)
		<-releaseSecond
	})

	scheduler.Enqueue([]string{"chat:1"}, func() {
		close(thirdStarted)
	})

	waitForSignal(t, firstStarted, "first job to start")
	assertNoSignal(t, secondStarted, 100*time.Millisecond, "second job started before user scope was released")
	assertNoSignal(t, thirdStarted, 100*time.Millisecond, "third job overtook the earlier chat-scoped job")

	close(releaseFirst)

	waitForSignal(t, secondStarted, "second job to start after the shared user scope is released")
	assertNoSignal(t, thirdStarted, 100*time.Millisecond, "third job started before the earlier chat-scoped job completed")

	close(releaseSecond)

	waitForSignal(t, thirdStarted, "third job to start after the earlier chat-scoped job completed")
}

func TestAsyncUpdateSchedulerHandlesNilRunAndZeroKeys(t *testing.T) {
	t.Parallel()

	scheduler := newAsyncUpdateScheduler()
	scheduler.Enqueue([]string{"user:1"}, nil)

	if len(scheduler.jobs) != 0 {
		t.Fatalf("scheduler jobs len = %d, want 0", len(scheduler.jobs))
	}

	started := make(chan struct{})
	scheduler.Enqueue(nil, func() {
		close(started)
	})

	waitForSignal(t, started, "zero-key job to start immediately")
}

func TestAsyncUpdateSchedulerFinishRemovesNonHeadJob(t *testing.T) {
	t.Parallel()

	scheduler := newAsyncUpdateScheduler()
	first := &asyncUpdateJob{
		keys:    []string{"user:1"},
		run:     func() {},
		started: true,
	}
	second := &asyncUpdateJob{
		keys: []string{"user:1"},
		run:  func() {},
	}

	scheduler.jobs = []*asyncUpdateJob{first, second}
	scheduler.keyQueues["user:1"] = []*asyncUpdateJob{first, second}
	scheduler.busyKeys["user:1"] = struct{}{}

	scheduler.finish(second)

	if !second.done {
		t.Fatal("finish() did not mark the job as done")
	}

	if got := len(scheduler.keyQueues["user:1"]); got != 1 {
		t.Fatalf("key queue len = %d, want 1", got)
	}

	if scheduler.keyQueues["user:1"][0] != first {
		t.Fatal("finish() removed the wrong queued job")
	}
}

func TestUniqueStrings(t *testing.T) {
	t.Parallel()

	if got := uniqueStrings([]string{"user:1"}); len(got) != 1 || got[0] != "user:1" {
		t.Fatalf("uniqueStrings(single) = %v, want [user:1]", got)
	}

	got := uniqueStrings([]string{"user:1", "user:1", "chat:2"})
	want := []string{"user:1", "chat:2"}

	if len(got) != len(want) {
		t.Fatalf("uniqueStrings(dedup) len = %d, want %d", len(got), len(want))
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("uniqueStrings(dedup) = %v, want %v", got, want)
		}
	}
}

func TestBindCreatesOrderedSchedulerOnlyWhenFeatureEnabled(t *testing.T) {
	t.Parallel()

	disabled := &Dispatcher{}
	disabledTGDispatcher := tg.NewUpdateDispatcher()
	disabled.Bind(&disabledTGDispatcher, false)
	if disabled.updateScheduler != nil {
		t.Fatal("update scheduler was initialized without the feature flag")
	}

	enabled := &Dispatcher{Features: FeatureSequentialUpdates}
	enabledTGDispatcher := tg.NewUpdateDispatcher()
	enabled.Bind(&enabledTGDispatcher, false)
	if enabled.updateScheduler == nil {
		t.Fatal("update scheduler was not initialized with the feature flag enabled")
	}
}

func TestWrapAsyncSequentializesSharedUpdateScopes(t *testing.T) {
	t.Parallel()

	scheduler := newAsyncUpdateScheduler()
	firstStarted := make(chan struct{})
	secondStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	var calls atomic.Int32

	handler := wrapAsync(
		false,
		scheduler,
		func(_ tg.Entities, upd *tg.UpdateBotInlineQuery) (UpdateData, bool) {
			return UpdateData{
				userID: upd.UserID,
				chatID: upd.UserID,
				ent:    tg.Entities{},
				update: upd,
			}, true
		},
		func(_ context.Context, deps UpdateData) yaerrors.Error {
			if calls.Add(1) == 1 {
				close(firstStarted)
				<-releaseFirst
			} else {
				close(secondStarted)
			}

			return nil
		},
	)

	if err := handler(context.Background(), tg.Entities{}, &tg.UpdateBotInlineQuery{UserID: 1}); err != nil {
		t.Fatalf("first handler() error = %v", err)
	}

	if err := handler(context.Background(), tg.Entities{}, &tg.UpdateBotInlineQuery{UserID: 1}); err != nil {
		t.Fatalf("second handler() error = %v", err)
	}

	waitForSignal(t, firstStarted, "first wrapped handler to start")
	assertNoSignal(t, secondStarted, 100*time.Millisecond, "second wrapped handler started before the first one completed")

	close(releaseFirst)

	waitForSignal(t, secondStarted, "second wrapped handler to start after the first one completed")
}

func waitForSignal(t *testing.T, ch <-chan struct{}, description string) {
	t.Helper()

	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for %s", description)
	}
}

func assertNoSignal(t *testing.T, ch <-chan struct{}, timeout time.Duration, description string) {
	t.Helper()

	select {
	case <-ch:
		t.Fatal(description)
	case <-time.After(timeout):
	}
}
