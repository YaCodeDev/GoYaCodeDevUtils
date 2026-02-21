package yatgbot

import (
	"context"
	"io/fs"
	"net/http"
	"strconv"
	"strings"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yacache"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yafsm"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yalocales"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yalogger"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yatgbot/messagequeue"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yatgclient"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yatgmessageencoding"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yatgstorage"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/updates"
	"github.com/gotd/td/tg"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type Options struct {
	DefaultLang               string
	AppID                     int
	AppHash                   string
	BotToken                  string
	PoolDB                    *gorm.DB
	MessageQueueRatePerSecond uint
	EmbeddedLocales           fs.FS
	Cache                     yacache.Cache[*redis.Client]
	MainRouter                *RouterGroup
	ParseMode                 yatgmessageencoding.MessageEncoding
	Sync                      bool
	Log                       yalogger.Logger
}

// InitYaTgBot initializes and returns a Dispatcher for the Telegram bot.
// It sets up the necessary components such as the Telegram client, session storage,
// FSM storage, localizer, and message dispatcher.
//
// Example usage:
//
//	options := yatgbot.Options{
//		DefaultLang:     "en",
//		AppID:           123456,
//		AppHash:         "your_app_hash",
//		BotToken:        "your_bot_token",
//		PoolDB:          yourGormDBInstance,
//		Cache:           yourRedisCacheInstance,
//		MainRouter:      yourMainRouterGroup,
//		ParseMode:       yourParseModeInstance,
//		Log:             yourLoggerInstance,
//		Sync:            false,
//		EmbeddedLocales: yourEmbeddedLocalesFS,
//	}
//
//	dispatcher, err := yatgbot.InitYaTgBot(ctx, options)
//	if err != nil {
//		// handle error
//	}
//
//nolint:gocritic // The Options is not that large to pass it by pointer and change API for that
func InitYaTgBot(
	ctx context.Context,
	options Options,
) (Dispatcher, yaerrors.Error) {
	head, _, _ := strings.Cut(options.BotToken, ":")

	BotID, err := strconv.ParseInt(strings.TrimSpace(head), 10, 64)
	if err != nil || BotID <= 0 {
		return Dispatcher{}, yaerrors.FromError(
			http.StatusBadRequest,
			err,
			"invalid bot token provided",
		)
	}

	telegramDispatcher := tg.NewUpdateDispatcher()

	fsmStorage := yafsm.NewDefaultFSMStorage(options.Cache, yafsm.EmptyState{})

	localizer := yalocales.NewLocalizer(options.DefaultLang, true)
	if yaErr := localizer.LoadLocales(options.EmbeddedLocales); yaErr != nil {
		return Dispatcher{}, yaErr
	}

	gormSessionRepo, yaErr := yatgstorage.NewGormSessionStorage(options.PoolDB)
	if yaErr != nil {
		return Dispatcher{}, yaErr
	}

	sessionStorage := yatgstorage.NewSessionStorageWithCustomRepo(
		BotID,
		options.BotToken,
		gormSessionRepo,
	)
	stateStorage := yatgstorage.NewStorage(options.Cache, options.Log)

	gaps := yatgclient.NewUpdateManagerWithYaStorage(
		BotID,
		telegramDispatcher,
		stateStorage,
	)

	client := yatgclient.NewClient(
		yatgclient.ClientOptions{
			AppID:    options.AppID,
			AppHash:  options.AppHash,
			EntityID: BotID,
			TelegramOptions: telegram.Options{
				SessionStorage: sessionStorage.TelegramSessionStorageCompatible(),
				UpdateHandler:  gaps,
			},
		},
		options.Log,
	)

	msgDispatcher := messagequeue.NewDispatcher(
		ctx,
		client,
		stateStorage,
		options.MessageQueueRatePerSecond,
		options.ParseMode,
		options.Log,
	)

	if err := client.BackgroundConnect(ctx); err != nil {
		return Dispatcher{}, err
	}

	if err := client.BotAuthorization(ctx, options.BotToken); err != nil {
		return Dispatcher{}, err
	}

	_ = client.RunUpdatesManager(ctx, gaps, updates.AuthOptions{IsBot: true}, nil)

	botUser, err := client.Self(ctx)
	if err != nil {
		return Dispatcher{}, yaerrors.FromError(
			http.StatusInternalServerError,
			err,
			"failed to get bot user",
		)
	}

	dispatcher := Dispatcher{
		FSMStore:          fsmStorage,
		Log:               options.Log,
		BotUser:           botUser,
		MessageDispatcher: msgDispatcher,
		Localizer:         localizer,
		Client:            client,
		MainRouter:        options.MainRouter,
	}

	dispatcher.Bind(&telegramDispatcher, options.Sync)

	return dispatcher, nil
}
