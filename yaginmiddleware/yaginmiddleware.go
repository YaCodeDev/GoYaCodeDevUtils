// Package yamiddleware provides ready-to-use Gin middlewares and helpers
// for secure, structured HTTP communication.
//
// The main feature in this package is RSASecureHeader — a generic middleware
// that allows you to transparently transmit compact, encrypted payloads inside
// HTTP headers using the following transformation pipeline:
//
//	Go struct -> MessagePack -> gzip -> RSA encrypt -> base64 -> HTTP header
//
// On the receiving end, RSASecureHeader middleware automatically:
//   - Reads the encrypted header from the request
//   - Decrypts and decompresses the payload
//   - Decodes the MessagePack bytes into the original Go type
//   - Injects the resulting object into gin.Context under a configurable key
//
// This pattern is especially useful when you want to safely transmit
// small request-bound data without exposing secrets or relying on JWTs.
//
// Example end-to-end usage:
//
//	package main
//
//	import (
//	    "crypto/rand"
//	    "crypto/rsa"
//	    "fmt"
//	    "net/http"
//
//	    "github.com/gin-gonic/gin"
//	    "github.com/YaCodeDev/GoYaCodeDevUtils/yaginmiddleware"
//	)
//
//	type Session struct {
//	    UserID uint64
//	    Token  string
//	}
//
//	func main() {
//	    key, _ := rsa.GenerateKey(rand.Reader, 2048)
//
//	    // Create RSA-secured header middleware
//	    secureHeader := yaginmiddleware.NewEncodeRSA[Session](
//	        "X-Secure",  // header name
//	        "session",   // context key
//	        key,         // RSA private key
//	        true,        // abort if decoding fails
//	    )
//
//	    // Setup Gin engine
//	    r := gin.Default()
//	    r.Use(secureHeader.Handle)
//
//	    // Example route reading decoded payload
//	    r.GET("/me", func(c *gin.Context) {
//	        v, _ := c.Get("session")
//	        sess := v.(*Session)
//	        c.JSON(http.StatusOK, gin.H{
//	            "user":  sess.UserID,
//	            "token": sess.Token,
//	        })
//	    })
//
//	    // Example: encode outgoing header client-side
//	    s := Session{UserID: 10, Token: "abc123"}
//	    enc, _ := secureHeader.Encode(s)
//	    fmt.Println("Attach header X-Secure:", enc)
//
//	    _ = r.Run(":8080")
//	}
//
// Internally, RSASecureHeader relies on these YaCodeDev utilities:
//   - yaencoding — MessagePack serialization / base64 helpers
//   - yagzip — gzip compression
//   - yarsa — RSA chunk encryption
//   - yaerrors — structured error wrapping
//
// Each step’s failure produces a yaerrors.Error for consistent handling.
package yamiddleware

import (
	"crypto/rsa"
	"net/http"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaencoding"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yagzip"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yarsa"
	"github.com/gin-gonic/gin"
)

// Middleware defines the standard contract for middlewares in this package.
// Every middleware must implement the Handle(*gin.Context) method.
type Middleware interface {
	Handle(ctx *gin.Context)
}

// RSASecureHeader provides RSA-encrypted header transmission for structured payloads.
//
// It transparently handles the following pipeline:
//  1. Marshal (MessagePack via yaencoding.EncodeMessagePack)
//  2. Compress (gzip via yagzip)
//  3. Encrypt (RSA via yarsa)
//  4. Base64 encode (via yaencoding.ToString)
//
// During decoding, this process is reversed.
//
// Typical use case: securely transmit small JSON/struct data through
// an HTTP header (e.g., "X-Enc") while ensuring confidentiality and integrity.
//
// # Example
//
//	// Generate RSA key
//	key, _ := rsa.GenerateKey(rand.Reader, 2048)
//
//	// Create the middleware handler
//	secureHeader := yamiddleware.NewEncodeRSA[MyPayload](
//	    "X-Enc",     // header name to read/write
//	    "payload",   // context key to store decoded data
//	    key,         // RSA private key
//	    true,        // abort context if decoding fails
//	)
//
//	// Encode example payload to header-safe string
//	token, _ := secureHeader.Encode(MyPayload{
//	    ID:   1,
//	    Name: "RZK",
//	})
//
//	// Example: attach token in header and send request
//	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
//	req.Header.Set("X-Enc", token)
//
//	// Gin middleware automatically decodes and injects payload
//	engine := gin.New()
//	engine.Use(secureHeader.Handle)
//	engine.GET("/ping", func(c *gin.Context) {
//	    val, _ := c.Get("payload")
//	    fmt.Println(val.(*MyPayload))
//	    c.JSON(200, val)
//	})
type RSASecureHeader[T any] struct {
	RSA          *rsa.PrivateKey
	HeaderName   string
	ContextKey   string
	ContextAbort bool
}

