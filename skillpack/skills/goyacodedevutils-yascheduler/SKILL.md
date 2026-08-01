---
name: goyacodedevutils-yascheduler
description: Executor-side library of the yascheduler distributed job scheduler - typed function registry, remote TCP client and in-process local scheduler behind one Scheduler interface, job upserts with request/response result delivery, and routing-label revision. Use instead of hand-rolling cron loops, job queues, or RPC-over-queue plumbing.
---

# yascheduler Skill

Import path: `github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler` (sub-packages `protocol`, `store`, `store/memstore`, `engine`).

Executor-side library of the yascheduler distributed job scheduling system: register typed functions, run a `Scheduler`, submit jobs, await delivered results.

## Key API

- `Registry`; `NewRegistry()`; `RegisterFunction[A, R](registry, name, version, fn)` where `fn` is `func(ctx context.Context, args A) (R, error)`. Signatures derive once at registration; execution is a prepared closure (no reflection on the hot path). `NonRetryable(err)` marks a function error as consuming no retries; `Void` as `R` reports a valueless result.
- `Scheduler` interface: `Run`, `AwaitReady`, `UpsertJob`, `AnnounceLabels`, `WithdrawLabels`, `InstanceID`. Two implementations, same semantics — code moves between them without change:
  - `New(cfg *Config, registry, log) (*Client, yaerrors.Error)` — raw-TCP connection to the yascheduler service: heartbeats, jittered reconnect backoff, stable instance ID across reconnects.
  - `NewLocal(cfg *LocalConfig, registry, log) (*Local, yaerrors.Error)` — the full scheduling engine in process: no service, no socket; in-memory store by default, injectable `store.Store`.
- `UpsertJob(ctx, spec *JobSpec) (*Submission, yaerrors.Error)`. `JobSpec`: `Key`, `Function`, `Args`, `Schedule`, `Backfill`, `Retry`, `Overlap`, `Pin` (label pinning), `ResultMode`. Empty `Key` = RPC-style one-shot keyed by the minted job UUID.
- `Submission`: `JobUUID`, `Await(ctx) (*Result, yaerrors.Error)`, `Close()`. `Result`: `Success`, `HasValue`, `Payload`, `Cause`; decode with `DecodeResult[R](result)`.
- Labels: `AnnounceLabels`/`WithdrawLabels` revise the routing labels live (wire `LabelUpdate` round trip on `Client`, engine call on `Local`); jobs pinned via `JobSpec.Pin` route only to executors holding the label (strict) or preferably (preferred).

## Request/response usage

```go
submission, err := scheduler.UpsertJob(ctx, &yascheduler.JobSpec{
    Function:   protocol.FunctionSpec{Name: "report"},
    Args:       args,
    Schedule:   protocol.ScheduleSpec{Kind: protocol.ScheduleKindOneShot, StartUnixNano: time.Now().UnixNano()},
    ResultMode: protocol.ResultModeDeliver,
})
// handle err, then:
result, err := submission.Await(ctx)
// handle err, then:
value, err := yascheduler.DecodeResult[ReportResult](result)
```

- Default `ResultMode` is `protocol.ResultModeIgnore`: the result is discarded and `Await` answers `ErrResultNotRequested`.
- `ResultModeDeliver` is one-shot-only; the scheduler holds the result until acknowledged and redelivers across reconnects within its retention budget.
- Call `Close()` when abandoning a submission without awaiting; `Await` closes on every return path (one submission answers at most one result).
- A `Void`-returning function delivers `HasValue: false`; `DecodeResult` answers `ErrResultHasNoValue`.

## Usage Notes

- Execution and result delivery are at-least-once: functions may run more than once per occurrence — key external effects off `ExecutionID`; duplicate result deliveries are deduped client-side by the one-slot waiter buffer.
- Result waiters are keyed by job UUID and survive reconnects; correlation-scoped waits (upsert/label acks) fail with the connection.
- `AwaitReady` before the first `UpsertJob` when racing startup; label revisions made while disconnected apply locally and replay via registration.
- `Config` requires `Address` + `ExecutorType`; `LocalConfig` requires `ExecutorType`. Everything else defaults.
- Sub-packages: `protocol` (versioned binary wire format), `store`/`store/memstore` (persistence contract and in-memory implementation), `engine` (the scheduling engine `Local` embeds; the standalone service `YaCodeDevGoScheduler` drives the same engine over TCP).
