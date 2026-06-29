package auth

import (
	"testing"
	"time"
)

func TestIssueVerifyRoundTrip(t *testing.T) {
	secret := []byte("test-secret")
	now := time.Unix(1_700_000_000, 0)

	tok, err := Issue(secret, "user@example.com", time.Hour, now)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	claims, err := Verify(secret, tok, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if claims.Sub != "user@example.com" {
		t.Errorf("sub = %q, want user@example.com", claims.Sub)
	}
	if claims.Exp != now.Add(time.Hour).Unix() {
		t.Errorf("exp = %d, want %d", claims.Exp, now.Add(time.Hour).Unix())
	}
}

func TestVerifyRejectsExpired(t *testing.T) {
	secret := []byte("test-secret")
	now := time.Unix(1_700_000_000, 0)
	tok, _ := Issue(secret, "u", time.Hour, now)

	if _, err := Verify(secret, tok, now.Add(2*time.Hour)); err != ErrExpired {
		t.Fatalf("err = %v, want ErrExpired", err)
	}
}

func TestVerifyRejectsTamperedSignature(t *testing.T) {
	secret := []byte("test-secret")
	now := time.Unix(1_700_000_000, 0)
	tok, _ := Issue(secret, "u", time.Hour, now)

	// flip the last character of the signature
	tampered := tok[:len(tok)-1] + map[bool]string{true: "A", false: "B"}[tok[len(tok)-1] != 'A']
	if _, err := Verify(secret, tampered, now); err == nil {
		t.Fatal("expected error for tampered token, got nil")
	}
}

func TestVerifyRejectsWrongSecret(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	tok, _ := Issue([]byte("secret-a"), "u", time.Hour, now)

	if _, err := Verify([]byte("secret-b"), tok, now); err != ErrSignature {
		t.Fatalf("err = %v, want ErrSignature", err)
	}
}

func TestVerifyRejectsMalformed(t *testing.T) {
	if _, err := Verify([]byte("s"), "not-a-jwt", time.Now()); err != ErrMalformed {
		t.Fatalf("err = %v, want ErrMalformed", err)
	}
}
