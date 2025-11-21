package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/meet-app/backend/internal/api/middleware"
	"github.com/meet-app/backend/internal/repository"
	"github.com/meet-app/backend/internal/service"
)

type AuthHandler struct {
	authService service.AuthService
}

func NewAuthHandler(authService service.AuthService) *AuthHandler {
	return &AuthHandler{
		authService: authService,
	}
}

type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Username string `json:"username" binding:"required,min=3,max=30"`
	Password string `json:"password" binding:"required,min=8"`
	Name     string `json:"name" binding:"required,min=1"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type AuthResponse struct {
	User         interface{} `json:"user"`
	AccessToken  string      `json:"access_token"`
	RefreshToken string      `json:"refresh_token"`
	ExpiresAt    string      `json:"expires_at"`
}

// Register godoc
// @Summary Register a new user
// @Description Register a new user with email, username, password, and name
// @Tags auth
// @Accept json
// @Produce json
// @Param request body RegisterRequest true "Registration request"
// @Success 201 {object} AuthResponse
// @Failure 400 {object} middleware.ErrorResponse
// @Failure 500 {object} middleware.ErrorResponse
// @Router /auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	user, tokens, err := h.authService.Register(req.Email, req.Username, req.Password, req.Name)
	if err != nil {
		if err == repository.ErrEmailAlreadyExists {
			middleware.RespondWithError(c, http.StatusConflict, "Email already exists")
			return
		}
		if err == repository.ErrUsernameAlreadyExists {
			middleware.RespondWithError(c, http.StatusConflict, "Username already exists")
			return
		}
		if err == service.ErrWeakPassword {
			middleware.RespondWithError(c, http.StatusBadRequest, err.Error())
			return
		}
		middleware.RespondWithError(c, http.StatusInternalServerError, "Failed to register user")
		return
	}

	c.JSON(http.StatusCreated, AuthResponse{
		User:         user.ToResponse(),
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresAt:    tokens.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

// Login godoc
// @Summary Login user
// @Description Authenticate user with email and password
// @Tags auth
// @Accept json
// @Produce json
// @Param request body LoginRequest true "Login request"
// @Success 200 {object} AuthResponse
// @Failure 400 {object} middleware.ErrorResponse
// @Failure 401 {object} middleware.ErrorResponse
// @Failure 500 {object} middleware.ErrorResponse
// @Router /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	user, tokens, err := h.authService.Login(req.Email, req.Password)
	if err != nil {
		if err == service.ErrInvalidCredentials {
			middleware.RespondWithError(c, http.StatusUnauthorized, "Invalid email or password")
			return
		}
		middleware.RespondWithError(c, http.StatusInternalServerError, "Failed to login")
		return
	}

	c.JSON(http.StatusOK, AuthResponse{
		User:         user.ToResponse(),
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresAt:    tokens.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

// GetMe godoc
// @Summary Get current user
// @Description Get the currently authenticated user's information
// @Tags auth
// @Produce json
// @Security BearerAuth
// @Success 200 {object} models.UserResponse
// @Failure 401 {object} middleware.ErrorResponse
// @Failure 500 {object} middleware.ErrorResponse
// @Router /auth/me [get]
func (h *AuthHandler) GetMe(c *gin.Context) {
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		middleware.RespondWithError(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	user, err := h.authService.GetUserByID(userID)
	if err != nil {
		middleware.RespondWithError(c, http.StatusInternalServerError, "Failed to get user")
		return
	}

	c.JSON(http.StatusOK, user.ToResponse())
}

// RefreshToken godoc
// @Summary Refresh access token
// @Description Get a new access token using refresh token
// @Tags auth
// @Accept json
// @Produce json
// @Param request body map[string]string true "Refresh token request"
// @Success 200 {object} AuthResponse
// @Failure 400 {object} middleware.ErrorResponse
// @Failure 401 {object} middleware.ErrorResponse
// @Router /auth/refresh [post]
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	tokens, err := h.authService.RefreshToken(req.RefreshToken)
	if err != nil {
		middleware.RespondWithError(c, http.StatusUnauthorized, "Invalid refresh token")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token":  tokens.AccessToken,
		"refresh_token": tokens.RefreshToken,
		"expires_at":    tokens.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

// Logout godoc
// @Summary Logout user
// @Description Logout the current user (client should discard tokens)
// @Tags auth
// @Security BearerAuth
// @Success 200 {object} map[string]string
// @Router /auth/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	// In a stateless JWT system, logout is handled client-side
	// The client should discard the tokens
	// For a more secure implementation, you could maintain a token blacklist in Redis

	c.JSON(http.StatusOK, gin.H{
		"message": "Logged out successfully",
	})
}
