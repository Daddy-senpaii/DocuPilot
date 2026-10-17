package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	ContextUserIDKey    = "auth_user_id"
	ContextUserEmailKey = "auth_user_email"
)

// AuthMiddleware extracts and validates Bearer JWT tokens from the Authorization header.
// If required is true, requests without a valid token are aborted with 401 Unauthorized.
// If required is false, valid tokens populate user context while unauthenticated requests proceed.
func AuthMiddleware(jwtManager *JWTManager, required bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			if required {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authentication required. Please sign in."})
				return
			}
			c.Next()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			if required {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid Authorization format. Expected 'Bearer <token>'"})
				return
			}
			c.Next()
			return
		}

		tokenStr := strings.TrimSpace(parts[1])
		claims, err := jwtManager.ValidateToken(tokenStr)
		if err != nil {
			if required {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
				return
			}
			c.Next()
			return
		}

		c.Set(ContextUserIDKey, claims.UserID)
		c.Set(ContextUserEmailKey, claims.Email)
		c.Next()
	}
}

// GetAuthUserID retrieves the authenticated user ID from context if present.
func GetAuthUserID(c *gin.Context) (int64, bool) {
	val, exists := c.Get(ContextUserIDKey)
	if !exists {
		return 0, false
	}
	id, ok := val.(int64)
	return id, ok
}

// GetAuthEmail retrieves the authenticated user email from context if present.
func GetAuthEmail(c *gin.Context) (string, bool) {
	val, exists := c.Get(ContextUserEmailKey)
	if !exists {
		return "", false
	}
	email, ok := val.(string)
	return email, ok
}
