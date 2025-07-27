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

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yalogger"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yatgstorage"

	"github.com/gotd/contrib/bg"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/updates"
	"github.com/gotd/td/tgerr"
)

// Client wrapper
type Client struct {
	*telegram.Client
	entityID int64
	log      yalogger.Logger
}

// Options to create a Client.
type ClientOptions struct {
	AppID           int
	AppHash         string
	EntityID        int64
	TelegramOptions telegram.Options
}

// NewClient constructs a wrapper around gotd’s *telegram.Client.
//
// Example:
//
//	cli := yatgclient.NewClient(yatgclient.ClientOptions{
//		AppID: 12345, AppHash: "abcd", EntityID: 42,
//		TelegramOptions: telegram.Options{},
//	}, log)
func NewClient(options ClientOptions, log yalogger.Logger) *Client {
	client := telegram.NewClient(options.AppID, options.AppHash, options.TelegramOptions)

	return &Client{
		Client:   client,
		entityID: options.EntityID,
		log:      log,
	}
}

// BackgroundConnect dials Telegram in a goroutine and stops automatically when
// ctx is cancelled.
//
// Example:
//
//	_ = cli.BackgroundConnect(ctx)
func (c *Client) BackgroundConnect(ctx context.Context) yaerrors.Error {
	stop, err := bg.Connect(c, bg.WithContext(ctx))
	if err != nil {
		return yaerrors.FromErrorWithLog(
			http.StatusInternalServerError,
			err,
			"failed to connect background client",
			c.log,
		)
	}

	go func() {
		<-ctx.Done()

		if err := stop(); err != nil {
			c.log.Errorf("Failed to stop telegram client connection: %v", err)
		}
	}()

	return nil
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
			"failed to check status bot authorization",
			c.log,
		)
	}

	if !status.Authorized {
		if _, err := c.Auth().Bot(ctx, botToken); err != nil {
			tgerr := &tgerr.Error{}
			if errors.As(err, &tgerr) {
				c.log.Errorf("%s", tgerr.Error())
			} else {
				c.log.Errorf("%v", err)
			}

			return yaerrors.FromErrorWithLog(
				http.StatusInternalServerError,
				err,
				"failed to bot authorization",
				c.log,
			)
		}
	}

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
		c := make(chan EntityError)
		channel = &c
	}

	c.log.Debug("Fetching self...")

	user, err := c.Self(ctx)
	if err != nil {
		go func() {
			*channel <- EntityError{
				Err: yaerrors.FromErrorWithLog(
					http.StatusInternalServerError,
					err,
					"failed to get self updates manager",
					c.log,
				),
				EntityID: c.entityID,
			}
		}()

		return *channel
	}

	c.log.Debug("Running updates manager...")

	go func() {
		if err = gaps.Run(ctx, c.API(), user.ID, options); err != nil {
			*channel <- EntityError{
				Err: yaerrors.FromErrorWithLog(
					http.StatusInternalServerError,
					err,
					"failed to run updates manager",
					c.log,
				),
				EntityID: c.entityID,
			}
		}
	}()

	c.log.Debug("Updates manager started...")

	return *channel
}

// NewUpdateManagerWithYaStorage creates an updates.Manager pre‑wired to a
// yatgstorage implementation.
//
// Example:
//
//	gaps := yatgclient.NewUpdateManagerWithYaStorage(storage)
func NewUpdateManagerWithYaStorage(storage yatgstorage.IStorage) *updates.Manager {
	return updates.New(updates.Config{
		Handler:      storage.AccessHashSaveHandler(),
		Storage:      storage.TelegramStorageCompatible(),
		AccessHasher: storage.TelegramAccessHasherCompatible(),
	})
}
