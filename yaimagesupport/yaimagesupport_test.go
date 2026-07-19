package yaimagesupport_test

import (
	"bytes"
	"image"
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaimagesupport"
)

func TestInitRegistersMatchImageFormats(t *testing.T) {
	yaimagesupport.Init()

	tests := []struct {
		name    string
		format  string
		payload []byte
	}{
		{
			name:    "jpeg",
			format:  "jpeg",
			payload: []byte{0xff, 0xd8, 0xff, 0xdb},
		},
		{
			name:    "png",
			format:  "png",
			payload: []byte("\x89PNG\r\n\x1a\n"),
		},
		{
			name:    "gif",
			format:  "gif",
			payload: []byte("GIF89a"),
		},
		{
			name:    "bmp",
			format:  "bmp",
			payload: []byte("BM\x00\x00\x00\x00\x00\x00\x00\x00"),
		},
		{
			name:    "tiff",
			format:  "tiff",
			payload: []byte("II*\x00"),
		},
		{
			name:    "webp",
			format:  "webp",
			payload: []byte("RIFF\x00\x00\x00\x00WEBPVP8"),
		},
		{
			name:    "ico",
			format:  "ico",
			payload: []byte{0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		},
		{
			name:   "xpm",
			format: "xpm",
			payload: []byte(`/* XPM */
static char * sample_xpm[] = {
"2 2 2 1",
"  c #FFFFFF",
". c #000000",
". ",
" ."
};
`),
		},
		{
			name:    "ppm",
			format:  "ppm",
			payload: []byte("P6"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, format, _ := image.DecodeConfig(bytes.NewReader(tt.payload))
			if format != tt.format {
				t.Fatalf("format = %q, want %q", format, tt.format)
			}
		})
	}
}

func TestInitRegistersSVG(t *testing.T) {
	yaimagesupport.Init()

	payload := []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="16" height="12">` +
		`<rect x="0" y="0" width="16" height="12" fill="#000"/></svg>`)

	config, format, err := image.DecodeConfig(bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("decode svg config: %v", err)
	}

	if format != "svg" {
		t.Fatalf("format = %q, want svg", format)
	}

	if config.Width != 16 || config.Height != 12 {
		t.Fatalf("config = %dx%d, want 16x12", config.Width, config.Height)
	}
}
