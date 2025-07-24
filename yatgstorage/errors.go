package yatgstorage

import "errors"

var (
	ErrFailedToSetState       = errors.New("failed to set telegram bot state")
	ErrFailedToSetQts         = errors.New("failed to set telegram bot qts")
	ErrFailedToSetPts         = errors.New("failed to set telegram bot pts")
	ErrFailedToSetDate        = errors.New("failed to set telegram bot date")
	ErrFailedToSetSeq         = errors.New("failed to set telegram bot seq")
	ErrFailedToSetDateSeq     = errors.New("failed to set telegram bot date and seq")
	ErrFailedToGetState       = errors.New("failed to get telegram bot state")
	ErrFailedToUnmarshalState = errors.New("failed to unmarshal telegram bot state")
)
