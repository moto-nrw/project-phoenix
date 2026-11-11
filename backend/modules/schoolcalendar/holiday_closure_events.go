package schoolcalendar

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Notices to the parents of booked holiday weeks after a school closing day
// changed (#6442, #6457, ADR 0196). Saving, changing, shortening, moving or
// deleting a school closing day only proposes a notice; it sends nothing. A
// click of the school admin or of the Träger-Büro confirms the proposal and
// writes one durable command in the school's transaction; both clicks find
// the same command. The Träger's worker processes it in a Träger
// transaction of its own. Neither proposal nor command holds a family, a
// child or an address.

// Channels that confirm a proposal.
const (
	HolidayClosureChannelSchool  = "school"
	HolidayClosureChannelTraeger = "traeger"
)

// HolidayClosureStatus is what both places show of a notice (ADR 0196
// section 5).
type HolidayClosureStatus string

const (
	// HolidayClosureSuggested: proposed, nobody confirmed it yet.
	HolidayClosureSuggested HolidayClosureStatus = "suggested"
	// HolidayClosureSending: confirmed, waiting or being processed.
	HolidayClosureSending HolidayClosureStatus = "sending"
	// HolidayClosureNoMail: no booking needed a mail.
	HolidayClosureNoMail HolidayClosureStatus = "no_mail"
	// HolidayClosureSent: every mail was accepted for transport.
	HolidayClosureSent HolidayClosureStatus = "sent"
	// HolidayClosureFailed: a known failure; a click takes it up again.
	HolidayClosureFailed HolidayClosureStatus = "failed"
	// HolidayClosureUnknown: a mail's transport outcome is unknown; no blind
	// repetition.
	HolidayClosureUnknown HolidayClosureStatus = "unknown"
)

// Event states of the durable command.
const (
	HolidayClosureEventPending           = "pending"
	HolidayClosureEventLeased            = "leased"
	HolidayClosureEventPrepared          = "prepared"
	HolidayClosureEventCompleted         = "completed"
	HolidayClosureEventNoMail            = "no_mail"
	HolidayClosureEventFailed            = "failed"
	HolidayClosureEventUnknown           = "unknown"
	HolidayClosureEventBlocked           = "blocked"
	HolidayClosureEventNeedsConfirmation = "needs_confirmation"
)

var (
	// ErrHolidayClosureNoticeNotFound reports a proposal outside the caller's
	// school or Träger.
	ErrHolidayClosureNoticeNotFound = errors.New("holiday closure notice not found")
	// ErrHolidayClosureNoticeChanged reports a click on a proposal whose
	// closing day changed since the caller saw it; nothing was confirmed.
	ErrHolidayClosureNoticeChanged = errors.New("the closing day of the holiday closure notice changed")
	// ErrHolidayClosureLeaseLost reports a worker whose lease expired: it
	// writes no result.
	ErrHolidayClosureLeaseLost = errors.New("the lease of the holiday closure event was lost")
)

// HolidayClosureRange is a closing day range.
type HolidayClosureRange struct {
	StartsOn calendar.Date `json:"start_date"`
	EndsOn   calendar.Date `json:"end_date"`
}

// HolidayClosureNotice is a proposal with the state of its command: the
// school, the closing day before and after the change (nil when there was
// none before, or none after a deletion) and the days it may touch.
// Changed marks a proposal that replaces a command whose closing day changed
// before it was processed.
type HolidayClosureNotice struct {
	ProposalID     int64
	SchoolID       int64
	OrganizationID int64
	ClosingDayID   int64
	Reason         string
	Previous       *HolidayClosureRange
	Current        *HolidayClosureRange
	SpanFrom       calendar.Date
	SpanTo         calendar.Date
	Revision       int64
	OperationID    string
	Status         HolidayClosureStatus
	Changed        bool
	EventID        int64
	ConfirmedAt    *time.Time
	UpdatedAt      time.Time
}

// CanNotify reports whether a click does something: a proposal nobody
// confirmed or a command that failed.
func (n HolidayClosureNotice) CanNotify() bool {
	return n.Status == HolidayClosureSuggested || n.Status == HolidayClosureFailed
}

// HolidayClosureActor is the account that clicks and where; a click of the
// Träger-Büro names its Träger, which must be the proposal's.
type HolidayClosureActor struct {
	ImpactToken    string
	AccountID      int64
	Channel        string
	OrganizationID int64
}

// HolidayClosureConfirmation is the displayed proposal revision and impact.
type HolidayClosureConfirmation struct {
	ProposalID, Revision int64
	ImpactToken          string
}

