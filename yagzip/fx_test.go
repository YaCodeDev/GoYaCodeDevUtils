package yagzip_test

import (
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yagzip"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

func TestGzipModule(t *testing.T) {
	t.Parallel()

	t.Run(
		"when GzipModule is wired / then it resolves a usable *Gzip",
		func(t *testing.T) {
			t.Parallel()

			const (
				wantLevel               = yagzip.DefaultCompression
				wantMaxDecompressedSize = yagzip.DefaultMaxDecompressedSize
			)

			var codec *yagzip.Gzip

			fxtest.New(
				t,
				yagzip.GzipModule,
				fx.Populate(&codec),
			)

			if codec == nil {
				t.Fatalf("expected GzipModule to populate a non-nil *Gzip")
			}

			if codec.Level != wantLevel {
				t.Errorf(
					"expected codec Level to be the package default %d, got %d",
					wantLevel,
					codec.Level,
				)
			}

			if codec.MaxDecompressedSize != wantMaxDecompressedSize {
				t.Errorf(
					"expected codec MaxDecompressedSize to be the package default %d, got %d",
					wantMaxDecompressedSize,
					codec.MaxDecompressedSize,
				)
			}
		},
	)
}
