package yaginmiddleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaginmiddleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func newStaticBearerEngine(secret string) *gin.Engine {
	engine := gin.New()
	engine.Use(yaginmiddleware.NewErrorBoundary(newTestLogger()).Handle)
	engine.Use(yaginmiddleware.NewStaticBearerAuth(secret).Handle)
	engine.GET("/ping", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"message": "pong"})
	})

	return engine
}

func doStaticBearerRequest(
	t *testing.T,
	engine *gin.Engine,
	header, value string,
) *httptest.ResponseRecorder {
	t.Helper()

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/ping", nil)

	if header != "" {
		req.Header.Set(header, value)
	}

	engine.ServeHTTP(rec, req)

	return rec
}

func TestStaticBearerAuth_Handle(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	const secret = "s3cret"

	t.Run("[CorrectToken] Passes", func(t *testing.T) {
		t.Parallel()

		rec := doStaticBearerRequest(
			t,
			newStaticBearerEngine(secret),
			"Authorization",
			"Bearer "+secret,
		)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("[CorrectToken] AccessTokenHeaderFallback", func(t *testing.T) {
		t.Parallel()

		rec := doStaticBearerRequest(t, newStaticBearerEngine(secret), "AccessToken", secret)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("[WrongToken] Rejects", func(t *testing.T) {
		t.Parallel()

		rec := doStaticBearerRequest(
			t,
			newStaticBearerEngine(secret),
			"Authorization",
			"Bearer wrong",
		)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("[MissingHeader] Rejects", func(t *testing.T) {
		t.Parallel()

		rec := doStaticBearerRequest(t, newStaticBearerEngine(secret), "", "")

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("[UnconfiguredSecret] FailsClosedEvenWithAToken", func(t *testing.T) {
		t.Parallel()

		rec := doStaticBearerRequest(
			t,
			newStaticBearerEngine(""),
			"Authorization",
			"Bearer anything",
		)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}
