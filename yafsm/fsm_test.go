package yafsm_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yacache"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yafsm"
)

type ExampleState struct {
	yafsm.BaseState[ExampleState]

	Param string `json:"param"`
}

func TestFSMStorage_SetGetRoundTrip(t *testing.T) {
	ctx := context.Background()

	cache := yacache.NewCache(yacache.NewMemoryContainer())

	fsm := yafsm.NewDefaultFSMStorage(cache, yafsm.EmptyState{})

	uid := "12345"
	wantParam := "exampleparam"

	// 1) set
	if err := fsm.SetState(ctx, uid, ExampleState{Param: wantParam}); err != nil {
		t.Fatalf("SetState failed: %v", err)
	}

	// 2) get state name + raw payload
	stateName, raw, err := fsm.GetState(ctx, uid)
	if err != nil {
		t.Fatalf("GetState failed: %v", err)
	}

	if stateName != (ExampleState{}).StateName() {
		t.Fatalf("unexpected state name: want %q, got %q",
			(ExampleState{}).StateName(), stateName)
	}

	// 3) unmarshal into struct
	var got ExampleState
	if err := fsm.GetStateData(raw, &got); err != nil {
		t.Fatalf("GetStateData failed: %v", err)
	}

	if got.Param != wantParam {
		t.Fatalf("unexpected param: want %q, got %q", wantParam, got.Param)
	}
}

func TestFSMStorage_DefaultStateReturned(t *testing.T) {
	ctx := context.Background()

	cache := yacache.NewCache(yacache.NewMemoryContainer())
	fsm := yafsm.NewDefaultFSMStorage(cache, yafsm.EmptyState{})

	uid := "non-existent"

	name, raw, err := fsm.GetState(ctx, uid)
	if err != nil {
		t.Fatalf("GetState failed: %v", err)
	}

	if name != (yafsm.EmptyState{}).StateName() {
		t.Fatalf("expected default state name %q, got %q",
			(yafsm.EmptyState{}).StateName(), name)
	}

	if raw != "" {
		t.Fatalf("expected empty raw data, got %q", raw)
	}
}

func TestFSMStorage_CorruptedPayload(t *testing.T) {
	ctx := context.Background()

	cache := yacache.NewCache(yacache.NewMemoryContainer())
	fsm := yafsm.NewDefaultFSMStorage(cache, yafsm.EmptyState{})

	uid := "bad:user"

	err := cache.Set(ctx, uid, "{not:a:json}", 0)
	if err != nil {
		t.Fatalf("failed to set corrupted data: %v", err)
	}

	_, _, err = fsm.GetState(ctx, uid)
	if err == nil {
		t.Fatal("expected error on corrupted JSON, got nil")
	}

	var syntaxErr *json.SyntaxError
	if !errors.As(err, &syntaxErr) {
		t.Fatalf("expected json.SyntaxError, got %v", err)
	}
}
