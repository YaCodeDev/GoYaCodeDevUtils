package yatgstorage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yacache"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yalogger"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/updates"
	"github.com/gotd/td/tg"
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

	LoggerEntityID  = "entity_id"
	LoggerEntityKey = "entity_key"
	LoggerUserID    = "user_id"
	LoggerChannelID = "channel_id"
)

type IStorage interface {
	updates.StateStorage
	updates.ChannelAccessHasher

	Ping(ctx context.Context) yaerrors.Error
	AccessHashSaveHandler() HandlerFunc
	SaveUserAccessHash(ctx context.Context, userID int64, accessHash int64)
	GetUserAccessHash(ctx context.Context, userID int64) (int64, yaerrors.Error)
}

type Storage struct {
	cache    yacache.Cache[*redis.Client]
	handler  telegram.UpdateHandler
	entityID int64
	log      yalogger.Logger
}

func NewStorage(
	cache yacache.Cache[*redis.Client],
	handler telegram.UpdateHandler,
	entityID int64,
	log yalogger.Logger,
) *Storage {
	return &Storage{
		cache:    cache,
		handler:  handler,
		entityID: entityID,
		log:      log,
	}
}

func (s *Storage) Ping(ctx context.Context) yaerrors.Error {
	return s.cache.Ping(ctx)
}

func (s *Storage) GetState(ctx context.Context, entityID int64) (updates.State, bool, error) {
	key := getBotStorageKey(entityID)

	log := s.initBaseFieldsLog("Fetching entity state", key)

	data, yaerr := s.cache.Raw().JSONGet(ctx, key).Result()
	if yaerr != nil {
		return updates.State{}, false, nil
	}

	var state updates.State

	err := json.Unmarshal([]byte(data), &state)
	if err != nil {
		return state, false, nil
	}

	log.Info("Fetched entity state")

	return state, true, nil
}

func (s *Storage) SetState(ctx context.Context, entityID int64, state updates.State) error {
	key := getBotStorageKey(entityID)

	log := s.initBaseFieldsLog("Setting entity state", key).WithField(LoggerEntityID, entityID)

	if err := s.cache.Raw().JSONSet(ctx, key, BasePathRedisJSON, state).Err(); err != nil {
		return errors.Join(err, ErrFailedToSetState)
	}

	log.Info("Have set entity state")

	return nil
}

func (s *Storage) SetPts(ctx context.Context, entityID int64, pts int) error {
	key := getBotStorageKey(entityID)

	log := s.
		initBaseFieldsLog("Setting pts in entity state", key).
		WithField(LoggerEntityID, entityID)

	if err := s.cache.Raw().JSONSet(ctx, key, PtsPathRedisJSON, pts).Err(); err != nil {
		return errors.Join(err, ErrFailedToSetPts)
	}

	log.Debug("Have set pts in entity state")

	return nil
}

func (s *Storage) SetQts(ctx context.Context, entityID int64, qts int) error {
	key := getBotStorageKey(entityID)

	log := s.
		initBaseFieldsLog("Setting qts in bot state", key).
		WithField(LoggerEntityID, entityID)

	if err := s.cache.Raw().JSONSet(ctx, key, QtsPathRedisJSON, qts).Err(); err != nil {
		return errors.Join(err, ErrFailedToSetQts)
	}

	log.Debug("Have set qts in bot state")

	return nil
}

func (s *Storage) SetDate(ctx context.Context, entityID int64, date int) error {
	key := getBotStorageKey(entityID)

	log := s.
		initBaseFieldsLog("Setting seq in state", key).
		WithField(LoggerEntityID, entityID)

	if err := s.cache.Raw().JSONSet(ctx, key, DatePathRedisJSON, date).Err(); err != nil {
		return errors.Join(err, ErrFailedToSetDate)
	}

	log.Debug("Have set date in bot state")

	return nil
}

