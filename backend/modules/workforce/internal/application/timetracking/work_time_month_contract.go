package timetracking

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// MonthSummary is the Monatskarte aggregate for one staff member and month
// (#1842). Everything is computed on read — the Übertrag is live: a
// correction in a past month automatically flows into every later card.
//
// The saldo semantics mirror the frontend metrics engine
// (staff-metrics-helpers.ts, "Ansatz B"): sick/vacation/training days credit
// the day's contractual target (half-days credit half, rounded down), so a
// legitimate absence never produces minus hours. TargetMinutes is the full
// month's contractual Soll; TargetMinutesToDate clamps at today so the
// current month's balance doesn't count days that haven't happened yet. In
// the account-start month both are clamped at the configured start date —
// days before the account existed belong to no balance.
type MonthSummary struct {
	StaffID int64 `json:"staff_id"`
	Year    int   `json:"year"`
	Month   int   `json:"month"`

	CarryInMinutes          int     `json:"carry_in_minutes"`
	TargetMinutes           int     `json:"target_minutes"`
	TargetMinutesToDate     int     `json:"target_minutes_to_date"`
	ActualMinutes           int     `json:"actual_minutes"`
	CreditedSickMinutes     int     `json:"credited_sick_minutes"`
	CreditedVacationMinutes int     `json:"credited_vacation_minutes"`
	CreditedTrainingMinutes int     `json:"credited_training_minutes"`
	CreditedOtherMinutes    int     `json:"credited_other_minutes"`
	SickDays                float64 `json:"sick_days"`
	VacationDays            float64 `json:"vacation_days"`
	TrainingDays            float64 `json:"training_days"`
	// CreditedByAbsenceType splits the credit of the school's own absence
	// types for the DATEV export (#4807); not part of the month card.
	CreditedByAbsenceType map[int64]AbsenceTypeCredit `json:"-"`

	// PlannedShiftMinutes is the Dienstplan coverage (display only, never
	// saldo-relevant); nil = no shifts maintained in this month.
	PlannedShiftMinutes *int `json:"planned_shift_minutes,omitempty"`

	// AdjustmentMinutes is the signed sum of this month's Stundenkonto
	// transactions (#1420: payout / comp-time grant / reset); Adjustments
	// lists them so the Monatskarte can render one line per entry.
	AdjustmentMinutes int              `json:"adjustment_minutes"`
	Adjustments       []AdjustmentView `json:"adjustments,omitempty"`

	// BalanceMinutes is this month's own saldo:
	// actual + credited − target_to_date.
	BalanceMinutes int `json:"balance_minutes"`
	// ClosingBalanceMinutes = CarryInMinutes + BalanceMinutes; for the
	// current month this is the live Stundenkonto as of today.
	ClosingBalanceMinutes int `json:"closing_balance_minutes"`

	// --- Monatsabschluss (#1417) -----------------------------------------
	// IsClosed reports whether THIS month is frozen. The remaining fields
	// below describe the freeze.
	IsClosed    bool       `json:"is_closed"`
	ClosedAt    *time.Time `json:"closed_at,omitempty"`
	ClosedBy    *int64     `json:"closed_by,omitempty"`
	CloseReason string     `json:"close_reason,omitempty"`
	// FrozenClosingBalanceMinutes is the value that carries forward once this
	// month is closed. ClosingBalanceMinutes stays the LIVE number so the
	// card's carry_in + balance identity holds.
	FrozenClosingBalanceMinutes *int `json:"frozen_closing_balance_minutes,omitempty"`
	// DriftMinutes = live ClosingBalanceMinutes − frozen value, i.e. how far
	// this month has moved since it was closed (0 when not closed). It is
	// deliberately local to this month — the chain is spliced at the latest
	// snapshot BEFORE it — so drift stays additive per month instead of each
	// month inheriting its predecessors'.
	DriftMinutes int `json:"drift_minutes"`
	// CarryInFrozen reports that CarryInMinutes came from a frozen month
	// instead of being summed from the account start; CarryInFrozenFromMonth
	// names that month ("2026-08"). This is what explains a visible gap
	// between the previous month's displayed closing balance and this
	// month's carry-in: the gap is exactly that month's DriftMinutes.
	CarryInFrozen          bool    `json:"carry_in_frozen"`
	CarryInFrozenFromMonth *string `json:"carry_in_frozen_from_month,omitempty"`
}

