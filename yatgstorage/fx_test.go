package yatgstorage_test

import (
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yacache"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yalogger"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yatgstorage"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"gorm.io/gorm"
)

func TestStoreModule_GraphResolves(t *testing.T) {
	t.Parallel()

	err := fx.ValidateApp(
		yatgstorage.StoreModule,
		fx.Provide(func() yacache.Cache[*redis.Client] { return nil }),
		fx.Provide(func() yalogger.Logger { return nil }),
		fx.Populate(new(yatgstorage.Store)),
	)

	require.NoError(t, err)
}

func TestSessionRepoModule_GraphResolves(t *testing.T) {
	t.Parallel()

	err := fx.ValidateApp(
		yatgstorage.SessionRepoModule,
		fx.Provide(func() *gorm.DB { return nil }),
		fx.Populate(new(yatgstorage.EntitySessionStorageRepo)),
	)

	require.NoError(t, err)
}
