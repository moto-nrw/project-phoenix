package test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWriteCompositionInventoryUsesCanonicalTopLevelOrder(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "composition.json")
	inventory := compositionInventory{}
	seed, err := json.MarshalIndent(inventory, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, append(seed, '\n'), 0o600))
	writeCompositionInventory(t, path, inventory)

	require.Equal(t, []string{
		"commands",
		"constructor_calls",
		"evidence_commit",
		"evidence_constructor_calls",
		"evidence_legacy_callers",
		"legacy_callers",
		"production_roots",
		"runtime_baseline",
		"schema_version",
		"smoke_tests",
		"test_roots",
		"worker_job_ids",
	}, topLevelJSONKeys(t, path))
}

func TestReplaceCompositionFieldsPreservesUnownedFields(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "composition.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"zeta":{"keep":true},"alpha":1}`), 0o600))
	require.NoError(t, ReplaceCompositionFields(path, map[string]any{"alpha": 2}))

	content, err := os.ReadFile(path) // #nosec G304 -- test-owned temporary path
	require.NoError(t, err)
	require.JSONEq(t, `{"alpha":2,"zeta":{"keep":true}}`, string(content))
	require.Equal(t, []string{"alpha", "zeta"}, topLevelJSONKeys(t, path))
}

func TestReplaceCompositionFieldsLeavesInvalidManifestUntouched(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "composition.json")
	const invalidManifest = "{not-json}\n"
	require.NoError(t, os.WriteFile(path, []byte(invalidManifest), 0o600))

	err := ReplaceCompositionFields(path, map[string]any{"alpha": 2})

	require.ErrorContains(t, err, "decode composition manifest")
	content, readErr := os.ReadFile(path) // #nosec G304 -- test-owned temporary path
	require.NoError(t, readErr)
	require.Equal(t, invalidManifest, string(content))
}

func topLevelJSONKeys(t *testing.T, path string) []string {
	t.Helper()
	file, err := os.Open(path) // #nosec G304 -- test-owned temporary path
	require.NoError(t, err)
	defer func() { require.NoError(t, file.Close()) }()
	decoder := json.NewDecoder(file)
	token, err := decoder.Token()
	require.NoError(t, err)
	require.Equal(t, json.Delim('{'), token)
	var keys []string
	for decoder.More() {
		key, tokenErr := decoder.Token()
		require.NoError(t, tokenErr)
		keys = append(keys, key.(string))
		var value json.RawMessage
		require.NoError(t, decoder.Decode(&value))
	}
	return keys
}
