package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	catalogv1 "pulsebid/gen/pulsebid/catalog/v1"

	"github.com/stretchr/testify/require"
)

type retainingBodyTransport struct{ bodies []io.ReadCloser }

func (transport *retainingBodyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// RoundTripper may close the body asynchronously after returning an error.
	transport.bodies = append(transport.bodies, req.Body)
	return nil, errors.New("connection interrupted")
}

type responseBodyTransport struct {
	body func(context.Context) io.ReadCloser
}

func (transport responseBodyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	_ = req.Body.Close()
	return &http.Response{StatusCode: http.StatusOK, Body: transport.body(req.Context()), Header: make(http.Header)}, nil
}

type failedResponseBody struct {
	payload string
	err     error
	closed  bool
}

func (body *failedResponseBody) Read(p []byte) (int, error) {
	n := copy(p, body.payload)
	body.payload = body.payload[n:]
	if len(body.payload) > 0 {
		return n, nil
	}
	return n, body.err
}

func (body *failedResponseBody) Close() error { body.closed = true; return nil }

func TestFanoutResponseBodyFailuresTripBreaker(t *testing.T) {
	for _, tc := range []struct {
		name, payload, outcome string
		err                    error
	}{
		{name: "empty broken body", outcome: "error", err: io.ErrUnexpectedEOF},
		{name: "valid JSON with read failure", payload: `{"bids":[]}`, outcome: "error", err: io.ErrUnexpectedEOF},
		{name: "body deadline", outcome: "timeout", err: context.DeadlineExceeded},
		{name: "oversized body", payload: `{"bids":[]}` + strings.Repeat(" ", 64*1024), outcome: "error", err: io.EOF},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := &failedResponseBody{payload: tc.payload, err: tc.err}
			fanout := NewFanout(Config{BidderMaxInflight: 1, CBMinRequests: 1, CBFailureRatio: 0.5, CBOpenFor: time.Minute}, nil, nil,
				responseBodyTransport{body: func(context.Context) io.ReadCloser { return body }})
			bidder := &catalogv1.Bidder{Id: "bidder", EndpointUrl: "http://bidder.test/bid", TimeoutMs: 1000}
			result := fanout.callOne(context.Background(), "bidder", bidder, nil, &BidRequest{ID: "body-failure"})
			require.Equal(t, tc.outcome, result.Outcome)
			require.True(t, body.closed)
			result = fanout.callOne(context.Background(), "bidder", bidder, nil, &BidRequest{ID: "after-failure"})
			require.Equal(t, "circuit_open", result.Outcome)
		})
	}
}

func TestFanoutFailedRequestBodyIsNotReused(t *testing.T) {
	transport := &retainingBodyTransport{}
	fanout := NewFanout(Config{BidderMaxInflight: 1, CBMinRequests: 100}, nil, nil, transport)
	bidder := &catalogv1.Bidder{Id: "bidder", EndpointUrl: "http://bidder.test/bid", TimeoutMs: 100}
	candidates := []Candidate{{Campaign: &CompiledCampaign{ID: "campaign", MaxBidCPMMicros: 2_000_000}, Creative: &catalogv1.Creative{Id: "creative"}}}
	for _, id := range []string{"first", "second"} {
		result := fanout.callOne(context.Background(), "bidder", bidder, candidates, &BidRequest{ID: id})
		require.Equal(t, "error", result.Outcome)
	}
	require.Len(t, transport.bodies, 2)
	for i, body := range transport.bodies {
		payload, err := io.ReadAll(body)
		require.NoError(t, err)
		require.NoError(t, body.Close())
		var decoded outboundBidRequest
		require.NoError(t, json.Unmarshal(payload, &decoded))
		require.Equal(t, []string{"first", "second"}[i], decoded.ID)
	}
}
