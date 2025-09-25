package yafsm

import (
	"context"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
)

type EntityFSMStorage struct {
	storage FSM
	uid     string
}

func NewUserFSMStorage(
	storage FSM,
	uid string,
) *EntityFSMStorage {
	return &EntityFSMStorage{
		storage: storage,
		uid:     uid,
	}
}

func (b *EntityFSMStorage) SetState(
	ctx context.Context,
	stateData State,
) yaerrors.Error {
	return b.storage.SetState(ctx, b.uid, stateData)
}

func (b *EntityFSMStorage) GetState(
	ctx context.Context,
) (string, StateDataMarshalled, yaerrors.Error) {
	return b.storage.GetState(ctx, b.uid)
}

func (b *EntityFSMStorage) GetStateData(
	stateData StateDataMarshalled,
	emptyState State,
) yaerrors.Error {
	return b.storage.GetStateData(stateData, emptyState)
}