// NewEncodeRSA creates a new RSA-secured header middleware instance.
//
// Parameters:
//   - headerName: name of the header that carries the encoded data
//   - contextKey: name used in gin.Context for decoded payload
//   - rsa: RSA private key (encryption uses rsa.PublicKey)
//   - contextAbort: whether to call ctx.Abort() on decode failure
//
// Example:
//
//	key, _ := rsa.GenerateKey(rand.Reader, 2048)
//	header := yamiddleware.NewEncodeRSA[MyType]("X-Enc", "payload", key, true)
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

// Encode serializes, compresses, encrypts, and base64-encodes the given data.
//
// The process:
//  1. MessagePack encode (using yaencoding.EncodeMessagePack)
//  2. Gzip compress (using yagzip.NewGzip().Zip)
//  3. RSA encrypt (using yarsa.Encrypt)
//  4. Base64 encode (using yaencoding.ToString)
//
// Returns an encrypted header-safe string and possible yaerrors.Error.
//
// Example:
//
//	enc, err := header.Encode(MyStruct{Field: "value"})
//	if err != nil {
//	    log.Fatalf("encode failed: %v", err)
//	}
//	req.Header.Set("X-Enc", enc)
func (e *RSASecureHeader[T]) Encode(data any) (string, yaerrors.Error) {
	bytes, err := yaencoding.EncodeMessagePack(data)
	if err != nil {
		return "", err.Wrap("[RSA HEADER] failed to encode data to bytes")
	}

	zip, err := yagzip.NewGzip().Zip(bytes)
	if err != nil {
		return "", err.Wrap("[RSA HEADER] failed to zip bytes")
	}

	rsa, err := yarsa.Encrypt(zip, &e.RSA.PublicKey)
	if err != nil {
		return "", err.Wrap("[RSA HEADER] failed to encrypt zipped")
	}

	return yaencoding.ToString(rsa), nil
}

// Decode performs the inverse process of Encode:
//  1. Base64 decode → bytes
//  2. RSA decrypt → zipped data
//  3. Gzip decompress → plaintext MessagePack
//  4. Decode MessagePack → typed struct `T`
//
// It returns a typed pointer to the decoded struct or an error.
//
// Example:
//
//	out, err := header.Decode(encString)
//	if err != nil {
//	    log.Fatalf("decode failed: %v", err)
//	}
//	fmt.Printf("Decoded struct: %+v\n", out)
func (e *RSASecureHeader[T]) Decode(data string) (*T, yaerrors.Error) {
	bytes, err := yaencoding.ToBytes(data)
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

	res, err := yaencoding.DecodeMessagePack[T](plaintext)
	if err != nil {
		return nil, err.Wrap("[RSA HEADER] failed to decode plaintext")
	}

	return res, nil
}

// Handle implements gin.HandlerFunc.
//
// It reads the encrypted header, decrypts it, and injects the resulting
// struct pointer into the gin context using `ContextKey`. If decoding fails,
// the middleware will record the error in ctx.Errors and optionally abort
// further handler execution if ContextAbort is true.
//
// Example:
//
//	key, _ := rsa.GenerateKey(rand.Reader, 2048)
//	header := yamiddleware.NewEncodeRSA[UserData]("X-User", "payload", key, true)
//
//	engine := gin.New()
//	engine.Use(header.Handle)
//
//	engine.GET("/me", func(c *gin.Context) {
//	    val, _ := c.Get("payload")
//	    user := val.(*UserData)
//	    c.JSON(200, user)
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
