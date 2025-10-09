// Package yaginmiddleware provides secure middleware utilities for Gin.
package yaginmiddleware

import (
	"bytes"
	"crypto/rsa"
	"net/http"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaencoding"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yagzip"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yarsa"
	"github.com/gin-gonic/gin"
)

// Middleware represents a generic Gin middleware component
// capable of processing requests via a `Handle` method.
type Middleware interface {
	Handle(ctx *gin.Context)
}

// RSASecureHeader is a generic Gin middleware that enables transparent,
// type-safe encryption and decryption of structured data in HTTP headers
// using RSA-OAEP + GZIP + MessagePack.
//
// It provides methods to encode/decode any struct `T` into a secure,
// base64-encoded header value, and a middleware handler (`Handle`) that
// automatically decrypts incoming headers and injects the resulting struct
// into Gin’s request context.
//
// Pipeline:
//
//	struct -> MessagePack -> gzip -> RSA encrypt -> base64
//	base64 -> RSA decrypt -> gunzip -> MessagePack -> struct
//
// Example:
//
//	type Payload struct {
//	    ID   uint16
//	    Text string
//	}
//
//	key, _ := rsa.GenerateKey(rand.Reader, 2048)
//	header := yaginmiddleware.NewEncodeRSA[Payload]("X-Data", "payload", key, true)
//
//	in := Payload{ID: 7, Text: "Hello"}
//	enc, _ := header.Encode(in)
//
//	_, out, _ := header.Decode(enc)
//	fmt.Println(out.Text) // "Hello"
//
// In a Gin app:
//
//	r := gin.New()
//	r.Use(header.Handle)
//	r.GET("/ping", func(c *gin.Context) {
//	    v, _ := c.Get("payload")
//	    fmt.Println(v.(*Payload))
//	})
type RSASecureHeader[T any] struct {
	RSA          *rsa.PrivateKey
	HeaderName   string
	ContextKey   string
	ContextAbort bool
}

// NewEncodeRSA constructs a new RSA-secure header middleware for a specific type `T`.
//
// Parameters:
//   - headerName: name of the HTTP header carrying the encrypted data
//   - contextKey: key under which decoded data will be stored in Gin context
//   - rsa: RSA private key (its public key used for encryption)
//   - contextAbort: whether to abort the request on decode error
//
// Example:
//
//	key, _ := rsa.GenerateKey(rand.Reader, 2048)
//	header := yaginmiddleware.NewEncodeRSA[MyType]("X-Enc", "payload", key, true)
func NewEncodeRSA[T any](
	headerName string,
	contextKey string,
	rsa *rsa.PrivateKey,
	contextAbort bool,
) *RSASecureHeader[T] {
	return &RSASecureHeader[T]{
		RSA:          rsa,
		ContextKey:   contextKey,
		HeaderName:   headerName,
		ContextAbort: contextAbort,
	}
}

// Encode serializes and encrypts the provided data into a base64-encoded string.
//
// The process includes:
//  1. MessagePack encoding
//  2. GZIP compression
//  3. RSA encryption (public key)
//  4. Base64 encoding
//
// Returns the encoded header string or a `yaerrors.Error`.
//
// Example:
//
//	type Payload struct {
//	    Name string
//	}
//
//	key, _ := rsa.GenerateKey(rand.Reader, 2048)
//	header := yaginmiddleware.NewEncodeRSA[Payload]("X-Enc", "payload", key, true)
//
//	in := Payload{Name: "RZK"}
//	enc, _ := header.Encode(in)
//	fmt.Println(enc) // eyJ... (long base64)
func (h *RSASecureHeader[T]) Encode(data T) (string, yaerrors.Error) {
	bytes, err := yaencoding.EncodeMessagePack(data)
	if err != nil {
		return "", err.Wrap("[RSA HEADER] failed to encode data to bytes")
	}

	zip, err := yagzip.NewGzip().Zip(bytes)
	if err != nil {
		return "", err.Wrap("[RSA HEADER] failed to zip bytes")
	}

	rsa, err := yarsa.Encrypt(zip, &h.RSA.PublicKey)
	if err != nil {
		return "", err.Wrap("[RSA HEADER] failed to encrypt zipped")
	}

	return yaencoding.ToString(rsa), nil
}

// emptySymbol is an invisible Unicode character used internally as a separator
// between the optional plaintext “source” prefix and the binary MessagePack data.
//
// It helps `EncodeWithSrc` and `Decode` distinguish readable prefix text
// from encoded payload bytes.
const emptySymbol = "ᅠ"

