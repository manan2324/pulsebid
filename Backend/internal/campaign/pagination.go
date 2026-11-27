package campaign

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strconv"
	"time"
)

type pageRequest struct {
	Limit                         int32
	HasCursor                     bool
	Time                          time.Time
	ID, Scope, Advertiser, Status string
}
type cursor struct {
	Version int       `json:"v"`
	Scope   string    `json:"s"`
	Time    time.Time `json:"t"`
	ID      string    `json:"id"`
}

func parsePage(query url.Values, scope string) (pageRequest, error) {
	p := pageRequest{Limit: 50, ID: "00000000-0000-0000-0000-000000000000", Time: time.Unix(0, 0).UTC(), Scope: scope}
	if value, ok := query["limit"]; ok {
		if len(value) != 1 {
			return p, invalid("limit", "must appear once")
		}
		n, err := strconv.ParseInt(value[0], 10, 32)
		if err != nil || n < 1 || n > 200 {
			return p, invalid("limit", "must be between 1 and 200")
		}
		p.Limit = int32(n)
	}
	if values, ok := query["cursor"]; ok {
		if len(values) != 1 || len(values[0]) > 2048 || values[0] == "" {
			return p, invalid("cursor", "invalid cursor")
		}
		data, err := base64.RawURLEncoding.DecodeString(values[0])
		if err != nil {
			return p, invalid("cursor", "invalid cursor")
		}
		var c cursor
		if err = json.Unmarshal(data, &c); err != nil || c.Version != 1 || c.Scope != scope || c.Time.IsZero() {
			return p, invalid("cursor", "invalid cursor or mismatched filters")
		}
		if scope == "bidders" {
			if !validSlug(c.ID) {
				return p, invalid("cursor", "invalid bidder cursor")
			}
		} else if !validUUID(c.ID) {
			return p, invalid("cursor", "invalid UUID cursor")
		}
		p.HasCursor, p.Time, p.ID = true, c.Time.UTC(), c.ID
	}
	return p, nil
}

func paginate[T any](rows []T, p pageRequest, key func(T) (time.Time, string)) Page[T] {
	if rows == nil {
		rows = []T{}
	}
	page := Page[T]{Data: rows}
	if len(rows) > int(p.Limit) {
		page.Data = rows[:p.Limit]
		at, id := key(page.Data[len(page.Data)-1])
		data, _ := json.Marshal(cursor{Version: 1, Scope: p.Scope, Time: at.UTC(), ID: id})
		next := base64.RawURLEncoding.EncodeToString(data)
		page.NextCursor = &next
	}
	return page
}
