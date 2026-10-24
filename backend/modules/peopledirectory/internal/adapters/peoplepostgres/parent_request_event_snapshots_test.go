package users

import (
	"encoding/json"
	"testing"
	"time"

	directory "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/stretchr/testify/require"
)

func TestNativeRequestEventPayloadKeepsLargeNumbersAndNilObjects(t *testing.T) {
	t.Parallel()
	for _, raw := range []json.RawMessage{nil, []byte(`null`), []byte(`{}`), []byte(`{"large":9007199254740993,"nested":{"n":9223372036854775807},"nil":null,"empty":[]}`)} {
		row := &directory.ParentRequestEventSnapshot{Payload: raw, StudentID: 23, RequestID: 29, RequestType: "master_data", EventType: "submitted", Version: "frozen"}
		row.ID, row.TenantID = 31, 37
		row.CreatedAt = time.Date(2026, time.October, 17, 1, 2, 3, 4, time.UTC)
		row.UpdatedAt = row.CreatedAt
		row.ActorAccountID = &row.ID
		retained, err := retainedParentRequestEvent(row)
		require.NoError(t, err)
		if raw == nil || string(raw) == "null" {
			require.Nil(t, retained.Payload)
		} else {
			require.NotNil(t, retained.Payload)
		}
		if len(retained.Payload) > 0 {
			require.Equal(t, json.Number("9007199254740993"), retained.Payload["large"])
		}
		got, err := nativeParentRequestEvent(retained)
		require.NoError(t, err)
		expected := *row
		if string(raw) == "null" {
			expected.Payload = nil
		}
		require.Equal(t, expected.ID, got.ID)
		require.Equal(t, expected.TenantID, got.TenantID)
		require.Equal(t, expected.CreatedAt, got.CreatedAt)
		require.Equal(t, expected.UpdatedAt, got.UpdatedAt)
		require.Equal(t, expected.ActorAccountID, got.ActorAccountID)
		require.Equal(t, expected.Version, got.Version)
		if expected.Payload == nil {
			require.Nil(t, got.Payload)
		} else {
			require.JSONEq(t, string(expected.Payload), string(got.Payload))
		}
	}
}
func TestNativeRequestEventPayloadRejectsTrailingJSON(t *testing.T) {
	t.Parallel()
	_, err := retainedParentRequestEvent(&directory.ParentRequestEventSnapshot{Payload: []byte(`{} {}`)})
	require.Error(t, err)
}
