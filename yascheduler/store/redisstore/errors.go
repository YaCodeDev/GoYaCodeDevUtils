package redisstore

import "errors"

// ErrUnexpectedScriptReply reports a redis script answering with a shape
// the store does not know, which no healthy backend ever produces.
var ErrUnexpectedScriptReply = errors.New("unexpected script reply")
