package careplan

import (
	"context"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Kursanmeldung durch Eltern (#3075, ADR 0012). A course is an AG a family
// reaches through a care offering bound to it, so a course request is an
// ordinary offering change request: the child's current booking plus the
// course. The waiting position is read, never stored.

// Reasons a school has no course requests. Wire-stable identifiers; the
// German copy lives in the parents portal.
const (
	CourseRequestsReasonSchoolOff    = "school_disabled"
	CourseRequestsReasonNoEnrollment = "no_enrollment"
	CourseRequestsReasonNoCourses    = "no_courses"
	CourseRequestsReasonNoPermission = "no_permission"
)

// CourseCatalogItem is one course a family can see: what it is, whether the
// child is in it, and, when it is full, where the child stands.
type CourseCatalogItem struct {
	OfferingID      int64
	ActivityGroupID int64
	Name            string
	Description     string
	// AvailableDays are the canonical day keys the course runs on.
	AvailableDays []string
	// Capacity is the smaller of the AG's Teilnehmergrenze and the offering's
	// capacity. Nil means unlimited, and then FreeSlots is nil too.
	Capacity  *int
	FreeSlots *int
	// Booked marks a course the child already attends.
	Booked bool
	// Requested marks a course the child's open request asks for.
	Requested bool
	// Waitlisted is true for a requested course with no free slot left.
	Waitlisted bool
	// WaitlistPosition is 1-based and only set together with Waitlisted.
	WaitlistPosition int
}

// CourseCatalog is everything the parents portal needs for the Kurse
// section.
type CourseCatalog struct {
	// Enabled is false when the school does not offer parent course requests;
	// DisabledReason then names why, and Items stays empty.
	Enabled        bool
	DisabledReason string
	PhaseName      string
	// EffectiveFrom is the date a new course request would take effect on.
	EffectiveFrom calendar.Date
	// PendingRequestID is the child's open course request, 0 when there is
	// none. PendingSubmittedBySelf says whether the reading guardian may
	// withdraw it.
	PendingRequestID       int64
	PendingSubmittedBySelf bool
	// OtherRequestPending marks an open request that changes care offerings
	// rather than courses; it blocks a new course request.
	OtherRequestPending bool
	CanRequest          bool
	ReasonRequired      bool
	Items               []CourseCatalogItem
}

// CreateCourseRequestInput is one parent asking for one course.
type CreateCourseRequestInput struct {
	StudentID  int64
	AccountID  int64
	OfferingID int64
	Note       string
}

// CourseRequests is the parents-portal course surface.
type CourseRequests interface {
	// CourseCatalog lists the school's courses with the child's state.
	CourseCatalog(ctx context.Context, studentID, accountID int64) (*CourseCatalog, error)
	// CreateCourseRequest stores an ordinary offering change request: the
	// child's current booking plus that course.
	CreateCourseRequest(ctx context.Context, input CreateCourseRequestInput) (*OfferingChangeRequest, error)
	// WithdrawCourseRequest takes back the caller's own open course request.
	WithdrawCourseRequest(ctx context.Context, requestID, accountID, studentID int64) error
}
