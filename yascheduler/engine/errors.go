package engine

import "errors"

// Label admission failures. Every one of them is answered on the wire, so a
// connection whose label set was left untouched learns why instead of
// watching its pinned jobs park forever.
var (
	// ErrUnknownInstance reports a label update for an instance the
	// registry holds no live registration for.
	ErrUnknownInstance = errors.New("executor instance is not registered")

	// ErrEmptyLabel reports a label update carrying an empty label, which
	// names no routing target and is already what an unpinned job means.
	ErrEmptyLabel = errors.New("routing label is empty")

	// ErrLabelLimitExceeded reports a label update that would push one
	// connection past the label cap.
	ErrLabelLimitExceeded = errors.New("routing label limit exceeded")
)
