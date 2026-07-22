package yaginmiddleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	debugCORSAllowMethods = "GET, POST, PUT, PATCH, DELETE, OPTIONS"
	debugCORSAllowHeaders = "Authorization, Content-Type, Accept, Origin, User-Agent, " +
		"X-Requested-With, CF-Connecting-IP"
	debugCORSExposeHeaders = "Content-Length, Content-Range"
	debugCORSMaxAge        = "1728000"
	debugCORSPlainText     = "text/plain; charset=utf-8"
)

// DebugCORS is a Gin middleware that reflects the request's Origin header back as an
// allow-all CORS policy. It exists for non-production environments only — production
// traffic should go through a real, restrictive CORS policy (or none, if the API is
// not browser-facing) instead of this middleware.
type DebugCORS struct{}

// NewDebugCORS constructs a new DebugCORS middleware.
func NewDebugCORS() *DebugCORS {
	return &DebugCORS{}
}

// Handle implements the Middleware interface.
func (d *DebugCORS) Handle(ctx *gin.Context) {
	if origin := ctx.GetHeader("Origin"); origin != "" {
		header := ctx.Writer.Header()
		header.Set("Access-Control-Allow-Origin", origin)
		header.Set("Access-Control-Allow-Credentials", "true")
		header.Set("Access-Control-Allow-Methods", debugCORSAllowMethods)
		header.Set("Access-Control-Allow-Headers", debugCORSAllowHeaders)
		header.Set("Access-Control-Expose-Headers", debugCORSExposeHeaders)
		header.Set("Access-Control-Max-Age", debugCORSMaxAge)
		header.Set("Vary", "Origin")
	}

	if ctx.Request.Method == http.MethodOptions {
		header := ctx.Writer.Header()
		header.Set("Content-Type", debugCORSPlainText)
		header.Set("Content-Length", "0")
		ctx.AbortWithStatus(http.StatusNoContent)

		return
	}

	ctx.Next()
}
