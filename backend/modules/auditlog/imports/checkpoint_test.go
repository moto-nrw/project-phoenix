package imports

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckpointAttachmentValidatesAndDetachesPayload(t *testing.T) {
	t.Parallel()
	payload := json.RawMessage(`{"created":1}`)
	checkpoint := ImportCheckpoint{ImportKey: strings.Repeat("a", 64), LastRow: 1, TotalRows: 2, Payload: payload}
	var record DataImport
	require.NoError(t, record.AttachImportCheckpoint(checkpoint))
	payload[0] = '['
	require.JSONEq(t, `{"created":1}`, string(record.Checkpoint.Payload), "the persisted receipt must not share the caller's mutable payload")

	for _, invalid := range []ImportCheckpoint{
		{ImportKey: "invalid", LastRow: 1, TotalRows: 2, Payload: json.RawMessage(`{}`)},
		{ImportKey: strings.Repeat("a", 64), LastRow: 3, TotalRows: 2, Payload: json.RawMessage(`{}`)},
		{ImportKey: strings.Repeat("a", 64), LastRow: 1, TotalRows: 2, Payload: json.RawMessage(`invalid`)},
	} {
		require.Error(t, record.AttachImportCheckpoint(invalid))
		require.Equal(t, 1, record.Checkpoint.LastRow, "an invalid receipt must leave the previous attachment intact")
		require.JSONEq(t, `{"created":1}`, string(record.Checkpoint.Payload))
	}
}
