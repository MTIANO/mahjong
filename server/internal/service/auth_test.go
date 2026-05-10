package service

import (
	"testing"
)

func TestGenerateAndParseJWT(t *testing.T) {
	auth := &AuthService{jwtSecret: "test-secret", expireHours: 24}

	token, err := auth.GenerateToken(42)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	userID, err := auth.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if userID != 42 {
		t.Errorf("expected userID 42, got %d", userID)
	}
}

func TestParseToken_Expired(t *testing.T) {
	auth := &AuthService{jwtSecret: "test-secret", expireHours: -1}

	token, _ := auth.GenerateToken(1)
	_, err := auth.ParseToken(token)
	if err == nil {
		t.Error("expected error for expired token")
	}
}

func TestParseToken_Invalid(t *testing.T) {
	auth := &AuthService{jwtSecret: "test-secret", expireHours: 24}

	_, err := auth.ParseToken("invalid-token")
	if err == nil {
		t.Error("expected error for invalid token")
	}
}
