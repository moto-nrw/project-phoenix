package httpadapter_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	timetableModule "github.com/moto-nrw/project-phoenix/modules/timetable"
	schedulesAPI "github.com/moto-nrw/project-phoenix/modules/timetable/http/activities"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSchedulesErrorRenderer_DateframeNotFound(t *testing.T) {
	t.Parallel()

	err := &careplan.ScheduleError{
		Op:  "GetDateframe",
		Err: schoolcalendar.ErrDateframeNotFound,
	}

	renderer := schedulesAPI.SchedulesErrorRenderer(err)
	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusNotFound, errResp.HTTPStatusCode)
	assertEnvelope(t, errResp, "general.input")
	assert.Contains(t, errResp.Detail, "dateframe not found")
}

func TestSchedulesErrorRenderer_TimeframeNotFound(t *testing.T) {
	t.Parallel()

	err := &careplan.ScheduleError{
		Op:  "GetTimeframe",
		Err: timetableModule.ErrTimeframeNotFound,
	}

	renderer := schedulesAPI.SchedulesErrorRenderer(err)
	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusNotFound, errResp.HTTPStatusCode)
	assertEnvelope(t, errResp, "general.input")
	assert.Contains(t, errResp.Detail, "timeframe not found")
}

func TestSchedulesErrorRenderer_TimeframeCareOfferingConflict(t *testing.T) {
	t.Parallel()

	err := &careplan.ScheduleError{
		Op:  "UpdateTimeframe",
		Err: timetableModule.ErrTimeframeRequiredByCareOffering,
	}

	renderer := schedulesAPI.SchedulesErrorRenderer(err)
	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusConflict, errResp.HTTPStatusCode)
	assert.Contains(t, errResp.Detail, "geändert oder gelöscht")
}

func TestSchedulesErrorRenderer_RecurrenceRuleNotFound(t *testing.T) {
	t.Parallel()

	err := &careplan.ScheduleError{
		Op:  "GetRecurrenceRule",
		Err: timetableModule.ErrRecurrenceRuleNotFound,
	}

	renderer := schedulesAPI.SchedulesErrorRenderer(err)
	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusNotFound, errResp.HTTPStatusCode)
	assertEnvelope(t, errResp, "general.input")
	assert.Contains(t, errResp.Detail, "recurrence rule not found")
}

func TestSchedulesErrorRenderer_InvalidDateRange(t *testing.T) {
	t.Parallel()

	err := &careplan.ScheduleError{
		Op:  "ValidateDateRange",
		Err: timetableModule.ErrInvalidRecurrenceRange,
	}

	renderer := schedulesAPI.SchedulesErrorRenderer(err)
	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusBadRequest, errResp.HTTPStatusCode)
	assertEnvelope(t, errResp, "general.input")
	assert.Contains(t, errResp.Detail, "invalid date range")
}

func TestSchedulesErrorRenderer_InvalidTimeRange(t *testing.T) {
	t.Parallel()

	err := &careplan.ScheduleError{
		Op:  "ValidateTimeRange",
		Err: timetableModule.ErrInvalidTimeRange,
	}

	renderer := schedulesAPI.SchedulesErrorRenderer(err)
	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusBadRequest, errResp.HTTPStatusCode)
	assertEnvelope(t, errResp, "general.input")
	assert.Contains(t, errResp.Detail, "invalid time range")
}

func TestSchedulesErrorRenderer_InvalidDuration(t *testing.T) {
	t.Parallel()

	err := &careplan.ScheduleError{
		Op:  "ValidateDuration",
		Err: timetableModule.ErrInvalidDuration,
	}

	renderer := schedulesAPI.SchedulesErrorRenderer(err)
	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusBadRequest, errResp.HTTPStatusCode)
	assertEnvelope(t, errResp, "general.input")
	assert.Contains(t, errResp.Detail, "invalid duration")
}

func TestSchedulesErrorRenderer_RoomCapacityExceeded(t *testing.T) {
	t.Parallel()

	err := testpkg.CreateTestRoomCapacityError(t)

	renderer := schedulesAPI.SchedulesErrorRenderer(err)
	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok)
	assert.Equal(t, http.StatusConflict, errResp.HTTPStatusCode)
}

func TestSchedulesErrorRenderer_UnknownScheduleError(t *testing.T) {
	t.Parallel()

	// ScheduleError with unknown underlying error should fall to default case
	unknownErr := errors.New("unknown schedule error")
	err := &careplan.ScheduleError{
		Op:  "UnknownOperation",
		Err: unknownErr,
	}

	renderer := schedulesAPI.SchedulesErrorRenderer(err)
	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusInternalServerError, errResp.HTTPStatusCode)
	assertEnvelope(t, errResp, "general.server")
	assert.Contains(t, errResp.Detail, "unknown schedule error")
}

func TestSchedulesErrorRenderer_NonScheduleError(t *testing.T) {
	t.Parallel()

	// Non-ScheduleError should be treated as internal server error
	err := errors.New("some random error")

	renderer := schedulesAPI.SchedulesErrorRenderer(err)
	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusInternalServerError, errResp.HTTPStatusCode)
	assertEnvelope(t, errResp, "general.server")
	assert.Contains(t, errResp.Detail, "some random error")
}

func TestSchedulesErrorRenderer_ScheduleErrorNilUnwrap(t *testing.T) {
	t.Parallel()

	// ScheduleError with nil Err (Unwrap returns nil) should fall to default case
	err := &careplan.ScheduleError{
		Op:  "SomeOperation",
		Err: nil,
	}

	renderer := schedulesAPI.SchedulesErrorRenderer(err)
	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusInternalServerError, errResp.HTTPStatusCode)
	assertEnvelope(t, errResp, "general.server")
}
