package reporting

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"pulsebid/internal/platform/httpx"
)

// Handler provides HTTP endpoints for generating campaign performance reports.
type Handler struct {
	svc *Service
}

// NewHandler constructs a Handler wrapping the reporting service.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes attaches the reporting routes to the mux guarded by API-key authentication.
func RegisterRoutes(mux *http.ServeMux, svc *Service, apiKeys []string) {
	auth := httpx.APIKey(apiKeys)
	h := NewHandler(svc)
	mux.Handle("GET /v1/reports/campaigns", auth(http.HandlerFunc(h.handleCampaignReports)))
	mux.Handle("GET /v1/reports/campaigns/{id}", auth(http.HandlerFunc(h.handleSingleCampaignReport)))
}

func (h *Handler) handleCampaignReports(w http.ResponseWriter, r *http.Request) {
	params, err := parseQueryParams(r)
	if err != nil {
		httpx.WriteError(w, r, &httpx.Error{Code: "invalid_request", Message: err.Error()})
		return
	}

	campaignIDStrs := r.URL.Query()["campaign_id"]
	if len(campaignIDStrs) > MaxCampaignFilters {
		httpx.WriteError(w, r, &httpx.Error{Code: "invalid_request", Message: "maximum of 100 campaign_id filters allowed"})
		return
	}

	campaignIDs := make([]uuid.UUID, 0, len(campaignIDStrs))
	for _, s := range campaignIDStrs {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		id, err := uuid.Parse(s)
		if err != nil {
			httpx.WriteError(w, r, &httpx.Error{Code: "invalid_request", Message: "invalid campaign_id UUID: " + s})
			return
		}
		campaignIDs = append(campaignIDs, id)
	}
	params.CampaignIDs = campaignIDs

	h.executeReport(w, r, params)
}

func (h *Handler) handleSingleCampaignReport(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	campaignID, err := uuid.Parse(idStr)
	if err != nil {
		httpx.WriteError(w, r, &httpx.Error{Code: "invalid_request", Message: "invalid campaign_id UUID in path"})
		return
	}

	params, err := parseQueryParams(r)
	if err != nil {
		httpx.WriteError(w, r, &httpx.Error{Code: "invalid_request", Message: err.Error()})
		return
	}
	params.CampaignIDs = []uuid.UUID{campaignID}

	h.executeReport(w, r, params)
}

func (h *Handler) executeReport(w http.ResponseWriter, r *http.Request, params QueryParams) {
	report, err := h.svc.GenerateReport(r.Context(), params)
	if err != nil {
		if errors.Is(err, ErrMissingFrom) ||
			errors.Is(err, ErrMissingTo) ||
			errors.Is(err, ErrInvalidRange) ||
			errors.Is(err, ErrInvalidGranularity) ||
			errors.Is(err, ErrRangeTooLarge) ||
			errors.Is(err, ErrTooManyCampaigns) ||
			errors.Is(err, ErrRowLimitExceeded) {
			httpx.WriteError(w, r, &httpx.Error{Code: "invalid_request", Message: err.Error()})
			return
		}
		httpx.WriteError(w, r, &httpx.Error{Code: "internal_error", Message: err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(report)
}

func parseQueryParams(r *http.Request) (QueryParams, error) {
	q := r.URL.Query()

	fromStr := q.Get("from")
	if fromStr == "" {
		return QueryParams{}, errors.New("from parameter is required")
	}
	from, err := time.Parse(time.RFC3339, fromStr)
	if err != nil {
		return QueryParams{}, errors.New("invalid from timestamp: must be RFC 3339")
	}

	toStr := q.Get("to")
	if toStr == "" {
		return QueryParams{}, errors.New("to parameter is required")
	}
	to, err := time.Parse(time.RFC3339, toStr)
	if err != nil {
		return QueryParams{}, errors.New("invalid to timestamp: must be RFC 3339")
	}

	granStr := q.Get("granularity")
	if granStr == "" {
		granStr = string(GranularityTotal)
	}

	gran := Granularity(granStr)
	if gran != GranularityHour && gran != GranularityDay && gran != GranularityTotal {
		return QueryParams{}, errors.New("granularity must be hour, day, or total")
	}

	return QueryParams{
		From:        from,
		To:          to,
		Granularity: gran,
	}, nil
}
