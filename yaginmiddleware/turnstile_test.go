package yaginmiddleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yaginmiddleware"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yalogger"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yaturnstile"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

type fakeTurnstileVerifier struct {
	verified bool
	err      yaerrors.Error
	gotToken yaturnstile.Token
	gotIP    yaturnstile.RemoteIP
}

func (f *fakeTurnstileVerifier) Verify(
	_ context.Context,
	token yaturnstile.Token,
	remoteIP yaturnstile.RemoteIP,
	_ yalogger.Logger,
) (bool, yaerrors.Error) {
	f.gotToken = token
	f.gotIP = remoteIP

	return f.verified, f.err
}

func newTurnstileEngine(middleware *yaginmiddleware.Turnstile) *gin.Engine {
	engine := gin.New()
	engine.Use(yaginmiddleware.NewErrorBoundary(newTestLogger()).Handle)
	engine.Use(middleware.Handle)
	engine.GET("/ping", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"message": "pong"})
	})

	return engine
}

func doTurnstileRequest(
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

func TestTurnstile_Handle(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	const token = "widget-response-token"

	t.Run("[VerifiedToken] Passes", func(t *testing.T) {
		t.Parallel()

		verifier := &fakeTurnstileVerifier{verified: true}

		rec := doTurnstileRequest(
			t,
			newTurnstileEngine(yaginmiddleware.NewTurnstile(verifier, newTestLogger())),
			yaginmiddleware.DefaultTurnstileHeader,
			token,
		)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, yaturnstile.Token(token), verifier.gotToken)
		assert.NotEmpty(t, verifier.gotIP)
	})

	t.Run("[RejectedToken] Forbidden", func(t *testing.T) {
		t.Parallel()

		rec := doTurnstileRequest(
			t,
			newTurnstileEngine(
				yaginmiddleware.NewTurnstile(
					&fakeTurnstileVerifier{verified: false},
					newTestLogger(),
				),
			),
			yaginmiddleware.DefaultTurnstileHeader,
			token,
		)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("[MissingHeader] StillCallsVerifier", func(t *testing.T) {
		t.Parallel()

		verifier := &fakeTurnstileVerifier{verified: false}

		rec := doTurnstileRequest(
			t,
			newTurnstileEngine(yaginmiddleware.NewTurnstile(verifier, newTestLogger())),
			"",
			"",
		)

		assert.Equal(t, http.StatusForbidden, rec.Code)
		assert.Equal(t, yaturnstile.Token(""), verifier.gotToken)
	})

	t.Run("[SiteverifyUnreachable] PropagatesUpstreamStatus", func(t *testing.T) {
		t.Parallel()

		rec := doTurnstileRequest(
			t,
			newTurnstileEngine(
				yaginmiddleware.NewTurnstile(
					&fakeTurnstileVerifier{
						err: yaerrors.FromError(
							http.StatusBadGateway,
							yaturnstile.ErrDoRequest,
							"unreachable",
						),
					},
					newTestLogger(),
				),
			),
			yaginmiddleware.DefaultTurnstileHeader,
			token,
		)

		assert.Equal(t, http.StatusBadGateway, rec.Code)
	})

	t.Run("[CustomHeader] ReadsNamedHeader", func(t *testing.T) {
		t.Parallel()

		const customHeader = "X-Captcha"

		verifier := &fakeTurnstileVerifier{verified: true}

		rec := doTurnstileRequest(
			t,
			newTurnstileEngine(
				yaginmiddleware.NewTurnstileWithHeader(verifier, customHeader, newTestLogger()),
			),
			customHeader,
			token,
		)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, yaturnstile.Token(token), verifier.gotToken)
	})

	t.Run("[NilVerifier] InternalServerError", func(t *testing.T) {
		t.Parallel()

		rec := doTurnstileRequest(
			t,
			newTurnstileEngine(yaginmiddleware.NewTurnstile(nil, newTestLogger())),
			yaginmiddleware.DefaultTurnstileHeader,
			token,
		)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})

	t.Run("[DisabledRealVerifier] Passes", func(t *testing.T) {
		t.Parallel()

		rec := doTurnstileRequest(
			t,
			newTurnstileEngine(
				yaginmiddleware.NewTurnstile(
					yaturnstile.NewVerifier(
						&yaturnstile.Config{Disabled: true},
						newTestLogger(),
					),
					newTestLogger(),
				),
			),
			yaginmiddleware.DefaultTurnstileHeader,
			token,
		)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("[MisconfiguredRealVerifier] FailsClosed", func(t *testing.T) {
		t.Parallel()

		rec := doTurnstileRequest(
			t,
			newTurnstileEngine(
				yaginmiddleware.NewTurnstile(
					yaturnstile.NewVerifier(&yaturnstile.Config{}, newTestLogger()),
					newTestLogger(),
				),
			),
			yaginmiddleware.DefaultTurnstileHeader,
			token,
		)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}
