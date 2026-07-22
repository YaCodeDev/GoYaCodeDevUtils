package yaginmiddleware

import (
	"errors"
	"net/http"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yalogger"
	"github.com/gin-gonic/gin"
)

// ErrorResponseFunc builds the JSON body written for a resolved request error. status
// is the HTTP status code the response will be written with; message is the
// human-readable error text.
//
// Supply a custom ErrorResponseFunc via NewErrorBoundaryWithResponse when a service's
// existing API contract uses a response shape other than the default
// `{"error": message}`.
type ErrorResponseFunc func(status int, message string) any

// defaultErrorResponse produces the `{"error": message}` shape used by most of this
// org's Gin services.
func defaultErrorResponse(_ int, message string) any {
	return gin.H{"error": message}
}

// ErrorBoundary is a Gin middleware that centralizes HTTP error handling.
// Downstream handlers and middlewares record an error via ctx.Error(err); once the
// request finishes, ErrorBoundary maps whatever was recorded into a JSON response and
// a log line.
//
// # Behavior
//
//   - No recorded error: the response already written by the handler is left as-is.
//   - Exactly one yaerrors.Error recorded: its Code() becomes the HTTP status, its
//     UnwrapLastError() becomes the response message, and it is logged at Warn for
//     4xx/503-class codes or Error otherwise.
//   - Exactly one non-yaerrors.Error recorded: logged at Error, answered with a
//     generic 500.
//   - More than one error recorded: each is logged at Error, and the response is a
//     flat 418 "Backend developer is a teapot" — this mirrors the shared convention
//     already used across this org's Gin services for the "this should not happen"
//     case, not a joke unique to this package.
//
// Middlewares that abort a request on failure (StaticBearerAuth, JWTBearerAuth, or a
// service's own) must be registered *after* ErrorBoundary in the chain: they only
// record the error via ctx.Error and call ctx.Abort, relying on ErrorBoundary to turn
// that into an actual HTTP response.
//
// Example:
//
//	router := gin.New()
//	router.Use(yaginmiddleware.NewErrorBoundary(log).Handle)
//
//	router.GET("/ping", func(ctx *gin.Context) {
//	    _ = ctx.Error(yaerrors.FromString(http.StatusNotFound, "not found"))
//	})
type ErrorBoundary struct {
	log      yalogger.Logger
	response ErrorResponseFunc
}

// NewErrorBoundary constructs an ErrorBoundary middleware using the default
// `{"error": message}` response shape.
//
// Parameters:
//   - log: the fallback Logger used to report recorded errors.
func NewErrorBoundary(log yalogger.Logger) *ErrorBoundary {
	return &ErrorBoundary{log: log, response: defaultErrorResponse}
}

// NewErrorBoundaryWithResponse behaves like NewErrorBoundary but builds the response
// body via the given ErrorResponseFunc instead of the default `{"error": message}`
// shape. Passing a nil response falls back to the default.
func NewErrorBoundaryWithResponse(log yalogger.Logger, response ErrorResponseFunc) *ErrorBoundary {
	if response == nil {
		response = defaultErrorResponse
	}

	return &ErrorBoundary{log: log, response: response}
}

// Handle implements the Middleware interface, mapping any error(s) recorded via
// ctx.Error during the request into a JSON response and a log line.
func (e *ErrorBoundary) Handle(ctx *gin.Context) {
	ctx.Next()

	switch len(ctx.Errors) {
	case 0:
		return
	case 1:
		e.handleSingle(ctx, ctx.Errors[0].Err)
	default:
		e.handleMultiple(ctx)
	}
}

func (e *ErrorBoundary) handleSingle(ctx *gin.Context, err error) {
	var yaerr yaerrors.Error

	if !errors.As(err, &yaerr) {
		e.log.Errorf("unclassified error: %v", err)
		ctx.JSON(
			http.StatusInternalServerError,
			e.response(http.StatusInternalServerError, "Internal server error"),
		)

		return
	}

	if yaerr == nil {
		yaerr = yaerrors.FromString(http.StatusTeapot, "Backend developer is a teapot")
	}

	logAtLevel(e.log, yaerr)

	ctx.JSON(yaerr.Code(), e.response(yaerr.Code(), yaerr.UnwrapLastError()))
}

func (e *ErrorBoundary) handleMultiple(ctx *gin.Context) {
	for _, ginErr := range ctx.Errors {
		e.log.Errorf("error: %v", ginErr.Err)
	}

	const teapotMessage = "Backend developer is a teapot"

	ctx.JSON(http.StatusTeapot, e.response(http.StatusTeapot, teapotMessage))
}

func logAtLevel(log yalogger.Logger, err yaerrors.Error) {
	if isWarnLevel(err.Code()) {
		log.Warn(err.UnwrapLastError())

		return
	}

	log.Error(err.UnwrapLastError())
}

func isWarnLevel(code int) bool {
	if code >= http.StatusBadRequest && code < http.StatusInternalServerError {
		return true
	}

	return code == http.StatusServiceUnavailable
}