// WorkTimeMonthService owns the Monatskarte math. It lives outside
// workSessionService on purpose: monthly aggregation is read-model logic
// with its own dependency set.
type WorkTimeMonthService interface {
	GetMonthSummary(ctx context.Context, staffID int64, year, month int) (*MonthSummary, error)
	// GetMonthSummaryAtMonthEnd pins the clamp to the month's last day instead
	// of today. The Monatsabschluss (#1417) needs the month's true closing
	// value: with the clamp at today, a not-yet-finished month would charge the
	// full Soll against an Ist that stops today. Callers must therefore only
	// use it for months that are over — StaffMonthCloseService enforces that.
	GetMonthSummaryAtMonthEnd(ctx context.Context, staffID int64, year, month int) (*MonthSummary, error)
	// GetClosingBalanceAsOf returns the live carry-chain balance through cutoff.
	// Unlike GetMonthSummary, it never includes activity later in cutoff's month.
	GetClosingBalanceAsOf(ctx context.Context, staffID int64, cutoff calendar.Date) (int, error)
	// GetBalanceAdjustmentMinutes returns the signed ledger total whose effective
	// dates lie in [from, to]. Comp-time availability uses it to include
	// same-day payouts without treating same-day work as already accrued.
	GetBalanceAdjustmentMinutes(ctx context.Context, staffID int64, from, to calendar.Date) (int, error)
	// GetBalanceReductionCapacity returns the largest debit that can become
	// effective at the start of effectiveDate without invalidating an existing
	// later debit or comp-time commitment.
	GetBalanceReductionCapacity(ctx context.Context, staffID int64, effectiveDate calendar.Date) (int, error)
	// GetCompTimeDeductionMinutes returns how much a comp_time absence over
	// [start, end] can reduce the Stundenkonto. Both full and half days reserve
	// each target minus recorded net work, because comp_time itself credits
	// nothing (#1420).
	GetCompTimeDeductionMinutes(ctx context.Context, staffID int64, start, end calendar.Date, halfDay bool) (int, error)
	// GetFutureCompTimeCommitmentMinutes sums the deductions of every
	// effective comp-time absence on days after today, through the supported
	// balance horizon. The comp-time preview (#2873) subtracts it so already
	// planned Freizeitausgleiche show up in the projected balance.
	GetFutureCompTimeCommitmentMinutes(ctx context.Context, staffID int64) (int, error)
	// GetDailyProjection returns Soll, Gutschrift, Ist and Saldo per calendar
	// day for the daily table — the same arithmetic the Monatskarte sums.
	GetDailyProjection(ctx context.Context, staffID int64, from, to calendar.Date) ([]DailyProjection, error)
	// GetDailyTargets resolves only the date-valid Soll for callers that do not
	// need absence, session, or balance data.
	GetDailyTargets(ctx context.Context, staffID int64, from, to calendar.Date) ([]DailyTarget, error)
	// GetRangeAggregate sums Soll, Ist and balance over an arbitrary closed
	// range with the same daily arithmetic as the Monatskarte, WITHOUT a carry
	// (a range is not an account). The week view of the Leitungs-Dashboard
	// needs it because the chain itself is month-granular (#1417).
	GetRangeAggregate(ctx context.Context, staffID int64, from, to calendar.Date) (*RangeAggregate, error)
}

// AdjustmentView is one Stundenkonto transaction as the Monatskarte shows it
// (#1420). MinutesDelta is signed (reductions negative).
type AdjustmentView struct {
	ID            int64         `json:"id"`
	Type          string        `json:"type"`
	MinutesDelta  int           `json:"minutes_delta"`
	EffectiveDate calendar.Date `json:"effective_date"`
	Note          string        `json:"note"`
	DecidedBy     int64         `json:"decided_by"`
	DecidedAt     time.Time     `json:"decided_at"`
}

