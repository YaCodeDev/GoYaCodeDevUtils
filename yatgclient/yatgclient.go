package yatgclient

import (
	"context"
	"errors"
	"net/http"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yalogger"
	"github.com/gotd/contrib/bg"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/updates"
	"github.com/gotd/td/tgerr"
)

type Client struct {
	*telegram.Client
	entityID int64
	log      yalogger.Logger
}

func NewClient(appID int, appHash string, entityID int64, options telegram.Options, log yalogger.Logger) *Client {
	client := telegram.NewClient(appID, appHash, options)

	return &Client{
		Client:   client,
		entityID: entityID,
		log:      log,
	}
}

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

	<-ctx.Done()

	if err := stop(); err != nil {
		c.log.Errorf("Failed to srop telegram client connection: %v", err)
	}

	return nil
}

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

type BotError struct {
	Err      yaerrors.Error
	EntityID int64
}

func (c *Client) RunUpdatesManager(
	ctx context.Context,
	gaps *updates.Manager,
	options updates.AuthOptions,
	channel *chan BotError,
) <-chan BotError {
	if channel == nil {
		c := make(chan BotError)
		channel = &c
	}

	c.log.Debug("Fetching self...")
	user, err := c.Self(ctx)
	if err != nil {
		go func() {
			*channel <- BotError{
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
			*channel <- BotError{
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

	c.log.Debug("Runned updates manager...")

	return *channel
}
