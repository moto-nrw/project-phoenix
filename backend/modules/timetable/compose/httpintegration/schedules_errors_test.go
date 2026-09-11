package httpintegration_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/common"
	schedulesAPI "github.com/moto-nrw/project-phoenix/modules/timetable/compose/httpadapter"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSchedulesErrorRenderer_DateframeNotFound(t *testing.T) {
	t.Parallel()

	err := &scheduleSvc.ScheduleError{
		Op:  "GetDateframe",
		Err: scheduleSvc.ErrDateframeNotFound,
	}

	renderer := schedulesAPI.SchedulesErrorRenderer(err)
	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusNotFound, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.Status)
	assert.Contains(t, errResp.ErrorText, "dateframe not found")
}

func TestSchedulesErrorRenderer_TimeframeNotFound(t *testing.T) {
	t.Parallel()

	err := &scheduleSvc.ScheduleError{
		Op:  "GetTimeframe",
		Err: scheduleSvc.ErrTimeframeNotFound,
	}

	renderer := schedulesAPI.SchedulesErrorRenderer(err)
	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusNotFound, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.Status)
	assert.Contains(t, errResp.ErrorText, "timeframe not found")
}

func TestSchedulesErrorRenderer_TimeframeCareOfferingConflict(t *testing.T) {
	t.Parallel()

	err := &scheduleSvc.ScheduleError{
		Op:  "UpdateTimeframe",
		Err: scheduleSvc.ErrTimeframeRequiredByCareOffering,
	}

	renderer := schedulesAPI.SchedulesErrorRenderer(err)
	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusConflict, errResp.HTTPStatusCode)
	assert.Contains(t, errResp.ErrorText, "geändert oder gelöscht")
}

func TestSchedulesErrorRenderer_RecurrenceRuleNotFound(t *testing.T) {
	t.Parallel()

	err := &scheduleSvc.ScheduleError{
		Op:  "GetRecurrenceRule",
		Err: scheduleSvc.ErrRecurrenceRuleNotFound,
	}

	renderer := schedulesAPI.SchedulesErrorRenderer(err)
	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusNotFound, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.Status)
	assert.Contains(t, errResp.ErrorText, "recurrence rule not found")
}

func TestSchedulesErrorRenderer_InvalidDateRange(t *testing.T) {
	t.Parallel()

	err := &scheduleSvc.ScheduleError{
		Op:  "ValidateDateRange",
		Err: scheduleSvc.ErrInvalidDateRange,
	}

	renderer := schedulesAPI.SchedulesErrorRenderer(err)
	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusBadRequest, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.Status)
	assert.Contains(t, errResp.ErrorText, "invalid date range")
}

func TestSchedulesErrorRenderer_InvalidTimeRange(t *testing.T) {
	t.Parallel()

	err := &scheduleSvc.ScheduleError{
		Op:  "ValidateTimeRange",
		Err: scheduleSvc.ErrInvalidTimeRange,
	}

	renderer := schedulesAPI.SchedulesErrorRenderer(err)
	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusBadRequest, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.Status)
	assert.Contains(t, errResp.ErrorText, "invalid time range")
}

func TestSchedulesErrorRenderer_InvalidDuration(t *testing.T) {
	t.Parallel()

	err := &scheduleSvc.ScheduleError{
		Op:  "ValidateDuration",
		Err: scheduleSvc.ErrInvalidDuration,
	}

	renderer := schedulesAPI.SchedulesErrorRenderer(err)
	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusBadRequest, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.Status)
	assert.Contains(t, errResp.ErrorText, "invalid duration")
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
	err := &scheduleSvc.ScheduleError{
		Op:  "UnknownOperation",
		Err: unknownErr,
	}

	renderer := schedulesAPI.SchedulesErrorRenderer(err)
	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusInternalServerError, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.Status)
	assert.Contains(t, errResp.ErrorText, "unknown schedule error")
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
	assert.Equal(t, "error", errResp.Status)
	assert.Contains(t, errResp.ErrorText, "some random error")
}

func TestSchedulesErrorRenderer_ScheduleErrorNilUnwrap(t *testing.T) {
	t.Parallel()

	// ScheduleError with nil Err (Unwrap returns nil) should fall to default case
	err := &scheduleSvc.ScheduleError{
		Op:  "SomeOperation",
		Err: nil,
	}

	renderer := schedulesAPI.SchedulesErrorRenderer(err)
	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusInternalServerError, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.Status)
}
