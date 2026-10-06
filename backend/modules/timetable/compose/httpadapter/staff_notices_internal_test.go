package httpadapter

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
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

// Eine Kenntnisnahme aus einer veralteten Ansicht trägt ihren eigenen Code;
// der Status bleibt 400 (#2517).
func TestRenderNoticeServiceErrorCodesAnOutdatedView(t *testing.T) {
	t.Parallel()

	rr := httptest.NewRecorder()
	renderNoticeServiceError(rr, httptest.NewRequest(http.MethodPost, "/1/acknowledge", nil),
		fmt.Errorf("%w: notice does not apply today", timetable.ErrStaffNoticeOutdated))

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), `"code":"`+common.CodeCommunicationStaffNoticeOutdated+`"`)
}
