package yatgbot

import (
	"context"
	"errors"
	"testing"

	"github.com/gotd/td/telegram/updates"
)

func TestChannelStateForgettingStorageIgnoresStoredChannels(t *testing.T) {
	ctx := context.Background()
	storage := channelStateForgettingStorage{StateStorage: &channelStateTestStorage{}}

	pts, found, err := storage.GetChannelPts(ctx, 1, 2)
	if err != nil {
		t.Fatalf("GetChannelPts() error = %v", err)
	}
	if pts != 0 || found {
		t.Fatalf("GetChannelPts() = (%d, %t), want (0, false)", pts, found)
	}

	called := false
	if err := storage.ForEachChannels(ctx, 1, func(context.Context, int64, int) error {
		called = true
		return nil
	}); err != nil {
		t.Fatalf("ForEachChannels() error = %v", err)
	}
	if called {
		t.Fatal("ForEachChannels() called callback, want stored channel states ignored")
	}
}

func TestChannelStateForgettingStorageDelegatesUserState(t *testing.T) {
	ctx := context.Background()
	base := &channelStateTestStorage{}
	storage := channelStateForgettingStorage{StateStorage: base}
	state := updates.State{Pts: 10, Qts: 20, Date: 30, Seq: 40}

	if err := storage.SetState(ctx, 1, state); err != nil {
		t.Fatalf("SetState() error = %v", err)
	}

	got, found, err := storage.GetState(ctx, 1)
	if err != nil {
		t.Fatalf("GetState() error = %v", err)
	}
	if !found {
		t.Fatal("GetState() found = false, want true")
	}
	if got != state {
		t.Fatalf("GetState() = %+v, want %+v", got, state)
	}
}

type channelStateTestStorage struct {
	state updates.State
	found bool
}

func (s *channelStateTestStorage) GetState(context.Context, int64) (updates.State, bool, error) {
	return s.state, s.found, nil
}

func (s *channelStateTestStorage) SetState(_ context.Context, _ int64, state updates.State) error {
	s.state = state
	s.found = true

	return nil
}

func (s *channelStateTestStorage) SetPts(context.Context, int64, int) error {
	return nil
}

func (s *channelStateTestStorage) SetQts(context.Context, int64, int) error {
	return nil
}

func (s *channelStateTestStorage) SetDate(context.Context, int64, int) error {
	return nil
}

func (s *channelStateTestStorage) SetSeq(context.Context, int64, int) error {
	return nil
}

func (s *channelStateTestStorage) SetDateSeq(context.Context, int64, int, int) error {
	return nil
}

func (s *channelStateTestStorage) GetChannelPts(context.Context, int64, int64) (int, bool, error) {
	return 777, true, nil
}

func (s *channelStateTestStorage) SetChannelPts(context.Context, int64, int64, int) error {
	return nil
}

func (s *channelStateTestStorage) ForEachChannels(
	ctx context.Context,
	userID int64,
	f func(context.Context, int64, int) error,
) error {
	if err := f(ctx, userID+1, 777); err != nil {
		return errors.Join(err, errors.New("callback failed"))
	}

	return nil
}
