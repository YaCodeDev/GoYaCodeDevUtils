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
	Ping(ctx context.Context) yaerrors.Error

	GetState(ctx context.Context, entityID int64) (updates.State, bool, yaerrors.Error)
	SetState(ctx context.Context, entityID int64, state updates.State) yaerrors.Error
	SetPts(ctx context.Context, entityID int64, pts int) yaerrors.Error
	SetQts(ctx context.Context, entityID int64, qts int) yaerrors.Error
	SetDate(ctx context.Context, entityID int64, date int) yaerrors.Error
	SetSeq(ctx context.Context, entityID int64, seq int) yaerrors.Error
	SetDateSeq(ctx context.Context, entityID int64, date, seq int) yaerrors.Error
	SetChannelPts(ctx context.Context, userID, channelID int64, pts int) yaerrors.Error
	GetChannelPts(ctx context.Context, entityID, channelID int64) (int, bool, yaerrors.Error)
	ForEachChannels(
		ctx context.Context,
		entityID int64,
		action func(ctx context.Context, channelID int64, pts int) error,
	) yaerrors.Error
	SetChannelAccessHash(ctx context.Context, entityID, channelID, accessHash int64) yaerrors.Error
	GetChannelAccessHash(ctx context.Context, entityID, channelID int64) (int64, bool, error)

	AccessHashSaveHandler() HandlerFunc

	SaveUserAccessHash(ctx context.Context, userID int64, accessHash int64)
	GetUserAccessHash(ctx context.Context, userID int64) (int64, yaerrors.Error)

	TelegramStorageCompatible() updates.StateStorage
	TelegramAccessHasherCompatible() updates.ChannelAccessHasher
}

type Storage struct {
	cache     yacache.Cache[*redis.Client]
	handler   telegram.UpdateHandler
	entityID  int64
	stateKeys map[string]struct{}
	log       yalogger.Logger
}

func NewStorage(
	cache yacache.Cache[*redis.Client],
	handler telegram.UpdateHandler,
	entityID int64,
	log yalogger.Logger,
) *Storage {
	return &Storage{
		cache:     cache,
		handler:   handler,
		entityID:  entityID,
		stateKeys: map[string]struct{}{},
		log:       log,
	}
}

func (s *Storage) Ping(ctx context.Context) yaerrors.Error {
	return s.cache.Ping(ctx)
}

func (s *Storage) TelegramStorageCompatible() updates.StateStorage {
	return &telegramStorage{
		storage: s,
	}
}

func (s *Storage) TelegramAccessHasherCompatible() updates.ChannelAccessHasher {
	return &telegramHasher{
		storage: s,
	}
}

