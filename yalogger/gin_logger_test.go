package yalogger

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func newBufferedLogger(level Level) (Logger, *bytes.Buffer) {
	base := NewBaseLogger(&Config{
		BaseLoggerType:   Logrus,
		Level:            level,
		FullTimestamp:    false,
		DisableTimestamp: true,
	})

	buffer := bytes.NewBuffer(nil)
	base.(*baseLogrus).logger.SetOutput(buffer)

	return base.NewLogger(), buffer
}

func TestContextWithLoggerAndLoggerFromContext(t *testing.T) {
	fallback := NewBaseLogger(nil).NewLogger().WithField("scope", "global")
	ctxLog := NewBaseLogger(nil).NewLogger().WithField(KeyUserID, uint64(42))

	ctx := ContextWithLogger(nil, ctxLog)
	got := LoggerFromContext(ctx, fallback)

	if got == nil {
		t.Fatal("expected logger from context")
	}

	if got.GetField("scope") != "global" {
		t.Fatalf("expected fallback field, got %v", got.GetField("scope"))
	}

	if got.GetField(KeyUserID) != uint64(42) {
		t.Fatalf("expected context field, got %v", got.GetField(KeyUserID))
	}
}

func TestSetGinContextLoggerAndFallbackLookup(t *testing.T) {
	gin.SetMode(gin.TestMode)

	fallback := NewBaseLogger(nil).NewLogger().WithField("scope", "global")
	requestLog := NewBaseLogger(nil).NewLogger().WithField(KeyRequestID, "req-1")
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"/health",
		nil,
	)

	SetGinContextLogger(ctx, requestLog, nil)

	value, exists := ctx.Get(GinContextLoggerKey)
	if !exists {
		t.Fatal("expected logger in gin context")
	}

	if _, ok := value.(Logger); !ok {
		t.Fatalf("expected logger type, got %T", value)
	}

	got := GinLoggerFromContext(ctx, fallback, nil)
	if got == nil {
		t.Fatal("expected logger from gin context")
	}

	if got.GetField("scope") != "global" {
		t.Fatalf("expected fallback field, got %v", got.GetField("scope"))
	}

	if got.GetField(KeyRequestID) != "req-1" {
		t.Fatalf("expected request field, got %v", got.GetField(KeyRequestID))
	}

	fromRequestContext := LoggerFromContext(ctx.Request.Context(), fallback)
	if fromRequestContext.GetField(KeyRequestID) != "req-1" {
		t.Fatalf(
			"expected request context field, got %v",
			fromRequestContext.GetField(KeyRequestID),
		)
	}
}

func TestCompactGinLine(t *testing.T) {
	in := "  \x1b[31m [WARNING]   hello   world \x1b[0m  "

	out := compactGinLine(in)
	if out != "[WARNING] hello world" {
		t.Fatalf("unexpected compact output: %q", out)
	}
}

func TestWriteGinLogLineAndWriter(t *testing.T) {
	log, buffer := newBufferedLogger(TraceLevel)

	writeGinLogLine(log, "", ginLogLevelDebug)
	writeGinLogLine(log, "[WARNING] something happened", ginLogLevelDebug)
	writeGinLogLine(log, "[ERROR] something failed", ginLogLevelDebug)
	writeGinLogLine(log, "plain line", ginLogLevelDebug)

	writer := &ginLogWriter{log: log, fallback: ginLogLevelDebug}
	input := []byte("line one\nline two\n")

	n, err := writer.Write(input)
	if err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}

	if n != len(input) {
		t.Fatalf("expected %d bytes written, got %d", len(input), n)
	}

	output := buffer.String()
	for _, expected := range []string{
		"[GIN] WARN: something happened",
		"[GIN] ERROR: something failed",
		"[GIN] DEBUG: plain line",
		"[GIN] DEBUG: line one",
		"[GIN] DEBUG: line two",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected output to contain %q, got %q", expected, output)
		}
	}
}

func TestGinAccessLoggerFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)

	log, buffer := newBufferedLogger(TraceLevel)
	router := gin.New()
	router.Use(GinAccessLogger(log, nil))
	router.GET("/ok", func(ctx *gin.Context) {
		SetGinContextLogger(ctx, log.WithField(KeyRequestID, "req-ok"), nil)
		ctx.Status(http.StatusOK)
	})
	router.GET("/bad", func(ctx *gin.Context) {
		ctx.Status(http.StatusBadRequest)
	})
	router.GET("/err", func(ctx *gin.Context) {
		_ = ctx.Error(errors.New("forced error"))
		ctx.Status(http.StatusInternalServerError)
	})

	for _, path := range []string{"/ok", "/bad", "/err"} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, nil)
		router.ServeHTTP(rec, req)

		if rec.Code == 0 {
			t.Fatalf("expected non-zero status for %s", path)
		}
	}

	output := buffer.String()
	for _, expected := range []string{
		"[GIN] RUN status=200",
		"[GIN] RUN status=400",
		"[GIN] RUN status=500",
		"err=Error #01: forced error",
		"request_id=req-ok",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected output to contain %q, got %q", expected, output)
		}
	}
}

func TestGinAccessLoggerQueryAndUnknownRouteBranches(t *testing.T) {
	gin.SetMode(gin.TestMode)

	log, buffer := newBufferedLogger(TraceLevel)

	t.Run("query string branch", func(t *testing.T) {
		router := gin.New()
		router.Use(GinAccessLogger(log, nil))
		router.GET("/query", func(ctx *gin.Context) {
			ctx.Status(http.StatusNoContent)
		})

		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(
			context.Background(),
			http.MethodGet,
			"/query?x=1",
			nil,
		)
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Fatalf("expected status 204, got %d", rec.Code)
		}
	})

	t.Run("route path fallback branch", func(t *testing.T) {
		router := gin.New()
		router.Use(GinAccessLogger(log, nil))

		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/missing", nil)
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d", rec.Code)
		}
	})

	output := buffer.String()
	for _, expected := range []string{
		"path=/query?x=1",
		"route=/missing",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected output to contain %q, got %q", expected, output)
		}
	}
}

func TestGinRecoveryMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	log, buffer := newBufferedLogger(TraceLevel)
	router := gin.New()
	router.Use(GinRecovery(log, nil))
	router.GET("/panic", func(_ *gin.Context) {
		panic("boom")
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/panic", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rec.Code)
	}

	output := buffer.String()
	if !strings.Contains(output, "[GIN] PANIC method=GET path=/panic") {
		t.Fatalf("expected panic log, got %q", output)
	}
}

func TestGinRecoveryMiddlewareDebugMode(t *testing.T) {
	gin.SetMode(gin.DebugMode)
	defer gin.SetMode(gin.TestMode)

	log, buffer := newBufferedLogger(TraceLevel)
	router := gin.New()
	router.Use(GinRecovery(log, nil))
	router.GET("/panic", func(_ *gin.Context) {
		panic("boom")
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/panic", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rec.Code)
	}

	output := buffer.String()
	if !strings.Contains(output, "stack=") {
		t.Fatalf("expected stack trace in output, got %q", output)
	}
}

func TestConfigureGinDebugLogging(t *testing.T) {
	log, buffer := newBufferedLogger(TraceLevel)

	originalWriter := gin.DefaultWriter
	originalErrorWriter := gin.DefaultErrorWriter
	originalPrintFunc := gin.DebugPrintFunc
	originalRouteFunc := gin.DebugPrintRouteFunc
	defer func() {
		gin.DefaultWriter = originalWriter
		gin.DefaultErrorWriter = originalErrorWriter
		gin.DebugPrintFunc = originalPrintFunc
		gin.DebugPrintRouteFunc = originalRouteFunc
	}()

	ConfigureGinDebugLogging(log)

	gin.DebugPrintFunc("route: %s", "/health")
	gin.DebugPrintRouteFunc(http.MethodGet, "/health", "handler", 1)

	output := buffer.String()
	for _, expected := range []string{
		"[GIN] DEBUG: route: /health",
		"[GIN] ROUTE method=GET path=/health handler=handler handlers=1",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected output to contain %q, got %q", expected, output)
		}
	}
}
