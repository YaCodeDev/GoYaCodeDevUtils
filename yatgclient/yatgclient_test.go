package yatgclient_test

import (
	"fmt"
	"net"
	"strconv"
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yalogger"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yatgclient"
	"github.com/gotd/td/tg"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/proxy"
)

func TestSOCKS5_Works(t *testing.T) {
	const (
		username = "skalse"
		password = "lingvistka_sonya_echkere"
		host     = "yahost"
		port     = 8081
	)

	url := fmt.Sprintf(
		"socks5://%s:%s@%s",
		username,
		password,
		net.JoinHostPort(host, strconv.Itoa(port)),
	)

	log := yalogger.NewBaseLogger(nil).NewLogger()

	socks5, _ := yatgclient.NewSOCKS5WithParseURL(url, log)

	t.Run("Username correct", func(t *testing.T) {
		assert.Equal(t, username, *socks5.Username)
	})

	t.Run("Password correct", func(t *testing.T) {
		assert.Equal(t, password, *socks5.Password)
	})

	t.Run("Host correct", func(t *testing.T) {
		assert.Equal(t, host, socks5.Host)
	})

	t.Run("Port correct", func(t *testing.T) {
		assert.Equal(t, uint16(port), socks5.Port)
	})

	t.Run("URL correct", func(t *testing.T) {
		assert.Equal(t, url, socks5.String())
	})

	t.Run("Get Full Address works", func(t *testing.T) {
		expected := fmt.Sprintf("%s:%d", host, port)

		assert.Equal(t, expected, socks5.GetFullAddress())
	})

	t.Run("Get Full Address works", func(t *testing.T) {
		expected := proxy.Auth{User: username, Password: password}

		assert.Equal(t, expected, *socks5.GetAuth())
	})
}

func TestMTProto_Works(t *testing.T) {
	const (
		secret = "https://open.spotify.com/track/1e1JKLEDKP7hEQzJfNAgPl?si=0dea7a7e6162462e"
		host   = "ya_playboy_carti"
		port   = 1847
	)

	url := fmt.Sprintf("https://t.me/proxy?server=%s&port=%d&secret=%s", host, port, secret)

	log := yalogger.NewBaseLogger(nil).NewLogger()

	mtproto, _ := yatgclient.NewMTProtoWithParseURL(url, log)

	t.Run("Secret correct", func(t *testing.T) {
		assert.Equal(t, secret, mtproto.Secret)
	})

	t.Run("Host correct", func(t *testing.T) {
		assert.Equal(t, host, mtproto.Host)
	})

	t.Run("Port correct", func(t *testing.T) {
		assert.Equal(t, uint16(port), mtproto.Port)
	})

	t.Run("Get Full Address works", func(t *testing.T) {
		expected := fmt.Sprintf("%s:%d", host, port)

		assert.Equal(t, expected, mtproto.GetFullAddress())
	})

	t.Run("Get Input Client Proxy works", func(t *testing.T) {
		expected := tg.InputClientProxy{
			Address: host,
			Port:    port,
		}

		assert.Equal(t, expected, mtproto.GetInputClientProxy())
	})

	t.Run("URL correct", func(t *testing.T) {
		assert.Equal(t, url, mtproto.String())
	})
}
