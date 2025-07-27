package yatgclient

import (
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yalogger"

	"github.com/gotd/td/telegram/dcs"
	"github.com/gotd/td/tg"
)

// MTProto proxy helper
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

	if len(secret) == 0 {
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
			"proxy port equal zero",
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
