// Package yatgstorage implements a Redis‑backed persistence layer for
// Telegram updates.Manager state (pts/qts/seq/date) plus channel/user
// access‑hash bookkeeping.
//
// The storage is fully compatible with gotd/td’s updates.Manager via
// TelegramStorageCompatible and TelegramAccessHasherCompatible adapters.
//
// # Layout in Redis
//
//   - bot-state:<entity_id>                – RedisJSON root with pts/qts/seq/date
//   - bot-channel-pts:<entity_id>          – HSET <channelID>=<pts>
//   - bot-channel-access-hash:<entity_id>  – HSET <channelID>=<hash>
//   - bot-user-access-hash:<entity_id>     – HSET <userID>=<hash>
//
// All JSON operations use redisjson (ReJSON v2).  For high‑throughput
// production systems you can point s.cache at a sharded cluster without
// changing this code.
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
	"github.com/YaCodeDev/GoYaCodeDevUtils/yathreadsafeset"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/updates"
	"github.com/gotd/td/tg"
	"github.com/redis/go-redis/v9"
)

const (
	// BasePathRedisJSON is the JSON root (“$”) used by ReJSON.
	BasePathRedisJSON = "$"
	// PtsPathRedisJSON is $.Pts in bot‑state JSON.
	PtsPathRedisJSON = BasePathRedisJSON + ".Pts"
	// QtsPathRedisJSON is $.Qts in bot‑state JSON.
	QtsPathRedisJSON = BasePathRedisJSON + ".Qts"
	// DatePathRedisJSON is $.Date in bot‑state JSON.
	DatePathRedisJSON = BasePathRedisJSON + ".Date"
	// SeqPathRedisJSON is $.Seq in bot‑state JSON.
	SeqPathRedisJSON = BasePathRedisJSON + ".Seq"

	// AccessHashFieldRedisHSet is the field name for access‑hash in HSET buckets.
	AccessHashFieldRedisHSet = "AccessHash"
	// PtsFieldRedisHSet is the field name for pts in HSET buckets.
	PtsFieldRedisHSet = "Pts"

	// Structured‑logging keys.
	LoggerEntityID  = "entity_id"
	LoggerEntityKey = "entity_key"
	LoggerUserID    = "user_id"
	LoggerChannelID = "channel_id"
)

// IStorage exposes the behaviour required by your application **and** the
// gotd/td updates.Manager.  Code in higher layers (handlers, services, unit
// tests) should depend on this interface rather than *Storage so that you can
// swap the implementation (e.g. in‑memory fake).  All methods return a
// yaerrors.Error – a thin wrapper around the standard error enriched with an
// HTTP status and structured‑log context.
//
// Example:
//
//	var stg IStorage = yatgstorage.NewStorage(cache, dispatcher, 123, log)
//	if err := stg.SetPts(ctx, 123, 456); err != nil {
//	    log.Fatalf("failed: %v", err)
//	}
type IStorage interface {
	// Ping checks the backend yacache health.
	Ping(ctx context.Context) yaerrors.Error

	// Bot‑wide state getters / setters. ‘found==false’ means “no key yet”.
	GetState(ctx context.Context, entityID int64) (updates.State, bool, yaerrors.Error)
	SetState(ctx context.Context, entityID int64, state updates.State) yaerrors.Error
	SetPts(ctx context.Context, entityID int64, pts int) yaerrors.Error
	SetQts(ctx context.Context, entityID int64, qts int) yaerrors.Error
	SetDate(ctx context.Context, entityID int64, date int) yaerrors.Error
	SetSeq(ctx context.Context, entityID int64, seq int) yaerrors.Error
	SetDateSeq(ctx context.Context, entityID int64, date, seq int) yaerrors.Error

	// Per‑channel pts bookkeeping.
	SetChannelPts(ctx context.Context, entityID, channelID int64, pts int) yaerrors.Error
	GetChannelPts(ctx context.Context, entityID, channelID int64) (int, bool, yaerrors.Error)
	ForEachChannels(
		ctx context.Context,
		entityID int64,
		action func(ctx context.Context, channelID int64, pts int) error,
	) yaerrors.Error

	// Channel access‑hash bookkeeping.
	SetChannelAccessHash(ctx context.Context, entityID, channelID, accessHash int64) yaerrors.Error
	GetChannelAccessHash(
		ctx context.Context,
		entityID, channelID int64,
	) (int64, bool, yaerrors.Error)

	// Update‑pipeline helper: returns a handler that stores access‑hashes
	// from any incoming updates before forwarding to the real handler.
	AccessHashSaveHandler() HandlerFunc

	// User access‑hash bookkeeping.
	SetUserAccessHash(ctx context.Context, userID int64, accessHash int64) yaerrors.Error
	GetUserAccessHash(ctx context.Context, userID int64) (int64, yaerrors.Error)

	// gotd adapters
	TelegramStorageCompatible() updates.StateStorage
	TelegramAccessHasherCompatible() updates.ChannelAccessHasher
}

