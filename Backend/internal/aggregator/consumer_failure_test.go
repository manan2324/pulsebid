package aggregator

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"pulsebid/internal/events"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kgo"
)

type unavailableDLQ struct{ calls int }

func (d *unavailableDLQ) PublishDLQ(context.Context, *kgo.Record, string) error {
	d.calls++
	return errors.New("DLQ unavailable")
}
func (*unavailableDLQ) Close() {}

func TestConsumer_DLQFailureNeverCommitsSourceOffset(t *testing.T) {
	base := events.Event{SchemaVersion: 1, EventID: uuid.NewString(), Type: events.TypeAuction}
	cases := map[string]func(*events.Event){
		"schema":      func(e *events.Event) { e.SchemaVersion = 99 },
		"missing id":  func(e *events.Event) { e.EventID = "" },
		"bad id":      func(e *events.Event) { e.EventID = "bad" },
		"type":        func(e *events.Event) { e.Type = "unknown" },
		"campaign":    func(e *events.Event) { e.CampaignID = "bad" },
		"winner":      func(e *events.Event) { e.WinnerCampaignID = "bad" },
		"participant": func(e *events.Event) { e.Participants = []string{"bad"} },
		"malformed":   nil,
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			ev := base
			payload := []byte("{invalid")
			if mutate != nil {
				mutate(&ev)
				var err error
				payload, err = json.Marshal(ev)
				require.NoError(t, err)
			}
			dlq := &unavailableDLQ{}
			// No Kafka client/store: reaching ProcessBatch or CommitRecords would
			// panic. A failed DLQ write must return first, preserving redelivery.
			consumer := NewConsumer(nil, nil, dlq, Config{}, nil, nil)
			err := consumer.processChunk(context.Background(), []*kgo.Record{{Value: payload}})
			require.ErrorContains(t, err, "DLQ unavailable")
			require.Equal(t, 1, dlq.calls)
		})
	}
}
