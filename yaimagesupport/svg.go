package yaimagesupport

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"io"
	"math"
	"net/http"
	"sync/atomic"

	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
)

var (
	maxSVGDimension   atomic.Int64
	maxSVGPayloadSize atomic.Int64
)

func init() {
	maxSVGDimension.Store(DefaultMaxSVGDimension)
	maxSVGPayloadSize.Store(DefaultMaxSVGPayloadSize)
}

// SetMaxSVGDimension overrides the maximum rasterized width/height, in
// pixels, permitted for an SVG decode. It is safe to call concurrently with
// decoding. A non-positive value resets the limit to DefaultMaxSVGDimension.
func SetMaxSVGDimension(maxDimension int) {
	if maxDimension <= 0 {
		maxDimension = DefaultMaxSVGDimension
	}

	maxSVGDimension.Store(int64(maxDimension))
}

// SetMaxSVGPayloadSize overrides the maximum raw SVG payload size, in bytes,
// read before parsing. It is safe to call concurrently with decoding. A
// non-positive value resets the limit to DefaultMaxSVGPayloadSize.
func SetMaxSVGPayloadSize(maxPayloadSize int64) {
	if maxPayloadSize <= 0 {
		maxPayloadSize = DefaultMaxSVGPayloadSize
	}

	maxSVGPayloadSize.Store(maxPayloadSize)
}

func registerSVG() {
	image.RegisterFormat("svg", "<svg", decodeSVG, decodeSVGConfig)
	image.RegisterFormat("svg", "<?xml", decodeSVG, decodeSVGConfig)
}

func decodeSVG(r io.Reader) (image.Image, error) {
	icon, yaErr := readSVGIcon(r)
	if yaErr != nil {
		return nil, yaErr
	}

	width, height, yaErr := svgDimensions(icon)
	if yaErr != nil {
		return nil, yaErr
	}

	rgba := image.NewRGBA(image.Rect(0, 0, width, height))
	icon.SetTarget(0, 0, float64(width), float64(height))

	scanner := rasterx.NewScannerGV(width, height, rgba, rgba.Bounds())
	dasher := rasterx.NewDasher(width, height, scanner)
	icon.Draw(dasher, 1.0)

	return rgba, nil
}

func decodeSVGConfig(r io.Reader) (image.Config, error) {
	icon, yaErr := readSVGIcon(r)
	if yaErr != nil {
		return image.Config{}, yaErr
	}

	width, height, yaErr := svgDimensions(icon)
	if yaErr != nil {
		return image.Config{}, yaErr
	}

	return image.Config{
		ColorModel: color.NRGBAModel,
		Width:      width,
		Height:     height,
	}, nil
}

func readSVGIcon(r io.Reader) (*oksvg.SvgIcon, yaerrors.Error) {
	limit := maxSVGPayloadSize.Load()

	payload, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, yaerrors.FromError(http.StatusInternalServerError, err, "read svg payload")
	}

	if int64(len(payload)) > limit {
		return nil, yaerrors.FromError(
			http.StatusRequestEntityTooLarge,
			ErrSVGPayloadTooLarge,
			fmt.Sprintf("read svg payload: payload exceeds max %d bytes", limit),
		)
	}

	icon, err := oksvg.ReadIconStream(bytes.NewReader(bytes.TrimSpace(payload)))
	if err != nil {
		return nil, yaerrors.FromError(http.StatusUnprocessableEntity, err, "parse svg icon")
	}

	return icon, nil
}

func svgDimensions(icon *oksvg.SvgIcon) (int, int, yaerrors.Error) {
	width := int(math.Ceil(icon.ViewBox.W))
	height := int(math.Ceil(icon.ViewBox.H))

	if width <= 0 || height <= 0 {
		return defaultSVGSize, defaultSVGSize, nil
	}

	limit := int(maxSVGDimension.Load())

	if width > limit || height > limit {
		return 0, 0, yaerrors.FromError(
			http.StatusRequestEntityTooLarge,
			ErrSVGDimensionTooLarge,
			fmt.Sprintf("svg dimensions %dx%d exceed max %d", width, height, limit),
		)
	}

	return width, height, nil
}
