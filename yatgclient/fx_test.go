package yatgclient_test

import (
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yalogger"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yatgclient"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
)

func TestModule_GraphResolves(t *testing.T) {
	t.Parallel()

	err := fx.ValidateApp(
		yatgclient.Module,
		fx.Provide(func() *yatgclient.ClientOptions { return &yatgclient.ClientOptions{} }),
		fx.Provide(func() yalogger.Logger { return nil }),
		fx.Populate(new(*yatgclient.Client)),
	)

	require.NoError(t, err)
}
