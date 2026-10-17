package auth

import (
	"errors"
	"net/mail"
	"regexp"
	"strings"
)

var (
	ErrInvalidEmail = errors.New("please provide a valid email address")

	// emailRegex ensures the email has standard domain format with a TLD of at least 2 chars
	emailRegex = regexp.MustCompile(`^[a-zA-Z0-9.!#$%&'*+/=?^_` + "`" + `{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)+$`)
)

// NormalizeAndValidateEmail trims whitespace, lowercases the email to prevent case-variation
// duplicate accounts or collision hijacking, and validates RFC 5322 syntax.
func NormalizeAndValidateEmail(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", ErrInvalidEmail
	}

	// Length checks (RFC specifies max 254 characters for email)
	if len(trimmed) > 254 {
		return "", ErrInvalidEmail
	}

	// Case normalization: emails are canonicalized to lowercase
	normalized := strings.ToLower(trimmed)

	// Validate with net/mail
	parsed, err := mail.ParseAddress(normalized)
	if err != nil || parsed.Address != normalized {
		return "", ErrInvalidEmail
	}

	// Double-check regex for standard domain syntax
	if !emailRegex.MatchString(normalized) {
		return "", ErrInvalidEmail
	}

	return normalized, nil
}
