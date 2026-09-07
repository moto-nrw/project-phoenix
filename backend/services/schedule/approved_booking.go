package schedule

import "github.com/moto-nrw/project-phoenix/internal/timezone"

// ApprovedBooking is the student booking read by arrival and pickup projections.
type ApprovedBooking struct {
	StudentID int64
	Link      *BookingSelection
}

// BookingSelection contains only the offering and validity needed for planning.
// ValidUntil is exclusive.
type BookingSelection struct {
	CareOfferingID int64
	SelectedDays   []string
	ValidFrom      *timezone.Date
	ValidUntil     *timezone.Date
}
