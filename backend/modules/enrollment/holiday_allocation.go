package enrollment

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Allocation of holiday places by criteria (#4700, decision of 09.11.,
// model A). A period gives its places by arrival (#4699) or by criteria.
// By criteria no booking gets a place when the parents send the form: it is
// pending. The Träger names its criteria with their priority; the system
// sorts the pending bookings of each week by them and proposes the first
// ones for the free places, the Träger-Büro decides, one by one or by
// taking the proposal, and gives a reason for every decision that deviates
// from it. Nothing is decided automatically (Art. 22 GDPR). A decision puts
// the booking in its place or on the waiting list; from there the waiting
// list and moving up of #4699 hold, the waiting list sorted by the criteria
// before the arrival. While a week has a pending booking nobody moves up
// into it: its free places wait for the decision.
//
// A criterion is a yes or no question to the family or about each child.
// Only the answers it needs are asked (data minimisation); a criterion
// added after a registration stays unknown for it, which counts as not met
// and shows as such. The Büro may check an answer, which then counts
// instead of the parents' statement. Answers and decisions are read only by
// the Träger-Büro with the right to manage registrations, never by the
// parents of another family, the host school or an export.

// Allocation modes of a period.
const (
	HolidayAllocationArrival  = "arrival"
	HolidayAllocationCriteria = "criteria"
)

// Scopes of a criterion: one answer per registration or one per child.
const (
	HolidayCriterionFamily = "family"
	HolidayCriterionChild  = "child"
)

// Values of an answer to a criterion.
const (
	HolidayCriterionYes     = "yes"
	HolidayCriterionNo      = "no"
	HolidayCriterionUnknown = "unknown"
)

// Sources of the value that counts.
const (
	HolidayCriterionByParent = "parent"
	HolidayCriterionByOffice = "office"
)

// HolidayWishCriterionID names the holiday care wish of the online
// registration (#4703) among the criteria of a period that takes it into
// account: it comes before the criteria of the Träger, as one criterion of
// the allocation, and is no row of the Träger's list.
const (
	HolidayWishCriterionID   int64 = 0
	HolidayWishCriterionName       = "Ferienwunsch aus der OGS-Anmeldung"
)

// Limits of the criteria and the decisions.
const (
	HolidayCriteriaMax             = 10
	HolidayCriterionNameMaxLength  = 60
	HolidayCriterionQuestionMaxLen = 300
	HolidayAllocationReasonMaxLen  = 500
	HolidayAllocationChoicesMax    = 500
)

// Actions of the audit trail of the allocation: a decision and a check of a
// registration, a read of the allocation list of a period.
const (
	HolidayAuditAllocate      = "allocate"
	HolidayAuditAllocation    = "allocation"
	HolidayAuditCriteriaCheck = "criteria_check"
)

var (
	// ErrHolidayAllocationModeLocked reports a change of the allocation mode
	// of a period that has a registration.
	ErrHolidayAllocationModeLocked = errors.New("the allocation mode of a holiday period with registrations cannot change")
	// ErrHolidayAllocationPending reports a confirmation of a registration
	// with a booking the Büro has not decided on.
	ErrHolidayAllocationPending = errors.New("the holiday registration has bookings without a decision on their place")
	// ErrHolidayAllocationOff reports an allocation in a period that gives
	// its places by arrival.
	ErrHolidayAllocationOff = errors.New("the holiday period gives its places by arrival")
	// ErrHolidayAllocationChanged reports a decision on a booking that is no
	// longer pending or whose proposal changed since it was read.
	ErrHolidayAllocationChanged = errors.New("the holiday allocation changed since it was read")
	// ErrHolidayAllocationFull reports a place given in a week without a
	// free one.
	ErrHolidayAllocationFull = errors.New("the holiday week has no free place")
	// ErrHolidayAllocationReasonRequired reports a decision against the
	// proposal without a reason.
	ErrHolidayAllocationReasonRequired = errors.New("a decision against the proposal needs a reason")
)

// HolidayAllocationFullRefusal names the week without a free place. It
// unwraps to ErrHolidayAllocationFull.
type HolidayAllocationFullRefusal struct{ WeekID int64 }

func (r *HolidayAllocationFullRefusal) Error() string {
	return fmt.Sprintf("week %d: %v", r.WeekID, ErrHolidayAllocationFull)
}

func (r *HolidayAllocationFullRefusal) Unwrap() error { return ErrHolidayAllocationFull }

// HolidayAllocationReasonRefusal names the decision, by its index in the
// request, that deviates from the proposal without a reason. It unwraps to
// ErrHolidayAllocationReasonRequired.
type HolidayAllocationReasonRefusal struct{ Index int }

func (r *HolidayAllocationReasonRefusal) Error() string {
	return fmt.Sprintf("decision %d: %v", r.Index, ErrHolidayAllocationReasonRequired)
}

func (r *HolidayAllocationReasonRefusal) Unwrap() error { return ErrHolidayAllocationReasonRequired }

// HolidayCriterion is a criterion of the Träger. Position is its priority,
// 1 first; Answered reports answers to it, which removing keeps.
type HolidayCriterion struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Question string `json:"question"`
	Scope    string `json:"scope"`
	Active   bool   `json:"active"`
	Position int    `json:"position"`
	Answered bool   `json:"answered"`
}

