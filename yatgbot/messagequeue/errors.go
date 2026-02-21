package messagequeue

import "errors"

var (
	ErrJobCanceled = errors.New("job was canceled")
	ErrJobNil      = errors.New("job is nil")
)
