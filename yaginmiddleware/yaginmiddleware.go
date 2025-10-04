// Package yamiddleware exposes small Gin middlewares.
package yamiddleware

import (
	"crypto/rsa"
	"net/http"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yabase64"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yagzip"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yarsa"
	"github.com/gin-gonic/gin"
)

// Middleware is a minimal interface implemented by all Gin middlewares here.
type Middleware interface {
	Handle(ctx *gin.Context)
}

// RSASecureHeader[T] reads a request header that carries an RSA-encrypted,
// base64 string; it then:
//  1. base64-decodes the header value,
//  2. decrypts with the server RSA private key,
//  3. gunzips the result,
//  4. decodes base64(T),
//  5. stores *T in Gin context under the provided CtxKey.
//
// Server-side flow (what the middleware does):
//   - Read header with name HeaderKey.
//   - Normalize it (remove CR/LF; trim spaces).
//   - base64 -> []byte.
//   - RSA decrypt with RSAKey (private) -> zipped []byte.
//   - gunzip -> plaintext []byte.
//   - base64(T) -> *T.
//   - ctx.Set(CtxKey, *T), then continue the handler chain.
//
// Client-side flow (how to produce the header):
//   - Take value T.
//   - Encode as base64(T).
//   - gzip the bytes.
//   - RSA encrypt with the server's public key.
//   - Convert to base64 string; send it in the HTTP header named HeaderKey.
//
// Security/format notes:
//   - RSA padding/mode must match your yarsa implementation (e.g., OAEP or PKCS#1 v1.5) on both sides.
//   - Gzip is required; if the decrypted bytes are not a gzip stream, decompression fails.
//   - The header value is base64 text; newlines and carriage returns are removed automatically.
//
// Example (client-side: produce the header):
//
//	key, _ := rsa.GenerateKey(rand.Reader, 2048)
//	mw := yamiddleware.NewEncodeRSA[MyPayload]("X-Enc", "payload", key)
//	headerValue, _ := mw.Encode(MyPayload{ID: 1}, &key.PublicKey)
//	// Send request with header:  X-Enc: <headerValue>
//
// Example (server-side: use with Gin):
//
//	key, _ := rsa.GenerateKey(rand.Reader, 2048)
//	mw := yamiddleware.NewEncodeRSA[MyPayload]("X-Enc", "payload", key)
//
//	r := gin.New()
//	r.Use(mw.Handle)
//
//	r.GET("/ping", func(c *gin.Context) {
//	    v, ok := c.Get("payload") // "payload" == ContextKey
//	    if !ok {
//	        c.AbortWithStatus(http.StatusUnauthorized)
//	        return
//	    }
//	    payload := v.(*MyPayload) // type-safe by your generic T
//	    c.JSON(200, payload)
//	})
type RSASecureHeader[T any] struct {
	RSA          *rsa.PrivateKey
	HeaderName   string
	ContextKey   string
	ContextAbort bool
}

// NewEncodeRSA constructs a new EncodeRSA[T] with the given header
// name, context key, and server RSA private key.
//
// Example:
//
//	key, _ := rsa.GenerateKey(rand.Reader, 2048)
//	middleware := yamiddleware.NewEncodeRSA[MyPayload]("X-Enc", "payload", key)
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

// Encode prepares a header value suitable for sending to a server protected by
// EncodeRSA. It serializes data as base64(JSON), gzips the result, RSA
// encrypts it with the provided public key, and base64-encodes the final bytes.
//
// On success it returns the header string. On failure it returns yaerrors.Error.
//
// Example:
//
//	middleware := yamiddleware.NewEncodeRSA[Payload]("X-Enc", "payload", private)
//	headerValue, err := middleware.Encode(Payload{ID: 7}, &private.PublicKey)
//	if err != nil { log.Fatal(err) }
//	req.Header.Set("X-Enc", headerValue)
func (e *RSASecureHeader[T]) Encode(data any) (string, yaerrors.Error) {
	bytes, err := yabase64.Encode(data)
	if err != nil {
		return "", err.Wrap("[RSA HEADER] failed to encode data to bytes")
	}

	zip, err := yagzip.NewGzip().Zip([]byte(bytes))
	if err != nil {
		return "", err.Wrap("[RSA HEADER] failed to zip bytes")
	}

	rsa, err := yarsa.Encrypt(zip, &e.RSA.PublicKey)
	if err != nil {
		return "", err.Wrap("[RSA HEADER] failed to encrypt zipped")
	}

	return yabase64.ToString(rsa), nil
}

// Decode reverses Encode. It accepts a base64 string (as produced by Encode),
// validates RSA block alignment, decrypts with the private key, ungzips, and
// unmarshals into *T.
//
// On success it returns *T; otherwise yaerrors.Error.
//
// Example:
//
//	got, err := middleware.Decode(headerValue)
//	if err != nil { log.Fatal(err) }
//	fmt.Println(got.ID)
func (e *RSASecureHeader[T]) Decode(data string) (*T, yaerrors.Error) {
	bytes, err := yabase64.ToBytes(data)
	if err != nil {
		return nil, err.Wrap("failed to decode string to bytes")
	}

	if len(bytes)%e.RSA.Size() != 0 {
		return nil, yaerrors.FromString(
			http.StatusInternalServerError,
			"[RSA HEADER] bad block string size",
		)
	}

	zipped, err := yarsa.Decrypt(bytes, e.RSA)
	if err != nil {
		return nil, err.Wrap("[RSA HEADER] failed to decrypt to zipped data")
	}

	plaintext, err := yagzip.NewGzip().Unzip(zipped)
	if err != nil {
		return nil, err.Wrap("[RSA HEADER] failed to get plain text from zip")
	}

	res, err := yabase64.Decode[T](string(plaintext))
	if err != nil {
		return nil, err.Wrap("[RSA HEADER] failed to decode plaintext")
	}

	return res, nil
}

// Handle is the Gin middleware entrypoint. It reads the header named HeaderKey,
// cleans it up, decodes it with Decode, and stores the result under CtxKey in
// the Gin context. On error, it records the error, aborts the request, and does
// not call subsequent handlers.
//
// Example:
//
//	middleware := yamiddleware.NewEncodeRSA[Payload]("X-Enc", "payload", key)
//	r := gin.New()
//	r.Use(middleware.Handle)
//
//	r.GET("/ping", func(c *gin.Context) {
//	    v, ok := c.Get("payload")
//	    if !ok { c.AbortWithStatus(http.StatusUnauthorized); return }
//	    payload := v.(*Payload)
//	    c.JSON(200, payload)
//	})
func (e *RSASecureHeader[T]) Handle(ctx *gin.Context) {
	text := ctx.GetHeader(e.HeaderName)

	text = yarsa.StripCRLF(text)

	data, err := e.Decode(text)
	if err != nil {
		_ = ctx.Error(err)

		if e.ContextAbort {
			ctx.Abort()
		}

		return
	}

	ctx.Set(e.ContextKey, data)

	ctx.Next()
}