// Storage is the production implementation backed by a yacache.Cache[*redis.Client].
//
// It embeds a telegram.UpdateHandler (your own dispatcher) so we can inject a
// middle layer that persists access‑hashes before letting updates propagate.
// The zero value is **not** valid – use NewStorage.
//
// Example:
//
//	stg   := yatgstorage.NewStorage(cache, log)
//	_ = stg
//
// Because methods are safe for concurrent use (they only rely on redis, which
// is thread‑safe), you may share *Storage between goroutines.
type Storage struct {
	cache     yacache.Cache[*redis.Client]
	stateKeys *yathreadsafeset.ThreadSafeSet[string]
	log       yalogger.Logger
}

// NewStorage wires all dependencies and returns a ready‑to‑use *Storage.
//
//   - cache    – any yacache implementation; production code passes a Redis
//     client, tests may pass yacache.NewMock.
//   - log      – structured logger.
//
// Example:
//
//	stg   := yatgstorage.NewStorage(cache, log)
//	if err := stg.Ping(ctx); err != nil {
//	    log.Fatalf("redis down: %v", err)
//	}
func NewStorage(
	cache yacache.Cache[*redis.Client],
	log yalogger.Logger,
) *Storage {
	return &Storage{
		cache:     cache,
		stateKeys: yathreadsafeset.NewThreadSafeSet[string](),
		log:       log,
	}
}

// Ping checks that the yacache backend is operational.
//
// Example:
//
//	if err := stg.Ping(ctx); err != nil {
//	    log.Errorf("storage unhealthy: %v", err)
//	}
func (s *Storage) Ping(ctx context.Context) yaerrors.Error {
	return s.cache.Ping(ctx)
}

// TelegramStorageCompatible returns an adapter implementing updates.StateStorage
// so that gotd/td’s updates.Manager can persist pts/qts/seq/date directly into
// Redis.
//
// Example:
//
//	manager := updates.New(updates.Config{Handler: handler, Storage: stg.TelegramStorageCompatible()})
func (s *Storage) TelegramStorageCompatible() updates.StateStorage {
	return &telegramStorage{
		storage: s,
	}
}

// TelegramAccessHasherCompatible returns an adapter implementing
// updates.ChannelAccessHasher so that updates.Manager can resolve channel
// access hashes via Redis.
func (s *Storage) TelegramAccessHasherCompatible() updates.ChannelAccessHasher {
	return &telegramHasher{
		storage: s,
	}
}

// GetState retrieves the bot‑global State record (pts/qts/seq/date).
// found==false indicates the record does not exist yet.
//
// Example:
//
//	state, ok, err := stg.GetState(ctx, botID)
//	if err != nil { log.Fatal(err) }
//	if ok { fmt.Printf("pts=%d", st.Pts) }
func (s *Storage) GetState(
	ctx context.Context,
	entityID int64,
) (updates.State, bool, yaerrors.Error) {
	key := getBotStateKey(entityID)

	log := s.initBaseFieldsLog("Fetching entity state", entityID, key)

	data, err := s.cache.Raw().JSONGet(ctx, key).Result()
	if err != nil {
		return updates.State{}, false, yaerrors.FromErrorWithLog(
			http.StatusInternalServerError,
			err,
			"failed to fetch enity state",
			log,
		)
	}

	var state updates.State

	err = json.Unmarshal([]byte(data), &state)
	if err != nil {
		return state, false, nil
	}

	log.Debug("Entity state fetched")

	return state, true, nil
}

