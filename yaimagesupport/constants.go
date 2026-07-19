package yaimagesupport

// defaultSVGSize is the fallback raster width/height, in pixels, used when an
// SVG does not declare a usable viewBox.
const defaultSVGSize = 1024

// DefaultMaxSVGDimension is the default cap, in pixels, on the width and
// height rasterized from an SVG's declared viewBox. At this size a decoded
// RGBA image tops out around 64MiB (4096 * 4096 * 4 bytes). Override with
// SetMaxSVGDimension.
const DefaultMaxSVGDimension = 4096

// DefaultMaxSVGPayloadSize is the default cap, in bytes, on the raw SVG
// payload read before parsing. Override with SetMaxSVGPayloadSize.
const DefaultMaxSVGPayloadSize int64 = 10 << 20 // 10 MiB
