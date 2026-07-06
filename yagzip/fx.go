package yagzip

import "go.uber.org/fx"

// GzipModuleName is the fx module name for the default Gzip codec.
const GzipModuleName = "yagzip"

// GzipModule provides the default *Gzip codec built by NewGzip.
//
// The level/max-size variants (NewGzipWithLevel, NewGzipWithLevelAndMaxSize)
// are intentionally left out of this module: wiring more than one *Gzip
// provider into the same fx graph would create an ambiguous provider.
// Construct those directly when custom settings are needed.
//
// Example usage:
//
//	fx.New(yagzip.GzipModule)
var GzipModule = fx.Module(
	GzipModuleName,
	fx.Provide(NewGzip),
)