// SetState stores the full updates.State.
//
// Example:
//
//	err := stg.SetState(ctx, botID, updates.State{Pts: 10})
//	if err != nil { log.Fatal(err) }
func (s *Storage) SetState(
	ctx context.Context,
	entityID int64,
	state updates.State,
) yaerrors.Error {
	key := getBotStateKey(entityID)

	log := s.initBaseFieldsLog("Setting entity state", entityID, key).
		WithField(LoggerEntityID, entityID)

	if err := s.cache.Raw().JSONSet(ctx, key, BasePathRedisJSON, state).Err(); err != nil {
		return yaerrors.FromErrorWithLog(
			http.StatusInternalServerError,
			errors.Join(err, ErrFailedToSetState),
			"failed to set entity json",
			log,
		)
	}

	log.Debug("Entity state set")

	return nil
}

// SetPts updates only $.Pts inside the stored state.
//
// Example:
//
//	_ = stg.SetPts(ctx, botID, 123)
func (s *Storage) SetPts(ctx context.Context, entityID int64, pts int) yaerrors.Error {
	key := getBotStateKey(entityID)

	log := s.
		initBaseFieldsLog("Setting pts in entity state", entityID, key).
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

// SetQts writes $.Qts only.
//
// Example:
//
//	_ = stg.SetQts(ctx, botID, 77)
func (s *Storage) SetQts(ctx context.Context, entityID int64, qts int) yaerrors.Error {
	key := getBotStateKey(entityID)

	log := s.
		initBaseFieldsLog("Setting qts in entity state", entityID, key).
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

	log.Debug("Have set qts in entity state")

	return nil
}

// SetDate writes $.Date only.
//
// Example:
//
//	_ = stg.SetDate(ctx, botID, int(time.Now().Unix()))
func (s *Storage) SetDate(ctx context.Context, entityID int64, date int) yaerrors.Error {
	key := getBotStateKey(entityID)

	log := s.
		initBaseFieldsLog("Setting date in state", entityID, key).
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

	log.Debug("Have set date in entity state")

	return nil
}

// SetSeq writes $.Seq only.
//
// Example:
//
//	_ = stg.SetSeq(ctx, botID, 5)
func (s *Storage) SetSeq(ctx context.Context, entityID int64, seq int) yaerrors.Error {
	key := getBotStateKey(entityID)

	log := s.
		initBaseFieldsLog("Setting seq in state", entityID, key).
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

	log.Debug("Have set seq in entity state")

	return nil
}

// SetDateSeq atomically writes $.Date and $.Seq.
//
// Example:
//
//	_ = stg.SetDateSeq(ctx, botID, int(time.Now().Unix()), 9)
func (s *Storage) SetDateSeq(ctx context.Context, entityID int64, date, seq int) yaerrors.Error {
	key := getBotStateKey(entityID)

	log := s.
		initBaseFieldsLog("Setting date and seq in state", entityID, key).
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

// SetChannelPts stores channel pts value.
//
// Example:
//
//	_ = stg.SetChannelPts(ctx, botID, chID, 120)
func (s *Storage) SetChannelPts(
	ctx context.Context,
	entityID, channelID int64,
	pts int,
) yaerrors.Error {
	key := getChannelPtsKey(entityID)

	log := s.
		initBaseFieldsLog("Setting channel pts", entityID, key).
		WithField(LoggerEntityID, entityID).
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

// GetChannelPts returns pts for a channel.
//
// Example:
//
//	pts, ok, _ := stg.GetChannelPts(ctx, botID, chID)
func (s *Storage) GetChannelPts(
	ctx context.Context,
	entityID, channelID int64,
) (int, bool, yaerrors.Error) {
	key := getChannelPtsKey(entityID)

	log := s.
		initBaseFieldsLog("Fetching channel pts", entityID, key).
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

	if len(data) == 0 {
		return 0, false, nil
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

// ForEachChannels iterates over all channels.
//
// Example:
//
//	_ = stg.ForEachChannels(ctx, botID, func(ctx context.Context, id int64, pts int) error {
//		fmt.Println(id, pts); return nil
//	})
func (s *Storage) ForEachChannels(
	ctx context.Context,
	entityID int64,
	action func(ctx context.Context, channelID int64, pts int) error,
) yaerrors.Error {
	key := getChannelPtsKey(entityID)

	log := s.initBaseFieldsLog("Start action for each channels", entityID, key).
		WithField(LoggerUserID, entityID)

	channels, err := s.cache.HGetAll(ctx, key)
	if err != nil {
		return yaerrors.FromErrorWithLog(
			http.StatusInternalServerError,
			errors.Join(err, ErrFailedToGetAllChannelPts),
			"failed to get all channels",
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

// SetChannelAccessHash saves a channel access‑hash.
//
// Example:
//
//	_ = stg.SetChannelAccessHash(ctx, botID, chID, hash)
func (s *Storage) SetChannelAccessHash(
	ctx context.Context,
	entityID, channelID, accessHash int64,
) yaerrors.Error {
	key := getChannelAccessHashKey(entityID)

	log := s.
		initBaseFieldsLog("Setting channel access hash for channel", entityID, key).
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

// GetChannelAccessHash retrieves a saved access‑hash.
//
// Example:
//
//	hash, found, _ := stg.GetChannelAccessHash(ctx, botID, chID)
func (s *Storage) GetChannelAccessHash(
	ctx context.Context,
	entityID, channelID int64,
) (int64, bool, yaerrors.Error) {
	key := getChannelAccessHashKey(entityID)

	log := s.
		initBaseFieldsLog("Fetching channel access hash", entityID, key).
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

	if len(data) == 0 {
		return 0, false, nil
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

// HandlerFunc adapts a plain function into a gotd `telegram.UpdateHandler`.
//
// Example:
//
//	h := yatgstorage.HandlerFunc(func(ctx context.Context, u tg.UpdatesClass) error {
//	    fmt.Println("update received")
//	    return nil
//	})
//	_ = h.Handle(ctx, &tg.Updates{})
type HandlerFunc func(ctx context.Context, updates tg.UpdatesClass) error

// Handle implements telegram.UpdateHandler by delegating to the underlying
// function.
//
// Example:
//
//	_ = HandlerFunc(func(ctx context.Context, u tg.UpdatesClass) error { return nil }).Handle(ctx, &tg.Updates{})
func (h HandlerFunc) Handle(ctx context.Context, updates tg.UpdatesClass) error {
	return h(ctx, updates)
}

// AccessHashSaveHandler returns middleware that intercepts Updates{,Combined},
// saves every user’s AccessHash to Redis via SetUserAccessHash, then forwards
// the update to the real dispatcher.
//
// Example:
//
//	clientOpts.UpdateHandler = storage.AccessHashSaveHandler()
func (s *Storage) AccessHashSaveHandler(
	entityID int64,
	handler telegram.UpdateHandler,
) HandlerFunc {
	return HandlerFunc(func(ctx context.Context, updates tg.UpdatesClass) error {
		switch update := updates.(type) {
		case *tg.Updates:
			for _, user := range update.MapUsers().NotEmptyToMap() {
				if err := s.SetUserAccessHash(ctx, entityID, user.ID, user.AccessHash); err != nil {
					s.log.Errorf("Failed to save user(%d) access hash(%d)", user.ID, user.AccessHash)
				}
			}
		case *tg.UpdatesCombined:
			for _, user := range update.MapUsers().NotEmptyToMap() {
				if err := s.SetUserAccessHash(ctx, entityID, user.ID, user.AccessHash); err != nil {
					s.log.Errorf("Failed to save user(%d) access hash(%d)", user.ID, user.AccessHash)
				}
			}
		}

		return handler.Handle(ctx, updates)
	})
}

// SetUserAccessHash persists a user access‑hash unless the ID equals the
// special @Channel_Bot placeholder.
//
// Example:
//
//	_ = stg.SetUserAccessHash(ctx, 12345, 67890)
func (s *Storage) SetUserAccessHash(
	ctx context.Context,
	entityID int64,
	userID int64,
	accessHash int64,
) yaerrors.Error {
	const botChannelID = 136817688 // Ignore channel placeholder (@Channel_Bot - in Telegram)

	if userID != botChannelID {
		key := getUserAccessHashKey(entityID)

		log := s.initBaseFieldsLog("Saving access hash", entityID, key).
			WithField(LoggerUserID, userID)

		if err := s.cache.Raw().
			HSet(ctx, key, strconv.FormatInt(userID, 10), accessHash).Err(); err != nil {
			return yaerrors.FromErrorWithLog(
				http.StatusInternalServerError,
				err,
				"failed to save user access hash",
				log,
			)
		}

		log.Debugf("Saved user access hash")
	}

	return nil
}

// GetUserAccessHash retrieves a user’s access‑hash.
//
// Example:
//
//	hash, foundErr := stg.GetUserAccessHash(ctx, 12345)
func (s *Storage) GetUserAccessHash(
	ctx context.Context,
	entityID int64,
	userID int64,
) (int64, yaerrors.Error) {
	key := getUserAccessHashKey(entityID)

	log := s.initBaseFieldsLog("fetching user access hash", entityID, key).
		WithField(LoggerUserID, userID)

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

// initBaseFieldsLog attaches standard fields (entityID, redisKey) and issues a
// debug message.
//
// Example:
//
//	l := stg.initBaseFieldsLog("doing work", "redis:key")
func (s *Storage) initBaseFieldsLog(
	entryText string,
	entityID int64,
	botKey string,
) yalogger.Logger {
	log := s.log.WithField(LoggerEntityID, entityID).WithField(LoggerEntityKey, botKey)

	log.Debugf("%s", entryText)

	return log
}

// safetyBaseStateJSON lazily creates an empty JSON object at key "$" if absent
// to guarantee follow‑up JSONSet operations succeed.
//
// Example:
//
//	_ = stg.safetyBaseStateJSON(ctx, "bot-state:1", log)
func (s *Storage) safetyBaseStateJSON(
	ctx context.Context,
	key string,
	log yalogger.Logger,
) yaerrors.Error {
	if s.stateKeys.Has(key) {
		if res, err := s.cache.Raw().JSONGet(ctx, key, BasePathRedisJSON).Result(); err != nil ||
			len(res) == 0 {
			if err := s.cache.Raw().JSONSet(ctx, key, BasePathRedisJSON, updates.State{}).Err(); err != nil {
				return yaerrors.FromErrorWithLog(
					http.StatusInternalServerError,
					errors.Join(err, ErrFailedToSetState),
					"failed to create safety base root entity state",
					log,
				)
			}
		}

		s.stateKeys.Set(key)
	}

	return nil
}

// getUserAccessHashKey forms the HSET key for user access‑hashes.
//
// Example:
//
//	k := getUserAccessHashKey(42) // "bot-user-access-hash:42"
func getUserAccessHashKey(entityID int64) string {
	return fmt.Sprintf("bot-user-access-hash:%d", entityID)
}

// getBotStateKey forms the RedisJSON key for bot global state.
//
// Example:
//
//	k := getBotStateKey(42) // "bot-state:42"
func getBotStateKey(entityID int64) string {
	return fmt.Sprintf("bot-state:%d", entityID)
}

// // getChannelAccessHashKey forms the HSET key for channel access‑hashes.
//
// Example:
//
//	k := getChannelAccessHashKey(42) // "bot-channel-access-hash:42"
func getChannelAccessHashKey(entityID int64) string {
	return fmt.Sprintf("bot-channel-access-hash:%d", entityID)
}

// getChannelPtsKey forms the HSET key for channel pts.
//
// Example:
//
//	k := getChannelPtsKey(42) // "bot-channel-pts:42"
func getChannelPtsKey(entityID int64) string {
	return fmt.Sprintf("bot-channel-pts:%d", entityID)
}

// Implementation native `gotd` iterface storage
type telegramStorage struct {
	storage *Storage
}

// GetState proxies Storage.GetState.
//
// Example:
//
//	st, found, _ := stg.TelegramStorageCompatible().GetState(ctx, botID)
func (t *telegramStorage) GetState(
	ctx context.Context,
	userID int64,
) (state updates.State, found bool, err error) {
	return t.storage.GetState(ctx, userID)
}

// SetState proxies Storage.SetState.
//
// Example:
//
//	_ = stg.TelegramStorageCompatible().SetState(ctx, botID, updates.State{Pts: 1})
func (t *telegramStorage) SetState(ctx context.Context, userID int64, state updates.State) error {
	return t.storage.SetState(ctx, userID, state)
}

// SetPts proxies Storage.SetPts.
func (t *telegramStorage) SetPts(ctx context.Context, userID int64, pts int) error {
	return t.storage.SetPts(ctx, userID, pts)
}

// SetQts proxies Storage.SetQts.
func (t *telegramStorage) SetQts(ctx context.Context, userID int64, qts int) error {
	return t.storage.SetQts(ctx, userID, qts)
}

// SetDate proxies Storage.SetDate.
func (t *telegramStorage) SetDate(ctx context.Context, userID int64, date int) error {
	return t.storage.SetDate(ctx, userID, date)
}

// SetSeq proxies Storage.SetSeq.
func (t *telegramStorage) SetSeq(ctx context.Context, userID int64, seq int) error {
	return t.storage.SetSeq(ctx, userID, seq)
}

// SetDateSeq proxies Storage.SetDateSeq.
func (t *telegramStorage) SetDateSeq(ctx context.Context, userID int64, date, seq int) error {
	return t.storage.SetDateSeq(ctx, userID, date, seq)
}

// GetChannelPts proxies Storage.GetChannelPts.
func (t *telegramStorage) GetChannelPts(
	ctx context.Context,
	userID, channelID int64,
) (pts int, found bool, err error) {
	return t.storage.GetChannelPts(ctx, userID, channelID)
}

// SetChannelPts proxies Storage.SetChannelPts.
func (t *telegramStorage) SetChannelPts(
	ctx context.Context,
	userID, channelID int64,
	pts int,
) error {
	return t.storage.SetChannelPts(ctx, userID, channelID, pts)
}

// SetChannelPts proxies Storage.ForEachChannels.
func (t *telegramStorage) ForEachChannels(
	ctx context.Context,
	userID int64,
	f func(ctx context.Context, channelID int64, pts int) error,
) error {
	return t.storage.ForEachChannels(ctx, userID, f)
}

// Implementation native `gotd` interface hasher
type telegramHasher struct {
	storage *Storage
}

// SetChannelAccessHash proxies Storage.SetChannelAccessHash.
//
// Example:
//
//	_ = stg.TelegramAccessHasherCompatible().SetChannelAccessHash(ctx, botID, chID, hash)
func (t *telegramHasher) SetChannelAccessHash(
	ctx context.Context,
	userID, channelID, accessHash int64,
) error {
	return t.storage.SetChannelAccessHash(ctx, userID, channelID, accessHash)
}

// GetChannelAccessHash proxies Storage.GetChannelAccessHash.
//
// Example:
//
//	hash, found, _ := stg.TelegramAccessHasherCompatible().GetChannelAccessHash(ctx, botID, chID)
func (t *telegramHasher) GetChannelAccessHash(
	ctx context.Context,
	userID,
	channelID int64,
) (accessHash int64, found bool, err error) {
	return t.storage.GetChannelAccessHash(ctx, userID, channelID)
}
