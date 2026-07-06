package yatgbot_test

import (
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yatgbot"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
)

func TestModule_GraphResolves(t *testing.T) {
	t.Parallel()

	err := fx.ValidateApp(
		yatgbot.Module,
		fx.Provide(func() *yatgbot.Options { return &yatgbot.Options{} }),
		fx.Populate(new(*yatgbot.Dispatcher)),
	)

	require.NoError(t, err)
}
