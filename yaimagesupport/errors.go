package yaimagesupport

import "errors"

// ErrCGOSupportUnavailable indicates the binary was built without CGO native
// decoder support (CGO_ENABLED=0 or missing the yaimagesupport_native build
// tag).
var ErrCGOSupportUnavailable = errors.New(
	"yaimagesupport: cgo image support is unavailable; rebuild with CGO_ENABLED=1 -tags yaimagesupport_native",
)

// ErrSVGPayloadTooLarge indicates a raw SVG payload exceeded the configured
// MaxSVGPayloadSize before it could be parsed.
var ErrSVGPayloadTooLarge = errors.New("yaimagesupport: svg payload exceeds configured max size")

// ErrSVGDimensionTooLarge indicates an SVG's declared viewBox exceeds the
// configured MaxSVGDimension.
var ErrSVGDimensionTooLarge = errors.New("yaimagesupport: svg dimensions exceed configured max")
