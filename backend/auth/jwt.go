package auth

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

	"docupilot/backend/store"
	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken = errors.New("invalid or malformed token")
	ErrExpiredToken = errors.New("token has expired")
)

// Claims defines the JWT payload claims for an authenticated user.
type Claims struct {
	UserID int64  `json:"user_id"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

// JWTManager handles generating and validating HMAC-SHA256 JWT tokens.
type JWTManager struct {
	secretKey     []byte
	tokenDuration time.Duration
}

// NewJWTManager creates a new JWTManager. If secret is empty or insecure,
// a sensible development key is used with a warning.
func NewJWTManager(secret string, duration time.Duration) *JWTManager {
	if duration == 0 {
		duration = 24 * time.Hour
	}

	key := []byte(secret)
	if len(key) < 16 {
		log.Println("[Warning] JWT_SECRET is missing or shorter than 16 bytes. Using internal dev secret. Set a strong JWT_SECRET in production.")
		key = []byte("docupilot-default-development-secret-key-32b!")
	}

	return &JWTManager{
		secretKey:     key,
		tokenDuration: duration,
	}
}

// GenerateToken creates a signed JWT token containing user identity and expiration.
func (j *JWTManager) GenerateToken(user *store.User) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID: user.ID,
		Email:  user.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatInt(user.ID, 10),
			Issuer:    "docupilot",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(j.tokenDuration)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(j.secretKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return signed, nil
}

// ValidateToken parses and verifies the token signature and expiration.
// It explicitly guards against algorithm substitution attacks.
func (j *JWTManager) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(
		tokenString,
		&Claims{},
		func(t *jwt.Token) (interface{}, error) {
			// Explicit algorithm check: ensure algorithm is HMAC SHA-256
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return j.secretKey, nil
		},
	)

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}
