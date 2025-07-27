package yatgclient

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yalogger"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yatgstorage"
	"github.com/gotd/contrib/bg"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/dcs"
	"github.com/gotd/td/telegram/updates"
	"github.com/gotd/td/tg"
	"github.com/gotd/td/tgerr"
	"golang.org/x/net/proxy"
)

type Client struct {
	*telegram.Client
	entityID int64
	log      yalogger.Logger
}

type ClientOptions struct {
	AppID           int
	AppHash         string
	EntityID        int64
	TelegramOptions telegram.Options
}

func NewClient(options ClientOptions, log yalogger.Logger) *Client {
	client := telegram.NewClient(options.AppID, options.AppHash, options.TelegramOptions)

	return &Client{
		Client:   client,
		entityID: options.EntityID,
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

	go func() {
		<-ctx.Done()

		if err := stop(); err != nil {
			c.log.Errorf("Failed to stop telegram client connection: %v", err)
		}
	}()

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

type EntityError struct {
	Err      yaerrors.Error
	EntityID int64
}

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

func NewUpdateManagerWithYaStorage(storage yatgstorage.IStorage) *updates.Manager {
	return updates.New(updates.Config{
		Handler:      storage.AccessHashSaveHandler(),
		Storage:      storage.TelegramStorageCompatible(),
		AccessHasher: storage.TelegramAccessHasherCompatible(),
	})
}

type SOCKS5 struct {
	Host     string
	Port     uint16
	Username *string
	Password *string
}

func NewSOCKS5WithParseURL(url string, log yalogger.Logger) (*SOCKS5, yaerrors.Error) {
	socks5 := SOCKS5{}

	if err := socks5.ParseURL(url, log); err != nil {
		return nil, err.WrapWithLog("failed to create new socks5 proxy with url", log)
	}

	return &socks5, nil
}

func (s *SOCKS5) String() string {
	hostPort := s.GetFullAddress()

	if s.Username != nil && s.Password != nil {
		return fmt.Sprintf("socks5://%s:%s@%s", *s.Username, *s.Password, hostPort)
	}

	return "socks5://" + hostPort
}

func (s *SOCKS5) GetFullAddress() string {
	return net.JoinHostPort(s.Host, strconv.Itoa(int(s.Port)))
}

func (s *SOCKS5) GetAuth() *proxy.Auth {
	if s.Username == nil || s.Password == nil {
		return nil
	}

	return &proxy.Auth{User: *s.Username, Password: *s.Password}
}

func (s *SOCKS5) ParseURL(proxyURL string, log yalogger.Logger) yaerrors.Error {
	u, err := url.Parse(proxyURL)
	if err != nil {
		return yaerrors.FromErrorWithLog(
			http.StatusInternalServerError,
			err,
			"failed to parse proxy url",
			log,
		)
	}

	switch u.Scheme {
	case "socks5", "socks5h":
	default:
		return yaerrors.FromStringWithLog(
			http.StatusInternalServerError,
			fmt.Sprintf("unsupported proxy scheme %q (want socks5/socks5h)", u.Scheme),
			log,
		)
	}

	s.Host = u.Hostname()

	portStr := u.Port()
	if portStr == "" {
		log.Warn("proxy port not specified, using default 1080")

		portStr = "1080"
	}

	portInt, err := strconv.Atoi(portStr)
	if err != nil {
		return yaerrors.FromErrorWithLog(
			http.StatusInternalServerError,
			err,
			"invalid proxy port",
			log,
		)
	}

	if portInt <= 0 || portInt > 65535 {
		return yaerrors.FromStringWithLog(
			http.StatusInternalServerError,
			fmt.Sprintf("proxy port %d out of range 1–65535", portInt),
			log,
		)
	}

	s.Port = uint16(portInt)

	s.Username, s.Password = nil, nil

	if u.User != nil {
		user := u.User.Username()

		s.Username = &user
		if pass, ok := u.User.Password(); ok {
			s.Password = &pass
		}
	}

	return nil
}

func (s *SOCKS5) GetContextDialer(log yalogger.Logger) (proxy.ContextDialer, yaerrors.Error) {
	socks5, err := proxy.SOCKS5("tcp", s.GetFullAddress(), s.GetAuth(), proxy.Direct)
	if err != nil {
		return nil, yaerrors.FromErrorWithLog(
			http.StatusInternalServerError,
			err,
			"failed to create SOCKS5 proxy",
			log,
		)
	}

	contextDialer, ok := socks5.(proxy.ContextDialer)
	if !ok {
		return nil, yaerrors.FromErrorWithLog(
			http.StatusInternalServerError,
			err,
			"failed to cast proxy to ContextDialer",
			log,
		)
	}

	return contextDialer, nil
}

func (s *SOCKS5) GetResolver(log yalogger.Logger) (dcs.Resolver, yaerrors.Error) {
	dialer, err := s.GetContextDialer(log)
	if err != nil {
		return nil, yaerrors.FromErrorWithLog(
			http.StatusInternalServerError,
			err,
			"failed to get context dialer",
			log,
		)
	}

	return dcs.Plain(dcs.PlainOptions{Dial: dialer.DialContext}), nil
}

type MTProto struct {
	Host   string
	Port   uint16
	Secret string
}

func (m *MTProto) String() string {
	return fmt.Sprintf(
		"https://t.me/proxy?server=%s&port=%d&secret=%s",
		m.Host, m.Port, m.Secret,
	)
}

func (m *MTProto) GetFullAddress() string {
	return fmt.Sprintf("%s:%d", m.Host, m.Port)
}

func (m *MTProto) ParseURL(proxyURL string, log yalogger.Logger) yaerrors.Error {
	u, err := url.Parse(proxyURL)
	if err != nil {
		return yaerrors.FromErrorWithLog(
			http.StatusInternalServerError,
			err,
			"failed to parse url for mtproto",
			log,
		)
	}

	const queryHost = "server"
	const queryPort = "port"
	const querySecret = "secret"

	host := u.Query().Get(queryHost)
	if len(host) == 0 {
		return yaerrors.FromStringWithLog(
			http.StatusInternalServerError,
			"failed to get host query",
			log,
		)
	}

	port := u.Query().Get(queryPort)
	if len(port) == 0 {
		return yaerrors.FromStringWithLog(
			http.StatusInternalServerError,
			"failed to get port query",
			log,
		)
	}

	secret := u.Query().Get(querySecret)
	if len(port) == 0 {
		return yaerrors.FromStringWithLog(
			http.StatusInternalServerError,
			"failed to get secret query",
			log,
		)
	}

	portInt, err := strconv.Atoi(port)
	if err != nil {
		return yaerrors.FromErrorWithLog(
			http.StatusInternalServerError,
			err,
			"failed to parse port for mtproto",
			log,
		)
	}

	if portInt <= 0 || portInt > 65535 {
		return yaerrors.FromStringWithLog(
			http.StatusInternalServerError,
			fmt.Sprintf("proxy port %d out of range 1–65535", portInt),
			log,
		)
	}

	m.Host = host
	m.Secret = secret
	m.Port = uint16(portInt)

	return nil
}

func (m *MTProto) GetResolver(log yalogger.Logger) (dcs.Resolver, yaerrors.Error) {
	if len(m.Host) == 0 {
		return nil, yaerrors.FromStringWithLog(
			http.StatusInternalServerError,
			"empty host tag in mtproto",
			log,
		)
	}

	if m.Port == 0 {
		return nil, yaerrors.FromStringWithLog(
			http.StatusInternalServerError,
			"proxy port equel zero",
			log,
		)
	}

	secret, err := hex.DecodeString(m.Secret)
	if err != nil {
		return nil, yaerrors.FromErrorWithLog(
			http.StatusInternalServerError,
			err,
			"failed to decode string as hex bytes",
			log,
		)
	}

	proxy, err := dcs.MTProxy(m.GetFullAddress(), secret, dcs.MTProxyOptions{})
	if err != nil {
		return nil, yaerrors.FromErrorWithLog(
			http.StatusInternalServerError,
			err,
			"failed to create mtproto resolver",
			log,
		)
	}

	return proxy, nil
}

func (m *MTProto) GetInputClientProxy() tg.InputClientProxy {
	return tg.InputClientProxy{
		Address: m.Host,
		Port:    int(m.Port),
	}
}

func NewMTProtoWithParseURL(url string, log yalogger.Logger) (*MTProto, yaerrors.Error) {
	mtproto := MTProto{}

	if err := mtproto.ParseURL(url, log); err != nil {
		return nil, err.WrapWithLog("failed to create new mtproto proxy with url", log)
	}

	return &mtproto, nil
}
