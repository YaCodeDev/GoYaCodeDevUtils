//go:build cgo && yaimagesupport_native

package yaimagesupport

import (
	"sync"

	_ "github.com/jdeng/goheif"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
)

var initCGOOnce sync.Once

// InitCGO loads the pure-Go decoders plus native CGO-backed decoders.
func InitCGO() yaerrors.Error {
	Init()

	initCGOOnce.Do(func() {
		// Native decoders are registered by package init side effects.
	})

	return nil
}