func (s *Storage) GetState(ctx context.Context, entityID int64) (updates.State, bool, yaerrors.Error) {
	key := getBotStorageKey(entityID)

	log := s.initBaseFieldsLog("Fetching entity state", key)

	if err := s.safetyBaseStateJSON(ctx, key, log); err != nil {
		return updates.State{}, false, err.WrapWithLog("failed to get entity state", log)
	}

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

func (s *Storage) SetState(ctx context.Context, entityID int64, state updates.State) yaerrors.Error {
	key := getBotStorageKey(entityID)

	log := s.initBaseFieldsLog("Setting entity state", key).WithField(LoggerEntityID, entityID)

	if err := s.cache.Raw().JSONSet(ctx, key, BasePathRedisJSON, state).Err(); err != nil {
		return yaerrors.FromErrorWithLog(
			http.StatusInternalServerError,
			errors.Join(err, ErrFailedToSetState),
			"failed to set entity json",
			log,
		)
	}

	log.Info("Have set entity state")

	return nil
}

func (s *Storage) SetPts(ctx context.Context, entityID int64, pts int) yaerrors.Error {
	key := getBotStorageKey(entityID)

	log := s.
		initBaseFieldsLog("Setting pts in entity state", key).
		WithField(LoggerEntityID, entityID)

	if err := s.safetyBaseStateJSON(ctx, key, log); err != nil {
		return err.WrapWithLog("failed to set entity state pts", log)
	}

	if err := s.cache.Raw().JSONSet(ctx, key, PtsPathRedisJSON, pts).Err(); err != nil {
		return yaerrors.FromErrorWithLog(
			http.StatusInternalServerError,
			errors.Join(err, ErrFailedToSetPts),
			"failed to set entity state pts",
			log,
		)
	}

	log.Debug("Have set pts in entity state")

	return nil
}

func (s *Storage) SetQts(ctx context.Context, entityID int64, qts int) yaerrors.Error {
	key := getBotStorageKey(entityID)

	log := s.
		initBaseFieldsLog("Setting qts in bot state", key).
		WithField(LoggerEntityID, entityID)

	if err := s.safetyBaseStateJSON(ctx, key, log); err != nil {
		return err.WrapWithLog("failed to set entity state qts", log)
	}

	if err := s.cache.Raw().JSONSet(ctx, key, QtsPathRedisJSON, qts).Err(); err != nil {
		return yaerrors.FromErrorWithLog(
			http.StatusInternalServerError,
			errors.Join(err, ErrFailedToSetQts),
			"failed to set entity state qts",
			log,
		)
	}

	log.Debug("Have set qts in bot state")

	return nil
}

func (s *Storage) SetDate(ctx context.Context, entityID int64, date int) yaerrors.Error {
	key := getBotStorageKey(entityID)

	log := s.
		initBaseFieldsLog("Setting seq in state", key).
		WithField(LoggerEntityID, entityID)

	if err := s.safetyBaseStateJSON(ctx, key, log); err != nil {
		return err.WrapWithLog("failed to set entity state date", log)
	}

	if err := s.cache.Raw().JSONSet(ctx, key, DatePathRedisJSON, date).Err(); err != nil {
		return yaerrors.FromErrorWithLog(
			http.StatusInternalServerError,
			errors.Join(err, ErrFailedToSetDate),
			"failed to set entity state date",
			log,
		)
	}

	log.Debug("Have set date in bot state")

	return nil
}

func (s *Storage) SetSeq(ctx context.Context, entityID int64, seq int) yaerrors.Error {
	key := getBotStorageKey(entityID)

	log := s.
		initBaseFieldsLog("Setting seq in state", key).
		WithField(LoggerEntityID, entityID)

	if err := s.safetyBaseStateJSON(ctx, key, log); err != nil {
		return err.WrapWithLog("failed to set entity state seq", log)
	}

	if err := s.cache.Raw().JSONSet(ctx, key, SeqPathRedisJSON, seq).Err(); err != nil {
		return yaerrors.FromErrorWithLog(
			http.StatusInternalServerError,
			errors.Join(err, ErrFailedToSetSeq),
			"failed to set entity state seq",
			log,
		)
	}

	log.Debug("Have set seq in bot state")

	return nil
}

func (s *Storage) SetDateSeq(ctx context.Context, entityID int64, date, seq int) yaerrors.Error {
	key := getBotStorageKey(entityID)

	log := s.
		initBaseFieldsLog("Setting date and seq in state", key).
		WithField(LoggerEntityID, entityID)

	if err := s.safetyBaseStateJSON(ctx, key, log); err != nil {
		return err.WrapWithLog("failed to set entity state date and seq", log)
	}

	if err := s.cache.Raw().
		JSONMSet(ctx, key, DatePathRedisJSON, date, key, SeqPathRedisJSON, seq).Err(); err != nil {
		return yaerrors.FromErrorWithLog(
			http.StatusInternalServerError,
			errors.Join(err, ErrFailedToSetDateSeq),
			"failed to set entity state date and seq",
			log,
		)
	}

	log.Debug("Have set date and seq in state")

	return nil
}

func (s *Storage) SetChannelPts(ctx context.Context, userID, channelID int64, pts int) yaerrors.Error {
	key := getChannelPtsKey(userID)

	log := s.
		initBaseFieldsLog("Setting channel pts", key).
		WithField(LoggerUserID, userID).
		WithField(LoggerChannelID, channelID)

	if err := s.cache.Raw().
		HSet(ctx, key, strconv.FormatInt(channelID, 10), pts).Err(); err != nil {
		return yaerrors.FromErrorWithLog(
			http.StatusInternalServerError,
			errors.Join(err, ErrFailedToSetChannelPts),
			"failed to set channel pts",
			log,
		)
	}

	log.Debug("Have set channel pts")

	return nil
}

func (s *Storage) GetChannelPts(ctx context.Context, entityID, channelID int64) (int, bool, yaerrors.Error) {
	key := getChannelPtsKey(entityID)

	log := s.
		initBaseFieldsLog("Fetching channel pts", key).
		WithField(LoggerUserID, entityID).
		WithField(LoggerChannelID, channelID)

	data, yaerr := s.cache.HGet(ctx, key, strconv.FormatInt(channelID, 10))
	if yaerr != nil {
		return 0, false, yaerrors.FromErrorWithLog(
			http.StatusInternalServerError,
			errors.Join(yaerr, ErrFailedToGetChannelPts),
			"failed to get channel pts",
			log,
		)
	}

	res, err := strconv.ParseInt(data, 10, 0)
	if err != nil {
		return 0, false, yaerrors.FromErrorWithLog(
			http.StatusInternalServerError,
			errors.Join(err, ErrFailedToParsePtsAsInt),
			"failed to get channel pts",
			log,
		)
	}

	log.Debug("Fetched channel pts")

	return int(res), true, nil
}

func (s *Storage) ForEachChannels(
	ctx context.Context,
	entityID int64,
	action func(ctx context.Context, channelID int64, pts int) error,
) yaerrors.Error {
	key := getChannelPtsKey(entityID)

	log := s.initBaseFieldsLog("Start action for each channels", key).WithField(LoggerUserID, entityID)

	channels, err := s.cache.HGetAll(ctx, key)
	if err != nil {
		return yaerrors.FromErrorWithLog(
			http.StatusInternalServerError,
			errors.Join(err, ErrFailedToGetAllChannelPts),
			"failed to get all cannels",
			log,
		)
	}

	for c := range channels {
		id, err := strconv.ParseInt(c, 10, 64)
		if err != nil {
			return yaerrors.FromErrorWithLog(
				http.StatusInternalServerError,
				errors.Join(err, ErrFailedToParseIDAsInt),
				"failed to parse id as int",
				log,
			)
		}

		childLog := log.WithField(LoggerChannelID, id)

		pts, err := strconv.ParseInt(channels[c], 10, 0)
		if err != nil {
			return yaerrors.FromErrorWithLog(
				http.StatusInternalServerError,
				errors.Join(err, ErrFailedToParsePtsAsInt),
				"failed to parse pts as int",
				log,
			)
		}

		if err := action(ctx, id, int(pts)); err != nil {
			childLog.Errorf("%v", err)
			return yaerrors.FromErrorWithLog(
				http.StatusInternalServerError,
				errors.Join(err, ErrFromCalledActionOfChannel),
				"failed to action of channel",
				log,
			)
		}
	}

	log.Debug("Action manipulated for each channels")

	return nil
}

func (s *Storage) SetChannelAccessHash(ctx context.Context, entityID, channelID, accessHash int64) yaerrors.Error {
	key := getChannelAccessHashKey(entityID)

	log := s.
		initBaseFieldsLog("Setting channel access hash for channel", key).
		WithField(LoggerEntityID, entityID).
		WithField(LoggerChannelID, channelID)

	if err := s.cache.Raw().
		HSet(ctx, key, strconv.FormatInt(channelID, 10), accessHash).Err(); err != nil {
		return yaerrors.FromErrorWithLog(
			http.StatusInternalServerError,
			errors.Join(err, ErrFailedToSetChannelAccessHash),
			"failed to set channel access hash",
			log,
		)
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
		return 0, false, yaerrors.FromErrorWithLog(
			http.StatusInternalServerError,
			errors.Join(err, ErrFailedToGetChannelAccessHash),
			"failed to get channel access hash",
			log,
		)
	}

	res, err := strconv.ParseInt(data, 10, 64)
	if err != nil {
		return 0, false, yaerrors.FromErrorWithLog(
			http.StatusInternalServerError,
			errors.Join(err, ErrFailedToParseAccessHashAsInt64),
			"failed to parse channel access hash as int64",
			log,
		)
	}

	log.Debug("Fetched channel access hash")

	return res, true, nil
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
				_ = s.SaveUserAccessHash(ctx, user.ID, user.AccessHash)
			}
		case *tg.UpdatesCombined:
			for _, user := range update.MapUsers().NotEmptyToMap() {
				_ = s.SaveUserAccessHash(ctx, user.ID, user.AccessHash)
			}
		}

		return s.handler.Handle(ctx, updates)
	})
}

