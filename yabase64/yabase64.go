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
//	// Encode → base64(JSON(data))
//	buf, err := yabase64.Encode(data)
//	if err != nil {
//	    log.Fatalf("encode failed: %v", err)
//	}
//	b64 := buf.String()
//
//	// Decode ← base64(JSON(T))
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
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
)

// Encode marshals v to JSON and base64-encodes the JSON bytes.
//
// Returns:
//   - *bytes.Buffer containing base64 text (StdEncoding) of JSON(v)
//   - error wrapping the underlying cause (e.g., JSON or encoder close)
//
// Behavior:
//   - A trailing newline is emitted by json.Encoder; this newline becomes part
//     of the base64 output (this matches standard library defaults).
//   - The returned buffer owns its contents and can be read via Bytes()/String().
//
// Example:
//
//	type Payload struct {
//	    Token string `json:"token"`
//	}
//
//	buf, err := yabase64.Encode(Payload{Token: "abc"})
//	if err != nil {
//	    log.Fatalf("encode failed: %v", err)
//	}
//	fmt.Println(buf.String()) // e.g. eyJ0b2tlbiI6ImFiYyJ9Cg==
func Encode[T any](v T) (*bytes.Buffer, error) {
	var buf bytes.Buffer

	encoder := base64.NewEncoder(base64.StdEncoding, &buf)

	err := json.NewEncoder(encoder).Encode(v)
	if err != nil {
		return nil, yaerrors.FromError(
			http.StatusInternalServerError,
			err,
			fmt.Sprintf("[BASE64] failed to encode `%T` to bytes", v),
		)
	}

	if err := encoder.Close(); err != nil {
		return nil, yaerrors.FromError(
			http.StatusInternalServerError,
			err,
			"[BASE64] failed to close encoder",
		)
	}

	return &buf, nil
}

// Decode base64-decodes value and then unmarshals JSON into T.
//
// Parameters:
//   - value: base64 string created by Encode[T] (i.e., base64(JSON(T)) )
//
// Returns:
//   - *T on success
//   - yaerrors.Error on failure with http.StatusInternalServerError semantics
//
// Example:
//
//	type User struct {
//	    ID int `json:"id"`
//	}
//
//	buf, _ := yabase64.Encode(User{ID: 42})
//	u, err := yabase64.Decode[User](buf.String())
//	if err != nil {
//	    log.Fatalf("decode failed: %v", err)
//	}
//	fmt.Println(u.ID) // 42
func Decode[T any](value string) (*T, yaerrors.Error) {
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, yaerrors.FromError(
			http.StatusInternalServerError,
			err,
			"[BASE64] failed to decode string to bytes",
		)
	}

	var result T

	err = json.NewDecoder(bytes.NewReader(decoded)).Decode(&result)
	if err != nil {
		return nil, yaerrors.FromError(
			http.StatusInternalServerError,
			err,
			fmt.Sprintf("[BASE64] failed to decode string to `%T`", result),
		)
	}

	return &result, nil
}