func (s *Storage) SetSeq(ctx context.Context, entityID int64, seq int) error {
	key := getBotStorageKey(entityID)

	log := s.
		initBaseFieldsLog("Setting seq in state", key).
		WithField(LoggerEntityID, entityID)

	if err := s.cache.Raw().JSONSet(ctx, key, SeqPathRedisJSON, seq).Err(); err != nil {
		return errors.Join(err, ErrFailedToSetSeq)
	}

	log.Debug("Have set seq in bot state")

	return nil
}

func (s *Storage) SetDateSeq(ctx context.Context, entityID int64, date, seq int) error {
	key := getBotStorageKey(entityID)

	log := s.
		initBaseFieldsLog("Setting date and seq in state", key).
		WithField(LoggerEntityID, entityID)

	if err := s.cache.Raw().
		JSONMSet(ctx, key, DatePathRedisJSON, date, key, SeqPathRedisJSON, seq).Err(); err != nil {
		return errors.Join(err, ErrFailedToSetDateSeq)
	}

	log.Debug("Have set date and seq in state")

	return nil
}

func (s *Storage) SetChannelPts(ctx context.Context, userID, channelID int64, pts int) error {
	key := getChannelPtsKey(userID)

	log := s.
		initBaseFieldsLog("Setting channel pts", key).
		WithField(LoggerUserID, userID).
		WithField(LoggerChannelID, channelID)

	if err := s.cache.Raw().
		HSet(ctx, key, strconv.FormatInt(channelID, 10), pts).Err(); err != nil {
		return errors.Join(err, ErrFailedToSetChannelPts)
	}

	log.Debug("Have set channel pts")

	return nil
}

func (s *Storage) GetChannelPts(ctx context.Context, entityID, channelID int64) (int, bool, error) {
	key := getChannelPtsKey(entityID)

	log := s.
		initBaseFieldsLog("Fetching channel pts", key).
		WithField(LoggerUserID, entityID).
		WithField(LoggerChannelID, channelID)

	data, yaerr := s.cache.HGet(ctx, key, strconv.FormatInt(channelID, 10))
	if yaerr != nil {
		return 0, false, errors.Join(yaerr, ErrFailedToGetChannelPts)
	}

	res, err := strconv.ParseInt(data, 10, 0)
	if err != nil {
		return 0, false, errors.Join(yaerr, ErrFailedToParsePtsAsInt)
	}

	log.Debug("Fetched channel pts")

	return int(res), true, nil
}

func (s *Storage) ForEachChannels(
	ctx context.Context,
	entityID int64,
	action func(ctx context.Context, channelID int64, pts int) error,
) error {
	key := getChannelPtsKey(entityID)

	log := s.initBaseFieldsLog("Start action for each channels", key).WithField(LoggerUserID, entityID)

	channels, err := s.cache.HGetAll(ctx, key)
	if err != nil {
		return errors.Join(err, ErrFailedToGetAllChannelPts)
	}

	for c := range channels {
		id, err := strconv.ParseInt(c, 10, 64)
		if err != nil {
			return errors.Join(err, ErrFailedToParseIDAsInt)
		}

		childLog := log.WithField(LoggerChannelID, id)

		pts, err := strconv.ParseInt(channels[c], 10, 0)
		if err != nil {
			return errors.Join(err, ErrFailedToParsePtsAsInt)
		}

		if err := action(ctx, id, int(pts)); err != nil {
			childLog.Errorf("%v", err)

			return errors.Join(err, ErrFromCalledActionOfChannel)
		}
	}

	log.Debug("Action manipulated for each channels")

	return nil
}

func (s *Storage) SetChannelAccessHash(ctx context.Context, entityID, channelID, accessHash int64) error {
	key := getChannelAccessHashKey(entityID)

	log := s.
		initBaseFieldsLog("Setting channel access hash for channel", key).
		WithField(LoggerEntityID, entityID).
		WithField(LoggerChannelID, channelID)

	if err := s.cache.Raw().
		HSet(ctx, key, strconv.FormatInt(channelID, 10), accessHash).Err(); err != nil {
		return errors.Join(err, ErrFailedToSetChannelAccessHash)
	}

	log.Debug("Have set channel access hash")

	return nil
}

