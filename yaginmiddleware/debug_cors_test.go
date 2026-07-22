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

func TestDebugCORS_Handle(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.Use(yaginmiddleware.NewDebugCORS().Handle)
	engine.GET("/ping", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"message": "pong"})
	})

	t.Run("[WithOrigin] ReflectsOriginAndAllowsRequest", func(t *testing.T) {
		t.Parallel()

		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/ping", nil)
		req.Header.Set("Origin", "https://example.com")

		engine.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "https://example.com", rec.Header().Get("Access-Control-Allow-Origin"))
		assert.Equal(t, "true", rec.Header().Get("Access-Control-Allow-Credentials"))
	})

	t.Run("[Preflight] RespondsNoContent", func(t *testing.T) {
		t.Parallel()

		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(
			context.Background(),
			http.MethodOptions,
			"/ping",
			nil,
		)
		req.Header.Set("Origin", "https://example.com")

		engine.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNoContent, rec.Code)
	})

	t.Run("[NoOrigin] NoCORSHeadersSet", func(t *testing.T) {
		t.Parallel()

		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/ping", nil)

		engine.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
	})
}
