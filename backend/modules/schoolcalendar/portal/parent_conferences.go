package portal

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Elternsprechtag (#4801). A Lehrkraft offers a day for one of the classes
// she teaches (education.class_teachers); the families of the class book one
// slot per child. Each booking shows as an appointment in the parents'
// calendar and in the Lehrkraft's calendar in moto schule, and the existing
// appointment mail and push tell the families about booking, move and
// cancellation.

// ParentConferenceChangeWindow is how long before a slot families may still
// book, move or cancel it.
const ParentConferenceChangeWindow = 24 * time.Hour

// NotificationTypeParentConferenceBooking is the moto schule notice to the
// Lehrkraft when a family books, moves or cancels a slot. Delivery owns the
// catalogue entry under the same key.
const NotificationTypeParentConferenceBooking = "school_parent_conference_booking"

var (
	// ErrParentConferenceSlotTaken: another child holds the slot.
	ErrParentConferenceSlotTaken = errors.New("parent conference slot already booked")
	// ErrParentConferenceChangeClosed: the slot starts in less than 24 hours.
	ErrParentConferenceChangeClosed = errors.New("parent conference slot can no longer be changed")
)

// SchoolParentConferences is the Lehrkraft's side in moto schule: her days,
// their occupancy and the classes she may offer one for.
type SchoolParentConferences interface {
	ListSchoolParentConferences(ctx context.Context) (*SchoolParentConferenceList, error)
	GetSchoolParentConference(ctx context.Context, id int64) (*ParentConferenceView, error)
	CreateSchoolParentConference(ctx context.Context, req ParentConferenceRequest) (*ParentConferenceView, error)
	UpdateSchoolParentConference(ctx context.Context, id int64, req ParentConferenceRequest) (*ParentConferenceView, error)
	DeleteSchoolParentConference(ctx context.Context, id int64) error
}

// ParentConferenceBookings is the families' side. The caller has already
// resolved the child through its guardian relationship with the permission
// the action needs, and runs the call in the transaction of the child's
// school; Child names the school, the child and the guardian.
type ParentConferenceBookings interface {
	ListChildParentConferences(ctx context.Context, child ParentConferenceChild) ([]ParentConferenceOffer, error)
	// BookParentConference books the slot or moves the child's booking there.
	BookParentConference(ctx context.Context, child ParentConferenceChild, conferenceID int64, startTime string) (*ParentConferenceOffer, error)
	CancelParentConferenceBooking(ctx context.Context, child ParentConferenceChild, conferenceID int64) error
}

// ParentConferenceChild is a child whose guardian relationship the caller
// checked.
type ParentConferenceChild struct {
	TenantID          int64
	StudentID         int64
	GuardianProfileID int64
}

// ParentConferenceRequest is a day as the Lehrkraft enters it. Times are
// "HH:MM" in the school's wall clock.
type ParentConferenceRequest struct {
	SchoolClass string                   `json:"school_class"`
	Date        calendar.Date            `json:"date"`
	StartTime   string                   `json:"start_time"`
	EndTime     string                   `json:"end_time"`
	SlotMinutes int                      `json:"slot_minutes"`
	Breaks      []ParentConferenceWindow `json:"breaks"`
	Location    *string                  `json:"location,omitempty"`
}

// ParentConferenceWindow is a span of the day, "HH:MM" to "HH:MM".
type ParentConferenceWindow struct {
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
}

// SchoolParentConferenceList is the Lehrkraft's days and the classes she
// may offer a new one for.
type SchoolParentConferenceList struct {
	Conferences []ParentConferenceView `json:"conferences"`
	Classes     []string               `json:"classes"`
}

// ParentConferenceView is one day with its occupancy, as the Lehrkraft sees
// it.
type ParentConferenceView struct {
	ID          string                   `json:"id"`
	SchoolClass string                   `json:"school_class"`
	Date        calendar.Date            `json:"date"`
	StartTime   string                   `json:"start_time"`
	EndTime     string                   `json:"end_time"`
	SlotMinutes int                      `json:"slot_minutes"`
	Breaks      []ParentConferenceWindow `json:"breaks"`
	Location    *string                  `json:"location,omitempty"`
	BookedCount int                      `json:"booked_count"`
	Slots       []ParentConferenceSlot   `json:"slots"`
}

// ParentConferenceSlot is one slot of a day. StudentName is set for a
// booked slot in the Lehrkraft's view only.
type ParentConferenceSlot struct {
	StartTime   string  `json:"start_time"`
	EndTime     string  `json:"end_time"`
	Booked      bool    `json:"booked"`
	StudentID   *string `json:"student_id,omitempty"`
	StudentName *string `json:"student_name,omitempty"`
}

// ParentConferenceOffer is one day as a family sees it for one child: the
// free slots, the child's own booking and whether it can still change.
type ParentConferenceOffer struct {
	ID          string                   `json:"id"`
	SchoolClass string                   `json:"school_class"`
	TeacherName string                   `json:"teacher_name"`
	Date        calendar.Date            `json:"date"`
	Location    *string                  `json:"location,omitempty"`
	FreeSlots   []ParentConferenceWindow `json:"free_slots"`
	Booking     *ParentConferenceWindow  `json:"booking,omitempty"`
	// CanChange is false when the child's booking starts in less than 24
	// hours: then only the school can still change it.
	CanChange bool `json:"can_change"`
}
