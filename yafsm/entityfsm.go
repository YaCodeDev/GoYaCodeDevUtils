package yafsm

import (
	"context"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
)

// EntityFSMStorage is a wrapper over FSM to work with specific entity (user, chat, etc).
type EntityFSMStorage struct {
	storage FSM
	uid     string
}

// NewUserFSMStorage creates a new EntityFSMStorage for a specific user ID.
func NewUserFSMStorage(
	storage FSM,
	uid string,
) *EntityFSMStorage {
	return &EntityFSMStorage{
		storage: storage,
		uid:     uid,
	}
}

// SetState sets the state for the entity.
//
// Example usage:
//
// err := userFSMStorage.SetState(ctx, &SomeState{Field: "value"})
//
//	if err != nil {
//	    // handle error
//	}
func (b *EntityFSMStorage) SetState(
	ctx context.Context,
	stateData State,
) yaerrors.Error {
	return b.storage.SetState(ctx, b.uid, stateData)
}

// GetState retrieves the current state and its data for the entity.
//
// Example usage:
//
// stateName, stateData, err := userFSMStorage.GetState(ctx)
//
//	if err != nil {
//	    // handle error
//	}
func (b *EntityFSMStorage) GetState(
	ctx context.Context,
) (string, StateDataMarshalled, yaerrors.Error) {
	return b.storage.GetState(ctx, b.uid)
}

// GetStateData unmarshals the state data into the provided empty state struct.
//
// Example usage:
//
// var stateData SomeState
//
// err := userFSMStorage.GetStateData(marshalledData, &stateData)
//
//	if err != nil {
//	    // handle error
//	}
func (b *EntityFSMStorage) GetStateData(
	stateData StateDataMarshalled,
	emptyState State,
) yaerrors.Error {
	return b.storage.GetStateData(stateData, emptyState)
}
