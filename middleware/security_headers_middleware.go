package middleware

import "github.com/gin-gonic/gin"

func SecurityHeaders() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		headers := ctx.Writer.Header()
		headers.Set("X-Content-Type-Options", "nosniff")
		headers.Set("X-Frame-Options", "DENY")
		headers.Set("Referrer-Policy", "no-referrer")
		headers.Set("X-Permitted-Cross-Domain-Policies", "none")
		headers.Set("Cross-Origin-Opener-Policy", "same-origin")

		ctx.Next()
	}
}
