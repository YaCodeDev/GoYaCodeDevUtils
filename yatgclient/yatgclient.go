// Package yatgclient provides a thin convenience wrapper around gotd’s
// telegram.Client adding:
//   - background‑connect helper with graceful shutdown
//   - automatic bot‑token authorisation
//   - updates.Manager wiring to yatgstorage (pts/qts/etc.)
//   - SOCKS5 and MTProto proxy helpers (URL ↔ struct, dialer/resolver utilities)
package yatgclient

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yabackoff"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yalogger"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yatgstorage"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/updates"
	"github.com/gotd/td/tgerr"
)

type runClientFunc func(ctx context.Context, f func(ctx context.Context) error) error

type BackgroundConnectConfig struct {
	InitialInterval time.Duration
	Multiplier      float64
	MaxInterval     time.Duration
	ResetAfter      time.Duration
}

var errBackgroundConnectLoopExited = errors.New(
	"telegram background connect loop exited without error",
)

var errUpdatesManagerLoopExited = errors.New(
	"updates manager loop exited without error",
)

const (
	defaultBackgroundReconnectInitialInterval = time.Second
	defaultBackgroundReconnectMultiplier      = 2.0
	defaultBackgroundReconnectMaxInterval     = 2 * time.Minute
	defaultBackgroundReconnectResetAfter      = 5 * time.Minute
)

// Client wrapper
type Client struct {
	*telegram.Client
	ClientOptions
	log   yalogger.Logger
	IsBot bool
}

// Options to create a Client.
type ClientOptions struct {
	AppID                   int
	AppHash                 string
	EntityID                int64
	TelegramOptions         telegram.Options
	ChunkSize               int64
	BackgroundConnectConfig BackgroundConnectConfig
}

// NewClient constructs a wrapper around gotd’s *telegram.Client.
//
// Example:
//
//	cli := yatgclient.NewClient(&yatgclient.ClientOptions{
//		AppID: 12345, AppHash: "abcd", EntityID: 42,
//		TelegramOptions: telegram.Options{},
//	}, log)
func NewClient(
	options *ClientOptions,
	log yalogger.Logger,
) *Client {
	client := telegram.NewClient(options.AppID, options.AppHash, options.TelegramOptions)

	clientOptions := *options

	if clientOptions.ChunkSize == 0 {
		clientOptions.ChunkSize = DefaultChunkSize
	}

	clientOptions.BackgroundConnectConfig = normalizeBackgroundConnectConfig(
		clientOptions.BackgroundConnectConfig,
	)

	return &Client{
		Client:        client,
		ClientOptions: clientOptions,
		log:           log,
	}
}

// BackgroundConnect dials Telegram in a goroutine and stops automatically when
// ctx is cancelled.
//
// Example:
//
//	_ = cli.BackgroundConnect(ctx)
func (c *Client) BackgroundConnect(ctx context.Context) yaerrors.Error {
	return backgroundConnect(
		ctx,
		c.Run,
		c.log,
		c.BackgroundConnectConfig,
	)
}

func backgroundConnect(
	ctx context.Context,
	run runClientFunc,
	log yalogger.Logger,
	config BackgroundConnectConfig,
) yaerrors.Error {
	connected := make(chan struct{})
	fatalErrC := make(chan error, 1)

	go func() {
		reconnectBackoff := yabackoff.NewExponential(
			config.InitialInterval,
			config.Multiplier,
			config.MaxInterval,
			config.ResetAfter,
		)

		hasConnected := false

		for {
			if ctx.Err() != nil {
				return
			}

			connectedThisRun := make(chan struct{}, 1)

			err := run(ctx, func(runCtx context.Context) error {
				select {
				case connectedThisRun <- struct{}{}:
				default:
				}

				if !hasConnected {
					hasConnected = true

					close(connected)
				}

				<-runCtx.Done()

				if errors.Is(runCtx.Err(), context.Canceled) {
					return nil
				}

				return runCtx.Err()
			})

			if ctx.Err() != nil {
				return
			}

			if err == nil {
				err = errBackgroundConnectLoopExited
			}

			connectedInRun := false

			select {
			case <-connectedThisRun:
				connectedInRun = true
			default:
			}

			if !connectedInRun {
				select {
				case fatalErrC <- err:
				default:
				}

				return
			}

			log.Errorf("[YaTGClient] Telegram client connection stopped: %v", err)

			delay := reconnectBackoff.Next()
			log.Warnf("[YaTGClient] Retrying telegram client connection in %s", delay)

			retryTimer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				retryTimer.Stop()

				return
			case <-retryTimer.C:
			}
		}
	}()

	select {
	case <-connected:
		return nil
	case err := <-fatalErrC:
		return yaerrors.FromErrorWithLog(
			http.StatusInternalServerError,
			err,
			"[YaTGClient] failed to connect background client",
			log,
		)
	case <-ctx.Done():
		return yaerrors.FromErrorWithLog(
			http.StatusInternalServerError,
			ctx.Err(),
			"[YaTGClient] failed to connect background client",
			log,
		)
	}
}

