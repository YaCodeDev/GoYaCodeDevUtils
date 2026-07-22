package yaginmiddleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaginmiddleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func newJWTBearerEngine(secret []byte, minRole uint8) *gin.Engine {
	engine := gin.New()
	engine.Use(yaginmiddleware.NewErrorBoundary(newTestLogger()).Handle)
	engine.Use(yaginmiddleware.NewJWTBearerAuth(secret, minRole).Handle)
	engine.GET("/ping", func(ctx *gin.Context) {
		claims, _ := ctx.Get(yaginmiddleware.DefaultJWTContextKey)

		ctx.JSON(http.StatusOK, gin.H{"claims": claims})
	})

	return engine
}

func doJWTBearerRequest(t *testing.T, engine *gin.Engine, token string) *httptest.ResponseRecorder {
	t.Helper()

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/ping", nil)

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	engine.ServeHTTP(rec, req)

	return rec
}

func TestValidateJWT_RoundTrip(t *testing.T) {
	t.Parallel()

	secret := []byte("s3cret")

	tokenString, err := yaginmiddleware.GenerateJWT(42, 5, 1, time.Hour, secret)
	assert.Nil(t, err, "generate should not fail")

	claims, err := yaginmiddleware.ValidateJWT(tokenString, secret)
	assert.Nil(t, err, "validate should not fail")
	assert.Equal(t, uint64(42), claims.Sub)
	assert.Equal(t, uint8(5), claims.Role)
	assert.Equal(t, uint8(1), claims.Status)
}

func TestValidateJWT_RejectsExpired(t *testing.T) {
	t.Parallel()

	secret := []byte("s3cret")

	tokenString, err := yaginmiddleware.GenerateJWT(42, 5, 1, -time.Hour, secret)
	assert.Nil(t, err, "generate should not fail")

	_, err = yaginmiddleware.ValidateJWT(tokenString, secret)
	assert.NotNil(t, err, "expired token should be rejected")
}

func TestJWTBearerAuth_Handle(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	secret := []byte("s3cret")

	t.Run("[SufficientRole] Passes", func(t *testing.T) {
		t.Parallel()

		tokenString, genErr := yaginmiddleware.GenerateJWT(1, 5, 0, time.Hour, secret)
		assert.Nil(t, genErr)

		rec := doJWTBearerRequest(t, newJWTBearerEngine(secret, 5), tokenString)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("[InsufficientRole] Rejects", func(t *testing.T) {
		t.Parallel()

		tokenString, genErr := yaginmiddleware.GenerateJWT(1, 1, 0, time.Hour, secret)
		assert.Nil(t, genErr)

		rec := doJWTBearerRequest(t, newJWTBearerEngine(secret, 5), tokenString)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("[MissingHeader] Rejects", func(t *testing.T) {
		t.Parallel()

		rec := doJWTBearerRequest(t, newJWTBearerEngine(secret, 0), "")

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("[GarbageToken] Rejects", func(t *testing.T) {
		t.Parallel()

		rec := doJWTBearerRequest(t, newJWTBearerEngine(secret, 0), "not-a-jwt")

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("[WrongSecret] Rejects", func(t *testing.T) {
		t.Parallel()

		tokenString, genErr := yaginmiddleware.GenerateJWT(
			1,
			0,
			0,
			time.Hour,
			[]byte("other-secret"),
		)
		assert.Nil(t, genErr)

		rec := doJWTBearerRequest(t, newJWTBearerEngine(secret, 0), tokenString)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}
