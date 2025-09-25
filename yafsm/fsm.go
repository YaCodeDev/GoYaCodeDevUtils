package yafsm

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yacache"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
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

type StateDataMarshalled string

type StateAndData struct {
	State     string `json:"state"`
	StateData string `json:"stateData"`
}

type FSM interface {
	SetState(ctx context.Context, uid string, state State) yaerrors.Error
	GetState(ctx context.Context, uid string) (string, StateDataMarshalled, yaerrors.Error)
	GetStateData(stateData StateDataMarshalled, emptyState State) yaerrors.Error
}

type DefaultFSMStorage[T yacache.Container] struct {
	storage      yacache.Cache[T]
	defaultState State
}

func NewDefaultFSMStorage[T yacache.Container](
	storage yacache.Cache[T],
	defaultState State,
) *DefaultFSMStorage[T] {
	return &DefaultFSMStorage[T]{
		storage:      storage,
		defaultState: defaultState,
	}
}

func (b *DefaultFSMStorage[T]) SetState(
	ctx context.Context,
	uid string,
	stateData State,
) yaerrors.Error {
	val, err := json.Marshal(stateData)
	if err != nil {
		return yaerrors.FromError(
			http.StatusInternalServerError,
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
			http.StatusInternalServerError,
			err,
			"failed to marshal state data",
		)
	}

	return b.storage.Set(ctx, uid, string(val), 0)
}

func (b *DefaultFSMStorage[T]) GetState(
	ctx context.Context,
	uid string,
) (string, StateDataMarshalled, yaerrors.Error) {
	data, err := b.storage.Get(ctx, uid)
	if err != nil {
		return b.defaultState.StateName(), "", nil
	}

	var stateAndData map[string]string

	if err := json.Unmarshal([]byte(data), &stateAndData); err != nil {
		return "", "", yaerrors.FromError(
			http.StatusInternalServerError,
			err,
			"failed to unmarshal state data map",
		)
	}

	state, ok := stateAndData[stateKey]

	if !ok {
		return "", "", yaerrors.FromString(
			http.StatusNotFound,
			"failed to get state",
		)
	}

	return state, StateDataMarshalled(data), nil
}

func (b *DefaultFSMStorage[T]) GetStateData(
	stateData StateDataMarshalled,
	emptyState State,
) yaerrors.Error {
	if stateData == "" {
		return nil
	}

	var stateAndData map[string]string

	if err := json.Unmarshal([]byte(stateData), &stateAndData); err != nil {
		return yaerrors.FromError(
			http.StatusInternalServerError,
			err,
			"failed to unmarshal state data map",
		)
	}

	stateDataMarshalled, ok := stateAndData[stateDataKey]

	if !ok {
		return yaerrors.FromString(
			http.StatusNotFound,
			"failed to get state data",
		)
	}

	if err := json.Unmarshal([]byte(stateDataMarshalled), emptyState); err != nil {
		return yaerrors.FromError(
			http.StatusInternalServerError,
			err,
			"failed to unmarshal state data",
		)
	}

	return nil
}
