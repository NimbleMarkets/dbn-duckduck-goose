// Copyright (c) 2025 Neomantra Corp

package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

var allowedOrigins = GetEnvDefault("ALLOWED_ORIGINS", "*")
var allowedHeaders = GetEnvDefault("ALLOWED_HEADERS", "Accept, Content-Type, Content-Length, Accept-Encoding, Authorization")

func CorsOptionHandlerWithVerbs(verbs ...string) gin.HandlerFunc {
	allowMethods := strings.Join(verbs, ", ")
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", allowedOrigins)
		c.Header("Access-Control-Allow-Methods", allowMethods)
		c.Header("Access-Control-Allow-Headers", allowedHeaders)
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
