package auth

import (
	"testing"

	"github.com/google/uuid"
)

func TestPasswordHashing(t *testing.T) {
	svc := NewService("secret", 60)

	hash, err := svc.HashPassword("pass123")
	if err != nil {
		t.Fatalf("hash error: %v", err)
	}

	if err := svc.CheckPassword(hash, "pass123"); err != nil {
		t.Fatalf("expected password match, got error: %v", err)
	}

	if err := svc.CheckPassword(hash, "wrong"); err == nil {
		t.Fatalf("expected password mismatch")
	}
}

func TestTokenRoundTrip(t *testing.T) {
	svc := NewService("secret", 60)
	userID := uuid.New()

	token, err := svc.IssueToken(userID)
	if err != nil {
		t.Fatalf("issue token error: %v", err)
	}

	parsed, err := svc.ParseToken(token)
	if err != nil {
		t.Fatalf("parse token error: %v", err)
	}

	if parsed != userID {
		t.Fatalf("expected %s, got %s", userID, parsed)
	}
}
