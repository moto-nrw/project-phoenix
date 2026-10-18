package test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// FixtureJSON encodes native JSON fixture fields without a retained model.
func FixtureJSON(t testing.TB, value any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(value)
	require.NoError(t, err)
	return raw
}

// FixtureJSONObject reads the stored values without interpreting them.
func FixtureJSONObject(t testing.TB, raw json.RawMessage) map[string]any {
	t.Helper()
	var values map[string]any
	require.NoError(t, json.Unmarshal(raw, &values))
	return values
}
