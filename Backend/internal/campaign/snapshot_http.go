package campaign

import (
	"net/http"
	"strconv"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"
	"pulsebid/internal/platform/httpx"
	"pulsebid/internal/platform/logging"
)

// Handler serves protobuf JSON with proto field names and exact int64 strings.
// Conditional reads still check the database version before returning 304.
func (s *SnapshotService) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		snapshot, err := s.GetSnapshot(r.Context(), 0)
		if err != nil {
			httpx.WriteError(w, r, publicError(err))
			return
		}
		etag := `"` + strconv.FormatInt(snapshot.Version, 10) + `"`
		w.Header().Set("ETag", etag)
		w.Header().Set("Cache-Control", "private, no-cache")
		if matchesETag(strings.Join(r.Header.Values("If-None-Match"), ","), etag) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		data, err := (protojson.MarshalOptions{UseProtoNames: true, EmitDefaultValues: true}).Marshal(snapshot)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err = w.Write(data); err != nil {
			logging.FromContext(r.Context()).DebugContext(r.Context(), "snapshot response write failed")
		}
	})
}

// GET validators use weak comparison; quoted commas remain inside opaque tags.
func matchesETag(header, current string) bool {
	header = strings.TrimSpace(header)
	if header == "*" {
		return true
	}
	matched := false
	for header != "" {
		header = strings.TrimSpace(header)
		header = strings.TrimPrefix(header, "W/")
		if len(header) < 2 || header[0] != '"' {
			return false
		}
		end := strings.IndexByte(header[1:], '"')
		if end < 0 {
			return false
		}
		end++
		tag := header[:end+1]
		for _, c := range tag[1 : len(tag)-1] {
			if c < 0x21 || c == 0x7f {
				return false
			}
		}
		matched = matched || tag == current
		header = strings.TrimSpace(header[end+1:])
		if header == "" {
			return matched
		}
		if header[0] != ',' {
			return false
		}
		header = strings.TrimSpace(header[1:])
		if header == "" {
			return false
		}
	}
	return false
}
