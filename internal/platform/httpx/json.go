package httpx

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
)

// DecodeJSON reads the complete bounded body before decoding, so even oversized
// trailing whitespace or malformed payloads are classified consistently as 413.
func DecodeJSON(w http.ResponseWriter, r *http.Request, destination any, maxBytes int64, strict bool) error {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		return &Error{Code: "unsupported_media_type", Message: "Content-Type must be application/json"}
	}
	if maxBytes <= 0 {
		return &Error{Code: "internal", Message: "invalid body limit"}
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	data, err := io.ReadAll(r.Body)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return &Error{Code: "payload_too_large", Message: "request body exceeds size limit"}
		}
		return &Error{Code: "invalid_request", Message: "could not read request body", Cause: err}
	}
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return &Error{Code: "invalid_request", Message: "request body must not be null"}
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if strict {
		decoder.DisallowUnknownFields()
	}
	if err = decoder.Decode(destination); err != nil {
		return &Error{Code: "invalid_request", Message: "invalid JSON request body", Cause: err}
	}
	var extra any
	if err = decoder.Decode(&extra); err != io.EOF {
		return &Error{Code: "invalid_request", Message: "request body must contain one JSON value"}
	}
	return nil
}
