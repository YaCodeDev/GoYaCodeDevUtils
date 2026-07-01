package config

import "errors"

var (
	ErrConfigStructMustBeStruct = errors.New("config struct must be a struct")
	ErrValueIsRequired          = errors.New("value is required")
	ErrInvalidDotEnvFileFormat  = errors.New("invalid .env file format")
	ErrUnsupportedYaToolsValue  = errors.New("unsupported yatools config value")
	ErrNilYaToolsDestination    = errors.New("yatools config destination must not be nil")
	ErrNilYaToolsValue          = errors.New("yatools config value must not be nil")
)
