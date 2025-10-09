package yaginmiddleware_test

import (
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaginmiddleware"
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

		header := yaginmiddleware.NewEncodeRSA[testData]("X-Data", "payload", key, true)

		enc, _ := header.Encode(in)

		_, out, _ := header.Decode(enc)

		assert.Equal(t, &in, out, "Data mismatch")
	})

	t.Run("[Middleware] Success", func(t *testing.T) {
		t.Parallel()

		key, err := rsa.GenerateKey(rand.Reader, 2048)
		assert.NoError(t, err)

		lol := "OK"
		in := testData{ID: 7, Text: &lol, Data: []byte{9, 8, 7}}

		header := yaginmiddleware.NewEncodeRSA[testData]("X-Enc", "payload", key, true)

		enc, yaerr := header.Encode(in)
		assert.Nil(t, yaerr, "encode failed: %v", yaerr)

		gin.SetMode(gin.TestMode)
		engine := gin.New()
		engine.Use(header.Handle)

		engine.GET("/ping", func(c *gin.Context) {
			v, _ := c.Get("payload")

			assert.Equal(t, &in, v, "failed to decode response")

			c.JSON(http.StatusOK, v)
		})

		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		req.Header.Set("X-Enc", enc)

		rec := httptest.NewRecorder()

		engine.ServeHTTP(rec, req)
	})

	t.Run("[Middleware] AbortOnInvalidHeader", func(t *testing.T) {
		t.Parallel()

		key, err := rsa.GenerateKey(rand.Reader, 2048)
		assert.NoError(t, err)

		header := yaginmiddleware.NewEncodeRSA[testData]("X-Enc", "payload", key, true)

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
}
