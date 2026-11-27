// Package token implements HMAC-SHA256 signed event tokens for impression and click tracking (Section 10.3).
package token

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
	// ErrInvalidToken is returned when signature mismatch, malformed payload, or missing fields occur.
	ErrInvalidToken = errors.New("invalid_token")

	// ErrExpiredToken is returned when the token's expiration timestamp has passed.
	ErrExpiredToken = errors.New("token_expired")
)

// Payload defines the compact JSON structure inside an event token (Section 10.3).
type Payload struct {
	BidID        string `json:"b"`
	RequestID    string `json:"r"`
	CampaignID   string `json:"c"`
	CreativeID   string `json:"cr"`
	AdvertiserID string `json:"a"`
	BidderID     string `json:"bd"`
	PriceMicros  int64  `json:"p"`
	LandingURL   string `json:"u"`
	IssuedAt     int64  `json:"iat"`
	ExpiresAt    int64  `json:"exp"`
}

// Sign creates a signed event token:
// token = base64url(payloadJSON) + "." + base64url(HMAC_SHA256(secret, payloadJSON))
func Sign(secret []byte, p Payload) (string, error) {
	if len(secret) == 0 {
		return "", errors.New("token secret cannot be empty")
	}

	payloadJSON, err := json.Marshal(p)
	if err != nil {
		return "", err
	}

	mac := hmac.New(sha256.New, secret)
	mac.Write(payloadJSON)
	sig := mac.Sum(nil)

	payloadPart := base64.RawURLEncoding.EncodeToString(payloadJSON)
	sigPart := base64.RawURLEncoding.EncodeToString(sig)

	return payloadPart + "." + sigPart, nil
}

// Verify validates the token's signature, decodes the payload, and checks expiry.
// Constant-time HMAC comparison (hmac.Equal) is used to prevent timing attacks.
func Verify(secret []byte, tokenStr string, now time.Time) (Payload, error) {
	if len(secret) == 0 || tokenStr == "" {
		return Payload{}, ErrInvalidToken
	}

	parts := strings.Split(tokenStr, ".")
	if len(parts) != 2 {
		return Payload{}, ErrInvalidToken
	}

	payloadBytes, err := decodeBase64URL(parts[0])
	if err != nil {
		return Payload{}, ErrInvalidToken
	}

	sigBytes, err := decodeBase64URL(parts[1])
	if err != nil {
		return Payload{}, ErrInvalidToken
	}

	mac := hmac.New(sha256.New, secret)
	mac.Write(payloadBytes)
	expectedSig := mac.Sum(nil)

	if !hmac.Equal(sigBytes, expectedSig) {
		return Payload{}, ErrInvalidToken
	}

	var p Payload
	if err := json.Unmarshal(payloadBytes, &p); err != nil {
		return Payload{}, ErrInvalidToken
	}

	if p.BidID == "" || p.RequestID == "" || p.CampaignID == "" {
		return Payload{}, ErrInvalidToken
	}

	if now.Unix() > p.ExpiresAt {
		return Payload{}, ErrExpiredToken
	}

	return p, nil
}

func decodeBase64URL(s string) ([]byte, error) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err == nil {
		return b, nil
	}
	return base64.URLEncoding.DecodeString(s)
}
