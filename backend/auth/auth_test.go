package auth

import (
	"strings"
	"testing"
	"time"

	"docupilot/backend/store"
)

func TestPasswordValidationAndHashing(t *testing.T) {
	// Too short (< 8 chars)
	if err := ValidatePassword("short1!"); err != ErrPasswordTooShort {
		t.Fatalf("expected ErrPasswordTooShort, got %v", err)
	}

	// Too long (> 72 bytes)
	longPass := strings.Repeat("A1!", 25) // 75 bytes
	if err := ValidatePassword(longPass); err != ErrPasswordTooLong {
		t.Fatalf("expected ErrPasswordTooLong, got %v", err)
	}

	// Missing letter or digit/symbol
	if err := ValidatePassword("onlyletters"); err != ErrPasswordTooSimple {
		t.Fatalf("expected ErrPasswordTooSimple for letters only, got %v", err)
	}
	if err := ValidatePassword("1234567890"); err != ErrPasswordTooSimple {
		t.Fatalf("expected ErrPasswordTooSimple for digits only, got %v", err)
	}

	// Valid password
	validPass := "Secur3P@ssw0rd!"
	if err := ValidatePassword(validPass); err != nil {
		t.Fatalf("expected valid password, got error: %v", err)
	}

	// Hash generation and check
	hashed, err := HashPassword(validPass)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	if !CheckPassword(hashed, validPass) {
		t.Fatalf("expected password verification to succeed")
	}

	if CheckPassword(hashed, "WrongPassword123!") {
		t.Fatalf("expected wrong password to fail verification")
	}
}

func TestDummyPasswordCheck(t *testing.T) {
	// Must execute without panic
	DummyPasswordCheck("testpassword")
}

func TestEmailNormalizationAndValidation(t *testing.T) {
	testCases := []struct {
		input       string
		expected    string
		expectError bool
	}{
		{"  user@example.com  ", "user@example.com", false},
		{"USER.Name@Example.COM", "user.name@example.com", false},
		{"valid+tag@sub.domain.co", "valid+tag@sub.domain.co", false},
		{"invalid-email", "", true},
		{"@missingusername.com", "", true},
		{"missingdomain@", "", true},
		{"spaces in@domain.com", "", true},
		{"", "", true},
	}

	for _, tc := range testCases {
		norm, err := NormalizeAndValidateEmail(tc.input)
		if tc.expectError {
			if err == nil {
				t.Fatalf("expected error for input %q, got normalized %q", tc.input, norm)
			}
		} else {
			if err != nil {
				t.Fatalf("unexpected error for input %q: %v", tc.input, err)
			}
			if norm != tc.expected {
				t.Fatalf("expected %q, got %q", tc.expected, norm)
			}
		}
	}
}

func TestJWTGenerationAndValidation(t *testing.T) {
	jwtMgr := NewJWTManager("super-secret-key-32-bytes-long!", 1*time.Hour)
	user := &store.User{
		ID:    42,
		Email: "pilot@example.com",
	}

	token, err := jwtMgr.GenerateToken(user)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	claims, err := jwtMgr.ValidateToken(token)
	if err != nil {
		t.Fatalf("failed to validate token: %v", err)
	}

	if claims.UserID != 42 {
		t.Fatalf("expected user ID 42, got %d", claims.UserID)
	}
	if claims.Email != "pilot@example.com" {
		t.Fatalf("expected email pilot@example.com, got %q", claims.Email)
	}

	// Tampered token check
	tampered := token + "tampered"
	if _, err := jwtMgr.ValidateToken(tampered); err != ErrInvalidToken {
		t.Fatalf("expected ErrInvalidToken for tampered token, got %v", err)
	}
}

func TestJWTExpiredToken(t *testing.T) {
	// Create manager with negative duration to simulate immediate expiry
	jwtMgr := NewJWTManager("super-secret-key-32-bytes-long!", -1*time.Minute)
	user := &store.User{
		ID:    1,
		Email: "expired@example.com",
	}

	token, err := jwtMgr.GenerateToken(user)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	_, err = jwtMgr.ValidateToken(token)
	if err != ErrExpiredToken {
		t.Fatalf("expected ErrExpiredToken, got %v", err)
	}
}

func TestIPRateLimiter(t *testing.T) {
	limiter := NewIPRateLimiter(1 * time.Minute)
	defer limiter.Close()

	ip := "192.168.1.100"
	maxRequests := 3
	window := 500 * time.Millisecond

	// First 3 requests must be allowed
	for i := 0; i < maxRequests; i++ {
		if !limiter.Allow(ip, maxRequests, window) {
			t.Fatalf("request %d should have been allowed", i+1)
		}
	}

	// 4th request must be blocked
	if limiter.Allow(ip, maxRequests, window) {
		t.Fatalf("request beyond limit should have been blocked")
	}

	// Wait for window to expire
	time.Sleep(window + 50*time.Millisecond)

	// Next request should be allowed again
	if !limiter.Allow(ip, maxRequests, window) {
		t.Fatalf("request after window expiry should be allowed")
	}
}
