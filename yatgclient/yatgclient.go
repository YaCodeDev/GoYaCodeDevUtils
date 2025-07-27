// Package yatgclient provides a thin convenience wrapper around gotd’s
// telegram.Client adding:
//   - background‑connect helper with graceful shutdown
//   - automatic bot‑token authorisation
//   - updates.Manager wiring to yatgstorage (pts/qts/etc.)
//   - SOCKS5 and MTProto proxy helpers (URL ↔ struct, dialer/resolver utilities)
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

// -----------------------------------------------------------------------------
// Client wrapper
// -----------------------------------------------------------------------------
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

// NewClient constructs a wrapper around gotd’s *telegram.Client.
//
// Example:
//
//	cli := yatgclient.NewClient(yatgclient.ClientOptions{
//	    AppID: 12345, AppHash: "abcd", EntityID: 42,
//	    TelegramOptions: telegram.Options{},
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

// -----------------------------------------------------------------------------
// SOCKS5 helper
// -----------------------------------------------------------------------------

type SOCKS5 struct {
	Host     string
	Port     uint16
	Username *string
	Password *string
}

// NewSOCKS5WithParseURL parses a socks5:// URL into a SOCKS5 struct(socks5://username:password@host:port)
//
// Example:
//
//	p, _ := yatgclient.NewSOCKS5WithParseURL("socks5://user:pass@1.2.3.4:1080", log)
func NewSOCKS5WithParseURL(url string, log yalogger.Logger) (*SOCKS5, yaerrors.Error) {
	socks5 := SOCKS5{}

	if err := socks5.ParseURL(url, log); err != nil {
		return nil, err.WrapWithLog("failed to create new socks5 proxy with url", log)
	}

	return &socks5, nil
}

// String returns socks5://… representation.
func (s *SOCKS5) String() string {
	hostPort := s.GetFullAddress()

	if s.Username != nil && s.Password != nil {
		return fmt.Sprintf("socks5://%s:%s@%s", *s.Username, *s.Password, hostPort)
	}

	return "socks5://" + hostPort
}

// GetFullAddress returns host:port.
func (s *SOCKS5) GetFullAddress() string {
	return net.JoinHostPort(s.Host, strconv.Itoa(int(s.Port)))
}

// GetAuth converts embedded creds into *proxy.Auth.
func (s *SOCKS5) GetAuth() *proxy.Auth {
	if s.Username == nil || s.Password == nil {
		return nil
	}

	return &proxy.Auth{User: *s.Username, Password: *s.Password}
}

// ParseURL fills the struct from a socks5:// URL.
//
// Example:
//
//	var socks5 yatgclient.SOCKS5
//	_ = socks5.ParseURL("socks5://1.2.3.4:1080", log)
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

// GetContextDialer converts SOCKS5 config into proxy.ContextDialer.
//
// Example:
//
//	dialer, _ := socks5.GetContextDialer(log)
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

// GetResolver returns a DC resolver using the SOCKS5 dialer.
//
// Example:
//
//	resolver, _ := socks5.GetResolver(log)
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

// -----------------------------------------------------------------------------
// MTProto proxy helper
// -----------------------------------------------------------------------------

type MTProto struct {
	Host   string
	Port   uint16
	Secret string
}

// NewMTProtoWithParseURL is a helper that allocates MTProto and calls ParseURL.
//
// Example:
//
//	mtproto, _ := yatgclient.NewMTProtoWithParseURL("https://t.me/proxy?server=1.2.3.4&port=443&secret=abcdef", log)
func NewMTProtoWithParseURL(url string, log yalogger.Logger) (*MTProto, yaerrors.Error) {
	mtproto := MTProto{}

	if err := mtproto.ParseURL(url, log); err != nil {
		return nil, err.WrapWithLog("failed to create new mtproto proxy with url", log)
	}

	return &mtproto, nil
}

// String assembles a `t.me/proxy` share link from the struct fields.
//
// Example:
//
//	m := yatgclient.MTProto{Host: "1.2.3.4", Port: 443, Secret: "abcdef"}
//	link := m.String() // https://t.me/proxy?server=1.2.3.4&port=443&secret=abcdef
func (m *MTProto) String() string {
	return fmt.Sprintf(
		"https://t.me/proxy?server=%s&port=%d&secret=%s",
		m.Host, m.Port, m.Secret,
	)
}

// GetFullAddress returns the `host:port` pair suitable for dialing.
//
// Example:
//
//	addr := m.GetFullAddress() // "1.2.3.4:443"
func (m *MTProto) GetFullAddress() string {
	return fmt.Sprintf("%s:%d", m.Host, m.Port)
}

// ParseURL populates the struct from a t.me/proxy share link.
//
// Supported formats:
//
//	https://t.me/proxy?server=<host>&port=<port>&secret=<hex>
//
// Example:
//
//	var m yatgclient.MTProto
//	_ = m.ParseURL("https://t.me/proxy?server=1.2.3.4&port=443&secret=abcdef", log)
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

	const (
		queryHost   = "server"
		queryPort   = "port"
		querySecret = "secret"
	)

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

// GetResolver builds a gotd `dcs.Resolver` backed by an MTProxy.
//
// Example:
//
//	resolver, _ := mtproto.GetResolver(log)
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

// GetInputClientProxy converts the struct into tg.InputClientProxy from gotd
//
// Example:
//
//	inputClientProxy := m.GetInputClientProxy()
func (m *MTProto) GetInputClientProxy() tg.InputClientProxy {
	return tg.InputClientProxy{
		Address: m.Host,
		Port:    int(m.Port),
	}
}
