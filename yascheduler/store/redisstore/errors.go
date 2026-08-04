package redisstore

import "errors"

// ErrUnexpectedScriptReply reports a redis script answering with a shape
// the store does not know, which no healthy backend ever produces.
var ErrUnexpectedScriptReply = errors.New("unexpected script reply")

// ErrConcurrentUpdate reports a record rewritten concurrently on every try
// of the store's bounded retry loop, so no run of its script ever observed
// the value the preparatory read declared.
var ErrConcurrentUpdate = errors.New("concurrent update exhausted retries")
