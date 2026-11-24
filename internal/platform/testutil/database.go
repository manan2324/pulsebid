package testutil

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"

	"pulsebid/migrations"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// DatabaseURL creates a migrated private database on the configured test server.
// Retention and repository tests must not mutate the application's live tables.
func DatabaseURL(t testing.TB, serverURL string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	admin, err := pgx.Connect(ctx, serverURL)
	if err != nil {
		Unavailable(t, "PostgreSQL test server unavailable: %v", err)
	}
	name := "pulsebid_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	identifier := pgx.Identifier{name}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+identifier); err != nil {
		_ = admin.Close(ctx)
		t.Fatalf("create isolated test database: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		defer func() { _ = admin.Close(ctx) }()
		if _, err := admin.Exec(ctx, "DROP DATABASE "+identifier+" WITH (FORCE)"); err != nil {
			t.Errorf("drop isolated test database: %v", err)
		}
	})
	parsed, err := url.Parse(serverURL)
	if err != nil || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") {
		t.Fatal("test server must use a PostgreSQL URL")
	}
	parsed.Path = "/" + name
	privateURL := parsed.String()
	if err := migrations.Apply(ctx, privateURL, "up"); err != nil {
		t.Fatalf("migrate isolated test database: %v", err)
	}
	return privateURL
}
