package ids_test

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"pulsebid/internal/platform/ids"
	"testing"
)

func TestIdentifiers(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for range 100 {
		id, err := ids.New()
		require.NoError(t, err)
		parsed, err := uuid.Parse(id)
		require.NoError(t, err)
		require.Equal(t, uuid.Version(7), parsed.Version())
		require.False(t, seen[id])
		seen[id] = true
	}
	id, err := ids.V5("6ba7b810-9dad-11d1-80b4-00c04fd430c8", "www.example.com")
	require.NoError(t, err)
	require.Equal(t, "2ed6657d-e927-568b-95e1-2665a8aea6a2", id)
	again, err := ids.V5("6ba7b810-9dad-11d1-80b4-00c04fd430c8", "www.example.com")
	require.NoError(t, err)
	require.Equal(t, id, again)
	_, err = ids.V5("not-a-uuid", "event")
	require.Error(t, err)
}
