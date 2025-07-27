package yatgclient_test

import (
	"fmt"
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yalogger"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yatgclient"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/proxy"
)

func TestSOCKS5_Works(t *testing.T) {
	const username = "skalse"
	const password = "lingvistka_sonya_echkere"
	const host = "yahost"
	const port = 8081

	url := fmt.Sprintf("socks5://%s:%s@%s:%d", username, password, host, port)

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

	t.Run("Get Full Address works", func(t *testing.T) {
		expected := fmt.Sprintf("%s:%d", host, port)

		assert.Equal(t, expected, socks5.GetFullAddress())
	})

	t.Run("Get Full Address works", func(t *testing.T) {
		expected := proxy.Auth{User: username, Password: password}

		assert.Equal(t, expected, *socks5.GetAuth())
	})

}
