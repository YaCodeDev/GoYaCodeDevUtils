---
name: goyacodedevutils-yascheduler
description: Executor-side library of the yascheduler distributed job scheduler - typed function registry, remote TCP client and in-process local scheduler behind one Scheduler interface, job upserts with request/response result delivery, and routing-label revision. Use instead of hand-rolling cron loops, job queues, or RPC-over-queue plumbing.
---

# yascheduler Skill

Import path: `github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler` (sub-packages `protocol`, `store`, `store/memstore`, `store/redisstore`, `engine`).

Executor-side library of the yascheduler distributed job scheduling system: register typed functions, run a `Scheduler`, submit jobs, await delivered results.

## Key API

- `Registry`; `NewRegistry()`; `RegisterFunction[A, R](registry, name, version, fn)` where `fn` is `func(ctx context.Context, args A) (R, error)`. Signatures derive once at registration; execution is a prepared closure (no reflection on the hot path). `NonRetryable(err)` marks a function error as consuming no retries; `Void` as `R` reports a valueless result.
- `Scheduler` interface: `Run`, `AwaitReady`, `UpsertJob`, `DeleteJob`, `AnnounceLabels`, `WithdrawLabels`, `InstanceID`. Two implementations, same semantics — code moves between them without change:
  - `New(cfg *Config, registry, log) (*Client, yaerrors.Error)` — raw-TCP connection to the yascheduler service: heartbeats, jittered reconnect backoff, stable instance ID across reconnects.
  - `NewLocal(cfg *LocalConfig, registry, log) (*Local, yaerrors.Error)` — the full scheduling engine in process: no service, no socket; in-memory store by default, injectable `store.Store` (`store/redisstore` for restart survival).
- `UpsertJob(ctx, spec *JobSpec) (*Submission, yaerrors.Error)`. `JobSpec`: `Key`, `Function`, `Args`, `Schedule`, `Backfill`, `Retry`, `Overlap`, `Pin` (label pinning), `ResultMode`. Empty `Key` = RPC-style one-shot keyed by the minted job UUID.
- `DeleteJob(ctx, executorType, key) (bool, yaerrors.Error)` withdraws the job addressed by `(executorType, key)`; empty `executorType` = the scheduler's own. Pending occurrences are cancelled, a held result is dropped, and the key is freed for a fresh job; running work finishes on its own. An absent job answers `false` with no error, so replays are idempotent (wire `JobDelete`/`JobDeleteAck`, protocol version 3).
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

## Persistence

- `Local` defaults to `store/memstore`: every job dies with the process. Set `LocalConfig.Store` to persist.
- `redisstore.NewStore(client *redis.Client, cfg redisstore.Config) *redisstore.Store` — a `store.Store` on a redis-protocol backend over an already-configured `go-redis` client. `Config` (zero fields take package defaults): `KeyPrefix` (namespaces every key), `MaxResults`/`MaxResultsPerInstance` (pending-result caps).
- Dragonfly-compatible by design: hashes, sorted sets, sets, strings, INCR, and EVAL lua only — no modules, no keyspace notifications, no key expiry (the engine drives retention itself). Multi-step invariants run as single lua scripts, so concurrent schedulers over one backend never observe a half-applied write.
- What survives a restart: jobs (a rebuilt `Local` over the same backend fires them without a re-upsert), executions and attempts (interrupted work is abandoned into redispatch), held results (redelivered or expired per the engine's retention budget), and deletes (a withdrawn key stays gone).
- What does not: result waiters are in-memory per runtime — a pending `Await` dies with its process even though the scheduler-side held result survives.

## Usage Notes

- Execution and result delivery are at-least-once: functions may run more than once per occurrence — key external effects off `ExecutionID`; duplicate result deliveries are deduped client-side by the one-slot waiter buffer.
- Result waiters are keyed by job UUID and survive reconnects; correlation-scoped waits (upsert/label acks) fail with the connection.
- `AwaitReady` before the first `UpsertJob` when racing startup; label revisions made while disconnected apply locally and replay via registration.
- `Config` requires `Address` + `ExecutorType`; `LocalConfig` requires `ExecutorType`. Everything else defaults.
- Sub-packages: `protocol` (versioned binary wire format), `store`/`store/memstore`/`store/redisstore` (persistence contract, in-memory and redis-protocol implementations), `engine` (the scheduling engine `Local` embeds; the standalone service `YaCodeDevGoScheduler` drives the same engine over TCP).
