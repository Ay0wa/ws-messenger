package service

import "testing"

func TestInviteCodeLength(t *testing.T) {
	code, err := newInviteCode(10)
	if err != nil {
		t.Fatalf("invite code error: %v", err)
	}

	if len(code) != 10 {
		t.Fatalf("expected length 10, got %d", len(code))
	}
}
