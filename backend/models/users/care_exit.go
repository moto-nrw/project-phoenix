package users

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
)

// Exit reasons a school picks from when it ends a child's care (#2487). A
// regular departure is not a data deletion, so these are deliberately separate
// from the deletion reasons in services/users: nothing here says "the record
// was wrong", they say why the child stopped coming.
const (
	CareExitReasonMovedAway  = "moved_away"
	CareExitReasonNoCareNeed = "no_care_needed"
	CareExitReasonOther      = "other"
)

// MaxCareExitNoteLen bounds the free text that "Anderer Grund" asks for. The
// column is TEXT; the bound keeps a reason field from becoming a second,
// unreviewed notes column on the child.
const MaxCareExitNoteLen = 200

var (
	// ErrCareExitInvalidReason means the reason is not one of the three the
	// product offers.
	ErrCareExitInvalidReason = errors.New("users: invalid care exit reason")
	// ErrCareExitNoteRequired means "Anderer Grund" was chosen without the
	// short free text that makes it readable later.
	ErrCareExitNoteRequired = errors.New("users: care exit reason note is required for the other reason")
	// ErrCareExitNoteNotAllowed means a categorised reason carried free text.
	ErrCareExitNoteNotAllowed = errors.New("users: care exit reason note is only allowed for the other reason")
)

// CareExit records WHY a child's care ended and who wrote that down.
//
// It deliberately does not carry the last care day. users.students
// .enrolled_until is the business time boundary every operational reader
// already honours, and duplicating it here would create two dates that can
// disagree about the same child. This row exists for the two things the
// interval cannot express: the categorised reason (plus its optional note) and
// the acting person. Both are read behind users:delete, which is also why they
// do not live on the student row half the app reads.
type CareExit struct {
	base.Model `bun:"schema:users,table:student_care_exits"`
	base.TenantModel
	StudentID  int64   `bun:"student_id,notnull" json:"student_id"`
	Reason     string  `bun:"reason,notnull" json:"reason"`
	ReasonNote *string `bun:"reason_note" json:"reason_note,omitempty"`
	RecordedBy *int64  `bun:"recorded_by" json:"recorded_by,omitempty"`
}

// Validate normalizes and checks the reason pair.
func (e *CareExit) Validate() error {
	if e.StudentID <= 0 {
		return errors.New("users: care exit requires a student")
	}
	if !IsValidCareExitReason(e.Reason) {
		return ErrCareExitInvalidReason
	}
	if e.ReasonNote != nil {
		trimmed := strings.TrimSpace(*e.ReasonNote)
		if trimmed == "" {
			e.ReasonNote = nil
		} else {
			if utf8.RuneCountInString(trimmed) > MaxCareExitNoteLen {
				return fmt.Errorf("users: care exit note must be at most %d characters", MaxCareExitNoteLen)
			}
			e.ReasonNote = &trimmed
		}
	}
	if e.Reason == CareExitReasonOther && e.ReasonNote == nil {
		return ErrCareExitNoteRequired
	}
	if e.Reason != CareExitReasonOther && e.ReasonNote != nil {
		return ErrCareExitNoteNotAllowed
	}
	return nil
}

// IsValidCareExitReason reports whether the value is one of the three reasons
// the product offers.
func IsValidCareExitReason(reason string) bool {
	switch reason {
	case CareExitReasonMovedAway, CareExitReasonNoCareNeed, CareExitReasonOther:
		return true
	default:
		return false
	}
}

// CareEndedOn reports whether the child's care has already ended on the given
// calendar day — the enrollment interval's upper bound is INCLUSIVE, so the
// last care day itself still counts as care.
//
// Pure derivation from two fields the row already carries: no policy, no
// thresholds, no clock (backend-conventions rule 12). Every operational gate
// spells the question this way so "ab dem Folgetag" cannot be interpreted
// half a day differently in two places.
func (s *Student) CareEndedOn(day timezone.Date) bool {
	return s != nil && s.EnrolledUntil != nil && day.After(*s.EnrolledUntil)
}

// CareEndsLater reports whether an end of care is recorded but has not taken
// effect yet on the given day. This is the "Betreuung endet am …" state the
// child management shows while the child still attends normally.
func (s *Student) CareEndsLater(day timezone.Date) bool {
	return s != nil && s.EnrolledUntil != nil && !day.After(*s.EnrolledUntil)
}

