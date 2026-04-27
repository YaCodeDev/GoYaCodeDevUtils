package yatgbot

import (
	"context"

	"github.com/gotd/td/telegram/updates"
)

type channelStateForgettingStorage struct {
	updates.StateStorage
}

func (s channelStateForgettingStorage) GetChannelPts(
	_ context.Context,
	_, _ int64,
) (pts int, found bool, err error) {
	return 0, false, nil
}

func (s channelStateForgettingStorage) ForEachChannels(
	_ context.Context,
	_ int64,
	_ func(ctx context.Context, channelID int64, pts int) error,
) error {
	return nil
}
