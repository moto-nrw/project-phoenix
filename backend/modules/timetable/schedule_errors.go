package timetable

import "errors"

// Schedule-service failures preserve the existing HTTP error messages.
var (
	// ErrCategoryNotAssignable rejects assigning missing or archived categories.
	ErrCategoryNotAssignable = errors.New("activity category is not available for new assignments")
	// ErrTimeframeRequiredByCareOffering protects linked offerings from incompatible edits or deletion.
	ErrTimeframeRequiredByCareOffering = errors.New("Zeitrahmen kann nicht geändert oder gelöscht werden: Ein verknüpftes Betreuungsangebot benötigt ihn") //nolint:staticcheck // ST1005: stable user-facing contract
	ErrInvalidTimeRange                = errors.New("invalid time range")
	ErrInvalidDuration                 = errors.New("invalid duration")
)
