package yafsm

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yacache"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
)

// State is an interface that all states must implement.
type State interface {
	StateName() string
}

// BaseState provides a default implementation of the State interface.
type BaseState[T State] struct{}

// StateName returns the name of the state type.
func (BaseState[T]) StateName() string {
	var zero T

	t := reflect.TypeOf(zero)
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	return t.Name()
}

// Empty state is implementation of State interface with no data.
type EmptyState struct {
	BaseState[EmptyState]
}

// StateDataMarshalled is a type alias for marshalled state data.
type StateDataMarshalled string

// StateAndData is a struct that holds the state name and its marshalled data.
type StateAndData struct {
	State     string `json:"state"`
	StateData string `json:"stateData"`
}

// FSM is an interface for finite state machine storage.\
type FSM interface {
	SetState(ctx context.Context, uid string, state State) yaerrors.Error
	GetState(ctx context.Context, uid string) (string, StateDataMarshalled, yaerrors.Error)
	GetStateData(stateData StateDataMarshalled, emptyState State) yaerrors.Error
}

// DefaultFSMStorage is a default implementation of the FSM interface using yacache.
type DefaultFSMStorage[T yacache.Container] struct {
	storage      yacache.Cache[T]
	defaultState State
}

// NewDefaultFSMStorage creates a new instance of DefaultFSMStorage.
func NewDefaultFSMStorage[T yacache.Container](
	storage yacache.Cache[T],
	defaultState State,
) *DefaultFSMStorage[T] {
	return &DefaultFSMStorage[T]{
		storage:      storage,
		defaultState: defaultState,
	}
}

// SetState sets the state for a given user ID.
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

// GetState retrieves the current state and its marshalled data for a given user ID.
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

// GetStateData unmarshals the state data into the provided empty state struct.
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
