// Package yagzip provides tiny helpers to gzip-compress and decompress []byte
// payloads using the standard library's gzip implementation.
//
// Notes:
//   - Zip writes gzip data into an internal buffer and returns its bytes.
//   - Unzip reads a full gzip stream from memory and returns the decompressed bytes.
//   - Errors are wrapped with yaerrors (HTTP 500 semantics) for consistency with
//     the rest of your codebase, while keeping the exported signatures as (.., error).
//
// Example (basic round-trip):
//
//	data := []byte("Hello, RZK!")
//	z, err := yagzip.Zip(data)
//	if err != nil {
//	    log.Fatalf("zip failed: %v", err)
//	}
//	uz, err := yagzip.Unzip(z)
//	if err != nil {
//	    log.Fatalf("unzip failed: %v", err)
//	}
//	fmt.Println(string(uz)) // "Hello, RZK!"
package yagzip

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"errors"
	"io"
	"net/http"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
)

const (
	DefaultCompression               = flate.DefaultCompression
	DefaultMaxDecompressedSize int64 = 64 << 20 // 64 MiB
)

var ErrDecompressedPayloadTooLarge = errors.New("decompressed payload exceeds configured limit")

type Gzip struct {
	Level               int
	MaxDecompressedSize int64
}

func NewGzipWithLevelAndMaxSize(level int, maxDecompressedSize int64) *Gzip {
	return &Gzip{
		Level:               level,
		MaxDecompressedSize: maxDecompressedSize,
	}
}

func NewGzipWithLevel(level int) *Gzip {
	return &Gzip{
		Level:               level,
		MaxDecompressedSize: DefaultMaxDecompressedSize,
	}
}

func NewGzip() *Gzip {
	return &Gzip{
		Level:               flate.DefaultCompression,
		MaxDecompressedSize: DefaultMaxDecompressedSize,
	}
}

// Zip compresses object using gzip and returns the compressed bytes.
//
// Returns:
//   - []byte: gzip-compressed data
//   - yaerror:  wrapped with err on failure
//
// Example:
//
//	in := []byte("payload")
//	out, err := yagzip.Zip(in)
//	if err != nil { /* handle */ }
func (g *Gzip) Zip(object []byte) ([]byte, yaerrors.Error) {
	var buf bytes.Buffer

	w, err := gzip.NewWriterLevel(&buf, g.Level)
	if err != nil {
		return nil, yaerrors.FromError(
			http.StatusInternalServerError,
			err,
			"[GZIP] failed to create write",
		)
	}

	_, err = w.Write(object)
	if err != nil {
		return nil, yaerrors.FromError(
			http.StatusInternalServerError,
			err,
			"[GZIP] failed to write payload to gzip writer",
		)
	}

	if err := w.Close(); err != nil {
		return nil, yaerrors.FromError(
			http.StatusInternalServerError,
			err,
			"[GZIP] failed to close gzip writer",
		)
	}

	return buf.Bytes(), nil
}

// Unzip decompresses gzip-compressed data back to its original bytes.
//
// Returns:
//   - []byte: decompressed payload
//   - yaerror:  wrapped with err on failure
//
// Example:
//
//	payload, err := yagzip.Unzip(zipped)
//	if err != nil { /* handle */ }
func (g *Gzip) Unzip(compressed []byte) ([]byte, yaerrors.Error) {
	r, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, yaerrors.FromError(
			http.StatusInternalServerError,
			err,
			"[GZIP] failed to create gzip reader",
		)
	}
	defer r.Close()

	maxSize := g.MaxDecompressedSize
	if maxSize <= 0 {
		maxSize = DefaultMaxDecompressedSize
	}

	var out bytes.Buffer

	_, err = io.Copy(&out, io.LimitReader(r, maxSize+1))
	if err != nil {
		return nil, yaerrors.FromError(
			http.StatusInternalServerError,
			err,
			"[GZIP] failed to read from gzip stream",
		)
	}

	if int64(out.Len()) > maxSize {
		return nil, yaerrors.FromError(
			http.StatusInternalServerError,
			ErrDecompressedPayloadTooLarge,
			"[GZIP] decompressed payload is too large",
		)
	}

	return out.Bytes(), nil
}
