package yacache

import "errors"

var (
	ErrFailedToSet             = errors.New("[CACHE] failed to set new value with ttl")
	ErrFailedToHSetEx          = errors.New("[CACHE] failed to hash set new value with ttl")
	ErrFailedToGetValue        = errors.New("[CACHE] failed to get a value")
	ErrFailedToMGetValues      = errors.New("[CACHE] failed to get multi values")
	ErrFailedToDelValue        = errors.New("[CACHE] failed to delete a value")
	ErrFailedToGetValues       = errors.New("[CACHE] failed to get values")
	ErrFailedToGetDelValue     = errors.New("[CACHE] faildet to get and delete value")
	ErrFailedToGetDeleteSingle = errors.New("[CACHE] faildet to get and delete single value")
	ErrNotFoundValue           = errors.New("[CACHE] not found a value")
	ErrFailedToGetLen          = errors.New("[CACHE] failed to get len")
	ErrFailedToExists          = errors.New("[CACHE] failed to get exists a value")
	ErrFailedToHExist          = errors.New("[CACHE] failed to get hash exists a value")
	ErrFailedToDeleteSingle    = errors.New("[CACHE] failed to delete value")
	ErrFailedPing              = errors.New("[CACHE] failed to get `PONG` from ping")
	ErrFailedToCloseBackend    = errors.New("[CACHE] failed to close backend")
)
