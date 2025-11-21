package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/meet-app/backend/internal/config"
	"github.com/meet-app/backend/pkg/auth"
)

// AuthMiddleware validates JWT tokens and adds user info to context
func AuthMiddleware(cfg *config.JWTConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		var tokenString string

		// Try to get token from Authorization header first
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			// Extract token from "Bearer <token>" format
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && parts[0] == "Bearer" {
				tokenString = parts[1]
			}
		}

		// Fallback to query parameter (for SSE/EventSource which can't send headers)
		if tokenString == "" {
			tokenString = c.Query("token")
		}

		// If still no token, return error
		if tokenString == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authorization token is required",
			})
			c.Abort()
			return
		}

		// Validate token
		claims, err := auth.ValidateToken(tokenString, cfg.Secret)
		if err != nil {
			if err == auth.ErrExpiredToken {
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": "Token has expired",
				})
			} else {
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": "Invalid token",
				})
			}
			c.Abort()
			return
		}

		// Add user info to context
		c.Set("user_id", claims.UserID)
		c.Set("email", claims.Email)
		c.Set("username", claims.Username)

		c.Next()
	}
}

// GetUserIDFromContext retrieves user ID from Gin context
func GetUserIDFromContext(c *gin.Context) (uuid.UUID, error) {
	userID, exists := c.Get("user_id")
	if !exists {
		return uuid.Nil, ErrUserNotInContext
	}

	uid, ok := userID.(uuid.UUID)
	if !ok {
		return uuid.Nil, ErrInvalidUserID
	}

	return uid, nil
}

// GetEmailFromContext retrieves email from Gin context
func GetEmailFromContext(c *gin.Context) (string, error) {
	email, exists := c.Get("email")
	if !exists {
		return "", ErrUserNotInContext
	}

	emailStr, ok := email.(string)
	if !ok {
		return "", ErrInvalidEmail
	}

	return emailStr, nil
}

// GetUsernameFromContext retrieves username from Gin context
func GetUsernameFromContext(c *gin.Context) (string, error) {
	username, exists := c.Get("username")
	if !exists {
		return "", ErrUserNotInContext
	}

	usernameStr, ok := username.(string)
	if !ok {
		return "", ErrInvalidUsername
	}

	return usernameStr, nil
}
