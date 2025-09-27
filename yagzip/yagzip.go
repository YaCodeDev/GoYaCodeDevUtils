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
	"compress/gzip"
	"io"
	"net/http"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
)

// Zip compresses object using gzip and returns the compressed bytes.
//
// Returns:
//   - []byte: gzip-compressed data
//   - yaerror:  wrapped with err on failure
//
// Behavior:
//   - Uses gzip.NewWriter (default level).
//   - Ensures the writer is closed/finished on both success and failure paths.
//
// Example:
//
//	in := []byte("payload")
//	out, err := yagzip.Zip(in)
//	if err != nil { /* handle */ }
func Zip(object []byte) ([]byte, yaerrors.Error) {
	var buf bytes.Buffer

	w := gzip.NewWriter(&buf)

	_, err := w.Write(object)
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
func Unzip(compressed []byte) ([]byte, yaerrors.Error) {
	r, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, yaerrors.FromError(
			http.StatusInternalServerError,
			err,
			"[GZIP] failed to create gzip reader",
		)
	}
	defer r.Close()

	var out bytes.Buffer

	_, err = io.Copy(&out, r)
	if err != nil {
		return nil, yaerrors.FromError(
			http.StatusInternalServerError,
			err,
			"[GZIP] failed to read from gzip stream",
		)
	}

	return out.Bytes(), nil
}
