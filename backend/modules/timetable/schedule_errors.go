package timetable

import "errors"

// Schedule-service failures preserve the existing HTTP error messages.
var (
	// ErrCategoryNotAssignable rejects assigning missing or archived categories.
	ErrCategoryNotAssignable = errors.New("activity category is not available for new assignments")
	// ErrTimeframeRequiredByCareOffering protects linked offerings from incompatible edits or deletion.
	ErrTimeframeRequiredByCareOffering = errors.New("zeitrahmen kann nicht geändert oder gelöscht werden: Ein verknüpftes Betreuungsangebot benötigt ihn")
	ErrInvalidTimeRange                = errors.New("invalid time range")
	ErrInvalidDuration                 = errors.New("invalid duration")
)