// CareExitListFilter narrows the archive view ("Beendete Betreuungen").
type CareExitListFilter struct {
	// Search matches first name, last name or school class, case-insensitively.
	Search string
	// SchoolClasses, when non-empty, restricts to those exact classes.
	SchoolClasses []string
	Page          int
	PageSize      int
}

// EndedCare is one row of the archive view: a child whose care interval has
// run out, joined with the reason when one was recorded.
type EndedCare struct {
	StudentID   int64          `bun:"student_id"`
	FirstName   string         `bun:"first_name"`
	LastName    string         `bun:"last_name"`
	SchoolClass string         `bun:"school_class"`
	LastCareDay timezone.Date  `bun:"last_care_day"`
	Reason      *string        `bun:"reason"`
	ReasonNote  *string        `bun:"reason_note"`
	RecordedBy  *int64         `bun:"recorded_by"`
	RecordedAt  *timezone.Date `bun:"recorded_at"`
}

// CareEndedDecisionReason is the German sentence written onto every parent
// request that is closed because the child's care ended. It is shown to the
// family verbatim, so it says what happened rather than naming a decision
// nobody made.
const CareEndedDecisionReason = "Die Betreuung dieses Kindes ist beendet. Die Anfrage wurde deshalb geschlossen."

// CareExitCleanupRepository owns the two cross-schema operations that ending a
// child's care needs. Both span schemas no single domain repository owns.
type CareExitCleanupRepository interface {
	// CountOpenRequests counts, per student, the still-open rows across the
	// four parent request queues.
	CountOpenRequests(ctx context.Context, studentIDs []int64) (map[int64]int, error)
	// CloseOpenRequests moves those rows to the care_ended terminal state.
	CloseOpenRequests(ctx context.Context, studentIDs []int64, reviewedBy *int64, at time.Time) (int, error)
	// FindOpenPresence reports which children still hold an open attendance
	// row or room visit.
	FindOpenPresence(ctx context.Context, studentIDs []int64) (map[int64]bool, error)
	// CloseOpenPresence closes those records at `at`.
	CloseOpenPresence(ctx context.Context, studentIDs []int64, at time.Time) (int, error)

	// CountPlannedByStudentIDsAfter counts, per student, the still-planned
	// roster rows on non-cancelled instances dated strictly after `after`.
	CountPlannedByStudentIDsAfter(ctx context.Context, studentIDs []int64, after timezone.Date) (map[int64]int, error)
	// DeletePlannedByStudentIDsAfter removes those same rows.
	DeletePlannedByStudentIDsAfter(ctx context.Context, studentIDs []int64, after timezone.Date) (int, error)

	// CountRunningByStudentIDsAfter counts, per student, the offering and
	// activity bookings still running at validUntil (exclusive bound).
	CountRunningByStudentIDsAfter(ctx context.Context, studentIDs []int64, validUntil timezone.Date) (map[int64]int, error)
	// CapByStudentIDs ends those bookings at validUntil.
	CapByStudentIDs(ctx context.Context, studentIDs []int64, validUntil timezone.Date) (int64, error)

	// RestoreRemovals puts back what the children's current exit removed from
	// the plan and clears the ledger. It is what makes a planned exit
	// changeable and cancellable; both counting methods above therefore count
	// the restorable rows in, so the preview describes the baseline the
	// confirmation actually works from.
	RestoreRemovals(ctx context.Context, studentIDs []int64) (int, error)
	// DiscardRemovals drops the ledger unreplayed — once the exit has taken
	// effect, and on a resume, where nothing may switch itself back on.
	DiscardRemovals(ctx context.Context, studentIDs []int64) error
}

// CareExitRepository owns access to the reason rows and the archive read model.
type CareExitRepository interface {
	// FindByStudentIDs returns the recorded reason per student, for the ids
	// that have one.
	FindByStudentIDs(ctx context.Context, studentIDs []int64) (map[int64]*CareExit, error)
	// Upsert writes the single current reason row for the child.
	Upsert(ctx context.Context, exit *CareExit) error
	// DeleteByStudentIDs removes the reason rows of the given children, used
	// when a planned exit is cancelled or the care is resumed.
	DeleteByStudentIDs(ctx context.Context, studentIDs []int64) error
	// ListEnded returns children whose care interval ended on or before asOf,
	// newest last care day first, with the total row count.
	ListEnded(ctx context.Context, asOf timezone.Date, filter CareExitListFilter) ([]*EndedCare, int, error)
}
