package careplan

import (
	"context"
	"errors"
	"strings"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The care lifecycle (#2487, #3427): ending a child's care ("Betreuung
// beenden"), cancelling or changing a planned end, the guided
// close-out of a full withdrawal, the dated participation decision every
// operational list applies, and the booking-led care switch. There is one
// lifecycle contract; there must not be a second way for a child to leave the
// OGS.

// MaxCareExitBatchSize bounds one "Betreuung beenden" action. It matches the
// selection cap the child management already enforces, so a selection the UI
// allows can always be confirmed.
const MaxCareExitBatchSize = 500

// CareEndedDecisionReason is the German decision text an open family request
// carries once care ending closed it. It is shown to the family verbatim.
const CareEndedDecisionReason = "Die Betreuung dieses Kindes ist beendet. Die Anfrage wurde deshalb geschlossen."

// Withdrawal task urgency, a view of the first bookingless day against a
// reference date.
const (
	WithdrawalUrgencyPlanned = "planned"
	WithdrawalUrgencyOverdue = "overdue"
)

// Care lifecycle sentinels carry diagnostic detail; clients show the code catalog.
var (
	ErrCareExitNoStudents            = errors.New("bitte wählen Sie mindestens ein Kind aus")
	ErrCareExitTooManyStudents       = errors.New("es können höchstens 500 Kinder auf einmal beendet werden")
	ErrCareExitDayInPast             = errors.New("der letzte Betreuungstag darf nicht in der Vergangenheit liegen")
	ErrCareExitPreviewChanged        = errors.New("die Betreuung wurde nicht beendet. Die Daten haben sich seit der Vorschau geändert. Bitte prüfen Sie die Vorschau erneut")
	ErrCareExitBlocked               = errors.New("die Betreuung wurde nicht beendet")
	ErrCareExitNotPlanned            = errors.New("für dieses Kind ist kein Ende der Betreuung geplant")
	ErrCareExitAlreadyEffective      = errors.New("die Betreuung ist bereits beendet und kann nicht mehr storniert werden. Nutzen Sie „Betreuung wieder aufnehmen“")
	ErrCareResumeNotEnded            = errors.New("die Betreuung dieses Kindes läuft noch")
	ErrCareResumeMissing             = errors.New("für dieses Kind ist keine beendete Betreuung hinterlegt")
	ErrCareResumeStartInPast         = errors.New("der neue Beginn darf nicht in der Vergangenheit liegen")
	ErrCareResumeNotChecked          = errors.New("bitte bestätigen Sie zuerst die Prüfung. Gruppe, Angebote, Wochenplan und Zeiten bleiben sonst ungeprüft")
	ErrCareResumeUnknownStudent      = errors.New("dieses Kind gibt es in Ihrer Schule nicht (mehr)")
	ErrCareResumeAlumnus             = errors.New("dieses Kind wurde beim Schuljahreswechsel abgemeldet. Sie finden es unter Schuljahreswechsel, Bereich Abgänge")
	ErrCareWithdrawalNotFound        = errors.New("diese Abmeldung gibt es nicht oder nicht mehr")
	ErrCareWithdrawalAfterGap        = errors.New("der letzte Betreuungstag muss vor dem ersten Tag ohne Buchung liegen")
	ErrCareWithdrawalAlreadyResolved = errors.New("die Abmeldung wurde bereits erledigt oder ist nicht mehr aktuell")
	ErrBookingAuthorityBlocked       = errors.New("der Buchungsmodus kann nicht aktiviert werden. Für mindestens ein aktuell betreutes Kind ist kein Betreuungstag gebucht")
)

// Per-child blocker sentences. They name the child's own situation, never a
// technical condition, because they are listed one per child under the
// headline "Die Betreuung wurde nicht beendet".
const (
	CareBlockerUnknown       = "Dieses Kind gibt es in Ihrer Schule nicht (mehr)."
	CareBlockerAlumnus       = "Dieses Kind wurde beim Schuljahreswechsel abgemeldet. Sie finden es unter Schuljahreswechsel, Bereich Abgänge."
	CareBlockerAlreadyEnded  = "Die Betreuung dieses Kindes ist bereits beendet."
	CareBlockerBeforeStart   = "Die Betreuung dieses Kindes beginnt erst am %s. Bitte wählen Sie einen späteren letzten Betreuungstag."
	CareBlockerResumeMissing = "Für dieses Kind ist keine beendete Betreuung hinterlegt."
)

// CareWithdrawalDateError is a client-correctable retroactive-date conflict
// whose message explains the concrete boundary.
type CareWithdrawalDateError struct{ Message string }

func (e *CareWithdrawalDateError) Error() string { return e.Message }

// CareExitSourceOffering is one concrete source booking summarized for the
// binding preview. Days use the canonical mon..fri codes.
type CareExitSourceOffering struct {
	Name string   `json:"name"`
	Days []string `json:"days"`
}

// CareExitInput is the whole action: one set of children, one last care day,
// one reason. Every acceptance criterion that says "alle Kinder einer Aktion"
// holds because there is nowhere to put a per-child value.
type CareExitInput struct {
	StudentIDs  []int64
	LastCareDay calendar.Date
	Reason      string
	ReasonNote  string
}

// CareExitImpact is what ending the care will do to ONE child, named.
type CareExitImpact struct {
	StudentID          int64
	FirstName          string
	LastName           string
	SchoolClass        string
	PlannedRosterRows  int
	ActivityBookings   int
	OpenParentRequests int
	HasRFIDTag         bool
	CurrentlyPresent   bool
	SourceOfferings    []CareExitSourceOffering
	WeeklyPlans        []string
	// PlannedEndsOn is the exit already recorded for this child, if any. A
	// second run over the same child is a CHANGE, not a blocker.
	PlannedEndsOn *calendar.Date
	// Blocker is empty when the child can be ended, otherwise the German
	// sentence explaining why not.
	Blocker string
}

// CanEnd reports whether this child is free of blockers.
func (i CareExitImpact) CanEnd() bool { return i.Blocker == "" }

// CareExitPreview is the immutable state a confirmation has to quote back.
type CareExitPreview struct {
	Token       string
	LastCareDay calendar.Date
	Reason      string
	ReasonNote  string
	Students    []CareExitImpact
	Blocked     bool
}

// CareExitResult reports what the confirmation actually changed.
type CareExitResult struct {
	StudentsEnded     int
	RosterRowsRemoved int
	BookingsEnded     int
	// ReappliedStudentIDs are the children whose earlier planned exit this
	// confirmation replaced: their removed plan was put back before the new
	// last care day applied (#5235).
	ReappliedStudentIDs []int64
}

// CareResumeInput reopens one child's care.
type CareResumeInput struct {
	StudentID      int64
	NewStart       calendar.Date
	ActorAccountID int64
	// Checked is the explicit confirmation that the previous group,
	// offerings, weekly plan and arrival/pickup times were reviewed. Nothing
	// is re-enabled automatically, so the flag is the only thing standing
	// between "resumed" and "resumed with a year-old plan nobody looked at".
	Checked bool
}

// EndedCareFilter narrows the archive view ("Beendete Betreuungen").
type EndedCareFilter struct {
	// Search matches first name, last name or school class, case-insensitively.
	Search string
	// SchoolClasses, when non-empty, restricts to those exact classes.
	SchoolClasses []string
	Page          int
	PageSize      int
}

// EndedCare is one row of the archive: a child whose care interval has run
// out, joined with the reason when one was recorded.
type EndedCare struct {
	StudentID   int64
	FirstName   string
	LastName    string
	SchoolClass string
	LastCareDay calendar.Date
	Reason      *string
	ReasonNote  *string
	RecordedBy  *int64
	RecordedAt  *calendar.Date
}

// CareWithdrawalFilter selects one page of withdrawal tasks.
type CareWithdrawalFilter struct {
	Search    string
	StudentID int64
	Page      int
	PageSize  int
}

// Normalized returns the canonical search and pagination values shared by the
// HTTP response metadata and the query.
func (f CareWithdrawalFilter) Normalized() CareWithdrawalFilter {
	f.Search = strings.TrimSpace(f.Search)
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 || f.PageSize > 100 {
		f.PageSize = 20
	}
	return f
}

// UrgencyOn is the planned/overdue view of a task against a reference day.
func (c WithdrawalCompletion) UrgencyOn(reference calendar.Date) string {
	if calendar.Date(c.FirstBookinglessDay).After(reference) {
		return WithdrawalUrgencyPlanned
	}
	return WithdrawalUrgencyOverdue
}

// CareWithdrawalBookingChange is an authoritative booking mutation Care Plan
// reconciles its withdrawal tasks against.
type CareWithdrawalBookingChange struct {
	StudentID             int64
	FirstBookinglessDay   calendar.Date
	WasCompleteWithdrawal bool
	SourceAdjustmentID    int64
	SourceRequestChildID  int64
	ConfirmedBy           int64
	ConfirmedRole         string
	SourceOfferings       []CareExitSourceOffering
}

// CareParticipationResolution is the dated visibility decision list and
// report readers consume. CandidateIDs is the resolved school population when
// a caller supplied no narrower candidate set.
type CareParticipationResolution struct {
	CandidateIDs       []int64
	ParticipatingIDs   map[int64]bool
	ActuallyPresentIDs map[int64]bool
}

// GroupRosterIncludesPendingWithdrawals is the includePending argument of
// ResolveListParticipation for the dated group roster People Directory reads
// (#3350). The roster shows the children who still take part in care, so a
// pending withdrawal task does not keep a child on it; only the
// administrative lists retain those children.
const GroupRosterIncludesPendingWithdrawals = false

// BookingAuthorityImpactChild is one child an operator reviews before
// switching to booking-led care.
type BookingAuthorityImpactChild struct {
	StudentID           string         `json:"student_id"`
	FirstName           string         `json:"first_name"`
	LastName            string         `json:"last_name"`
	SchoolClass         string         `json:"school_class"`
	FirstBookinglessDay *calendar.Date `json:"first_bookingless_day,omitempty"`
}

// BookingAuthorityImpact lists the children that would be left without any
// care day and the completions the switch would plan.
type BookingAuthorityImpact struct {
	ReferenceDate      calendar.Date                 `json:"reference_date"`
	BlockingChildren   []BookingAuthorityImpactChild `json:"blocking_children"`
	PlannedCompletions []BookingAuthorityImpactChild `json:"planned_completions"`
}

// CareExitCommands ends and cancels care, and applies the effect-day
// housekeeping once an end has taken effect.
type CareExitCommands interface {
	// Preview describes what ending the care would do, per child.
	Preview(ctx context.Context, input CareExitInput) (*CareExitPreview, error)
	// Confirm ends the care for exactly the previewed state, or changes
	// nothing at all.
	Confirm(ctx context.Context, token string, input CareExitInput, actorAccountID int64) (*CareExitResult, error)
	// Cancel withdraws exits that have not taken effect yet.
	Cancel(ctx context.Context, studentIDs []int64, actorAccountID int64) (int, error)
	// ApplyDueEffects performs the effect-day housekeeping for every child
	// whose care ended before asOf: closing what is still open, freeing the
	// bracelet, closing the open family requests. Idempotent.
	ApplyDueEffects(ctx context.Context, asOf calendar.Date) (int, error)
}

// CareExitArchive reads the ended care and the recorded exits.
type CareExitArchive interface {
	// ListEnded is the archive view.
	ListEnded(ctx context.Context, filter EndedCareFilter) ([]EndedCare, int, error)
	// RecordedExitStudentIDs reports which of the given children carry a
	// recorded exit. The child management tells a manual "Betreuung beenden"
	// apart from the ordinary end of an enrolment phase with it.
	RecordedExitStudentIDs(ctx context.Context, studentIDs []int64) (map[int64]bool, error)
}

// CareWithdrawals is the guided close-out after a full withdrawal.
type CareWithdrawals interface {
	ListPendingWithdrawals(ctx context.Context, filter CareWithdrawalFilter) ([]WithdrawalCompletion, int, error)
	ListResolvedWithdrawals(ctx context.Context, filter CareWithdrawalFilter) ([]WithdrawalCompletion, int, error)
	GetPendingWithdrawal(ctx context.Context, id int64) (WithdrawalCompletion, error)
	PreviewWithdrawalCareEnd(ctx context.Context, completionID int64, input CareExitInput) (*CareExitPreview, error)
	ConfirmWithdrawalCareEnd(ctx context.Context, completionID int64, token string, input CareExitInput, actorAccountID int64) (*CareExitResult, error)
	// ReconcileAuthoritativeBookingChange keeps the withdrawal tasks in step
	// with a booking mutation in the caller's transaction.
	ReconcileAuthoritativeBookingChange(ctx context.Context, change CareWithdrawalBookingChange) error
}

// CareParticipation is the dated participation decision. Actual presence
// always wins: safety information stays visible after expected care ended.
type CareParticipation interface {
	ResolveListParticipation(ctx context.Context, studentIDs []int64, on, today calendar.Date, includePending bool) (*CareParticipationResolution, error)
	ParticipatingStudentIDs(ctx context.Context, studentIDs []int64, on calendar.Date, actuallyPresent map[int64]bool) (map[int64]bool, error)
	ParticipatingStudentIDsByDate(ctx context.Context, studentIDs []int64, from, to calendar.Date) (map[calendar.Date]map[int64]bool, error)
}

// HistoricalCareParticipation reconstructs participation for retained reports.
// Alumni remain included before a known dated departure boundary. Alumni
// without such a boundary remain excluded; current visibility is separate.
type HistoricalCareParticipation interface {
	HistoricalParticipatingStudentIDsByDate(ctx context.Context, studentIDs []int64, from, to calendar.Date) (map[calendar.Date]map[int64]bool, error)
}

// BookingAuthority reviews and applies the switch to booking-led care.
type BookingAuthority interface {
	PreviewBookingAuthorityImpact(ctx context.Context, on calendar.Date) (*BookingAuthorityImpact, error)
	// ApplyBookingAuthoritySetting validates and reconciles a mode switch in
	// the caller's open tenant transaction. Enabling it while a cared-for
	// child has no care day returns ErrBookingAuthorityBlocked.
	ApplyBookingAuthoritySetting(ctx context.Context, on calendar.Date, enabled bool) (*BookingAuthorityImpact, error)
}

// CareLifecycle is the one care-lifecycle capability.
type CareLifecycle interface {
	CareExitCommands
	CareExitArchive
	CareWithdrawals
	CareParticipation
	HistoricalCareParticipation
	BookingAuthority
}
