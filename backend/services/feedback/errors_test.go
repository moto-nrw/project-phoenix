package feedback

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestEntryNotFoundError_Error(t *testing.T) {
	err := &EntryNotFoundError{EntryID: 123}
	assert.Equal(t, "feedback entry not found: 123", err.Error())
}

func TestEntryNotFoundError_Unwrap(t *testing.T) {
	err := &EntryNotFoundError{EntryID: 123}
	assert.Equal(t, ErrEntryNotFound, err.Unwrap())
	assert.True(t, errors.Is(err, ErrEntryNotFound))
}

func TestInvalidEntryDataError_Error(t *testing.T) {
	originalErr := errors.New("validation failed")
	err := &InvalidEntryDataError{Err: originalErr}
	assert.Equal(t, "invalid feedback entry data: validation failed", err.Error())
}

func TestInvalidEntryDataError_Unwrap(t *testing.T) {
	originalErr := errors.New("validation failed")
	err := &InvalidEntryDataError{Err: originalErr}

	// Should unwrap to ErrInvalidEntryData
	assert.Equal(t, ErrInvalidEntryData, err.Unwrap())
	assert.True(t, errors.Is(err, ErrInvalidEntryData))
}

func TestInvalidDateRangeError_Error(t *testing.T) {
	startDate := time.Date(2026, 1, 20, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2026, 1, 18, 0, 0, 0, 0, time.UTC)

	err := &InvalidDateRangeError{
		StartDate: startDate,
		EndDate:   endDate,
	}

	expected := "invalid date range: 2026-01-20 to 2026-01-18"
	assert.Equal(t, expected, err.Error())
}

func TestInvalidDateRangeError_Unwrap(t *testing.T) {
	err := &InvalidDateRangeError{
		StartDate: time.Now(),
		EndDate:   time.Now(),
	}

	assert.Equal(t, ErrInvalidDateRange, err.Unwrap())
	assert.True(t, errors.Is(err, ErrInvalidDateRange))
}

func TestBatchOperationError_Error(t *testing.T) {
	err := &BatchOperationError{
		Errors: []error{
			errors.New("error 1"),
			errors.New("error 2"),
			errors.New("error 3"),
		},
	}

	assert.Equal(t, "batch operation failed with 3 errors", err.Error())
}

func TestBatchOperationError_AddError(t *testing.T) {
	batchErr := &BatchOperationError{}

	// Initially empty
	assert.Empty(t, batchErr.Errors)
	assert.False(t, batchErr.HasErrors())

	// Add first error
	err1 := errors.New("error 1")
	batchErr.AddError(err1)
	assert.Len(t, batchErr.Errors, 1)
	assert.True(t, batchErr.HasErrors())
	assert.Equal(t, err1, batchErr.Errors[0])

	// Add second error
	err2 := errors.New("error 2")
	batchErr.AddError(err2)
	assert.Len(t, batchErr.Errors, 2)
	assert.True(t, batchErr.HasErrors())
	assert.Equal(t, err2, batchErr.Errors[1])
}

func TestBatchOperationError_HasErrors(t *testing.T) {
	tests := []struct {
		name      string
		errors    []error
		hasErrors bool
	}{
		{
			name:      "no errors",
			errors:    []error{},
			hasErrors: false,
		},
		{
			name:      "nil errors",
			errors:    nil,
			hasErrors: false,
		},
		{
			name: "one error",
			errors: []error{
				errors.New("error 1"),
			},
			hasErrors: true,
		},
		{
			name: "multiple errors",
			errors: []error{
				errors.New("error 1"),
				errors.New("error 2"),
			},
			hasErrors: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			batchErr := &BatchOperationError{Errors: tt.errors}
			assert.Equal(t, tt.hasErrors, batchErr.HasErrors())
		})
	}
}

func TestBatchOperationError_ErrorCount(t *testing.T) {
	batchErr := &BatchOperationError{}

	// Add errors one by one
	for i := 1; i <= 5; i++ {
		batchErr.AddError(fmt.Errorf("error %d", i))
		assert.Equal(t, i, len(batchErr.Errors))
		assert.Equal(t, fmt.Sprintf("batch operation failed with %d errors", i), batchErr.Error())
	}
}
