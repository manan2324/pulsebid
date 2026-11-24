package token

import (
	"errors"
	"testing"
	"time"
)

func FuzzVerify(f *testing.F) {
	secret := []byte("fuzz-only-secret")
	now := time.Unix(1_800_000_000, 0)
	valid, err := Sign(secret, Payload{
		BidID: "bid", RequestID: "request", CampaignID: "campaign",
		IssuedAt: now.Unix(), ExpiresAt: now.Add(time.Hour).Unix(),
	})
	if err != nil {
		f.Fatal(err)
	}
	for _, seed := range []string{"", ".", "a.b.c", "e30.invalid", valid} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		payload, err := Verify(secret, input, now)
		if err != nil {
			if !errors.Is(err, ErrInvalidToken) && !errors.Is(err, ErrExpiredToken) {
				t.Fatalf("unexpected error: %v", err)
			}
			return
		}
		if payload.BidID == "" || payload.RequestID == "" || payload.CampaignID == "" || payload.ExpiresAt < now.Unix() {
			t.Fatal("accepted an invalid payload")
		}
		// An accepted token must round-trip with identical signed fields.
		signed, err := Sign(secret, payload)
		if err != nil {
			t.Fatal(err)
		}
		roundTrip, err := Verify(secret, signed, now)
		if err != nil || roundTrip != payload {
			t.Fatalf("round trip: %+v, %v", roundTrip, err)
		}
	})
}
