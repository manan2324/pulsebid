// Package httpx implements the common HTTP contract and server middleware.
package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"pulsebid/internal/platform/logging"
)

// Detail identifies an invalid request field.
type Detail struct {
	Field string `json:"field"`
	Issue string `json:"issue"`
}

// Error is a safe public error, optionally wrapping a private cause.
type Error struct {
	Code      string   `json:"code"`
	Message   string   `json:"message"`
	Details   []Detail `json:"details,omitempty"`
	RequestID string   `json:"request_id"`
	Cause     error    `json:"-"`
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

// Unwrap exposes the private cause to errors.Is/As.
func (e *Error) Unwrap() error { return e.Cause }

// Status maps every contract error code to its HTTP status.
func Status(code string) int {
	switch code {
	case "invalid_request", "invalid_token", "bad_request":
		return http.StatusBadRequest
	case "unauthorized":
		return http.StatusUnauthorized
	case "not_found":
		return http.StatusNotFound
	case "conflict":
		return http.StatusConflict
	case "payload_too_large":
		return http.StatusRequestEntityTooLarge
	case "unsupported_media_type":
		return http.StatusUnsupportedMediaType
	case "token_expired":
		return http.StatusGone
	case "unavailable":
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// WriteError is the single transport mapping point; service handlers wrap their
// owning sentinel errors in Error so no database or internal error text escapes.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	public := Error{Code: "internal", Message: "internal server error"}
	var supplied *Error
	if errors.As(err, &supplied) && supplied != nil && Status(supplied.Code) != http.StatusInternalServerError {
		public = *supplied
	} else if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		public = Error{Code: "unavailable", Message: "request unavailable"}
	}
	public.RequestID = logging.RequestID(r.Context())
	public.Cause = nil
	WriteJSON(w, r, Status(public.Code), struct {
		Error Error `json:"error"`
	}{public})
}

// WriteJSON encodes before committing headers, preventing partial JSON on errors.
func WriteJSON(w http.ResponseWriter, r *http.Request, status int, value any) {
	if status == http.StatusNoContent {
		w.WriteHeader(status)
		return
	}
	data, err := json.Marshal(value)
	if err != nil {
		logging.FromContext(r.Context()).ErrorContext(r.Context(), "encode HTTP response")
		WriteError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, err = w.Write(append(data, '\n')); err != nil {
		logging.FromContext(r.Context()).DebugContext(r.Context(), "response write failed")
	}
}