// BotAuthorization ensures the client is authorised via botToken.
//
// Example:
//
//	_ = cli.BotAuthorization(ctx, "123:ABC")
func (c *Client) BotAuthorization(ctx context.Context, botToken string) yaerrors.Error {
	status, err := c.Auth().Status(ctx)
	if err != nil {
		return yaerrors.FromErrorWithLog(
			http.StatusInternalServerError,
			err,
			"[YaTGClient] failed to check status bot authorization",
			c.log,
		)
	}

	if !status.Authorized {
		if _, err := c.Auth().Bot(ctx, botToken); err != nil {
			tgerr := &tgerr.Error{}
			if errors.As(err, &tgerr) {
				c.log.Errorf("[YaTGClient] %s", tgerr.Error())
			} else {
				c.log.Errorf("[YaTGClient] %v", err)
			}

			return yaerrors.FromErrorWithLog(
				http.StatusInternalServerError,
				err,
				"[YaTGClient] failed to bot authorization",
				c.log,
			)
		}
	}

	c.IsBot = true

	return nil
}

// EntityError couples a processing error with the bot entityID.
// Used by RunUpdatesManager for multi‑bot setups.
type EntityError struct {
	Err      yaerrors.Error
	EntityID int64
}

// RunUpdatesManager starts an updates.Manager in the background and returns a
// channel where any fatal error is sent.
//
// Example:
//
//	errs := client.RunUpdatesManager(ctx, gaps, updates.AuthOptions{}, nil)
//	if err := <-errs; err.Err != nil { log.Fatalf("%v", err.Err) }
func (c *Client) RunUpdatesManager(
	ctx context.Context,
	gaps *updates.Manager,
	options updates.AuthOptions,
	channel *chan EntityError,
) <-chan EntityError {
	if channel == nil {
		ch := make(chan EntityError, 1)
		channel = &ch
	}

	c.log.Debug("[YaTGClient] Fetching self...")

	entityID := c.EntityID
	if entityID == 0 {
		user, err := c.Self(ctx)
		if err != nil {
			emitEntityError(
				channel,
				EntityError{
					Err: yaerrors.FromErrorWithLog(
						http.StatusInternalServerError,
						err,
						"[YaTGClient] failed to get self updates manager",
						c.log,
					),
					EntityID: c.EntityID,
				},
			)

			return *channel
		}

		entityID = user.ID
	}

	c.log.Debug("[YaTGClient] Running updates manager...")

	go func() {
		reconnectConfig := normalizeBackgroundConnectConfig(c.BackgroundConnectConfig)
		reconnectBackoff := yabackoff.NewExponential(
			reconnectConfig.InitialInterval,
			reconnectConfig.Multiplier,
			reconnectConfig.MaxInterval,
			reconnectConfig.ResetAfter,
		)

		for {
			if ctx.Err() != nil {
				return
			}

			gaps.Reset()

			err := gaps.Run(ctx, c.API(), entityID, options)
			if ctx.Err() != nil {
				return
			}

			if err == nil {
				err = errUpdatesManagerLoopExited
			}

			emitEntityError(
				channel,
				EntityError{
					Err: yaerrors.FromErrorWithLog(
						http.StatusInternalServerError,
						err,
						"[YaTGClient] failed to run updates manager",
						c.log,
					),
					EntityID: entityID,
				},
			)

			c.log.Errorf("[YaTGClient] Updates manager stopped: %v", err)

			delay := reconnectBackoff.Next()
			c.log.Warnf("[YaTGClient] Retrying updates manager in %s", delay)

			retryTimer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				retryTimer.Stop()

				return
			case <-retryTimer.C:
			}
		}
	}()

	c.log.Debug("[YaTGClient] Updates manager started...")

	return *channel
}

// NewUpdateManagerWithYaStorage creates an updates.Manager pre‑wired to a
// yatgstorage implementation.
//
// Example:
//
//	gaps := yatgclient.NewUpdateManagerWithYaStorage(entityID, handler, storage)
func NewUpdateManagerWithYaStorage(
	entityID int64,
	handler telegram.UpdateHandler,
	storage yatgstorage.Store,
) *updates.Manager {
	return updates.New(updates.Config{
		Handler:      storage.AccessHashSaveHandler(entityID, handler),
		Storage:      storage.TelegramStorageCompatible(),
		AccessHasher: storage.TelegramAccessHasherCompatible(),
	})
}

func emitEntityError(channel *chan EntityError, entityError EntityError) {
	select {
	case *channel <- entityError:
	default:
	}
}

func normalizeBackgroundConnectConfig(config BackgroundConnectConfig) BackgroundConnectConfig {
	if config.InitialInterval == 0 {
		config.InitialInterval = defaultBackgroundReconnectInitialInterval
	}

	if config.Multiplier == 0 {
		config.Multiplier = defaultBackgroundReconnectMultiplier
	}

	if config.MaxInterval == 0 {
		config.MaxInterval = defaultBackgroundReconnectMaxInterval
	}

	if config.ResetAfter == 0 {
		config.ResetAfter = defaultBackgroundReconnectResetAfter
	}

	return config
}
