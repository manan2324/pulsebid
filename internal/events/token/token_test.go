package token

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func samplePayload(now time.Time) Payload {
	return Payload{
		BidID:        "0193f0a2-1234-7000-8000-000000000001",
		RequestID:    "req-123",
		CampaignID:   "camp-1",
		CreativeID:   "cr-1",
		AdvertiserID: "adv-1",
		BidderID:     "bidder-1",
		PriceMicros:  1510,
		LandingURL:   "https://example.com/landing",
		IssuedAt:     now.Unix(),
		ExpiresAt:    now.Add(24 * time.Hour).Unix(),
	}
}

func TestToken_RoundTrip(t *testing.T) {
	secret := []byte("super-secret-key-for-test-32bytes!")
	now := time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)
	p := samplePayload(now)

	tok, err := Sign(secret, p)
	require.NoError(t, err)
	require.NotEmpty(t, tok)

	verified, err := Verify(secret, tok, now)
	require.NoError(t, err)
	require.Equal(t, p.BidID, verified.BidID)
	require.Equal(t, p.RequestID, verified.RequestID)
	require.Equal(t, p.CampaignID, verified.CampaignID)
	require.Equal(t, p.CreativeID, verified.CreativeID)
	require.Equal(t, p.AdvertiserID, verified.AdvertiserID)
	require.Equal(t, p.BidderID, verified.BidderID)
	require.Equal(t, p.PriceMicros, verified.PriceMicros)
	require.Equal(t, p.LandingURL, verified.LandingURL)
	require.Equal(t, p.IssuedAt, verified.IssuedAt)
	require.Equal(t, p.ExpiresAt, verified.ExpiresAt)
}

func TestToken_Sign_EmptySecret(t *testing.T) {
	p := samplePayload(time.Now())
	_, err := Sign(nil, p)
	require.Error(t, err)
}

func TestToken_TamperPayload(t *testing.T) {
	secret := []byte("super-secret-key-for-test-32bytes!")
	now := time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)
	p := samplePayload(now)

	tok, err := Sign(secret, p)
	require.NoError(t, err)

	parts := strings.Split(tok, ".")
	require.Len(t, parts, 2)

	// Tamper payload part by altering one character
	tamperedPayload := parts[0]
	if tamperedPayload[0] == 'A' {
		tamperedPayload = "B" + tamperedPayload[1:]
	} else {
		tamperedPayload = "A" + tamperedPayload[1:]
	}
	tamperedTok := tamperedPayload + "." + parts[1]

	_, err = Verify(secret, tamperedTok, now)
	require.ErrorIs(t, err, ErrInvalidToken)
}

func TestToken_TamperSignature(t *testing.T) {
	secret := []byte("super-secret-key-for-test-32bytes!")
	now := time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)
	p := samplePayload(now)

	tok, err := Sign(secret, p)
	require.NoError(t, err)

	parts := strings.Split(tok, ".")
	require.Len(t, parts, 2)

	// Tamper signature
	tamperedSig := parts[1]
	if tamperedSig[len(tamperedSig)-1] == 'a' {
		tamperedSig = tamperedSig[:len(tamperedSig)-1] + "b"
	} else {
		tamperedSig = tamperedSig[:len(tamperedSig)-1] + "a"
	}
	tamperedTok := parts[0] + "." + tamperedSig

	_, err = Verify(secret, tamperedTok, now)
	require.ErrorIs(t, err, ErrInvalidToken)
}

func TestToken_WrongSecret(t *testing.T) {
	secret1 := []byte("secret-key-1")
	secret2 := []byte("secret-key-2")
	now := time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)
	p := samplePayload(now)

	tok, err := Sign(secret1, p)
	require.NoError(t, err)

	_, err = Verify(secret2, tok, now)
	require.ErrorIs(t, err, ErrInvalidToken)
}

func TestToken_Malformed(t *testing.T) {
	secret := []byte("super-secret-key")
	now := time.Now()

	testCases := []string{
		"",
		"not-a-token",
		"too.many.dots.in.token",
		"invalid-base64-part.valid-sig",
		"valid-part.invalid-base64-sig!@#$",
		base64.RawURLEncoding.EncodeToString([]byte(`{"b":""}`)) + ".dummy",
	}

	for _, tc := range testCases {
		_, err := Verify(secret, tc, now)
		require.ErrorIs(t, err, ErrInvalidToken, "case: %s", tc)
	}

	// Empty secret in verify
	_, err := Verify(nil, "abc.def", now)
	require.ErrorIs(t, err, ErrInvalidToken)
}

func TestToken_MissingFields(t *testing.T) {
	secret := []byte("super-secret-key")
	now := time.Now()

	// Missing bid_id
	p1 := samplePayload(now)
	p1.BidID = ""
	tok1, _ := Sign(secret, p1)
	_, err := Verify(secret, tok1, now)
	require.ErrorIs(t, err, ErrInvalidToken)

	// Missing request_id
	p2 := samplePayload(now)
	p2.RequestID = ""
	tok2, _ := Sign(secret, p2)
	_, err = Verify(secret, tok2, now)
	require.ErrorIs(t, err, ErrInvalidToken)

	// Missing campaign_id
	p3 := samplePayload(now)
	p3.CampaignID = ""
	tok3, _ := Sign(secret, p3)
	_, err = Verify(secret, tok3, now)
	require.ErrorIs(t, err, ErrInvalidToken)
}

func TestToken_Expired(t *testing.T) {
	secret := []byte("super-secret-key")
	issuedAt := time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)
	p := samplePayload(issuedAt)
	tok, err := Sign(secret, p)
	require.NoError(t, err)

	// 23h later -> still valid
	_, err = Verify(secret, tok, issuedAt.Add(23*time.Hour))
	require.NoError(t, err)

	// 24h + 1s later -> expired
	_, err = Verify(secret, tok, issuedAt.Add(24*time.Hour+time.Second))
	require.ErrorIs(t, err, ErrExpiredToken)

	// 48h later -> expired
	_, err = Verify(secret, tok, issuedAt.Add(48*time.Hour))
	require.ErrorIs(t, err, ErrExpiredToken)
}

func TestToken_PaddedBase64(t *testing.T) {
	secret := []byte("super-secret-key")
	now := time.Now()
	p := samplePayload(now)

	tok, err := Sign(secret, p)
	require.NoError(t, err)

	// Verify standard decode with padding works
	parts := strings.Split(tok, ".")
	pBytes, _ := base64.RawURLEncoding.DecodeString(parts[0])
	sBytes, _ := base64.RawURLEncoding.DecodeString(parts[1])

	paddedP := base64.URLEncoding.EncodeToString(pBytes)
	paddedS := base64.URLEncoding.EncodeToString(sBytes)
	paddedTok := paddedP + "." + paddedS

	verified, err := Verify(secret, paddedTok, now)
	require.NoError(t, err)
	require.Equal(t, p.BidID, verified.BidID)
}
