package yatgstorage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yacache"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/gotd/td/telegram/updates"
	"github.com/redis/go-redis/v9"
)

const BaseStructJSONRedis = "$"

type IStorage interface {
	updates.StateStorage
	updates.ChannelAccessHasher

	Ping(ctx context.Context) yaerrors.Error
}

type Storage struct {
	cache yacache.Cache[*redis.Client]
	botID int
}

func NewStorage(cache yacache.Cache[*redis.Client], botID int) *Storage {
	return &Storage{
		cache: cache,
		botID: botID,
	}
}

func (s *Storage) Ping(ctx context.Context) yaerrors.Error {
	return s.cache.Ping(ctx)
}

func (s *Storage) GetState(ctx context.Context, userID int64) (updates.State, bool, error) {
	data, yaerr := s.cache.Raw().JSONGet(ctx, getKey(s.botID, userID)).Result()
	if yaerr != nil {
		return updates.State{}, false, errors.Join(yaerr, ErrFailedToGetState)
	}

	var state updates.State

	err := json.Unmarshal([]byte(data), &state)
	if err != nil {
		errors.Join(err, ErrFailedToUnmarshalState)
	}

	return state, true, nil
}

func (s *Storage) SetState(ctx context.Context, userID int64, state updates.State) error {
	if err := s.cache.Raw().JSONSet(ctx, getKey(s.botID, userID), BaseStructJSONRedis, state).Err(); err != nil {
		return errors.Join(err, ErrFailedToSetState)
	}

	return nil
}

func (s *Storage) SetPts(ctx context.Context, userID int64, pts int) error {
	return nil
}

func (s *Storage) SetQts(ctx context.Context, userID int64, qts int) error {
	return nil
}

func (s *Storage) SetDate(ctx context.Context, userID int64, date int) error {
	return nil
}

func (s *Storage) SetSeq(ctx context.Context, userID int64, seq int) error {
	return nil
}

func (s *Storage) SetDateSeq(ctx context.Context, userID int64, date, seq int) error {
	return nil
}

func (s *Storage) GetChannelPts(ctx context.Context, userID, channelID int64) (pts int, found bool, err error) {
	return 0, false, nil
}

func (s *Storage) SetChannelPts(ctx context.Context, userID, channelID int64, pts int) error {
	return nil
}

func (s *Storage) ForEachChannels(ctx context.Context, userID int64, f func(ctx context.Context, channelID int64, pts int) error) error {
	return nil
}

func (s *Storage) SetChannelAccessHash(ctx context.Context, userID, channelID, accessHash int64) error {
	return nil
}

func (s *Storage) GetChannelAccessHash(ctx context.Context, userID, channelID int64) (accessHash int64, found bool, err error) {
	return 0, false, nil
}

func getKey(botID int, userID int64) string {
	return fmt.Sprintf("bot-storage:%d-%d", botID, userID)
}