// HolidayCriteriaCatalog are the criteria of the Träger by priority and the
// periods that allocate by them: a change of the criteria changes the order
// of their pending bookings and waiting lists at once.
type HolidayCriteriaCatalog struct {
	Criteria []HolidayCriterion       `json:"criteria"`
	Periods  []HolidayCriterionPeriod `json:"periods"`
}

// HolidayCriterionPeriod names a period that allocates by criteria.
type HolidayCriterionPeriod struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// HolidayCriterionInput is a criterion as the Träger-Büro writes it; a zero
// ID adds one. The order of the list is the priority.
type HolidayCriterionInput struct {
	ID       int64
	Name     string
	Question string
	Scope    string
	Active   bool
}

// HolidayCriterionQuestion is an active criterion as the form asks it.
type HolidayCriterionQuestion struct {
	ID       int64  `json:"id"`
	Question string `json:"question"`
	Scope    string `json:"scope"`
}

// HolidayCriterionRef names an active criterion of the allocation list.
type HolidayCriterionRef struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Scope    string `json:"scope"`
	Position int    `json:"position"`
}

// HolidayCriterionValue is the value that counts of one criterion for one
// booking, and who gave it.
type HolidayCriterionValue struct {
	CriterionID int64  `json:"criterion_id"`
	Value       string `json:"value"`
	Source      string `json:"source"`
}

// HolidayAllocationCandidate is a pending booking of a week in the order of
// the proposal. Proposal is HolidayPlacementPlaced for the bookings that
// fit the free places in this order, HolidayPlacementWaitlisted else.
type HolidayAllocationCandidate struct {
	BookingID      int64                   `json:"booking_id"`
	Rank           int                     `json:"rank"`
	RegistrationID int64                   `json:"registration_id"`
	SubmittedAt    time.Time               `json:"submitted_at"`
	ChildID        int64                   `json:"child_id"`
	ChildFirstName string                  `json:"child_first_name"`
	ChildLastName  string                  `json:"child_last_name"`
	TimeModelName  string                  `json:"time_model_name"`
	Days           []calendar.Date         `json:"days,omitempty"`
	Criteria       []HolidayCriterionValue `json:"criteria"`
	Proposal       string                  `json:"proposal"`
}

// HolidayAllocationWeek is a week of the allocation list. Free is the
// number of free places, of the fullest day for a week by days; nil is
// unlimited.
type HolidayAllocationWeek struct {
	WeekID       int64                        `json:"week_id"`
	StartsOn     calendar.Date                `json:"starts_on"`
	EndsOn       calendar.Date                `json:"ends_on"`
	HostSchoolID int64                        `json:"host_school_id"`
	BookingUnit  string                       `json:"booking_unit"`
	Capacity     *int                         `json:"capacity"`
	Placed       int                          `json:"placed"`
	Free         *int                         `json:"free"`
	Pending      []HolidayAllocationCandidate `json:"pending"`
}

// HolidayAllocationDecision is one decision of the Büro as recorded:
// CriteriaMet names the criteria the booking met at that moment.
type HolidayAllocationDecision struct {
	ID                 int64         `json:"id"`
	WeekID             int64         `json:"week_id"`
	WeekStartsOn       calendar.Date `json:"week_starts_on"`
	WeekEndsOn         calendar.Date `json:"week_ends_on"`
	RegistrationID     int64         `json:"registration_id"`
	ChildID            int64         `json:"child_id"`
	ChildFirstName     string        `json:"child_first_name"`
	ChildLastName      string        `json:"child_last_name"`
	Placement          string        `json:"placement"`
	Proposal           string        `json:"proposal"`
	Rank               int           `json:"rank"`
	CriteriaMet        []string      `json:"criteria_met"`
	Reason             string        `json:"reason"`
	DecidedAt          time.Time     `json:"decided_at"`
	DecidedByAccountID *int64        `json:"decided_by_account_id"`
}