// EncodeWithSrc behaves like Encode but also prepends a plaintext “source” string
// before the encrypted MessagePack bytes, separated by an invisible rune (ᅠ).
//
// This allows embedding a readable prefix (e.g., client ID, version, signature)
// that survives decryption and can be retrieved alongside the struct.
//
// Example:
//
//	type Payload struct {
//	    ID uint16
//	}
//
//	key, _ := rsa.GenerateKey(rand.Reader, 2048)
//	header := yaginmiddleware.NewEncodeRSA[Payload]("X-Enc", "payload", key, true)
//
//	in := Payload{ID: 10}
//	enc, _ := header.EncodeWithSrc("ClientA", in)
//	fmt.Println(enc) // base64 ciphertext
func (h *RSASecureHeader[T]) EncodeWithSrc(src string, data T) (string, yaerrors.Error) {
	bytes, err := yaencoding.EncodeMessagePack(data)
	if err != nil {
		return "", err.Wrap("[RSA HEADER] failed to encode data to bytes")
	}

	bytes = append([]byte(src), append([]byte(emptySymbol), bytes...)...)

	zip, err := yagzip.NewGzip().Zip(bytes)
	if err != nil {
		return "", err.Wrap("[RSA HEADER] failed to zip bytes")
	}

	rsa, err := yarsa.Encrypt(zip, &h.RSA.PublicKey)
	if err != nil {
		return "", err.Wrap("[RSA HEADER] failed to encrypt zipped")
	}

	return yaencoding.ToString(rsa), nil
}

// Decode reverses the Encode / EncodeWithSrc process.
//
// It expects a base64-encoded ciphertext, decrypts it using the private key,
// decompresses, and decodes the underlying struct.
//
// Returns:
//   - optional prefix string (if EncodeWithSrc was used, else empty)
//   - pointer to decoded struct
//   - yaerrors.Error if failure occurred
//
// Example:
//
//	type Payload struct { Name string }
//	key, _ := rsa.GenerateKey(rand.Reader, 2048)
//
//	header := yaginmiddleware.NewEncodeRSA[Payload]("X-Enc", "payload", key, true)
//
//	in := Payload{Name: "Test"}
//	enc, _ := header.Encode(in)
//
//	src, out, _ := header.Decode(enc)
//	fmt.Println(src)     // ""
//	fmt.Println(out.Name) // "Test"
func (h *RSASecureHeader[T]) Decode(data string) (string, *T, yaerrors.Error) {
	rawData, err := yaencoding.ToBytes(data)
	if err != nil {
		return "", nil, err.Wrap("[RSA HEADER] failed to decode string to bytes")
	}

	if len(rawData)%h.RSA.Size() != 0 {
		return "", nil, yaerrors.FromString(
			http.StatusInternalServerError,
			"[RSA HEADER] bad block string size",
		)
	}

	zipped, err := yarsa.Decrypt(rawData, h.RSA)
	if err != nil {
		return "", nil, err.Wrap("[RSA HEADER] failed to decrypt to zipped data")
	}

	plaintext, err := yagzip.NewGzip().Unzip(zipped)
	if err != nil {
		return "", nil, err.Wrap("[RSA HEADER] failed to get plain text from zip")
	}

	index := bytes.IndexRune(plaintext, []rune(emptySymbol)[0])
	offset := len([]byte(emptySymbol))

	switch index {
	case 0:
		offset = 0
	case -1:
		index = 0
		offset = 0
	}

	res, err := yaencoding.DecodeMessagePack[T](plaintext[index+offset:])
	if err != nil {
		return "", nil, err.Wrap("[RSA HEADER] failed to decode plaintext")
	}

	return string(plaintext[:index+offset]), res, nil
}

// Handle implements Gin middleware interface to automatically decrypt,
// decode, and inject data into Gin context.
//
// The middleware performs the following:
//  1. Reads the header specified in `HeaderName`.
//  2. Strips CR/LF characters (for safety).
//  3. Calls Decode().
//  4. On success:
//     - Rewrites request header to the plaintext prefix (if present).
//     - Stores decoded struct in context under `ContextKey`.
//     - Calls `ctx.Next()`.
//  5. On failure:
//     - Logs error via ctx.Error(err).
//     - Optionally aborts request if `ContextAbort == true`.
//
// Example:
//
//	type Payload struct { Msg string }
//
//	key, _ := rsa.GenerateKey(rand.Reader, 2048)
//	header := yaginmiddleware.NewEncodeRSA[Payload]("X-Enc", "payload", key, true)
//
//	r := gin.New()
//	r.Use(header.Handle)
//
//	r.GET("/ping", func(c *gin.Context) {
//	    val, _ := c.Get("payload")
//	    fmt.Println(val.(*Payload).Msg)
//	})
func (h *RSASecureHeader[T]) Handle(ctx *gin.Context) {
	text := ctx.GetHeader(h.HeaderName)

	text = yarsa.StripCRLF(text)

	src, data, err := h.Decode(text)
	if err != nil {
		_ = ctx.Error(err)

		if h.ContextAbort {
			ctx.Abort()
		}

		return
	}

	ctx.Request.Header.Set(h.HeaderName, src)

	ctx.Set(h.ContextKey, data)

	ctx.Next()
}
