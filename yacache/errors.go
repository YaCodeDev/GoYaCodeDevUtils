package yacache

import "errors"

var (
	ErrFailedToSetNewValue     = errors.New("[CACHE] failed to set new value in `HSETEX`")
	ErrFailedToGetValue        = errors.New("[CACHE] failed to get value")
	ErrFailedToGetValues       = errors.New("[CACHE] failed to get values")
	ErrFailedToGetDeleteSingle = errors.New("[CACHE] faildet to get and delete value")
	ErrNotFoundValue           = errors.New("[CACHE] not found value")
	ErrFailedToGetLen          = errors.New("[CACHE] failed to get len")
	ErrFailedToGetExist        = errors.New("[CACHE] failed to get exists value")
	ErrFailedToDeleteSingle    = errors.New("[CACHE] failed to delete value")
	ErrFailedPing              = errors.New("[CACHE] failed to get `PONG` from ping")
	ErrFailedToCloseBackend    = errors.New("[CACHE] failed to close backend")
)
