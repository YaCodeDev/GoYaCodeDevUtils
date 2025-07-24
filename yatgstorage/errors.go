package yatgstorage

import "errors"

var (
	ErrFailedToSetState       = errors.New("failed to set telegram bot state")
	ErrFailedToGetState       = errors.New("failed to get telegram bot state")
	ErrFailedToUnmarshalState = errors.New("failed to unmarshal telegram bot state")
)
