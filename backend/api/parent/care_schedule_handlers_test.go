package parent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	parentService "github.com/moto-nrw/project-phoenix/services/parent"
	"github.com/stretchr/testify/assert"
)

func TestToCareScheduleResponseIncludesResolvedFieldCapabilities(t *testing.T) {
	response := toCareScheduleResponse(&parentService.ChildCareSchedule{
		CanRequest: true,
		RequestCapabilities: parentService.CareScheduleRequestCapabilities{
			Arrival:       true,
			Pickup:        false,
			DepartureMode: true,
		},
	})

	assert.True(t, response.CanRequest)
	assert.Equal(t, CareScheduleRequestCapabilitiesResponse{
		Arrival: true, Pickup: false, DepartureMode: true,
	}, response.RequestCapabilities)
}

func TestRenderParentWriteErrorMapsDisabledCareFieldToForbiddenCode(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/me/children/1/care-schedule/requests", nil)
	request = request.WithContext(context.Background())

	renderParentWriteError(recorder, request, parentService.ErrCareRequestFieldDisabled)

	assert.Equal(t, http.StatusForbidden, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"code":"care_request_field_disabled"`)
}
