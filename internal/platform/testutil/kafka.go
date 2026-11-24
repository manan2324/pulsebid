package testutil

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
)

// KafkaTopic creates a private six-partition topic, deleted after the test.
// Sharing the live application topic can mask consumer bugs: another consumer
// may update the test rows before the consumer under test has processed them.
func KafkaTopic(t testing.TB, broker string) string {
	t.Helper()
	client, err := kgo.NewClient(kgo.SeedBrokers(broker))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := client.Ping(ctx); err != nil {
		client.Close()
		Unavailable(t, "Kafka unavailable: %v", err)
	}
	name := "pulsebid-test-" + uuid.NewString()
	req := kmsg.NewPtrCreateTopicsRequest()
	req.Topics = []kmsg.CreateTopicsRequestTopic{{Topic: name, NumPartitions: 6, ReplicationFactor: 1}}
	response, err := req.RequestWith(ctx, client)
	if err != nil {
		client.Close()
		t.Fatal(err)
	}
	for _, topic := range response.Topics {
		if topic.ErrorCode != 0 {
			client.Close()
			t.Fatalf("create private topic: Kafka error %d", topic.ErrorCode)
		}
	}
	t.Cleanup(func() {
		defer client.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		req := kmsg.NewPtrDeleteTopicsRequest()
		req.TopicNames = []string{name}
		req.Topics = []kmsg.DeleteTopicsRequestTopic{{Topic: &name}}
		if _, err := req.RequestWith(ctx, client); err != nil {
			t.Errorf("delete private test topic: %v", err)
		}
	})
	return name
}
