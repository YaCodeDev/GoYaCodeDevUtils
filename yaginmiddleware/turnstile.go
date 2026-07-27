package yaginmiddleware

import (
	"net/http"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yalogger"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yaturnstile"
	"github.com/gin-gonic/gin"
)

// DefaultTurnstileHeader is the request header Turnstile reads a client's
// Cloudflare Turnstile token from unless NewTurnstileWithHeader names another one.
const DefaultTurnstileHeader = "X-Turnstile-Token"

// Turnstile is a Gin middleware that verifies the Cloudflare Turnstile token
// carried in a request header before letting the request reach its handler. A
// rejected token aborts the chain with 403; an unreachable siteverify aborts with
// the status yaturnstile reports, so a Cloudflare outage does not read as a client
// error.
//
// Verification is skipped entirely when the underlying Verifier has no secret key
// configured, which is how a development or test deployment opts out.
//
// The per-request logger is taken from the Gin context via
// yalogger.GinLoggerFromContext, falling back to the logger passed at
// construction when the request carries none.
//
// Must be registered after an ErrorBoundary in the middleware chain; see
// ErrorBoundary's doc comment.
type Turnstile struct {
	verifier yaturnstile.Verifier
	header   string
	log      yalogger.Logger
}

// NewTurnstile constructs a Turnstile reading tokens from DefaultTurnstileHeader.
func NewTurnstile(verifier yaturnstile.Verifier, log yalogger.Logger) *Turnstile {
	return NewTurnstileWithHeader(verifier, DefaultTurnstileHeader, log)
}

// NewTurnstileWithHeader behaves like NewTurnstile but reads the token from
// header instead of DefaultTurnstileHeader. An empty header falls back to
// DefaultTurnstileHeader.
func NewTurnstileWithHeader(
	verifier yaturnstile.Verifier,
	header string,
	log yalogger.Logger,
) *Turnstile {
	if header == "" {
		header = DefaultTurnstileHeader
	}

	if log == nil {
		log = yalogger.NewBaseLogger(nil).NewLogger()
	}

	return &Turnstile{verifier: verifier, header: header, log: log}
}

// Handle implements the Middleware interface.
func (t *Turnstile) Handle(ctx *gin.Context) {
	log := yalogger.GinLoggerFromContext(ctx, t.log, nil)

	if t.verifier == nil {
		abortWithError(
			ctx,
			yaerrors.FromString(
				http.StatusInternalServerError,
				"turnstile verifier is not configured",
			),
		)

		return
	}

	verified, err := t.verifier.Verify(
		ctx.Request.Context(),
		yaturnstile.Token(ctx.GetHeader(t.header)),
		yaturnstile.RemoteIP(ctx.ClientIP()),
		log,
	)
	if err != nil {
		abortWithError(ctx, err.Wrap("failed to verify turnstile token"))

		return
	}

	if !verified {
		abortWithError(
			ctx,
			yaerrors.FromString(http.StatusForbidden, "turnstile verification failed"),
		)

		return
	}

	ctx.Next()
}
