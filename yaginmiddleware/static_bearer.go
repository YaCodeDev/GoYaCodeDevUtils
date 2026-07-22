package yaginmiddleware

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/gin-gonic/gin"
)

const bearerPrefix = "Bearer "

// StaticBearerAuth is a Gin middleware that authenticates a request by comparing an
// Authorization (falling back to AccessToken) bearer token against a single static
// secret, in constant time.
//
// It fails closed: a missing header, an unconfigured (empty) secret, or a mismatched
// token are all rejected with 401. An unconfigured secret must never mean "no auth
// required" — configure the secret, or don't register this middleware.
//
// Must be registered after an ErrorBoundary in the middleware chain; see
// ErrorBoundary's doc comment.
type StaticBearerAuth struct {
	secret string
}

// NewStaticBearerAuth constructs a StaticBearerAuth middleware for the given static
// secret. An empty secret makes every request fail authentication.
func NewStaticBearerAuth(secret string) *StaticBearerAuth {
	return &StaticBearerAuth{secret: secret}
}

// Handle implements the Middleware interface.
func (a *StaticBearerAuth) Handle(ctx *gin.Context) {
	token := bearerToken(ctx)

	if a.secret == "" || token == "" ||
		subtle.ConstantTimeCompare([]byte(token), []byte(a.secret)) != 1 {
		abortWithError(
			ctx,
			yaerrors.FromString(http.StatusUnauthorized, "invalid or missing bearer token"),
		)

		return
	}

	ctx.Next()
}

func bearerToken(ctx *gin.Context) string {
	header := ctx.GetHeader("Authorization")
	if header == "" {
		header = ctx.GetHeader("AccessToken")
	}

	return strings.TrimPrefix(header, bearerPrefix)
}

// abortWithError records err on the Gin context and aborts the middleware chain,
// leaving response-writing to a later-unwound ErrorBoundary.
func abortWithError(ctx *gin.Context, err yaerrors.Error) {
	_ = ctx.Error(err) //nolint:errcheck // best-effort; ErrorBoundary reads ctx.Errors regardless

	ctx.Abort()
}
