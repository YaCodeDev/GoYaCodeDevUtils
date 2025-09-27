package yamiddleware_test

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yamiddleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

type testData struct {
	ID   uint16  `json:"id"`
	Text *string `json:"text"`
	Data []byte  `json:"data"`
}

func TestEncodeRSAHeader_Flow(t *testing.T) {
	t.Parallel()

	t.Run("[EncodeDecode] RoundTrip", func(t *testing.T) {
		t.Parallel()

		key, err := rsa.GenerateKey(rand.Reader, 2048)
		assert.NoError(t, err, "failed to generate rsa key")

		lol := "RZK&SKALSE<3"

		in := testData{
			ID:   100,
			Text: &lol,
			Data: []byte{1, 2, 3, 4, 5, 6, 7, 8},
		}

		header := yamiddleware.NewEncodeRSA[testData]("X-Data", "payload", key)

		enc, _ := header.Encode(in, &key.PublicKey)

		out, _ := header.Decode(enc, key)

		assert.Equal(t, string(in.Data), string(out.Data), "Data mismatch")
	})

	t.Run("[Middleware] Success", func(t *testing.T) {
		t.Parallel()

		key, err := rsa.GenerateKey(rand.Reader, 2048)
		assert.NoError(t, err)

		lol := "OK"
		in := testData{ID: 7, Text: &lol, Data: []byte{9, 8, 7}}

		header := yamiddleware.NewEncodeRSA[testData]("X-Enc", "payload", key)

		enc, yaerr := header.Encode(in, &key.PublicKey)
		assert.Nil(t, yaerr, "encode failed: %v", yaerr)

		gin.SetMode(gin.TestMode)
		engine := gin.New()
		engine.Use(header.Handle)

		engine.GET("/ping", func(c *gin.Context) {
			v, exists := c.Get("payload")
			assert.True(t, exists, "payload not set in context")

			td, ok := v.(*testData)
			assert.True(t, ok, "payload has wrong type: %T", v)
			c.JSON(http.StatusOK, td)
		})

		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		req.Header.Set("X-Enc", enc)

		rec := httptest.NewRecorder()

		engine.ServeHTTP(rec, req)

		var got testData

		assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got), "failed to decode JSON response")
	})

	t.Run("[Middleware] AbortOnInvalidHeader", func(t *testing.T) {
		t.Parallel()

		key, err := rsa.GenerateKey(rand.Reader, 2048)
		assert.NoError(t, err)

		header := yamiddleware.NewEncodeRSA[testData]("X-Enc", "payload", key)

		gin.SetMode(gin.TestMode)
		engine := gin.New()
		engine.Use(header.Handle)

		handlerCalled := false

		engine.GET("/ping", func(c *gin.Context) {
			handlerCalled = true

			c.JSON(http.StatusOK, gin.H{"message": "pong"})
		})

		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		req.Header.Set("X-Enc", "!!!not-base64!!!")

		rec := httptest.NewRecorder()

		engine.ServeHTTP(rec, req)

		assert.False(t, handlerCalled, "handler should NOT be called on abort")
	})

	t.Run("[Decode] WrongKey", func(t *testing.T) {
		t.Parallel()

		privateA, _ := rsa.GenerateKey(rand.Reader, 2048)
		privateB, _ := rsa.GenerateKey(rand.Reader, 2048)

		lol := "wrong-key"
		in := testData{ID: 1, Text: &lol}

		header := yamiddleware.NewEncodeRSA[testData]("X-Enc", "payload", privateA)

		enc, _ := header.Encode(in, &privateB.PublicKey)

		_, err := header.Decode(enc, privateA)

		assert.Error(t, err, "expected error when decrypting with wrong private key")
	})
}
