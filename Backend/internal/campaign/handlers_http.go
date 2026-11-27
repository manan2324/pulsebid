package campaign

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"pulsebid/internal/platform/httpx"
)

type endpoint func(context.Context, *http.Request) (int, any, error)

// Handler registers Phase 2 routes only; auth also protects unknown API routes.
func (s *Service) Handler(keys []string) *http.ServeMux {
	mux := http.NewServeMux()
	auth := httpx.APIKey(keys)
	register := func(pattern string, fn endpoint) {
		mux.Handle(pattern, auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			status, value, err := fn(r.Context(), r)
			if err != nil {
				httpx.WriteError(w, r, publicError(err))
				return
			}
			httpx.WriteJSON(w, r, status, value)
		})))
	}

	register("POST /v1/advertisers", func(ctx context.Context, r *http.Request) (int, any, error) {
		body, err := readBody(r)
		if err != nil {
			return 0, nil, err
		}
		v, err := s.CreateAdvertiser(ctx, body)
		return 201, v, err
	})
	register("GET /v1/advertisers", func(ctx context.Context, r *http.Request) (int, any, error) {
		v, err := s.Advertisers(ctx, r.URL.Query())
		return 200, v, err
	})
	register("GET /v1/advertisers/{id}", func(ctx context.Context, r *http.Request) (int, any, error) {
		v, err := s.Advertiser(ctx, r.PathValue("id"))
		return 200, v, err
	})
	register("PATCH /v1/advertisers/{id}", func(ctx context.Context, r *http.Request) (int, any, error) {
		body, err := readBody(r)
		if err != nil {
			return 0, nil, err
		}
		v, err := s.PatchAdvertiser(ctx, r.PathValue("id"), body)
		return 200, v, err
	})

	register("POST /v1/bidders", func(ctx context.Context, r *http.Request) (int, any, error) {
		body, err := readBody(r)
		if err != nil {
			return 0, nil, err
		}
		v, err := s.CreateBidder(ctx, body)
		return 201, v, err
	})
	register("GET /v1/bidders", func(ctx context.Context, r *http.Request) (int, any, error) {
		v, err := s.Bidders(ctx, r.URL.Query())
		return 200, v, err
	})
	register("GET /v1/bidders/{id}", func(ctx context.Context, r *http.Request) (int, any, error) {
		v, err := s.Bidder(ctx, r.PathValue("id"))
		return 200, v, err
	})
	register("PATCH /v1/bidders/{id}", func(ctx context.Context, r *http.Request) (int, any, error) {
		body, err := readBody(r)
		if err != nil {
			return 0, nil, err
		}
		v, err := s.PatchBidder(ctx, r.PathValue("id"), body)
		return 200, v, err
	})

	register("POST /v1/campaigns", func(ctx context.Context, r *http.Request) (int, any, error) {
		body, err := readBody(r)
		if err != nil {
			return 0, nil, err
		}
		v, err := s.CreateCampaign(ctx, body)
		return 201, v, err
	})
	register("GET /v1/campaigns", func(ctx context.Context, r *http.Request) (int, any, error) {
		v, err := s.Campaigns(ctx, r.URL.Query())
		return 200, v, err
	})
	register("GET /v1/campaigns/{id}", func(ctx context.Context, r *http.Request) (int, any, error) {
		v, err := s.Campaign(ctx, r.PathValue("id"))
		return 200, v, err
	})
	register("PATCH /v1/campaigns/{id}", func(ctx context.Context, r *http.Request) (int, any, error) {
		body, err := readBody(r)
		if err != nil {
			return 0, nil, err
		}
		v, err := s.PatchCampaign(ctx, r.PathValue("id"), body)
		return 200, v, err
	})

	register("DELETE /v1/advertisers/{id}", func(ctx context.Context, r *http.Request) (int, any, error) {
		return 204, nil, s.ArchiveAdvertiser(ctx, r.PathValue("id"))
	})
	register("DELETE /v1/bidders/{id}", func(ctx context.Context, r *http.Request) (int, any, error) {
		return 204, nil, s.DeleteBidder(ctx, r.PathValue("id"))
	})
	register("DELETE /v1/campaigns/{id}", func(ctx context.Context, r *http.Request) (int, any, error) {
		_, err := s.SetCampaignStatus(ctx, r.PathValue("id"), "archived")
		return 204, nil, err
	})
	for action, status := range map[string]string{"activate": "active", "pause": "paused"} {
		register("POST /v1/campaigns/{id}/"+action, func(ctx context.Context, r *http.Request) (int, any, error) {
			v, err := s.SetCampaignStatus(ctx, r.PathValue("id"), status)
			return 200, v, err
		})
	}
	register("GET /v1/campaigns/{id}/targeting", func(ctx context.Context, r *http.Request) (int, any, error) {
		v, err := s.Targeting(ctx, r.PathValue("id"))
		return 200, v, err
	})
	register("PUT /v1/campaigns/{id}/targeting", func(ctx context.Context, r *http.Request) (int, any, error) {
		body, err := readBody(r)
		if err != nil {
			return 0, nil, err
		}
		v, err := s.PutTargeting(ctx, r.PathValue("id"), body)
		return 200, v, err
	})
	register("GET /v1/campaigns/{id}/budget", func(ctx context.Context, r *http.Request) (int, any, error) {
		now := time.Now().UTC()
		if s.enableTestEndpoints {
			if th := r.Header.Get("X-PulseBid-Test-Time"); th != "" {
				if t, err := time.Parse(time.RFC3339, th); err == nil {
					now = t.UTC()
				}
			}
		}
		v, err := s.GetBudget(ctx, r.PathValue("id"), now)
		return 200, v, err
	})
	register("POST /v1/campaigns/{id}/budget/reset", func(ctx context.Context, r *http.Request) (int, any, error) {
		if !s.enableTestEndpoints {
			return 0, nil, ErrNotFound
		}
		now := time.Now().UTC()
		if th := r.Header.Get("X-PulseBid-Test-Time"); th != "" {
			if t, err := time.Parse(time.RFC3339, th); err == nil {
				now = t.UTC()
			}
		}
		err := s.ResetBudget(ctx, r.PathValue("id"), now)
		if err != nil {
			return 0, nil, err
		}
		return 200, map[string]any{"status": "ok", "campaign_id": r.PathValue("id")}, nil
	})
	register("POST /v1/campaigns/{id}/creatives", func(ctx context.Context, r *http.Request) (int, any, error) {
		body, err := readBody(r)
		if err != nil {
			return 0, nil, err
		}
		v, err := s.CreateCreative(ctx, r.PathValue("id"), body)
		return 201, v, err
	})
	register("GET /v1/campaigns/{id}/creatives", func(ctx context.Context, r *http.Request) (int, any, error) {
		v, err := s.Creatives(ctx, r.PathValue("id"), r.URL.Query())
		return 200, v, err
	})
	register("GET /v1/creatives/{id}", func(ctx context.Context, r *http.Request) (int, any, error) {
		v, err := s.Creative(ctx, r.PathValue("id"))
		return 200, v, err
	})
	register("PATCH /v1/creatives/{id}", func(ctx context.Context, r *http.Request) (int, any, error) {
		body, err := readBody(r)
		if err != nil {
			return 0, nil, err
		}
		v, err := s.PatchCreative(ctx, r.PathValue("id"), body)
		return 200, v, err
	})
	register("DELETE /v1/creatives/{id}", func(ctx context.Context, r *http.Request) (int, any, error) {
		return 204, nil, s.DeleteCreative(ctx, r.PathValue("id"))
	})
	register("/", func(context.Context, *http.Request) (int, any, error) { return 0, nil, ErrNotFound })
	return mux
}

// A 512 KiB transport bound accommodates JSON-escaped 64 KiB creative markup.
func readBody(r *http.Request) (map[string]json.RawMessage, error) {
	var body map[string]json.RawMessage
	err := httpx.DecodeJSON(nil, r, &body, 512<<10, true)
	return body, err
}

func publicError(err error) error {
	var already *httpx.Error
	if errors.As(err, &already) {
		return err
	}
	code, message := "internal", "internal server error"
	switch {
	case errors.Is(err, ErrValidation):
		code, message = "invalid_request", "request validation failed"
	case errors.Is(err, ErrNotFound):
		code, message = "not_found", "resource not found"
	case errors.Is(err, ErrConflict):
		code, message = "conflict", "resource conflicts with existing data or campaign state"
	case errors.Is(err, ErrUnavailable):
		code, message = "unavailable", "database unavailable"
	}
	result := &httpx.Error{Code: code, Message: message, Cause: err}
	var detail *ValidationError
	if errors.As(err, &detail) {
		result.Details = []httpx.Detail{{Field: detail.Field, Issue: detail.Issue}}
	}
	return result
}
