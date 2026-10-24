package careplan

import (
	"context"
	"encoding/json"
	"slices"
	"time"
)

type ReviewBooking struct {
	StudentID    int64
	OfferingID   int64
	SelectedDays []string
	ValidFrom    Date
	ValidUntil   Date
}

type ReviewBookings interface {
	ApprovedForStudents(context.Context, []int64, Date, Date) ([]ReviewBooking, error)
}

type ReviewClassTimes interface {
	ListClassArrivalTimes(context.Context, []string) (map[string]map[string]string, error)
}

type ScheduleReviewDirectory interface {
	FindStudents(context.Context, []int64) (map[int64]ReviewStudent, error)
	PersonNames(context.Context, []int64) (map[int64]PersonName, error)
	ReviewerNames(context.Context, []int64) (map[int64]PersonName, error)
	ListStudentDepartureModes(context.Context, []int64) (map[int64]map[string][]string, error)
}

type PickupReviewImpact struct {
	StudentID        int64
	Date             Date
	From             time.Time
	Enrolled         bool
	AutoExceptionIDs []int64
}

type PickupReviewBlock struct {
	ID        int64
	Title     string
	StartTime time.Time
	EndTime   time.Time
}

type PickupReviewBlocks interface {
	PreviewPickupBlocks(context.Context, PickupReviewImpact) ([]PickupReviewBlock, error)
}

type OfferingReviewDirectory interface {
	FindStudents(context.Context, []int64) (map[int64]ReviewStudent, error)
	PersonNames(context.Context, []int64) (map[int64]PersonName, error)
	ReviewerNames(context.Context, []int64) (map[int64]PersonName, error)
	SearchStudentIDs(context.Context, string, []int64) ([]int64, error)
}

type OfferingReviewChild struct {
	ID          int64
	RequestID   int64
	Grade       *int16
	SchoolClass string
}

type OfferingReviewRequest struct {
	ID      int64
	PhaseID int64
}

type OfferingReviewPhase struct {
	ID            int64
	Start         Date
	End           Date
	SelectionMode string
}

type OfferingReviewBooking struct {
	ChildID       int64
	OfferingID    int64
	SelectedDays  []string
	ManualDays    []string
	AutomaticDays []string
}

type OfferingReviewEnrollment interface {
	Children(context.Context, []int64) ([]OfferingReviewChild, error)
	Requests(context.Context, []int64) ([]OfferingReviewRequest, error)
	Phases(context.Context, []int64) ([]OfferingReviewPhase, error)
	Selections(context.Context, map[int64]Date) ([]OfferingReviewBooking, error)
}

type OfferingReviewCourseRef struct {
	OfferingID    int64
	LegacyGroupID *int64
}

type OfferingReviewCourse struct {
	Active        bool
	Grades        []int
	SchoolClasses []string
}

type OfferingReviewCourses interface {
	CourseGroups(context.Context, []OfferingReviewCourseRef, Date) (map[int64][]OfferingReviewCourse, error)
}

type MasterDataFieldChange struct {
	RequestID int64
	StudentID int64
	Target    string
	Field     string
	OldValue  json.RawMessage
	NewValue  json.RawMessage
}

type MasterDataFieldFacts struct {
	Student              *ReviewStudent
	FirstName            string
	LastName             string
	BulkEligible         bool
	BulkIneligibleReason string
	BulkIneligibleText   string
	CurrentValueChanged  *bool
}

// MasterDataFieldReviews supplies People Directory's baseline and validation
// facts for proposed field changes, keyed by request id.
type MasterDataFieldReviews interface {
	ReviewFields(context.Context, []MasterDataFieldChange) (map[int64]MasterDataFieldFacts, error)
}

// MasterDataDirectory supplies only People Directory facts, never request
// persistence or a decision. Scope remains a separate identity capability.
type MasterDataDirectory interface {
	MasterDataFieldReviews
	FindStudents(context.Context, []int64) (map[int64]ReviewStudent, error)
	PersonNames(context.Context, []int64) (map[int64]PersonName, error)
	ReviewerNames(context.Context, []int64) (map[int64]PersonName, error)
}

// ReviewStudent is the People Directory fact used to scope and name a child.
// EnrolledUntil is empty when no care end is recorded.
type ReviewStudent struct {
	ID            int64
	PersonID      int64
	SchoolClass   string
	GroupID       *int64
	Alumnus       bool
	EnrolledUntil Date
}

// PersonName is the display identity behind a student or reviewer.
type PersonName struct {
	FirstName string
	LastName  string
}

// ReviewScope covers the school or the listed groups.
type ReviewScope struct {
	SchoolWide bool
	GroupIDs   []int64
}

// ReviewScopeResolver resolves review reach from the verified request context.
type ReviewScopeResolver func(ctx context.Context) (ReviewScope, error)

// CareEndedOn reports whether care ended before the given day.
func (s ReviewStudent) CareEndedOn(day Date) bool {
	return !s.EnrolledUntil.IsZero() && day.After(s.EnrolledUntil)
}

// Allows reports whether the scope covers the student; nil is never covered.
func (s ReviewScope) Allows(student *ReviewStudent) bool {
	if student == nil {
		return false
	}
	if s.SchoolWide {
		return true
	}
	return student.GroupID != nil && slices.Contains(s.GroupIDs, *student.GroupID)
}
