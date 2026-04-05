package middleware

import (
	"strings"

	"boundless-be/service"

	"github.com/gin-gonic/gin"
)

type AuthMiddleware struct {
	tokenService AccessTokenValidator
}

type AccessTokenValidator interface {
	ValidateAccessToken(token string) (service.TokenClaims, error)
}

const (
	UserIDContextKey = "user_id"
	RoleContextKey   = "role"
	TokenContextKey  = "auth_token"
)

func NewAuthMiddleware(tokenService AccessTokenValidator) *AuthMiddleware {
	return &AuthMiddleware{tokenService: tokenService}
}

func (m *AuthMiddleware) RequireAuth() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		token := extractBearer(ctx.GetHeader("Authorization"))
		if token == "" {
			ctx.AbortWithStatusJSON(401, gin.H{"error": "authentication failed"})
			return
		}

		claims, err := m.tokenService.ValidateAccessToken(token)
		if err != nil {
			ctx.AbortWithStatusJSON(401, gin.H{"error": "authentication failed"})
			return
		}

		ctx.Set(TokenContextKey, token)
		ctx.Set(UserIDContextKey, claims.UserID)
		ctx.Set(RoleContextKey, claims.Role)
		ctx.Next()
	}
}

func (m *AuthMiddleware) RequireRole(role string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		currentRole, ok := ctx.Get(RoleContextKey)
		if !ok || currentRole != role {
			ctx.AbortWithStatusJSON(403, gin.H{"error": "forbidden"})
			return
		}
		ctx.Next()
	}
}

func extractBearer(header string) string {
	if !strings.HasPrefix(header, "Bearer ") {
		return ""
	}
	token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	if token == "" {
		return ""
	}
	return token
}
