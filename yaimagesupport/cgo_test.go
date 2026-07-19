//go:build cgo && yaimagesupport_native

package yaimagesupport_test

import (
	"bytes"
	"encoding/binary"
	"image"
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaimagesupport"
)

func TestInitCGORegistersHEIC(t *testing.T) {
	if err := yaimagesupport.InitCGO(); err != nil {
		t.Fatalf("init cgo: %v", err)
	}

	_, format, _ := image.DecodeConfig(bytes.NewReader(buildHEIFMagicPayload("heic")))
	if format != "heic" {
		t.Fatalf("format = %q, want heic", format)
	}
}

func buildHEIFMagicPayload(brand string) []byte {
	payload := make([]byte, 24)
	binary.BigEndian.PutUint32(payload[0:4], uint32(len(payload)))

	copy(payload[4:8], []byte("ftyp"))
	copy(payload[8:12], normalizeBrand(brand))
	copy(payload[16:20], []byte("mif1"))
	copy(payload[20:24], normalizeBrand(brand))

	return payload
}

func normalizeBrand(brand string) []byte {
	normalized := make([]byte, 4)
	copy(normalized, []byte(brand))

	for i := range normalized {
		if normalized[i] == 0 {
			normalized[i] = ' '
		}
	}

	return normalized
}