func (s *Storage) GetChannelAccessHash(ctx context.Context, entityID, channelID int64) (int64, bool, error) {
	key := getChannelAccessHashKey(entityID)

	log := s.
		initBaseFieldsLog("Fetching channel access hash", key).
		WithField(LoggerEntityID, entityID).
		WithField(LoggerChannelID, channelID)

	data, err := s.cache.Raw().
		HGet(ctx, key, strconv.FormatInt(channelID, 10)).Result()
	if err != nil {
		return 0, false, errors.Join(err, ErrFailedToGetChannelAccessHash)
	}

	res, err := strconv.ParseInt(data, 10, 64)
	if err != nil {
		return 0, false, errors.Join(err, ErrFailedToParseAccessHashAsInt64)
	}

	log.Debug("Fetched channel access hash")

	return res, true, nil
}

func (s *Storage) initBaseFieldsLog(
	entryText string,
	botKey string,
) yalogger.Logger {
	log := s.log.WithField(LoggerEntityID, s.entityID).WithField(LoggerEntityKey, botKey)

	log.Debugf("%s", entryText)

	return log
}

type HandlerFunc func(ctx context.Context, updates tg.UpdatesClass) error

func (h HandlerFunc) Handle(ctx context.Context, updates tg.UpdatesClass) error {
	return h(ctx, updates)
}

func (s *Storage) AccessHashSaveHandler() HandlerFunc {
	return HandlerFunc(func(ctx context.Context, updates tg.UpdatesClass) error {
		switch update := updates.(type) {
		case *tg.Updates:
			for _, user := range update.MapUsers().NotEmptyToMap() {
				s.SaveUserAccessHash(ctx, user.ID, user.AccessHash)
			}
		case *tg.UpdatesCombined:
			for _, user := range update.MapUsers().NotEmptyToMap() {
				s.SaveUserAccessHash(ctx, user.ID, user.AccessHash)
			}
		}

		return s.handler.Handle(ctx, updates)
	})
}

func (s *Storage) SaveUserAccessHash(ctx context.Context, userID int64, accessHash int64) {
	const botChannelID = 136817688 // Ignore channel placeholder (@Channel_Bot - in Telegram)

	if userID != botChannelID {
		key := getUserAccessHashKey(s.entityID)

		log := s.initBaseFieldsLog("saving access hash", key).WithField(LoggerUserID, userID)

		if err := s.cache.Raw().
			HSet(ctx, key, strconv.FormatInt(userID, 10), accessHash).Err(); err != nil {
			log.Errorf("failed to save user access hash: %v", err)
		}

		log.Debugf("Saved user access hash")
	}
}

func (s *Storage) GetUserAccessHash(ctx context.Context, userID int64) (int64, yaerrors.Error) {
	key := getUserAccessHashKey(s.entityID)

	log := s.initBaseFieldsLog("fetching user access hash", key).WithField(LoggerUserID, userID)

	hash, err := s.cache.Raw().HGet(ctx, key, strconv.FormatInt(userID, 10)).Result()
	if err != nil {
		return 0, yaerrors.FromErrorWithLog(
			http.StatusInternalServerError,
			err,
			"failed to fetch user access hash",
			log,
		)
	}

	res, err := strconv.ParseInt(hash, 10, 64)
	if err != nil {
		return 0, yaerrors.FromErrorWithLog(
			http.StatusBadRequest,
			err,
			ErrFailedToParseAccessHashAsInt64.Error(),
			log,
		)
	}

	log.Debugf("Fetched user access hash")

	return res, nil
}

func getUserAccessHashKey(entityID int64) string {
	return fmt.Sprintf("bot-user-access-hash:%d", entityID)
}

func getBotStorageKey(entityID int64) string {
	return fmt.Sprintf("bot-state:%d", entityID)
}

func getChannelAccessHashKey(entityID int64) string {
	return fmt.Sprintf("bot-channel-access-hash:%d", entityID)
}

func getChannelPtsKey(entityID int64) string {
	return fmt.Sprintf("bot-channel-pts:%d", entityID)
}
