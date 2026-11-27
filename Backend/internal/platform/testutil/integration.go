// Package testutil contains shared integration-test helpers.
package testutil

import (
	"os"
	"testing"
)

// Unavailable skips optional local integration tests, but fails required runs.
// CI and make test-int set REQUIRE_INTEGRATION=1 so a missing service cannot
// turn an unexecuted safety check into a passing build.
func Unavailable(t testing.TB, format string, args ...any) {
	t.Helper()
	if os.Getenv("REQUIRE_INTEGRATION") == "1" {
		t.Fatalf(format, args...)
	}
	t.Skipf(format, args...)
}
