package schedules

import (
	"errors"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/common"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrorInvalidRequest(t *testing.T) {
	err := errors.New("invalid input")
	renderer := ErrorInvalidRequest(err)

	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusBadRequest, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.Status)
	assert.Contains(t, errResp.ErrorText, "invalid input")
}

func TestErrorInternalServer(t *testing.T) {
	err := errors.New("internal failure")
	renderer := ErrorInternalServer(err)

	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusInternalServerError, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.Status)
	assert.Contains(t, errResp.ErrorText, "internal failure")
}

func TestErrorNotFound(t *testing.T) {
	err := errors.New("resource missing")
	renderer := ErrorNotFound(err)

	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusNotFound, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.Status)
	assert.Contains(t, errResp.ErrorText, "resource missing")
}

func TestErrorRenderer_DateframeNotFound(t *testing.T) {
	err := &scheduleSvc.ScheduleError{
		Op:  "GetDateframe",
		Err: scheduleSvc.ErrDateframeNotFound,
	}

	renderer := ErrorRenderer(err)
	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusNotFound, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.Status)
	assert.Contains(t, errResp.ErrorText, "dateframe not found")
}

func TestErrorRenderer_TimeframeNotFound(t *testing.T) {
	err := &scheduleSvc.ScheduleError{
		Op:  "GetTimeframe",
		Err: scheduleSvc.ErrTimeframeNotFound,
	}

	renderer := ErrorRenderer(err)
	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusNotFound, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.Status)
	assert.Contains(t, errResp.ErrorText, "timeframe not found")
}

func TestErrorRenderer_RecurrenceRuleNotFound(t *testing.T) {
	err := &scheduleSvc.ScheduleError{
		Op:  "GetRecurrenceRule",
		Err: scheduleSvc.ErrRecurrenceRuleNotFound,
	}

	renderer := ErrorRenderer(err)
	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusNotFound, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.Status)
	assert.Contains(t, errResp.ErrorText, "recurrence rule not found")
}

func TestErrorRenderer_InvalidDateRange(t *testing.T) {
	err := &scheduleSvc.ScheduleError{
		Op:  "ValidateDateRange",
		Err: scheduleSvc.ErrInvalidDateRange,
	}

	renderer := ErrorRenderer(err)
	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusBadRequest, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.Status)
	assert.Contains(t, errResp.ErrorText, "invalid date range")
}

func TestErrorRenderer_InvalidTimeRange(t *testing.T) {
	err := &scheduleSvc.ScheduleError{
		Op:  "ValidateTimeRange",
		Err: scheduleSvc.ErrInvalidTimeRange,
	}

	renderer := ErrorRenderer(err)
	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusBadRequest, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.Status)
	assert.Contains(t, errResp.ErrorText, "invalid time range")
}

func TestErrorRenderer_InvalidDuration(t *testing.T) {
	err := &scheduleSvc.ScheduleError{
		Op:  "ValidateDuration",
		Err: scheduleSvc.ErrInvalidDuration,
	}

	renderer := ErrorRenderer(err)
	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusBadRequest, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.Status)
	assert.Contains(t, errResp.ErrorText, "invalid duration")
}

func TestErrorRenderer_UnknownScheduleError(t *testing.T) {
	// ScheduleError with unknown underlying error should fall to default case
	unknownErr := errors.New("unknown schedule error")
	err := &scheduleSvc.ScheduleError{
		Op:  "UnknownOperation",
		Err: unknownErr,
	}

	renderer := ErrorRenderer(err)
	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusInternalServerError, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.Status)
	assert.Contains(t, errResp.ErrorText, "unknown schedule error")
}

func TestErrorRenderer_NonScheduleError(t *testing.T) {
	// Non-ScheduleError should be treated as internal server error
	err := errors.New("some random error")

	renderer := ErrorRenderer(err)
	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusInternalServerError, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.Status)
	assert.Contains(t, errResp.ErrorText, "some random error")
}

func TestErrorRenderer_ScheduleErrorNilUnwrap(t *testing.T) {
	// ScheduleError with nil Err (Unwrap returns nil) should fall to default case
	err := &scheduleSvc.ScheduleError{
		Op:  "SomeOperation",
		Err: nil,
	}

	renderer := ErrorRenderer(err)
	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusInternalServerError, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.Status)
}
