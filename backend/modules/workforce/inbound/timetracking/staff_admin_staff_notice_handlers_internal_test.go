package timetracking

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStaffNoticeToResponseAcknowledgementCountIsAdminOnly(t *testing.T) {
	t.Parallel()
	view := noticeFields{
		Title:             "Räumungsübung",
		ValidFrom:         "2026-08-05",
		AcknowledgedCount: 3,
	}

	teamResponse, err := json.Marshal(toNoticeResponse(view, false))
	require.NoError(t, err)
	assert.NotContains(t, string(teamResponse), "acknowledged_count")

	adminResponse, err := json.Marshal(toNoticeResponse(view, true))
	require.NoError(t, err)
	assert.Contains(t, string(adminResponse), `"acknowledged_count":3`)
}
