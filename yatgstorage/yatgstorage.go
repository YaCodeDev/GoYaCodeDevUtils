package yatgstorage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yacache"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/gotd/td/telegram/updates"
	"github.com/redis/go-redis/v9"
)

const (
	BasePathRedisJSON = "$"
	PtsPathRedisJSON  = BasePathRedisJSON + ".Pts"
	QtsPathRedisJSON  = BasePathRedisJSON + ".Qts"
	DatePathRedisJSON = BasePathRedisJSON + ".Date"
	SeqPathRedisJSON  = BasePathRedisJSON + ".Seq"

	AccessHashFieldRedisHSet = "AccessHash"
	PtsFieldRedisHSet        = "Pts"
)

type IStorage interface {
	updates.StateStorage
	updates.ChannelAccessHasher

	Ping(ctx context.Context) yaerrors.Error
}

type Storage struct {
	cache yacache.Cache[*redis.Client]
}

func NewStorage(cache yacache.Cache[*redis.Client]) *Storage {
	return &Storage{
		cache: cache,
	}
}

func (s *Storage) Ping(ctx context.Context) yaerrors.Error {
	return s.cache.Ping(ctx)
}

func (s *Storage) GetState(ctx context.Context, userID int64) (updates.State, bool, error) {
	data, yaerr := s.cache.Raw().JSONGet(ctx, getBotStorageKey(userID)).Result()
	if yaerr != nil {
		return updates.State{}, false, errors.Join(yaerr, ErrFailedToGetState)
	}

	var state updates.State

	err := json.Unmarshal([]byte(data), &state)
	if err != nil {
		return state, false, errors.Join(err, ErrFailedToUnmarshalState)
	}

	return state, true, nil
}

func (s *Storage) SetState(ctx context.Context, userID int64, state updates.State) error {
	if err := s.cache.Raw().JSONSet(ctx, getBotStorageKey(userID), BasePathRedisJSON, state).Err(); err != nil {
		return errors.Join(err, ErrFailedToSetState)
	}

	return nil
}

func (s *Storage) SetPts(ctx context.Context, userID int64, pts int) error {
	if err := s.cache.Raw().JSONSet(ctx, getBotStorageKey(userID), PtsPathRedisJSON, pts).Err(); err != nil {
		return errors.Join(err, ErrFailedToSetPts)
	}

	return nil
}

func (s *Storage) SetQts(ctx context.Context, userID int64, qts int) error {
	if err := s.cache.Raw().JSONSet(ctx, getBotStorageKey(userID), QtsPathRedisJSON, qts).Err(); err != nil {
		return errors.Join(err, ErrFailedToSetQts)
	}

	return nil
}

func (s *Storage) SetDate(ctx context.Context, userID int64, date int) error {
	if err := s.cache.Raw().JSONSet(ctx, getBotStorageKey(userID), DatePathRedisJSON, date).Err(); err != nil {
		return errors.Join(err, ErrFailedToSetDate)
	}

	return nil
}

func (s *Storage) SetSeq(ctx context.Context, userID int64, seq int) error {
	if err := s.cache.Raw().JSONSet(ctx, getBotStorageKey(userID), SeqPathRedisJSON, seq).Err(); err != nil {
		return errors.Join(err, ErrFailedToSetSeq)
	}

	return nil
}

func (s *Storage) SetDateSeq(ctx context.Context, userID int64, date, seq int) error {
	key := getBotStorageKey(userID)
	if err := s.cache.Raw().
		JSONMSet(ctx, key, DatePathRedisJSON, date, key, SeqPathRedisJSON, seq).Err(); err != nil {
		return errors.Join(err, ErrFailedToSetDateSeq)
	}

	return nil
}

func (s *Storage) SetChannelPts(ctx context.Context, userID, channelID int64, pts int) error {
	if err := s.cache.Raw().
		HSet(ctx, getChannelStorageKey(userID, channelID), PtsFieldRedisHSet, pts).Err(); err != nil {
		return errors.Join(err, ErrFailedToSetChannelPts)
	}

	return nil
}

func (s *Storage) GetChannelPts(ctx context.Context, userID, channelID int64) (int, bool, error) {
	data, yaerr := s.cache.HGet(ctx, getChannelStorageKey(userID, channelID), PtsFieldRedisHSet)
	if yaerr != nil {
		return 0, false, errors.Join(yaerr, ErrFailedToGetChannelPts)
	}

	res, err := strconv.ParseInt(data, 10, 0)
	if err != nil {
		return 0, false, errors.Join(yaerr, ErrFailedToParsePtsAsInt)
	}

	return int(res), true, nil
}

func (s *Storage) ForEachChannels(
	_ context.Context,
	_ int64,
	_ func(ctx context.Context, channelID int64, pts int) error,
) error {
	return nil
}

func (s *Storage) SetChannelAccessHash(ctx context.Context, userID, channelID, accessHash int64) error {
	if err := s.cache.Raw().
		HSet(ctx, getChannelStorageKey(userID, channelID), AccessHashFieldRedisHSet, accessHash).Err(); err != nil {
		return errors.Join(err, ErrFailedToSetChannelAccessHash)
	}

	return nil
}

func (s *Storage) GetChannelAccessHash(ctx context.Context, userID, channelID int64) (int64, bool, error) {
	data, err := s.cache.Raw().
		HGet(ctx, getChannelStorageKey(userID, channelID), AccessHashFieldRedisHSet).Result()
	if err != nil {
		return 0, false, errors.Join(err, ErrFailedToGetChannelAccessHash)
	}

	res, err := strconv.ParseInt(data, 10, 64)
	if err != nil {
		return 0, false, errors.Join(err, ErrFailedToParseAccessHashAsInt64)
	}

	return res, true, nil
}

func getBotStorageKey(userID int64) string {
	return fmt.Sprintf("bot-storage-state:%d", userID)
}

func getChannelStorageKey(userID int64, channelID int64) string {
	return fmt.Sprintf("bot-channel-storage-state:%d-%d", userID, channelID)
}
