package yalocales

import (
	"errors"
)

var (
	ErrInvalidLanguage        = errors.New("invalid language")
	ErrInvalidTranslation     = errors.New("invalid translation")
	ErrDuplicateKey           = errors.New("duplicate key")
	ErrSubMapNotFound         = errors.New("submap not found")
	ErrKeyNotFound            = errors.New("key not found")
	ErrNilLocale              = errors.New("nil locale")
	ErrMismatchedKeys         = errors.New("mismatched locale keys")
	ErrDefaultCoverage        = errors.New("default language missing keys")
	ErrMismatchedPlaceholders = errors.New("mismatched locale placeholders")
	ErrMissingFormatArgs      = errors.New("missing format arguments")
)
