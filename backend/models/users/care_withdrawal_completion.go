package users

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
)

const (
	CareWithdrawalStatePending  = "pending"
	CareWithdrawalStateResolved = "resolved"
	CareWithdrawalStateObsolete = "obsolete"

	CareWithdrawalTriggerDirectSchool   = "direct_school"
	CareWithdrawalTriggerBookingExpired = "booking_expired"
	CareWithdrawalOutcomeCareEnded      = "care_ended"
	CareWithdrawalObsoleteRebooked      = "rebooked_without_gap"
	CareWithdrawalObsoleteWeeklyPlans   = "weekly_plan_mode"

	CareWithdrawalUrgencyPlanned = "planned"
	CareWithdrawalUrgencyOverdue = "overdue"
)

var ErrCareWithdrawalAlreadyResolved = errors.New("Die Abmeldung wurde bereits erledigt oder ist nicht mehr aktuell.") //nolint:staticcheck // user-facing German message

// CareWithdrawalCompletion is the durable task created when a school removes
// a child's final care day from authoritative bookings. It stores events only;
// planned/overdue is a view of FirstBookinglessDay against a reference date.
type CareWithdrawalCompletion struct {
	base.Model `bun:"schema:users,table:care_withdrawal_completions"`
	base.TenantModel
	StudentID               *int64                   `bun:"student_id" json:"student_id,omitempty"`
	FirstBookinglessDay     timezone.Date            `bun:"first_bookingless_day,type:date,notnull" json:"first_bookingless_day"`
	Trigger                 string                   `bun:"trigger,notnull" json:"trigger"`
	SourceAdjustmentID      *int64                   `bun:"source_adjustment_id" json:"source_adjustment_id,omitempty"`
	SourceRequestChildID    *int64                   `bun:"source_request_child_id" json:"source_request_child_id,omitempty"`
	WithdrawalConfirmedBy   *int64                   `bun:"withdrawal_confirmed_by" json:"withdrawal_confirmed_by,omitempty"`
	WithdrawalConfirmedRole string                   `bun:"withdrawal_confirmed_role,notnull" json:"withdrawal_confirmed_role"`
	WithdrawalConfirmedAt   time.Time                `bun:"withdrawal_confirmed_at,notnull" json:"withdrawal_confirmed_at"`
	SourceOfferings         []CareExitSourceOffering `bun:"source_offerings,type:jsonb,notnull" json:"source_offerings"`
	State                   string                   `bun:"state,notnull" json:"state"`
	Outcome                 *string                  `bun:"outcome" json:"outcome,omitempty"`
	ObsoleteReason          *string                  `bun:"obsolete_reason" json:"obsolete_reason,omitempty"`
	ResolvedBy              *int64                   `bun:"resolved_by" json:"resolved_by,omitempty"`
	ResolvedAt              *time.Time               `bun:"resolved_at" json:"resolved_at,omitempty"`

	FirstName   string `bun:"first_name,scanonly" json:"first_name,omitempty"`
	LastName    string `bun:"last_name,scanonly" json:"last_name,omitempty"`
	SchoolClass string `bun:"school_class,scanonly" json:"school_class,omitempty"`
}

func (c CareWithdrawalCompletion) UrgencyOn(reference timezone.Date) string {
	if c.FirstBookinglessDay.After(reference) {
		return CareWithdrawalUrgencyPlanned
	}
	return CareWithdrawalUrgencyOverdue
}

type CareWithdrawalCompletionFilter struct {
	Search    string
	StudentID int64
	Page      int
	PageSize  int
}

// Normalized returns the canonical search and pagination values shared by the
// HTTP response metadata and the service/repository query.
func (f CareWithdrawalCompletionFilter) Normalized() CareWithdrawalCompletionFilter {
	f.Search = strings.TrimSpace(f.Search)
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 || f.PageSize > 100 {
		f.PageSize = 20
	}
	return f
}

type CareWithdrawalBookingChange struct {
	StudentID             int64
	FirstBookinglessDay   timezone.Date
	HasCareDays           bool
	WasCompleteWithdrawal bool
	SourceAdjustmentID    int64
	SourceRequestChildID  int64
	ConfirmedBy           int64
	ConfirmedRole         string
	SourceOfferings       []CareExitSourceOffering
}

type CareWithdrawalCompletionRepository interface {
	UpsertPending(ctx context.Context, completion *CareWithdrawalCompletion) error
	FindByID(ctx context.Context, id int64) (*CareWithdrawalCompletion, error)
	FindByIDForUpdate(ctx context.Context, id int64) (*CareWithdrawalCompletion, error)
	ListPending(ctx context.Context, filter CareWithdrawalCompletionFilter) ([]*CareWithdrawalCompletion, int, error)
	MarkResolved(ctx context.Context, id, actorAccountID int64, at time.Time) (bool, error)
	MarkObsoleteForRebooking(ctx context.Context, studentID int64, careStartsOn timezone.Date, at time.Time) (bool, error)
	MarkPendingObsoleteForWeeklyPlans(ctx context.Context, at time.Time) (int, error)
	ReopenAfterCancelledExit(ctx context.Context, completionID, studentID int64, at time.Time) (bool, error)
}
