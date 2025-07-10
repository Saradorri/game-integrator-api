package user

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"

	"github.com/saradorri/gameintegrator/internal/domain"
	"github.com/saradorri/gameintegrator/internal/infrastructure/auth"
	"github.com/saradorri/gameintegrator/internal/infrastructure/logger"
	"go.uber.org/zap"
)

// UserUseCase implements domain.UserUseCase
type UserUseCase struct {
	userRepo domain.UserRepository
	jwtSvc   auth.JWTService
	logger   *logger.Logger
}

// NewUserUseCase creates a new user use case
func NewUserUseCase(userRepo domain.UserRepository, jwtSvc auth.JWTService, logger *logger.Logger) domain.UserUseCase {
	return &UserUseCase{
		userRepo: userRepo,
		jwtSvc:   jwtSvc,
		logger:   logger,
	}
}

// Authenticate validates user credentials and returns a JWT token
func (uc *UserUseCase) Authenticate(username, password string) (string, error) {
	uc.logger.Info("Starting user authentication",
		zap.String("username", username))

	if username == "" || password == "" {
		uc.logger.Warn("Authentication attempt with empty credentials",
			zap.String("username", username),
			zap.Bool("has_password", password != ""))
		return "", domain.NewAppError(domain.ErrCodeInvalidCredentials, "Invalid credentials", 401, nil)
	}

	user, err := uc.userRepo.GetByUsername(username)
	if err != nil {
		uc.logger.Error("Failed to get user from database during authentication",
			zap.String("username", username),
			zap.Error(err))
		return "", domain.NewAppError(domain.ErrCodeDatabaseQuery, "Failed to get user", 500, err)
	}

	if user == nil {
		uc.logger.Warn("Authentication failed - user not found",
			zap.String("username", username))
		return "", domain.NewAppError(domain.ErrCodeInvalidCredentials, "Invalid credentials", 401, nil)
	}

	uc.logger.Debug("User found in database",
		zap.Int64("user_id", user.ID),
		zap.String("username", username),
		zap.String("currency", user.Currency))

	if !uc.verifyPassword(password, user.Password) {
		uc.logger.Warn("Authentication failed - invalid password",
			zap.Int64("user_id", user.ID),
			zap.String("username", username))
		return "", domain.NewAppError(domain.ErrCodeInvalidCredentials, "Invalid credentials", 401, nil)
	}

	uc.logger.Debug("Password verification successful",
		zap.Int64("user_id", user.ID),
		zap.String("username", username))

	token, err := uc.jwtSvc.GenerateToken(strconv.FormatInt(user.ID, 10), user.Username)
	if err != nil {
		uc.logger.Error("Failed to generate JWT token",
			zap.Int64("user_id", user.ID),
			zap.String("username", username),
			zap.Error(err))
		return "", domain.NewAppError(domain.ErrCodeTokenInvalid, "Token generation failed", 500, err)
	}

	uc.logger.Info("User authentication successful",
		zap.Int64("user_id", user.ID),
		zap.String("username", username),
		zap.String("currency", user.Currency))

	return token, nil
}

// GetUserInfo retrieves user information by user ID
func (uc *UserUseCase) GetUserInfo(userID int64) (*domain.User, error) {
	uc.logger.Info("Retrieving user information",
		zap.Int64("user_id", userID))

	if userID <= 0 {
		uc.logger.Warn("Invalid user ID provided",
			zap.Int64("user_id", userID))
		return nil, domain.NewAppError(domain.ErrCodeInvalidFormat, "Invalid user ID", 400, nil)
	}

	user, err := uc.userRepo.GetByID(userID)
	if err != nil {
		uc.logger.Error("Failed to get user from database",
			zap.Int64("user_id", userID),
			zap.Error(err))
		return nil, domain.NewAppError(domain.ErrCodeDatabaseQuery, "Failed to get user", 500, err)
	}

	if user == nil {
		uc.logger.Warn("User not found",
			zap.Int64("user_id", userID))
		return nil, domain.NewAppError(domain.ErrCodeUserNotFound, "User not found", 404, nil)
	}

	uc.logger.Info("User information retrieved successfully",
		zap.Int64("user_id", user.ID),
		zap.String("username", user.Username),
		zap.String("currency", user.Currency))

	return user, nil
}

// verifyPassword checks if the provided password matches the stored hash
func (uc *UserUseCase) verifyPassword(password, hashedPassword string) bool {
	uc.logger.Debug("Verifying password hash")

	if password == "" || hashedPassword == "" {
		uc.logger.Debug("Password verification failed - empty password or hash")
		return false
	}

	hash := sha256.Sum256([]byte(password))
	passwordHash := hex.EncodeToString(hash[:])

	isValid := passwordHash == hashedPassword

	if isValid {
		uc.logger.Debug("Password verification successful")
	} else {
		uc.logger.Debug("Password verification failed - hash mismatch")
	}

	return isValid
}
