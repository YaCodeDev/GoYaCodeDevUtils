package yatgclient

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yalogger"

	"github.com/gotd/td/telegram/dcs"
	"golang.org/x/net/proxy"
)

// SOCKS5 helper
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
