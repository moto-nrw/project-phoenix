package schedule

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScheduleError_Error_WithNilErr(t *testing.T) {
	err := &ScheduleError{
		Op:  "CreateDateframe",
		Err: nil,
	}

	expected := "schedule error during CreateDateframe"
	assert.Equal(t, expected, err.Error())
}

func TestScheduleError_Error_WithErr(t *testing.T) {
	originalErr := errors.New("database transaction failed")
	err := &ScheduleError{
		Op:  "CreateDateframe",
		Err: originalErr,
	}

	expected := "schedule error during CreateDateframe: database transaction failed"
	assert.Equal(t, expected, err.Error())
}

func TestScheduleError_Error_WithStandardError(t *testing.T) {
	err := &ScheduleError{
		Op:  "GetDateframe",
		Err: ErrDateframeNotFound,
	}

	expected := "schedule error during GetDateframe: dateframe not found"
	assert.Equal(t, expected, err.Error())
}

func TestScheduleError_Unwrap(t *testing.T) {
	originalErr := errors.New("underlying error")
	err := &ScheduleError{
		Op:  "UpdateTimeframe",
		Err: originalErr,
	}

	assert.Equal(t, originalErr, err.Unwrap())
}

func TestScheduleError_Unwrap_Nil(t *testing.T) {
	err := &ScheduleError{
		Op:  "DeleteRecurrenceRule",
		Err: nil,
	}

	assert.Nil(t, err.Unwrap())
}

func TestScheduleError_ErrorsIs(t *testing.T) {
	// Test that errors.Is works correctly with wrapped errors
	err := &ScheduleError{
		Op:  "FindTimeframe",
		Err: ErrTimeframeNotFound,
	}

	assert.True(t, errors.Is(err, ErrTimeframeNotFound))
	assert.False(t, errors.Is(err, ErrDateframeNotFound))
}

func TestScheduleError_ChainedWrapping(t *testing.T) {
	// Test multiple levels of wrapping
	baseErr := errors.New("validation failed")
	wrapped1 := &ScheduleError{
		Op:  "ValidateDateRange",
		Err: baseErr,
	}
	wrapped2 := &ScheduleError{
		Op:  "CreateSchedule",
		Err: wrapped1,
	}

	// Should unwrap through the chain
	assert.True(t, errors.Is(wrapped2, baseErr))
	assert.Contains(t, wrapped2.Error(), "CreateSchedule")
	assert.Contains(t, wrapped2.Error(), "ValidateDateRange")
}

func TestScheduleError_AllOperations(t *testing.T) {
	tests := []struct {
		name string
		op   string
		err  error
		want string
	}{
		{
			name: "dateframe not found",
			op:   "GetDateframe",
			err:  ErrDateframeNotFound,
			want: "schedule error during GetDateframe: dateframe not found",
		},
		{
			name: "timeframe not found",
			op:   "GetTimeframe",
			err:  ErrTimeframeNotFound,
			want: "schedule error during GetTimeframe: timeframe not found",
		},
		{
			name: "recurrence rule not found",
			op:   "GetRecurrenceRule",
			err:  ErrRecurrenceRuleNotFound,
			want: "schedule error during GetRecurrenceRule: recurrence rule not found",
		},
		{
			name: "invalid date range",
			op:   "CreateDateframe",
			err:  ErrInvalidDateRange,
			want: "schedule error during CreateDateframe: invalid date range",
		},
		{
			name: "invalid time range",
			op:   "CreateTimeframe",
			err:  ErrInvalidTimeRange,
			want: "schedule error during CreateTimeframe: invalid time range",
		},
		{
			name: "invalid duration",
			op:   "CalculateDuration",
			err:  ErrInvalidDuration,
			want: "schedule error during CalculateDuration: invalid duration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &ScheduleError{
				Op:  tt.op,
				Err: tt.err,
			}
			assert.Equal(t, tt.want, err.Error())
		})
	}
}

func TestScheduleError_DifferentOperationTypes(t *testing.T) {
	tests := []struct {
		name string
		op   string
	}{
		{name: "create", op: "CreateDateframe"},
		{name: "read", op: "GetDateframe"},
		{name: "update", op: "UpdateDateframe"},
		{name: "delete", op: "DeleteDateframe"},
		{name: "list", op: "ListDateframes"},
		{name: "validate", op: "ValidateDateRange"},
		{name: "calculate", op: "CalculateDuration"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &ScheduleError{
				Op:  tt.op,
				Err: nil,
			}
			assert.Contains(t, err.Error(), tt.op)
		})
	}
}
