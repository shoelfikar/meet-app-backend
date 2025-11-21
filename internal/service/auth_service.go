package service

import (
	"errors"

	"github.com/google/uuid"
	"github.com/meet-app/backend/internal/config"
	"github.com/meet-app/backend/internal/models"
	"github.com/meet-app/backend/internal/repository"
	"github.com/meet-app/backend/pkg/auth"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrWeakPassword       = errors.New("password must be at least 8 characters")
)

type AuthService interface {
	Register(email, username, password, name string) (*models.User, *auth.TokenPair, error)
	Login(email, password string) (*models.User, *auth.TokenPair, error)
	RefreshToken(refreshToken string) (*auth.TokenPair, error)
	GetUserByID(id uuid.UUID) (*models.User, error)
}

type authService struct {
	userRepo repository.UserRepository
	jwtCfg   *config.JWTConfig
}

func NewAuthService(userRepo repository.UserRepository, jwtCfg *config.JWTConfig) AuthService {
	return &authService{
		userRepo: userRepo,
		jwtCfg:   jwtCfg,
	}
}

func (s *authService) Register(email, username, password, name string) (*models.User, *auth.TokenPair, error) {
	// Validate password strength
	if !auth.IsPasswordValid(password) {
		return nil, nil, ErrWeakPassword
	}

	// Hash password
	hashedPassword, err := auth.HashPassword(password)
	if err != nil {
		return nil, nil, err
	}

	// Create user
	user := &models.User{
		Email:    email,
		Username: username,
		Password: hashedPassword,
		Name:     name,
	}

	if err := s.userRepo.Create(user); err != nil {
		return nil, nil, err
	}

	// Generate tokens
	tokens, err := auth.GenerateTokenPair(user.ID, user.Email, user.Username, s.jwtCfg)
	if err != nil {
		return nil, nil, err
	}

	return user, tokens, nil
}

func (s *authService) Login(email, password string) (*models.User, *auth.TokenPair, error) {
	// Find user by email
	user, err := s.userRepo.FindByEmail(email)
	if err != nil {
		if err == repository.ErrUserNotFound {
			return nil, nil, ErrInvalidCredentials
		}
		return nil, nil, err
	}

	// Verify password
	if err := auth.VerifyPassword(user.Password, password); err != nil {
		return nil, nil, ErrInvalidCredentials
	}

	// Generate tokens
	tokens, err := auth.GenerateTokenPair(user.ID, user.Email, user.Username, s.jwtCfg)
	if err != nil {
		return nil, nil, err
	}

	return user, tokens, nil
}

func (s *authService) RefreshToken(refreshToken string) (*auth.TokenPair, error) {
	return auth.RefreshAccessToken(refreshToken, s.jwtCfg)
}

func (s *authService) GetUserByID(id uuid.UUID) (*models.User, error) {
	return s.userRepo.FindByID(id)
}
