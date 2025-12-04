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
	"github.com/YaCodeDev/GoYaCodeDevUtils/yatgstorage"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/updates"
	"github.com/gotd/td/tg"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// InitYaTgBot initializes and returns a Dispatcher for the Telegram bot.
// It sets up the necessary components such as the Telegram client, session storage,
// FSM storage, localizer, and message dispatcher.
//
// Example usage:
//
//	dispatcher, err := InitYaTgBot(
//	    ctx,
//	    "en",
//	    appID,
//	    appHash,
//	    botToken,
//	    poolDB,
//	    10,
//	    embeddedLocales,
//	    log,
//	    cache,
//	    mainRouter,
//	)
//
//	If err != nil {
//	    // Handle error
//	}
func InitYaTgBot(
	ctx context.Context,
	defaultLang string,
	appID int,
	appHash string,
	botToken string,
	poolDB *gorm.DB,
	messageQueueRatePerSecond uint,
	embeddedLocales fs.FS,
	log yalogger.Logger,
	cache yacache.Cache[*redis.Client],
	mainRouter *RouterGroup,
) (Dispatcher, yaerrors.Error) {
	head, _, _ := strings.Cut(botToken, ":")

	BotID, err := strconv.ParseInt(strings.TrimSpace(head), 10, 64)
	if err != nil || BotID <= 0 {
		return Dispatcher{}, yaerrors.FromError(
			http.StatusBadRequest,
			err,
			"invalid bot token provided",
		)
	}

	telegramDispatcher := tg.NewUpdateDispatcher()

	fsmStorage := yafsm.NewDefaultFSMStorage(cache, yafsm.EmptyState{})

	localizer := yalocales.NewLocalizer(defaultLang, true)
	if yaErr := localizer.LoadLocales(embeddedLocales); yaErr != nil {
		return Dispatcher{}, yaErr
	}

	gormSessionRepo, yaErr := yatgstorage.NewGormSessionStorage(poolDB)
	if yaErr != nil {
		return Dispatcher{}, yaErr
	}

	sessionStorage := yatgstorage.NewSessionStorageWithCustomRepo(BotID, botToken, gormSessionRepo)
	stateStorage := yatgstorage.NewStorage(cache, log)

	gaps := yatgclient.NewUpdateManagerWithYaStorage(
		BotID,
		telegramDispatcher,
		stateStorage,
	)

	client := yatgclient.NewClient(
		yatgclient.ClientOptions{
			AppID:    appID,
			AppHash:  appHash,
			EntityID: BotID,
			TelegramOptions: telegram.Options{
				SessionStorage: sessionStorage.TelegramSessionStorageCompatible(),
				UpdateHandler:  gaps,
			},
		},
		log,
	)

	msgDispatcher := messagequeue.NewDispatcher(ctx, client, messageQueueRatePerSecond, log)

	if err := client.BackgroundConnect(ctx); err != nil {
		return Dispatcher{}, err
	}

	if err := client.BotAuthorization(ctx, botToken); err != nil {
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
		Log:               log,
		BotUser:           botUser,
		MessageDispatcher: msgDispatcher,
		Localizer:         localizer,
		Client:            client,
		MainRouter:        mainRouter,
	}

	dispatcher.Bind(&telegramDispatcher)

	return dispatcher, nil
}
