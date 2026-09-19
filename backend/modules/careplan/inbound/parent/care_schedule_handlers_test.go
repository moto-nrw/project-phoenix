package parent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/careplan/legacy/careschedule"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal/legacy"
	"github.com/stretchr/testify/assert"
)

func TestToCareScheduleResponseIncludesResolvedFieldCapabilities(t *testing.T) {
	t.Parallel()

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

func TestToCareScheduleResponseIncludesCareDayStatus(t *testing.T) {
	t.Parallel()

	response := toCareScheduleResponse(&parentService.ChildCareSchedule{
		Weekdays: []parentService.CareScheduleWeekday{
			{Weekday: 1, Status: careschedule.CareDayScheduled},
			{Weekday: 2, Status: careschedule.CareDayNotScheduled},
		},
	})

	assert.Equal(t, "scheduled", response.Weekdays[0].Status)
	assert.Equal(t, "not_scheduled", response.Weekdays[1].Status)
}

func TestRenderParentWriteErrorMapsDisabledCareFieldToForbiddenCode(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/me/children/1/care-schedule/requests", nil)
	request = request.WithContext(context.Background())

	renderParentWriteError(recorder, request, parentService.ErrCareRequestFieldDisabled)

	assert.Equal(t, http.StatusForbidden, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"code":"care_request_field_disabled"`)
}

func TestRenderParentWriteErrorMapsBookingLedCareToForbiddenCode(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/me/children/1/care-schedule/requests", nil)
	request = request.WithContext(context.Background())

	renderParentWriteError(recorder, request, parentService.ErrCareRequestBookingsAuthoritative)

	assert.Equal(t, http.StatusForbidden, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"code":"care_request_bookings_authoritative"`)
}

func TestRenderParentWriteErrorMapsAlreadyLeftToConflictCode(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/me/children/1/care-exception", nil)

	renderParentWriteError(recorder, request, parentService.ErrCareExceptionAlreadyLeft)

	assert.Equal(t, http.StatusConflict, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"code":"care_exception_already_left"`)
}

func TestRenderParentWriteErrorMapsPickupChangeCutoffToConflictCode(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/me/children/1/care-exception", nil)

	renderParentWriteError(recorder, request, parentService.ErrPickupChangeCutoffPassed)

	assert.Equal(t, http.StatusConflict, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"code":"pickup_change_cutoff_passed"`)
}
