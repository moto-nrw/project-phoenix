package staff

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
)

func TestStaffNoticeToResponseAcknowledgementCountIsAdminOnly(t *testing.T) {
	t.Parallel()
	view := &usersModels.StaffNoticeView{
		StaffNotice: &usersModels.StaffNotice{
			Title:     "Räumungsübung",
			ValidFrom: timezone.NewDate(2026, 8, 5),
		},
		AcknowledgedCount: 3,
	}

	teamResponse, err := json.Marshal(toNoticeResponse(view, false))
	require.NoError(t, err)
	assert.NotContains(t, string(teamResponse), "acknowledged_count")

	adminResponse, err := json.Marshal(toNoticeResponse(view, true))
	require.NoError(t, err)
	assert.Contains(t, string(adminResponse), `"acknowledged_count":3`)
}
