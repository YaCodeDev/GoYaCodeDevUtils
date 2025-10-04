// Package yabase64 provides tiny helpers to serialize a Go value as JSON and
// encode it using base64 (and the reverse operation). The API is intentionally
// minimal:
//
//   - Encode[T any](v T) -> *bytes.Buffer holding the base64 text of JSON(v)
//   - Decode[T any](s string) -> *T reconstructed from base64(JSON(T))
//
// Notes:
//
//   - The JSON encoder used by Encode writes a trailing newline by default
//     (standard library behavior). This is preserved inside the base64 output.
//   - The helpers are stateless and threadsafe.
//   - Errors are returned as yaerrors.Error on decode and wrapped with HTTP 500
//     semantics to match the rest of your codebase.
//
// Example (basic round-trip):
//
//	var data = struct {
//	    ID   int    `json:"id"`
//	    Name string `json:"name"`
//	}{ID: 7, Name: "RZK"}
//
//	// Encode - base64(JSON(data))
//	buf, err := yabase64.Encode(data)
//	if err != nil {
//	    log.Fatalf("encode failed: %v", err)
//	}
//	b64 := buf.String()
//
//	// Decode - base64(JSON(T))
//	got, yaerr := yabase64.Decode[struct {
//	    ID   int    `json:"id"`
//	    Name string `json:"name"`
//	}](b64)
//	if yaerr != nil {
//	    log.Fatalf("decode failed: %v", yaerr)
//	}
//
//	fmt.Println(got.ID, got.Name) // 7 RZK
package yabase64

import (
	"bytes"
	"encoding/base64"
	"encoding/gob"
	"fmt"
	"net/http"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
)

// Encode serializes a Go value using gob and base64-encodes the resulting bytes.
//
// Returns:
//   - *bytes.Buffer containing the base64-encoded gob data.
//   - yaerrors.Error wrapping the underlying cause (if any).
//
// Behavior:
//   - The returned buffer contains valid base64 text that represents gob-encoded data.
//   - The encoding is Go-specific and can only be decoded by Go using gob.
//   - The buffer is owned by the caller and can be accessed via Bytes() or String().
//
// Example:
//
//	type Payload struct {
//	    Token string
//	    ID    int
//	}
//
//	buf, err := yabase64.Encode(Payload{Token: "abc", ID: 42})
//	if err != nil {
//	    log.Fatalf("encode failed: %v", err)
//	}
//	fmt.Println(buf.String()) // e.g. "GgAAAAVQYXlsb2FkAgAAAAZhYmMIAAAAqg=="
func Encode[T any](v T) (string, yaerrors.Error) {
	var buf bytes.Buffer

	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(v); err != nil {
		return "", yaerrors.FromError(
			http.StatusInternalServerError,
			err,
			fmt.Sprintf("[BASE64] failed to encode `%T` using gob", v),
		)
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil

}

// Decode decodes a base64-encoded gob string into a Go struct of type T.
//
// Parameters:
//   - base: Base64-encoded string created by Encode[T].
//
// Returns:
//   - *T on success.
//   - yaerrors.Error on failure (e.g., invalid base64 or gob data).
//
// Example:
//
//	type User struct {
//	    ID    int
//	    Name  string
//	}
//
//	encoded, _ := yabase64.Encode(User{ID: 42, Name: "Alice"})
//
//	u, err := yabase64.Decode[User](encoded.String())
//	if err != nil {
//	    log.Fatalf("decode failed: %v", err)
//	}
//	fmt.Printf("%+v\n", u) // &{ID:42 Name:Alice}
func Decode[T any](base string) (*T, yaerrors.Error) {
	data, err := base64.StdEncoding.DecodeString(base)
	if err != nil {
		return nil, yaerrors.FromError(
			http.StatusInternalServerError,
			err,
			"[BASE64] failed to decode base64 string to bytes",
		)
	}

	var v T
	dec := gob.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&v); err != nil {
		return nil, yaerrors.FromError(
			http.StatusInternalServerError,
			err,
			fmt.Sprintf("[BASE64] failed to decode gob to `%T`", v),
		)
	}

	return &v, nil
}

// ToString encodes raw bytes to a base64 string (StdEncoding).
//
// Notes:
//   - This is a low-level helper and does NOT perform JSON marshaling.
//   - It is stateless and threadsafe.
//   - Use when you already have []byte and just need a base64 string.
//
// Example:
//
//	data := []byte("hello world")
//	b64 := yabase64.ToString(data)
//	fmt.Println(b64) // aGVsbG8gd29ybGQ=
func ToString(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// ToBytes decodes a base64 string (StdEncoding) back to raw bytes.
//
// Returns:
//   - []byte on success
//   - yaerrors.Error on failure with HTTP 500 semantics
//
// Notes:
//   - This is a low-level helper and does NOT perform JSON unmarshaling.
//   - Useful for working with binary data stored as base64 text.
//
// Example:
//
//	b64 := "aGVsbG8gd29ybGQ="
//	bytes, err := yabase64.ToBytes(b64)
//	if err != nil {
//	    log.Fatalf("decode failed: %v", err)
//	}
//	fmt.Println(string(bytes)) // hello world
func ToBytes(data string) ([]byte, yaerrors.Error) {
	bytes, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, yaerrors.FromError(
			http.StatusInternalServerError,
			err,
			"failed to decode string to bytes",
		)
	}

	return bytes, nil
}
