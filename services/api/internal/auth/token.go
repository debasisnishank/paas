// Package auth issues and verifies signed bearer tokens for the control plane.
//
// Phase 0: self-contained HS256 JWTs signed with a shared server secret — no
// external IdP. This is the seam where SPIFFE/SVID service-to-service auth and
// an OIDC device flow plug in later; callers depend only on Issue/Verify.
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var (
	ErrMalformed = errors.New("malformed token")
	ErrSignature = errors.New("invalid token signature")
	ErrExpired   = errors.New("token expired")
)

// Claims is the JWT payload we issue and trust.
type Claims struct {
	Sub string `json:"sub"` // subject (account email)
	Iat int64  `json:"iat"` // issued-at (unix seconds)
	Exp int64  `json:"exp"` // expiry (unix seconds)
}

type header struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func sign(secret []byte, signingInput string) string {
	m := hmac.New(sha256.New, secret)
	m.Write([]byte(signingInput))
	return b64(m.Sum(nil))
}

// Issue mints an HS256 JWT for subject sub, valid for ttl starting at now.
func Issue(secret []byte, sub string, ttl time.Duration, now time.Time) (string, error) {
	h, err := json.Marshal(header{Alg: "HS256", Typ: "JWT"})
	if err != nil {
		return "", err
	}
	p, err := json.Marshal(Claims{Sub: sub, Iat: now.Unix(), Exp: now.Add(ttl).Unix()})
	if err != nil {
		return "", err
	}
	signingInput := b64(h) + "." + b64(p)
	return signingInput + "." + sign(secret, signingInput), nil
}

// Verify checks the signature and expiry of token and returns its claims.
func Verify(secret []byte, token string, now time.Time) (Claims, error) {
	var claims Claims
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return claims, ErrMalformed
	}
	signingInput := parts[0] + "." + parts[1]
	if !hmac.Equal([]byte(sign(secret, signingInput)), []byte(parts[2])) {
		return claims, ErrSignature
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return claims, ErrMalformed
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return claims, ErrMalformed
	}
	if now.Unix() >= claims.Exp {
		return claims, ErrExpired
	}
	return claims, nil
}
