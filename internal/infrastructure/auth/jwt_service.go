package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/saradorri/gameintegrator/internal/config"
)

// Claims represents the JWT claims
type Claims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// JWTService defines the interface for the JWT service
type JWTService interface {
	GenerateToken(userID, username string) (string, error)
	ValidateToken(tokenString string) (*Claims, error)
	ExtractUserIDFromToken(tokenString string) (string, error)
}

// JWTService handles JWT operations
type jwtService struct {
	config *config.JWTConfig
}

func NewJWTService(config *config.JWTConfig) JWTService {
	return &jwtService{config}
}

// GenerateToken creates a signed JWT token for a user
func (j *jwtService) GenerateToken(userID, username string) (string, error) {
	claims := &Claims{
		UserID:   userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(j.config.Expiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "game-integrator",
			Subject:   userID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(j.config.Secret))
}

// ValidateToken parses and validates a JWT token
func (j *jwtService) ValidateToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrTokenSignatureInvalid
		}
		return []byte(j.config.Secret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, errors.New("could not parse claims")
	}

	if !token.Valid {
		return nil, errors.New("token is invalid")
	}

	return claims, nil
}

// ExtractUserIDFromToken pulls the user ID from a JWT token
func (j *jwtService) ExtractUserIDFromToken(tokenStr string) (string, error) {
	claims, err := j.ValidateToken(tokenStr)
	if err != nil {
		return "", err
	}
	return claims.UserID, nil
}
