package auth

import (
	"errors"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrPasswordTooShort  = errors.New("password must be at least 8 characters long")
	ErrPasswordTooLong   = errors.New("password must not exceed 72 bytes")
	ErrPasswordTooSimple = errors.New("password must include at least one letter and at least one number or symbol")
)

// dummyHash is a precomputed bcrypt hash used to prevent timing attacks when an email is not found.
// Precomputing this avoids running bcrypt.GenerateFromPassword on every missing lookup.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("DocuPilotDummyPassword123!"), bcrypt.DefaultCost)

// ValidatePassword ensures password meets security requirements:
// 1. Minimum 8 characters.
// 2. Maximum 72 bytes (standard bcrypt hard truncation limit; prevents truncation bugs and long-string DoS).
// 3. Contains at least one letter and one number or special character.
func ValidatePassword(password string) error {
	if len(password) < 8 {
		return ErrPasswordTooShort
	}
	if len([]byte(password)) > 72 {
		return ErrPasswordTooLong
	}

	var hasLetter, hasDigitOrPunct bool
	for _, ch := range password {
		if unicode.IsLetter(ch) {
			hasLetter = true
		} else if unicode.IsDigit(ch) || unicode.IsPunct(ch) || unicode.IsSymbol(ch) {
			hasDigitOrPunct = true
		}
	}

	if !hasLetter || !hasDigitOrPunct {
		return ErrPasswordTooSimple
	}

	return nil
}

// HashPassword validates complexity and generates a secure bcrypt hash.
func HashPassword(password string) (string, error) {
	if err := ValidatePassword(password); err != nil {
		return "", err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckPassword securely compares a bcrypt password hash with a plaintext password.
func CheckPassword(hashedPassword, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	return err == nil
}

// DummyPasswordCheck performs a constant-time dummy bcrypt check when a user is not found.
// This prevents attackers from enumerating valid emails via timing differences.
func DummyPasswordCheck(password string) {
	_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
}
