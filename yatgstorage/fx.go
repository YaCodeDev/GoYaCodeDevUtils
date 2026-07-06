package yatgstorage

import (
	"go.uber.org/fx"
	"gorm.io/gorm"
)

// StoreModuleName identifies the Fx module providing a Redis-backed Store.
const StoreModuleName = "yatgstorage-redis"

// StoreModule provides a Store backed by yacache.Cache[*redis.Client] via
// NewStorage. Consuming apps must additionally provide a
// yacache.Cache[*redis.Client] and a yalogger.Logger.
var StoreModule = fx.Module(
	StoreModuleName,
	fx.Provide(
		fx.Annotate(NewStorage, fx.As(new(Store))),
	),
)

// SessionRepoModuleName identifies the Fx module providing a GORM-backed
// EntitySessionStorageRepo.
const SessionRepoModuleName = "yatgstorage-gorm"

// SessionRepoModule provides an EntitySessionStorageRepo backed by GORM via
// NewGormSessionStorage. Consuming apps must additionally provide a
// *gorm.DB.
var SessionRepoModule = fx.Module(
	SessionRepoModuleName,
	fx.Provide(
		fx.Annotate(newGormSessionStorageForFx, fx.As(new(EntitySessionStorageRepo))),
	),
)

// newGormSessionStorageForFx adapts NewGormSessionStorage's yaerrors.Error
// return to the builtin error: fx.Annotate only recognizes a literal error
// as the trailing return of an annotated constructor, so a yaerrors.Error in
// that position is otherwise rejected by dig as a plain result field.
func newGormSessionStorageForFx(poolDB *gorm.DB) (*GormRepo, error) {
	repo, err := NewGormSessionStorage(poolDB)

	return repo, err
}
