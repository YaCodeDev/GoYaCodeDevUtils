package yaginmiddleware_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yaginmiddleware"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yalogger"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func newTestLogger() yalogger.Logger {
	return yalogger.NewBaseLogger(nil).NewLogger()
}

func TestErrorBoundary_Handle(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	t.Run("[NoError] PassesThroughHandlerResponse", func(t *testing.T) {
		t.Parallel()

		engine := gin.New()
		engine.Use(yaginmiddleware.NewErrorBoundary(newTestLogger()).Handle)
		engine.GET("/ping", func(ctx *gin.Context) {
			ctx.JSON(http.StatusOK, gin.H{"message": "pong"})
		})

		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/ping", nil)

		engine.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.JSONEq(t, `{"message":"pong"}`, rec.Body.String())
	})

	t.Run("[SingleYaError] MapsCodeAndMessage", func(t *testing.T) {
		t.Parallel()

		engine := gin.New()
		engine.Use(yaginmiddleware.NewErrorBoundary(newTestLogger()).Handle)
		engine.GET("/ping", func(ctx *gin.Context) {
			_ = ctx.Error(yaerrors.FromString(http.StatusNotFound, "not found"))
		})

		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/ping", nil)

		engine.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
		assert.JSONEq(t, `{"error":"not found"}`, rec.Body.String())
	})

	t.Run("[SingleYaError] CustomResponseShape", func(t *testing.T) {
		t.Parallel()

		response := func(status int, message string) any {
			return gin.H{"status": status, "message": message}
		}

		engine := gin.New()
		engine.Use(yaginmiddleware.NewErrorBoundaryWithResponse(newTestLogger(), response).Handle)
		engine.GET("/ping", func(ctx *gin.Context) {
			_ = ctx.Error(yaerrors.FromString(http.StatusBadRequest, "bad request"))
		})

		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/ping", nil)

		engine.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.JSONEq(t, `{"status":400,"message":"bad request"}`, rec.Body.String())
	})

	t.Run("[SingleNonYaError] GenericInternalServerError", func(t *testing.T) {
		t.Parallel()

		engine := gin.New()
		engine.Use(yaginmiddleware.NewErrorBoundary(newTestLogger()).Handle)
		engine.GET("/ping", func(ctx *gin.Context) {
			_ = ctx.Error(
				errors.New("boom"),
			) //nolint:err113 // intentional plain error for the test
		})

		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/ping", nil)

		engine.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		assert.JSONEq(t, `{"error":"Internal server error"}`, rec.Body.String())
	})

	t.Run("[MultipleErrors] RespondsTeapot", func(t *testing.T) {
		t.Parallel()

		engine := gin.New()
		engine.Use(yaginmiddleware.NewErrorBoundary(newTestLogger()).Handle)
		engine.GET("/ping", func(ctx *gin.Context) {
			_ = ctx.Error(yaerrors.FromString(http.StatusBadRequest, "first"))
			_ = ctx.Error(yaerrors.FromString(http.StatusBadRequest, "second"))
		})

		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/ping", nil)

		engine.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusTeapot, rec.Code)
		assert.JSONEq(t, `{"error":"Backend developer is a teapot"}`, rec.Body.String())
	})
}