// HolidayClosureProposals is the proposal side of the notices.
type HolidayClosureProposals interface {
	// SchoolHolidayClosureNotices lists the open and recent notices of the
	// school in the caller's school transaction.
	SchoolHolidayClosureNotices(ctx context.Context) ([]HolidayClosureNotice, error)
	// ConfirmHolidayClosureNotice confirms the proposal at revision in the
	// caller's school transaction, or takes its failed command up again.
	// A second click finds the same command and changes nothing.
	ConfirmHolidayClosureNotice(ctx context.Context, proposalID, revision int64, actor HolidayClosureActor) (HolidayClosureNotice, error)
	// ConfirmHolidayClosureNotices confirms a school's proposals in set-based
	// guarded reads and writes; any stale proposal rolls the whole unit back.
	ConfirmHolidayClosureNotices(ctx context.Context, confirmations []HolidayClosureConfirmation, actor HolidayClosureActor) ([]HolidayClosureNotice, error)
	// TraegerHolidayClosureNotices lists the notices of the current schools
	// of organizationID in the caller's Träger transaction.
	TraegerHolidayClosureNotices(ctx context.Context, organizationID int64) ([]HolidayClosureNotice, error)
}

// HolidayClosureEvent is a leased command as the worker processes it.
type HolidayClosureEvent struct {
	Ranges         []HolidayClosureRange
	ImpactToken    string
	ID             int64
	OrganizationID int64
	SchoolID       int64
	OperationID    string
	Revision       int64
	SpanFrom       calendar.Date
	SpanTo         calendar.Date
	LeaseToken     string
	Attempts       int
	State          string
}

// HolidayClosureCalendarDay is a closing day range of a school in the read
// channel of the worker.
type HolidayClosureCalendarDay struct {
	SourceID int64
	Traeger  bool
	StartsOn calendar.Date
	EndsOn   calendar.Date
	Reason   string
}

// HolidayClosureCalendar is what the worker reads of a current school of
// its Träger: name, stored federal state (empty for the registry default),
// revision and closing days.
type HolidayClosureCalendar struct {
	SchoolID     int64
	SchoolName   string
	FederalState string
	Revision     int64
	ClosingDays  []HolidayClosureCalendarDay
}

// HolidayClosureOutcome is the result the worker writes for a leased
// command: an event state and, for a failure, a technical code.
type HolidayClosureOutcome struct {
	State     string
	ErrorCode string
	// RetryAt makes a pending outcome due again then.
	RetryAt time.Time
}

// HolidayClosureSettlement moves a waiting command to its delivery state.
type HolidayClosureSettlement struct {
	EventID int64
	State   string
}

// HolidayClosureEventQueue is the worker side of the commands. Every
// operation but DueHolidayClosureTraeger runs in the caller's Träger
// transaction of organizationID.
type HolidayClosureEventQueue interface {
	// DueHolidayClosureTraeger names the Träger with due, expired, prepared
	// or retiring commands, nothing else.
	DueHolidayClosureTraeger(ctx context.Context) ([]int64, error)
	// LeaseHolidayClosureEvent leases the next due command of the Träger.
	LeaseHolidayClosureEvent(ctx context.Context, organizationID int64, leaseFor time.Duration) (HolidayClosureEvent, bool, error)
	// LockHolidayClosurePreparation holds memberships and calendars until commit,
	// before the command and Enrollment locks.
	LockHolidayClosurePreparation(ctx context.Context, organizationID int64, schoolIDs []int64) error
	// LockLeasedHolidayClosureEvent locks the command under its lease;
	// ErrHolidayClosureLeaseLost when another worker took it.
	LockLeasedHolidayClosureEvent(ctx context.Context, organizationID, eventID int64, leaseToken string) error
	// HolidayClosureCalendars reads, in two statements for any number of
	// schools, the closing days in [from, to] of those schools that are
	// current schools of the Träger; another school has no entry.
	HolidayClosureCalendars(ctx context.Context, organizationID int64, schoolIDs []int64, from, to calendar.Date) (map[int64]HolidayClosureCalendar, error)
	// HolidayClosureEventSuperseded reports a later change of the same
	// closing day that replaced the command before it was processed.
	HolidayClosureEventSuperseded(ctx context.Context, organizationID, eventID int64) (bool, error)
	// FinishHolidayClosureEvent writes the outcome of a leased command.
	FinishHolidayClosureEvent(ctx context.Context, organizationID, eventID int64, leaseToken string, outcome HolidayClosureOutcome) error
	// WaitingHolidayClosureEvents lists the prepared and unknown commands
	// whose mails the worker still follows.
	WaitingHolidayClosureEvents(ctx context.Context, organizationID int64) ([]HolidayClosureEvent, error)
	// SettleHolidayClosureEvents moves waiting commands to their delivery
	// states in one statement.
	SettleHolidayClosureEvents(ctx context.Context, organizationID int64, settlements []HolidayClosureSettlement) error
	// RetireHolidayClosureEvents removes commands finished more than 30
	// days ago with their proposals.
	RetireHolidayClosureEvents(ctx context.Context, organizationID int64) (int64, error)
}

// HolidayClosureWatchdog observes confirmed work independently of processing.
// It requires an administrative unit and exposes only an aggregate count.
type HolidayClosureWatchdog interface {
	OverdueHolidayClosureEvents(ctx context.Context, overdueFor time.Duration) (int, error)
}
