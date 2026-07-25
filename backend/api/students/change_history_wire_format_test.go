package students

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChangeHistoryEntry_IDIsSerializedAsString(t *testing.T) {
	payload, err := json.Marshal(changeHistoryEntry{ID: "9007199254740993"})
	require.NoError(t, err)

	assert.JSONEq(t, `{"id":"9007199254740993","field":"","old_value":"","new_value":"","edited_by":"","changed_at":"0001-01-01T00:00:00Z"}`, string(payload))
}
