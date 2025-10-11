// Package yaginmiddleware provides secure middleware utilities for Gin.
package yaginmiddleware

import "github.com/gin-gonic/gin"

// Middleware represents a generic Gin middleware component
// capable of processing requests via a `Handle` method.
type Middleware interface {
	Handle(ctx *gin.Context)
}
