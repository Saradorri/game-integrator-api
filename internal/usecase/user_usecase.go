package usecase

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/saradorri/gameintegrator/internal/domain"
	"github.com/saradorri/gameintegrator/internal/infrastructure/auth"
)

// UserUseCase implements domain.UserUseCase
type UserUseCase struct {
	userRepo domain.UserRepository
	jwtSvc   auth.JWTService
}

// NewUserUseCase creates a new user use case
func NewUserUseCase(userRepo domain.UserRepository, jwtSvc auth.JWTService) domain.UserUseCase {
	return &UserUseCase{
		userRepo: userRepo,
		jwtSvc:   jwtSvc,
	}
}

// Authenticate validates user credentials and returns a JWT token
func (uc *UserUseCase) Authenticate(username, password string) (string, error) {
	user, err := uc.userRepo.GetByUsername(username)
	if err != nil {
		return "", fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return "", errors.New("invalid credentials")
	}

	if !uc.verifyPassword(password, user.Password) {
		return "", errors.New("invalid credentials")
	}

	token, err := uc.jwtSvc.GenerateToken(user.ID, user.Username)
	if err != nil {
		return "", fmt.Errorf("failed to generate token: %w", err)
	}

	return token, nil
}

// GetUserInfo retrieves user information by user ID
func (uc *UserUseCase) GetUserInfo(userID string) (*domain.User, error) {
	user, err := uc.userRepo.GetByID(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return nil, errors.New("user not found")
	}

	return user, nil
}

// verifyPassword checks if the provided password matches the stored hash
func (uc *UserUseCase) verifyPassword(password, hashedPassword string) bool {
	hash := sha256.Sum256([]byte(password))
	passwordHash := hex.EncodeToString(hash[:])
	return passwordHash == hashedPassword
}
