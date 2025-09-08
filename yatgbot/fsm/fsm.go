package fsm

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yacache"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/redis/go-redis/v9"
)

type State interface {
	StateName() string
}

type BaseState[T State] struct{}

func (BaseState[T]) StateName() string {
	var zero T
	t := reflect.TypeOf(zero)
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	return t.Name()
}

type EmptyState struct {
	BaseState[EmptyState]
}

type stateDataMarshalled string

type StateAndData struct {
	State     string `json:"state"`
	StateData string `json:"state_data"`
}

type FSM interface {
	SetState(ctx context.Context, uid string, state State) yaerrors.Error
	GetState(ctx context.Context, uid string) (string, stateDataMarshalled, yaerrors.Error)
	GetStateData(stateData stateDataMarshalled, emptyState State) yaerrors.Error
}

type DefaultFSMStorage struct {
	storage      yacache.Cache[*redis.Client]
	defaultState State
}

func NewDefaultFSMStorage(
	storage yacache.Cache[*redis.Client],
	defaultState State,
) *DefaultFSMStorage {
	return &DefaultFSMStorage{
		storage:      storage,
		defaultState: defaultState,
	}
}

func (b *DefaultFSMStorage) SetState(
	ctx context.Context,
	uid string,
	stateData State,
) yaerrors.Error {
	val, err := json.Marshal(stateData)

	if err != nil {
		return yaerrors.FromError(
			500,
			err,
			"failed to marshal state data",
		)
	}

	val, err = json.Marshal(StateAndData{
		State:     stateData.StateName(),
		StateData: string(val),
	})

	if err != nil {
		return yaerrors.FromError(
			500,
			err,
			"failed to marshal state data",
		)
	}

	b.storage.Set(ctx, uid, string(val), 0)

	return nil
}

func (b *DefaultFSMStorage) GetState(
	ctx context.Context,
	uid string,
) (string, stateDataMarshalled, yaerrors.Error) {
	data, err := b.storage.Get(ctx, uid)
	if err != nil {
		return b.defaultState.StateName(), "", nil
	}

	var stateAndData map[string]string

	if err := json.Unmarshal([]byte(data), &stateAndData); err != nil {
		return "", "", yaerrors.FromError(
			500,
			err,
			"failed to unmarshal state data map",
		)
	}

	state, ok := stateAndData["state"]

	if !ok {
		return "", "", yaerrors.FromError(
			404,
			fmt.Errorf("state not found"),
			"failed to get state",
		)
	}
	return state, stateDataMarshalled(data), nil
}

func (b *DefaultFSMStorage) GetStateData(
	stateData stateDataMarshalled,
	emptyState State,
) yaerrors.Error {
	if stateData == "" {
		return nil
	}

	var stateAndData map[string]string

	if err := json.Unmarshal([]byte(stateData), &stateAndData); err != nil {
		return yaerrors.FromError(
			500,
			err,
			"failed to unmarshal state data map",
		)
	}

	stateDataMarshalled, ok := stateAndData["state_data"]

	if !ok {
		return yaerrors.FromError(
			404,
			fmt.Errorf("state data not found"),
			"failed to get state data",
		)
	}

	if err := json.Unmarshal([]byte(stateDataMarshalled), emptyState); err != nil {
		return yaerrors.FromError(
			500,
			err,
			"failed to unmarshal state data",
		)
	}

	return nil
}
