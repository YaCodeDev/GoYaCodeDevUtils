// Package yaencoding provides helpers for encoding and decoding data
// using Gob or MessagePack formats with Base64 string representation.
// It simplifies safe transmission of Go structures through text mediums (e.g., JSON, HTTP).
//
// Each encode/decode returns yaerrors.Error to unify structured error handling
// in backend systems that use GoYaCodeDevUtils.
//
// Supported formats:
//   - Gob (native Go binary serialization)
//   - MessagePack (efficient binary encoding similar to Protobuf)
//
// Example usage:
//
//	type User struct {
//	    ID   int
//	    Name string
//	}
//
//	// GOB Example
//	user := User{ID: 1, Name: "Alice"}
//
//	encoded, err := yaencoding.EncodeGob(user)
//	if err != nil {
//	    log.Fatalf("encode failed: %v", err)
//	}
//
//	decoded, err := yaencoding.DecodeGob[User](encoded)
//	if err != nil {
//	    log.Fatalf("decode failed: %v", err)
//	}
//
//	fmt.Println(decoded.Name) // Output: Alice
//
//	// MessagePack Example
//	msgpackStr, err := yaencoding.EncodeMessagePack(user)
//	if err != nil {
//	    log.Fatalf("encode failed: %v", err)
//	}
//
//	mpDecoded, err := yaencoding.DecodeMessagePack[User](msgpackStr)
//	if err != nil {
//	    log.Fatalf("decode failed: %v", err)
//	}
//
//	fmt.Println(mpDecoded.ID) // Output: 1
package yaencoding

import (
	"bytes"
	"encoding/base64"
	"encoding/gob"
	"fmt"
	"net/http"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/vmihailenco/msgpack/v5"
)

// EncodeGob serializes the given value `v` using the Gob encoder,
// then base64-encodes the binary data into a string.
//
// Returns the base64 string or a wrapped yaerrors.Error on failure.
//
// Example:
//
//	s := MyStruct{ID: 5, Name: "Ya Code"}
//	str, err := yaencoding.EncodeGob(s)
func EncodeGob(v any) ([]byte, yaerrors.Error) {
	var buf bytes.Buffer

	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(v); err != nil {
		return nil, yaerrors.FromError(
			http.StatusInternalServerError,
			err,
			fmt.Sprintf("[ENCODING] failed to encode `%T` using gob", v),
		)
	}

	return buf.Bytes(), nil
}

// DecodeGob decodes a base64 string that represents Gob-encoded data
// back into a Go structure of type T.
//
// Example:
//
//	out, err := yaencoding.DecodeGob[MyStruct](encoded)
func DecodeGob[T any](data []byte) (*T, yaerrors.Error) {
	var v T

	dec := gob.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&v); err != nil {
		return nil, yaerrors.FromError(
			http.StatusInternalServerError,
			err,
			fmt.Sprintf("[ENCODING] failed to decode gob to `%T`", v),
		)
	}

	return &v, nil
}

// EncodeMessagePack serializes `value` using the MessagePack format,
// then base64-encodes it for text-safe transport.
//
// Example:
//
//	str, err := yaencoding.EncodeMessagePack(myStruct)
func EncodeMessagePack(value any) ([]byte, yaerrors.Error) {
	bytes, err := msgpack.Marshal(value)
	if err != nil {
		return nil, yaerrors.FromError(
			http.StatusInternalServerError,
			err,
			fmt.Sprintf("[ENCODING] failed to marshal %T using message pack format", value),
		)
	}

	return bytes, nil
}

// DecodeMessagePack decodes a Base64 string containing MessagePack data
// into a Go structure of type T.
//
// Example:
//
//	val, err := yaencoding.DecodeMessagePack[User](encoded)
func DecodeMessagePack[T any](bytes []byte) (*T, yaerrors.Error) {
	var res T

	if err := msgpack.Unmarshal(bytes, &res); err != nil {
		return nil, yaerrors.FromError(
			http.StatusInternalServerError,
			err,
			fmt.Sprintf("[ENCODING] failed to marshal %T as message pack format", bytes),
		)
	}

	return &res, nil
}

// ToString converts a byte slice into a base64 string.
// Useful for manual conversions.
func ToString(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// ToBytes decodes a base64 string into bytes.
func ToBytes(data string) ([]byte, yaerrors.Error) {
	bytes, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, yaerrors.FromError(
			http.StatusInternalServerError,
			err,
			"[ENCODING] failed to decode string to bytes",
		)
	}

	return bytes, nil
}