// DailyProjection is the complete work-time picture of ONE calendar day, priced
// exactly as the Monatskarte prices it: the contractual Soll resolved against
// the schedule version valid ON that day (#1842), the absence credit that Soll
// earns, the net work time recorded, and the resulting day balance (#2443).
//
// The daily table reads these instead of deriving anything itself. Applying the
// current schedule to historical dates contradicted the Monatskarte the moment
// a staff member's hours changed (#1842); deriving the Saldo as "Ist minus
// Soll" left every absence day without one, so a Freizeitausgleich looked as if
// it cost nothing while the card below already deducted the full day (#2443).
//
// BalanceMinutes is ActualMinutes + CreditMinutes - TargetMinutes, and is 0 on
// days the card does not price either: future days and days before a mid-month
// account start. Stundenkonto-Buchungen (Auszahlung, Reset, pauschaler
// Freizeitausgleich) are deliberately NOT folded in — they are ledger entries
// with an effective date, not work time of that day, and the Monatskarte shows
// them as their own line. Summe der Tageszeilen + Buchungen = Monatssaldo.
type DailyProjection struct {
	Date           calendar.Date `json:"date"`
	TargetMinutes  int           `json:"target_minutes"`
	CreditMinutes  int           `json:"credit_minutes"`
	ActualMinutes  int           `json:"actual_minutes"`
	BalanceMinutes int           `json:"balance_minutes"`
	// TargetSource is "override" on a Sonderarbeitszeit day (#3259).
	TargetSource string `json:"target_source,omitempty"`
}

// DailyTarget is the contractual Soll of one calendar day, resolved against
// the schedule version that was valid ON that day (#1842).
type DailyTarget struct {
	Date          calendar.Date `json:"date"`
	TargetMinutes int           `json:"target_minutes"`
}

// The narrow dependency surfaces below are exactly what the month math
// needs; the full repositories satisfy them, and unit tests mock them
// without dragging in the complete repository interfaces.

// monthSettingsResolver answers the one tenant setting the monthly time
// accounts consult. It names the question rather than taking a registry key,
// so this package does not carry another owner's settings vocabulary.
type monthSettingsResolver interface {
	// AccountStartDate is the day the tenant's time accounts begin, as a
	// YYYY-MM-DD string, or empty when the tenant has not set one.
	AccountStartDate(ctx context.Context) (string, error)
}

// monthShiftReader is implemented by TimeTrackingShiftRepository.
type monthShiftReader interface {
	FindByStaffAndDateRange(ctx context.Context, staffID int64, start, end calendar.Date) ([]*TimeTrackingShift, error)
}

// monthSessionReader is implemented by WorkSessionRecords.
type monthSessionReader interface {
	ListOverlappingWorkSessions(ctx context.Context, staffIDs []int64, from time.Time, to *time.Time) ([]workforce.WorkSession, error)
}

// monthBreakReader is implemented by WorkSessionBreakRecords.
type monthBreakReader interface {
	ListWorkSessionBreaks(ctx context.Context, filter workforce.WorkSessionBreakFilter) ([]workforce.WorkSessionBreak, error)
}

// monthAbsenceReader is implemented by AbsenceRecords.
type monthAbsenceReader interface {
	ListStaffAbsences(context.Context, workforce.StaffAbsenceFilter) ([]workforce.StaffAbsence, error)
}

// monthAdjustmentReader is implemented by BalanceAdjustmentRecords.
type monthAdjustmentReader interface {
	ListStaffBalanceAdjustments(context.Context, workforce.StaffBalanceAdjustmentFilter) ([]workforce.StaffBalanceAdjustment, error)
}

// monthSnapshotReader is implemented by
// MonthSnapshots (#1417).
type monthSnapshotReader interface {
	LatestClosedMonth(ctx context.Context, staffID int64, year, month int) (*MonthSnapshot, error)
}

type StaffScheduleAssignment struct {
	WorkTimeModelID    *int64
	RotationAnchorDate *calendar.Date
}

type StaffScheduleQuery interface {
	ScheduleAssignment(ctx context.Context, staffID int64) (*StaffScheduleAssignment, error)
}

// monthScheduleReader is implemented by config.StaffWorkScheduleRepository.
type monthScheduleReader interface {
	TargetsForStaff(ctx context.Context, staffID int64, from, to calendar.Date) (WorkScheduleTargets, error)
	HasScheduleHistory(ctx context.Context, staffID int64) (bool, error)
}

// monthModelReader supplies the projected model used for target calculation.
type monthModelReader interface {
	FindByID(ctx context.Context, id int64) (*WorkTimeTargetModel, error)
}

// HolidayDatesReader is implemented by schedule.HolidayService. Public
// holidays zero the Soll of their day (#1418 3a). Exported because the
// factory injects it across service boundaries (month + session service).
type HolidayDatesReader interface {
	HolidayDates(ctx context.Context, from, to calendar.Date) (map[calendar.Date]bool, error)
}
