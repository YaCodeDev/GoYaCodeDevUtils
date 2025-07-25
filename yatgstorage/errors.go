package yatgstorage

import "errors"

var (
	ErrFailedToSetState                   = errors.New("failed to set telegram bot state")
	ErrFailedToSetQts                     = errors.New("failed to set telegram bot qts")
	ErrFailedToSetPts                     = errors.New("failed to set telegram bot pts")
	ErrFailedToSetDate                    = errors.New("failed to set telegram bot date")
	ErrFailedToSetSeq                     = errors.New("failed to set telegram bot seq")
	ErrFailedToSetDateSeq                 = errors.New("failed to set telegram bot date and seq")
	ErrFailedToGetState                   = errors.New("failed to get telegram bot state")
	ErrFailedToUnmarshalState             = errors.New("failed to unmarshal telegram bot state")
	ErrFailedToSetChannelPts              = errors.New("failed to set channel pts")
	ErrFailedToGetChannelPts              = errors.New("failed to get channel pts")
	ErrFailedToUnmarshalChannelPts        = errors.New("failed to unmarshal channel pts")
	ErrFailedToSetChannelAccessHash       = errors.New("failed to set channel access hash")
	ErrFailedToGetChannelAccessHash       = errors.New("failed to get channel access hash")
	ErrFailedToUnmarshalChannelAccessHash = errors.New("failed to unmarshal channel access hash")
	ErrFailedToParsePtsAsInt              = errors.New("failed to parse pts as int")
	ErrFailedToParseAccessHashAsInt64     = errors.New("failed to parse access hash as int64")
)
