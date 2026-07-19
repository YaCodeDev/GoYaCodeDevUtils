//go:build !cgo || !yaimagesupport_native

package yaimagesupport_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaimagesupport"
)

func TestInitCGO_WithoutNativeTagReturnsUnavailable(t *testing.T) {
	err := yaimagesupport.InitCGO()
	if err == nil {
		t.Fatalf("init cgo: got nil error, want ErrCGOSupportUnavailable")
	}

	if !errors.Is(err, yaimagesupport.ErrCGOSupportUnavailable) {
		t.Fatalf("init cgo error = %v, want ErrCGOSupportUnavailable", err)
	}

	if err.Code() != http.StatusNotImplemented {
		t.Fatalf("init cgo code = %d, want %d", err.Code(), http.StatusNotImplemented)
	}
}
