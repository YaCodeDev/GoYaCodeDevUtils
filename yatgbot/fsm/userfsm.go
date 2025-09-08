package fsm

import (
	"context"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
)

type UserFSMStorage struct {
	storage FSM
	uid     string
}

func NewUserFSMStorage(
	storage FSM,
	uid string,
) *UserFSMStorage {
	return &UserFSMStorage{
		storage: storage,
		uid:     uid,
	}
}

func (b *UserFSMStorage) SetState(
	ctx context.Context,
	stateData State,
) yaerrors.Error {
	return b.storage.SetState(ctx, b.uid, stateData)
}

func (b *UserFSMStorage) GetState(
	ctx context.Context,
) (string, stateDataMarshalled, yaerrors.Error) {
	return b.storage.GetState(ctx, b.uid)
}

func (b *UserFSMStorage) GetStateData(
	stateData stateDataMarshalled,
	emptyState State,
) yaerrors.Error {
	return b.storage.GetStateData(stateData, emptyState)
}