func (s *Storage) SaveUserAccessHash(ctx context.Context, userID int64, accessHash int64) yaerrors.Error {
	const botChannelID = 136817688 // Ignore channel placeholder (@Channel_Bot - in Telegram)

	if userID != botChannelID {
		key := getUserAccessHashKey(s.entityID)

		log := s.initBaseFieldsLog("saving access hash", key).WithField(LoggerUserID, userID)

		if err := s.cache.Raw().
			HSet(ctx, key, strconv.FormatInt(userID, 10), accessHash).Err(); err != nil {
			return yaerrors.FromErrorWithLog(
				http.StatusInternalServerError,
				err,
				"failed to save user access hash: %v",
				log,
			)
		}

		log.Debugf("Saved user access hash")
	}

	return nil
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

func (s *Storage) initBaseFieldsLog(
	entryText string,
	botKey string,
) yalogger.Logger {
	log := s.log.WithField(LoggerEntityID, s.entityID).WithField(LoggerEntityKey, botKey)

	log.Debugf("%s", entryText)

	return log
}

func (s *Storage) safetyBaseStateJSON(ctx context.Context, key string, log yalogger.Logger) yaerrors.Error {
	if _, ok := s.stateKeys[key]; !ok {
		if res, err := s.cache.Raw().JSONGet(ctx, key, BasePathRedisJSON).Result(); err != nil || len(res) == 0 {
			if err := s.cache.Raw().JSONSet(ctx, key, BasePathRedisJSON, updates.State{}).Err(); err != nil {
				return yaerrors.FromErrorWithLog(
					http.StatusInternalServerError,
					errors.Join(err, ErrFailedToSetState),
					"failed to create safety base root entity state",
					log,
				)
			}
		}

		s.stateKeys[key] = struct{}{}
	}

	return nil
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

type telegramStorage struct {
	storage *Storage
}

func (t *telegramStorage) GetState(ctx context.Context, userID int64) (state updates.State, found bool, err error) {
	return t.storage.GetState(ctx, userID)
}

func (t *telegramStorage) SetState(ctx context.Context, userID int64, state updates.State) error {
	return t.storage.SetState(ctx, userID, state)
}

func (t *telegramStorage) SetPts(ctx context.Context, userID int64, pts int) error {
	return t.storage.SetPts(ctx, userID, pts)
}

func (t *telegramStorage) SetQts(ctx context.Context, userID int64, qts int) error {
	return t.storage.SetQts(ctx, userID, qts)

}

func (t *telegramStorage) SetDate(ctx context.Context, userID int64, date int) error {
	return t.storage.SetDate(ctx, userID, date)
}

func (t *telegramStorage) SetSeq(ctx context.Context, userID int64, seq int) error {
	return t.storage.SetSeq(ctx, userID, seq)
}

func (t *telegramStorage) SetDateSeq(ctx context.Context, userID int64, date, seq int) error {
	return t.storage.SetDateSeq(ctx, userID, date, seq)
}

func (t *telegramStorage) GetChannelPts(ctx context.Context, userID, channelID int64) (pts int, found bool, err error) {
	return t.storage.GetChannelPts(ctx, userID, channelID)
}

func (t *telegramStorage) SetChannelPts(ctx context.Context, userID, channelID int64, pts int) error {
	return t.storage.SetChannelPts(ctx, userID, channelID, pts)
}

func (t *telegramStorage) ForEachChannels(
	ctx context.Context,
	userID int64,
	f func(ctx context.Context, channelID int64, pts int) error,
) error {
	return t.storage.ForEachChannels(ctx, userID, f)
}

type telegramHasher struct {
	storage *Storage
}

func (t *telegramHasher) SetChannelAccessHash(ctx context.Context, userID, channelID, accessHash int64) error {
	return t.storage.SetChannelAccessHash(ctx, userID, channelID, accessHash)
}

func (t *telegramHasher) GetChannelAccessHash(
	ctx context.Context,
	userID,
	channelID int64,
) (accessHash int64, found bool, err error) {
	return t.storage.GetChannelAccessHash(ctx, userID, channelID)
}
