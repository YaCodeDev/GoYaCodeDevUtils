//go:build !cgo || !yaimagesupport_native

package yaimagesupport

import (
	"net/http"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
)

// InitCGO reports that native decoder support was not compiled into the binary.
func InitCGO() yaerrors.Error {
	Init()

	return yaerrors.FromError(
		http.StatusNotImplemented,
		ErrCGOSupportUnavailable,
		"init cgo image support",
	)
}
