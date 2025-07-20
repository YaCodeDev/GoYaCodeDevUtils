package config

import "errors"

var (
	ErrConfigStructMustBeStruct = errors.New("config struct must be a struct")
	ErrValueIsRequired          = errors.New("value is required")
	ErrInvalidDotEnvFileFormat  = errors.New("invalid .env file format")
)
