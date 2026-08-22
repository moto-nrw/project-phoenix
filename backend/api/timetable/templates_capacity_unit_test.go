package timetable

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTemplateTimingAllowsUnlimitedCapacity(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/timetable/templates", nil)

	timing, ok := parseTemplateTiming(w, r, "08:00", "09:00", nil, nil)

	require.True(t, ok)
	assert.Zero(t, timing.maxParticipants)
}

func TestUpdateTemplateRequestTracksExplicitCapacityClear(t *testing.T) {
	t.Parallel()

	var req updateTemplateRequest
	require.NoError(t, json.Unmarshal([]byte(`{"max_participants":null}`), &req))

	assert.True(t, req.MaxParticipants.Set)
	assert.Nil(t, req.MaxParticipants.Value)
}
