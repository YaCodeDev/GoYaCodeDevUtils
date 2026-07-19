package yaimagesupport

import (
	"sync"

	// Standard library decoders register themselves with image.RegisterFormat via init.
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	// Third-party decoders register themselves with image.RegisterFormat via init.
	_ "github.com/fyne-io/image/ico"
	_ "github.com/fyne-io/image/xpm"
	_ "github.com/spakin/netpbm"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

var initOnce sync.Once

// Init loads the image format support imported by this package and registers
// custom decoders that are not provided by side-effect imports.
func Init() {
	initOnce.Do(func() {
		registerSVG()
	})
}
