package yaimagesupport_test

import (
	"bytes"
	"errors"
	"image"
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaimagesupport"
)

const testSVGPayload = `<svg xmlns="http://www.w3.org/2000/svg" width="16" height="12">` +
	`<rect x="0" y="0" width="16" height="12" fill="#000"/></svg>`

func TestDecodeSVG_WithinDefaultLimits(t *testing.T) {
	yaimagesupport.Init()

	_, format, err := image.Decode(bytes.NewReader([]byte(testSVGPayload)))
	if err != nil {
		t.Fatalf("decode svg: %v", err)
	}

	if format != "svg" {
		t.Fatalf("format = %q, want svg", format)
	}
}

func TestDecodeSVG_MalformedPayloadReturnsParseError(t *testing.T) {
	yaimagesupport.Init()

	malformed := []byte(`<svg xmlns="http://www.w3.org/2000/svg"><rect>`)

	_, _, err := image.Decode(bytes.NewReader(malformed))
	if err == nil {
		t.Fatalf("decode svg: got nil error, want a parse error")
	}
}

func TestDecodeSVG_DimensionExceedsMax(t *testing.T) {
	yaimagesupport.Init()

	t.Cleanup(func() {
		yaimagesupport.SetMaxSVGDimension(yaimagesupport.DefaultMaxSVGDimension)
	})

	yaimagesupport.SetMaxSVGDimension(8)

	_, _, err := image.Decode(bytes.NewReader([]byte(testSVGPayload)))
	if err == nil {
		t.Fatalf("decode svg: got nil error, want ErrSVGDimensionTooLarge")
	}

	if !errors.Is(err, yaimagesupport.ErrSVGDimensionTooLarge) {
		t.Fatalf("decode svg error = %v, want ErrSVGDimensionTooLarge", err)
	}
}

func TestDecodeSVGConfig_PayloadExceedsMaxSize(t *testing.T) {
	yaimagesupport.Init()

	t.Cleanup(func() {
		yaimagesupport.SetMaxSVGPayloadSize(yaimagesupport.DefaultMaxSVGPayloadSize)
	})

	yaimagesupport.SetMaxSVGPayloadSize(16)

	_, _, err := image.DecodeConfig(bytes.NewReader([]byte(testSVGPayload)))
	if err == nil {
		t.Fatalf("decode svg config: got nil error, want ErrSVGPayloadTooLarge")
	}

	if !errors.Is(err, yaimagesupport.ErrSVGPayloadTooLarge) {
		t.Fatalf("decode svg config error = %v, want ErrSVGPayloadTooLarge", err)
	}
}

func TestDecodeSVGConfig_DimensionExceedsMax(t *testing.T) {
	yaimagesupport.Init()

	t.Cleanup(func() {
		yaimagesupport.SetMaxSVGDimension(yaimagesupport.DefaultMaxSVGDimension)
	})

	yaimagesupport.SetMaxSVGDimension(8)

	_, _, err := image.DecodeConfig(bytes.NewReader([]byte(testSVGPayload)))
	if err == nil {
		t.Fatalf("decode svg config: got nil error, want ErrSVGDimensionTooLarge")
	}

	if !errors.Is(err, yaimagesupport.ErrSVGDimensionTooLarge) {
		t.Fatalf("decode svg config error = %v, want ErrSVGDimensionTooLarge", err)
	}
}

func TestSetMaxSVGDimension_NonPositiveResetsToDefault(t *testing.T) {
	yaimagesupport.Init()

	t.Cleanup(func() {
		yaimagesupport.SetMaxSVGDimension(yaimagesupport.DefaultMaxSVGDimension)
	})

	yaimagesupport.SetMaxSVGDimension(8)
	yaimagesupport.SetMaxSVGDimension(0)

	_, _, err := image.DecodeConfig(bytes.NewReader([]byte(testSVGPayload)))
	if err != nil {
		t.Fatalf("decode svg config after reset: %v", err)
	}
}

func TestSetMaxSVGPayloadSize_NonPositiveResetsToDefault(t *testing.T) {
	yaimagesupport.Init()

	t.Cleanup(func() {
		yaimagesupport.SetMaxSVGPayloadSize(yaimagesupport.DefaultMaxSVGPayloadSize)
	})

	yaimagesupport.SetMaxSVGPayloadSize(16)
	yaimagesupport.SetMaxSVGPayloadSize(-1)

	_, _, err := image.DecodeConfig(bytes.NewReader([]byte(testSVGPayload)))
	if err != nil {
		t.Fatalf("decode svg config after reset: %v", err)
	}
}
