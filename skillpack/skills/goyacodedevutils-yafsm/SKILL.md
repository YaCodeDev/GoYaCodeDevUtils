---
name: goyacodedevutils-yafsm
description: Finite-state-machine storage abstraction (state plus JSON-marshalled state data) on top of yacache, keyed per-entity. Use for per-user/per-chat conversational or workflow state instead of hand-rolled state tracking.
---

# yafsm Skill

Import path: `github.com/YaCodeDev/GoYaCodeDevUtils/yafsm`.

Finite-state-machine storage abstraction (state + JSON-marshalled state data) backed by `yacache`, keyed
per-entity (e.g. per user or chat).

## Key API

- `State` interface — `StateName() string`.
- `BaseState[T State]` struct — embed it to auto-derive `StateName` from the concrete type's name via reflection.
- `EmptyState` struct — default/no-op state.
- `StateAndData` struct — `{ State, StateData string }`.
- `FSM` interface — `SetState`, `GetState`, `GetStateData`.
- `DefaultFSMStorage[T yacache.Container]` struct + `NewDefaultFSMStorage[T](storage yacache.Cache[T], defaultState State) *DefaultFSMStorage[T]`.
- `EntityFSMStorage` struct (per-uid wrapper) + `NewUserFSMStorage(storage FSM, uid string) *EntityFSMStorage`.

## Usage Notes

- `SetState` marshals the whole state struct to JSON, wraps it as `{state, stateData}`, and stores it via `Cache.Set` with no TTL. `GetState` falls back to `defaultState.StateName()` (with a nil error) when the key isn't found.
- Define your own states by embedding `yafsm.BaseState[YourStateType]` so `StateName()` reflects the concrete type.
- Depends on `yacache` + `yaerrors`; used by `yatgbot` for per-user conversation state and its `StateIs` filter.