// HolidayAllocationBoard is the allocation list of a period: its active
// criteria by priority, the pending bookings of each week in the order of
// the proposal and the decisions, newest first.
type HolidayAllocationBoard struct {
	// PlacedRegistrations names registrations that gained a place in this
	// command, directly or by promotion. Billing recomputes their confirmed
	// sibling groups in the same transaction.
	PlacedRegistrations []int64                     `json:"-"`
	AllocationMode      string                      `json:"allocation_mode"`
	Criteria            []HolidayCriterionRef       `json:"criteria"`
	Weeks               []HolidayAllocationWeek     `json:"weeks"`
	Decisions           []HolidayAllocationDecision `json:"decisions"`
}

// HolidayAllocationChoice is a decision on one pending booking: Placement is
// HolidayPlacementPlaced or HolidayPlacementWaitlisted, Proposal the
// proposal the Büro saw, Reason why it deviates from it.
type HolidayAllocationChoice struct {
	BookingID int64
	Placement string
	Proposal  string
	Reason    string
}

// HolidayRegistrationCriterion is one answer of a registration as the Büro
// reads it: ChildID is nil for the family. Stated is the parents' value,
// Checked the Büro's, nil while unchecked.
type HolidayRegistrationCriterion struct {
	CriterionID int64      `json:"criterion_id"`
	Name        string     `json:"name"`
	Question    string     `json:"question"`
	Scope       string     `json:"scope"`
	ChildID     *int64     `json:"child_id"`
	Stated      string     `json:"stated"`
	Checked     *string    `json:"checked"`
	CheckedAt   *time.Time `json:"checked_at"`
}

// HolidayCriterionCheck is the Büro's check of one answer: Checked is
// HolidayCriterionYes or HolidayCriterionNo, nil takes the check back.
type HolidayCriterionCheck struct {
	CriterionID int64   `json:"criterion_id"`
	ChildID     *int64  `json:"child_id"`
	Checked     *string `json:"checked"`
}

// HolidayAllocation lets the Träger-Büro name its criteria and decide on the
// places of a period by them. Every operation runs in the Träger
// transaction of the reviewer's Träger.
type HolidayAllocation interface {
	// HolidayCriteria lists the criteria of the Träger by priority, archived
	// ones left out, with the periods that allocate by them.
	HolidayCriteria(ctx context.Context, reviewer HolidayReviewer) (HolidayCriteriaCatalog, error)
	// SaveHolidayCriteria makes criteria the criteria of the Träger in their
	// order; one left out is archived. Invalid texts or scopes are a
	// validation error on criteria.N.field.
	SaveHolidayCriteria(ctx context.Context, reviewer HolidayReviewer, criteria []HolidayCriterionInput) (HolidayCriteriaCatalog, error)
	// HolidayAllocationBoard reads the allocation list of a period and
	// records the read; a period by arrival is ErrHolidayAllocationOff.
	HolidayAllocationBoard(ctx context.Context, reviewer HolidayReviewer, periodID int64) (HolidayAllocationBoard, error)
	// DecideHolidayAllocation decides on pending bookings under the lock of
	// the period and their weeks, records every decision with its reason,
	// tells the parents by mail and returns the list.
	DecideHolidayAllocation(ctx context.Context, reviewer HolidayReviewer, periodID int64, choices []HolidayAllocationChoice) (HolidayAllocationBoard, error)
	// CheckHolidayCriteria records the Büro's check of answers of a
	// registration that is not cancelled and returns it.
	CheckHolidayCriteria(ctx context.Context, reviewer HolidayReviewer, periodID, registrationID int64, checks []HolidayCriterionCheck) (HolidayRegistrationDetail, error)
}

// HolidayCriterionAnswer is one stored answer to a criterion: ChildID is
// nil for the family; Stated is the parents' value, Checked the Büro's, each
// nil when not given. It serves the stores and the allocation only.
type HolidayCriterionAnswer struct {
	RegistrationID int64
	ChildID        *int64
	CriterionID    int64
	Stated         *bool
	Checked        *bool
	CheckedAt      *time.Time
}

// HolidayPendingBooking is a pending booking as the store reads it, with its
// rank in the order of the proposal of its week. It serves the allocation
// only.
type HolidayPendingBooking struct {
	BookingID      int64
	WeekID         int64
	Rank           int
	RegistrationID int64
	SubmittedAt    time.Time
	ChildID        int64
	ChildPosition  int
	ChildFirstName string
	ChildLastName  string
	TimeModelName  string
	Days           []calendar.Date
	// HolidayWish and HolidayWishSource are the child's holiday care wish
	// (#4703): "yes", "no" or empty when unknown, from the "account" or the
	// "office".
	HolidayWish       string
	HolidayWishSource string
}
