package yacache

import "errors"

var (
	ErrFailedToGetChildMap        = errors.New("[MEMORY] failed to get child map")
	ErrFailedToGetValueInChildMap = errors.New("[MEMORY] failed to get value in child map")
	ErrKeyNotFoundInChildMap      = errors.New("[MEMORY] childKey not found in childMap")

	ErrRedisKeyNotFound             = errors.New("[REDIS] key not found")
	ErrRedisNotFoundValueInChildMap = errors.New("[REDIS] not found value in child map")
)
