package storetest

import (
	"context"
	"testing"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/store"
)

// TestAttemptRepository runs the attempt conformance subtests against
// stores the factory builds: attempt creation, lookups, guarded state
// updates, and the execution and instance listings.
func TestAttemptRepository(t *testing.T, factory Factory) {
	t.Helper()

	t.Run(
		"when an attempt is created / then it starts dispatched",
		func(t *testing.T) {
			t.Parallel()

			const jobKey = store.JobKey("job-attempt-create")

			sut := factory(t)
			job := createJob(t, sut, jobKey)
			execution := createExecution(t, sut, job.ID, baseTime)

			attempt := createAttempt(t, sut, execution.ID, firstAttempt, suiteInstanceID)

			if attempt.ExecutionID != execution.ID {
				t.Errorf("the attempt should belong to its execution: got %d", attempt.ExecutionID)
			}

			if attempt.Number != firstAttempt {
				t.Errorf("the attempt should keep its ordinal: got %d", attempt.Number)
			}

			if attempt.InstanceID != suiteInstanceID {
				t.Errorf("the attempt should keep its instance: got %s", attempt.InstanceID)
			}

			if attempt.State != store.AttemptDispatched {
				t.Errorf("a created attempt should start dispatched: got %d", attempt.State)
			}

			if fetched := getAttempt(t, sut, attempt.ID); fetched.ExecutionID != execution.ID {
				t.Errorf("the stored attempt should be fetchable: got %d", fetched.ExecutionID)
			}
		},
	)

	t.Run(
		"when an attempt is created for an unknown execution / then the creation misses",
		func(t *testing.T) {
			t.Parallel()

			const unknownExecution = protocol.ExecutionID(404)

			sut := factory(t)

			_, err := sut.CreateAttempt(
				context.Background(),
				unknownExecution,
				firstAttempt,
				suiteInstanceID,
			)
			requireSentinel(
				t,
				err,
				store.ErrExecutionNotFound,
				"an attempt of an unknown execution must miss",
			)
		},
	)

	t.Run(
		"when an unknown attempt is fetched / then the lookup misses",
		func(t *testing.T) {
			t.Parallel()

			const unknownAttempt = protocol.AttemptID(404)

			sut := factory(t)

			_, err := sut.GetAttempt(context.Background(), unknownAttempt)
			requireSentinel(t, err, store.ErrAttemptNotFound, "an unknown attempt must miss")
		},
	)

	t.Run(
		"when the from states do not match / then the attempt is untouched",
		func(t *testing.T) {
			t.Parallel()

			const jobKey = store.JobKey("job-attempt-from")

			sut := factory(t)
			job := createJob(t, sut, jobKey)
			execution := createExecution(t, sut, job.ID, baseTime)
			attempt := createAttempt(t, sut, execution.ID, firstAttempt, suiteInstanceID)

			updated := updateAttemptState(
				t,
				sut,
				attempt.ID,
				[]store.AttemptState{store.AttemptAccepted},
				store.AttemptSucceeded,
				"",
			)

			if updated {
				t.Error("a mismatched from state should not update the attempt")
			}

			if fetched := getAttempt(t, sut, attempt.ID); fetched.State != store.AttemptDispatched {
				t.Errorf("a refused update should keep the state: got %d", fetched.State)
			}
		},
	)

	t.Run(
		"when the from state matches / then the attempt transitions and records the error",
		func(t *testing.T) {
			t.Parallel()

			const (
				jobKey      = store.JobKey("job-attempt-to")
				failureText = store.ErrorText("function exploded")
			)

			sut := factory(t)
			job := createJob(t, sut, jobKey)
			execution := createExecution(t, sut, job.ID, baseTime)
			attempt := createAttempt(t, sut, execution.ID, firstAttempt, suiteInstanceID)

			accepted := updateAttemptState(
				t,
				sut,
				attempt.ID,
				[]store.AttemptState{store.AttemptDispatched},
				store.AttemptAccepted,
				"",
			)

			if !accepted {
				t.Fatal("a matching from state should update the attempt")
			}

			failed := updateAttemptState(
				t,
				sut,
				attempt.ID,
				nil,
				store.AttemptFunctionFailed,
				failureText,
			)

			if !failed {
				t.Fatal("an unconditional update should apply")
			}

			fetched := getAttempt(t, sut, attempt.ID)

			if fetched.State != store.AttemptFunctionFailed {
				t.Errorf("the attempt should hold the new state: got %d", fetched.State)
			}

			if fetched.Error != failureText {
				t.Errorf("the attempt should record the error text: got %q", fetched.Error)
			}
		},
	)

	t.Run(
		"when an unknown attempt is updated / then the update misses",
		func(t *testing.T) {
			t.Parallel()

			const unknownAttempt = protocol.AttemptID(404)

			sut := factory(t)

			updated, err := sut.UpdateAttemptState(
				context.Background(),
				unknownAttempt,
				nil,
				store.AttemptAccepted,
				"",
			)
			requireSentinel(t, err, store.ErrAttemptNotFound, "an unknown attempt update must miss")

			if updated {
				t.Error("an unknown attempt update should not report an update")
			}
		},
	)

	t.Run(
		"when attempts are listed for an execution / then they return in creation order",
		func(t *testing.T) {
			t.Parallel()

			const (
				jobKey        = store.JobKey("job-attempt-list")
				secondAttempt = store.AttemptNumber(2)
				wantOwned     = 2
			)

			sut := factory(t)
			job := createJob(t, sut, jobKey)
			execution := createExecution(t, sut, job.ID, baseTime)
			foreignExecution := createExecution(t, sut, job.ID, baseTime.Add(time.Second))

			first := createAttempt(t, sut, execution.ID, firstAttempt, suiteInstanceID)
			foreign := createAttempt(t, sut, foreignExecution.ID, firstAttempt, otherInstanceID)
			second := createAttempt(t, sut, execution.ID, secondAttempt, suiteInstanceID)

			owned := attemptsForExecution(t, sut, execution.ID)

			if len(owned) != wantOwned {
				t.Fatalf("only the execution's attempts should return: got %d", len(owned))
			}

			if owned[0].ID != first.ID || owned[1].ID != second.ID {
				t.Errorf("the execution's attempts should keep creation order: got %v", owned)
			}

			others := attemptsForExecution(t, sut, foreignExecution.ID)

			if len(others) != 1 || others[0].ID != foreign.ID {
				t.Errorf("the sibling execution should keep its own attempt: got %v", others)
			}
		},
	)

	t.Run(
		"when attempts spread across instances / then filters select by instance and state",
		func(t *testing.T) {
			t.Parallel()

			const (
				jobKey        = store.JobKey("job-attempt-instance")
				secondAttempt = store.AttemptNumber(2)
				wantHeld      = 2
			)

			sut := factory(t)
			job := createJob(t, sut, jobKey)
			execution := createExecution(t, sut, job.ID, baseTime)

			dispatched := createAttempt(t, sut, execution.ID, firstAttempt, suiteInstanceID)
			accepted := createAttempt(t, sut, execution.ID, secondAttempt, suiteInstanceID)
			createAttempt(t, sut, execution.ID, firstAttempt, otherInstanceID)

			if !updateAttemptState(t, sut, accepted.ID, nil, store.AttemptAccepted, "") {
				t.Fatal("the accepted attempt update should apply")
			}

			all := attemptsOnInstance(t, sut, suiteInstanceID)

			if len(all) != wantHeld {
				t.Fatalf("the instance should hold two attempts: got %d", len(all))
			}

			if all[0].ID != dispatched.ID || all[1].ID != accepted.ID {
				t.Errorf("an unfiltered lookup should keep creation order: got %v", all)
			}

			onlyDispatched := attemptsOnInstance(t, sut, suiteInstanceID, store.AttemptDispatched)

			if len(onlyDispatched) != 1 || onlyDispatched[0].ID != dispatched.ID {
				t.Errorf(
					"a state filter should keep matching attempts only: got %v",
					onlyDispatched,
				)
			}

			if others := attemptsOnInstance(
				t,
				sut,
				otherInstanceID,
				store.AttemptAccepted,
			); len(others) != 0 {
				t.Errorf("a non-matching filter should return nothing: got %v", others)
			}
		},
	)

	t.Run(
		"when an attempt reaches a terminal state / then the instance listing drops it",
		func(t *testing.T) {
			t.Parallel()

			const jobKey = store.JobKey("job-attempt-terminal-prune")

			terminalStates := []store.AttemptState{
				store.AttemptSucceeded,
				store.AttemptFunctionFailed,
				store.AttemptInfraFailed,
				store.AttemptLost,
				store.AttemptCancelled,
			}

			sut := factory(t)
			job := createJob(t, sut, jobKey)

			for index, terminal := range terminalStates {
				execution := createExecution(
					t,
					sut,
					job.ID,
					baseTime.Add(time.Duration(index)*time.Second),
				)
				attempt := createAttempt(t, sut, execution.ID, firstAttempt, suiteInstanceID)

				if !updateAttemptState(t, sut, attempt.ID, nil, terminal, "") {
					t.Fatalf("the transition to state %d should apply", terminal)
				}

				if held := attemptsOnInstance(t, sut, suiteInstanceID); len(held) != 0 {
					t.Errorf(
						"a settled attempt should leave the instance listing for state %d: got %v",
						terminal,
						held,
					)
				}

				open := attemptsOnInstance(
					t,
					sut,
					suiteInstanceID,
					store.AttemptDispatched,
					store.AttemptAccepted,
				)
				if len(open) != 0 {
					t.Errorf(
						"a settled attempt should leave the open filter for state %d: got %v",
						terminal,
						open,
					)
				}

				if fetched := getAttempt(t, sut, attempt.ID); fetched.State != terminal {
					t.Errorf("the settled attempt should stay fetchable: got %d", fetched.State)
				}
			}
		},
	)

	t.Run(
		"when an attempt is accepted / then the instance listing keeps it",
		func(t *testing.T) {
			t.Parallel()

			const jobKey = store.JobKey("job-attempt-accept-keep")

			sut := factory(t)
			job := createJob(t, sut, jobKey)
			execution := createExecution(t, sut, job.ID, baseTime)
			attempt := createAttempt(t, sut, execution.ID, firstAttempt, suiteInstanceID)

			if !updateAttemptState(
				t,
				sut,
				attempt.ID,
				[]store.AttemptState{store.AttemptDispatched},
				store.AttemptAccepted,
				"",
			) {
				t.Fatal("the accepted attempt update should apply")
			}

			if held := attemptsOnInstance(t, sut, suiteInstanceID); len(held) != 1 ||
				held[0].ID != attempt.ID {
				t.Errorf("a non-terminal transition should keep the attempt listed: got %v", held)
			}

			open := attemptsOnInstance(
				t,
				sut,
				suiteInstanceID,
				store.AttemptDispatched,
				store.AttemptAccepted,
			)
			if len(open) != 1 || open[0].ID != attempt.ID {
				t.Errorf("the open filter should keep the accepted attempt: got %v", open)
			}
		},
	)

	t.Run(
		"when a stored attempt is deleted / then it is gone",
		func(t *testing.T) {
			t.Parallel()

			const jobKey = store.JobKey("job-attempt-delete")

			sut := factory(t)
			job := createJob(t, sut, jobKey)
			execution := createExecution(t, sut, job.ID, baseTime)
			attempt := createAttempt(t, sut, execution.ID, firstAttempt, suiteInstanceID)

			if !deleteAttempt(t, sut, attempt.ID) {
				t.Fatal("a stored attempt delete should report true")
			}

			_, err := sut.GetAttempt(context.Background(), attempt.ID)
			requireSentinel(t, err, store.ErrAttemptNotFound, "the deleted attempt should be gone")
		},
	)

	t.Run(
		"when an attempt is deleted twice / then the second delete reports false",
		func(t *testing.T) {
			t.Parallel()

			const jobKey = store.JobKey("job-attempt-delete-twice")

			sut := factory(t)
			job := createJob(t, sut, jobKey)
			execution := createExecution(t, sut, job.ID, baseTime)
			attempt := createAttempt(t, sut, execution.ID, firstAttempt, suiteInstanceID)

			if !deleteAttempt(t, sut, attempt.ID) {
				t.Fatal("the first delete should report true")
			}

			if deleteAttempt(t, sut, attempt.ID) {
				t.Error("a replayed delete should report false")
			}
		},
	)

	t.Run(
		"when an unknown attempt is deleted / then the delete reports false",
		func(t *testing.T) {
			t.Parallel()

			const unknownAttempt = protocol.AttemptID(404)

			sut := factory(t)

			if deleteAttempt(t, sut, unknownAttempt) {
				t.Error("deleting an unknown attempt should report false")
			}
		},
	)

	t.Run(
		"when one of several attempts is deleted / then its siblings survive",
		func(t *testing.T) {
			t.Parallel()

			const (
				jobKey        = store.JobKey("job-attempt-delete-siblings")
				secondAttempt = store.AttemptNumber(2)
			)

			sut := factory(t)
			job := createJob(t, sut, jobKey)
			execution := createExecution(t, sut, job.ID, baseTime)

			first := createAttempt(t, sut, execution.ID, firstAttempt, suiteInstanceID)
			second := createAttempt(t, sut, execution.ID, secondAttempt, suiteInstanceID)

			if !deleteAttempt(t, sut, first.ID) {
				t.Fatal("the delete should report true")
			}

			owned := attemptsForExecution(t, sut, execution.ID)

			if len(owned) != 1 || owned[0].ID != second.ID {
				t.Errorf("the surviving attempt should remain listed: got %v", owned)
			}

			held := attemptsOnInstance(t, sut, suiteInstanceID)

			if len(held) != 1 || held[0].ID != second.ID {
				t.Errorf("the instance listing should drop the deleted attempt: got %v", held)
			}
		},
	)
}
