package users

import (
	"errors"
	"strings"
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
	// ErrCareExitNoteTooLong means the optional explanation exceeds the limit.
	ErrCareExitNoteTooLong = errors.New("Die Begründung ist zu lang. Bitte kürzen Sie sie.") //nolint:staticcheck // user-facing German message
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
	StudentID              int64          `bun:"student_id,notnull" json:"student_id"`
	PreviousEnrolledUntil  *timezone.Date `bun:"previous_enrolled_until,type:date" json:"-"`
	Reason                 string         `bun:"reason,notnull" json:"reason"`
	ReasonNote             *string        `bun:"reason_note" json:"reason_note,omitempty"`
	RecordedBy             *int64         `bun:"recorded_by" json:"recorded_by,omitempty"`
	WithdrawalCompletionID *int64         `bun:"withdrawal_completion_id" json:"-"`
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
				return ErrCareExitNoteTooLong
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

// CareExitSourceOffering is one concrete source booking summarized for the
// binding preview. Days use the canonical mon..fri codes; the frontend renders
// the compact German pattern.
type CareExitSourceOffering struct {
	Name string   `bun:"name" json:"name"`
	Days []string `bun:"days,type:jsonb" json:"days"`
}
