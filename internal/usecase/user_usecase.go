package usecase

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"

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
		return "", domain.NewAppError(domain.ErrCodeDatabaseQuery, "Failed to get user", 500, err)
	}
	if user == nil {
		return "", domain.NewAppError(domain.ErrCodeInvalidCredentials, "Invalid credentials", 401, nil)
	}

	if !uc.verifyPassword(password, user.Password) {
		return "", domain.NewAppError(domain.ErrCodeInvalidCredentials, "Invalid credentials", 401, nil)
	}

	token, err := uc.jwtSvc.GenerateToken(strconv.FormatInt(user.ID, 10), user.Username)
	if err != nil {
		return "", domain.NewAppError(domain.ErrCodeTokenInvalid, "Token generation failed", 500, err)
	}

	return token, nil
}

// GetUserInfo retrieves user information by user ID
func (uc *UserUseCase) GetUserInfo(userID int64) (*domain.User, error) {
	user, err := uc.userRepo.GetByID(userID)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeDatabaseQuery, "Failed to get user", 500, err)
	}
	if user == nil {
		return nil, domain.NewAppError(domain.ErrCodeUserNotFound, "User not found", 404, nil)
	}

	return user, nil
}

// verifyPassword checks if the provided password matches the stored hash
func (uc *UserUseCase) verifyPassword(password, hashedPassword string) bool {
	hash := sha256.Sum256([]byte(password))
	passwordHash := hex.EncodeToString(hash[:])
	return passwordHash == hashedPassword
}
