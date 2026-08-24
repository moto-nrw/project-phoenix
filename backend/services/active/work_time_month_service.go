package active

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
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
	GetClosingBalanceAsOf(ctx context.Context, staffID int64, cutoff timezone.Date) (int, error)
	// GetBalanceAdjustmentMinutes returns the signed ledger total whose effective
	// dates lie in [from, to]. Comp-time availability uses it to include
	// same-day payouts without treating same-day work as already accrued.
	GetBalanceAdjustmentMinutes(ctx context.Context, staffID int64, from, to timezone.Date) (int, error)
	// GetBalanceReductionCapacity returns the largest debit that can become
	// effective at the start of effectiveDate without invalidating an existing
	// later debit or comp-time commitment.
	GetBalanceReductionCapacity(ctx context.Context, staffID int64, effectiveDate timezone.Date) (int, error)
	// GetCompTimeDeductionMinutes returns how much a comp_time absence over
	// [start, end] can reduce the Stundenkonto. Both full and half days reserve
	// each target minus recorded net work, because comp_time itself credits
	// nothing (#1420).
	GetCompTimeDeductionMinutes(ctx context.Context, staffID int64, start, end timezone.Date, halfDay bool) (int, error)
	// GetDailyProjection returns Soll, Gutschrift, Ist and Saldo per calendar
	// day for the daily table — the same arithmetic the Monatskarte sums.
	GetDailyProjection(ctx context.Context, staffID int64, from, to timezone.Date) ([]DailyProjection, error)
	// GetDailyTargets resolves only the date-valid Soll for callers that do not
	// need absence, session, or balance data.
	GetDailyTargets(ctx context.Context, staffID int64, from, to timezone.Date) ([]DailyTarget, error)
	// GetRangeAggregate sums Soll, Ist and balance over an arbitrary closed
	// range with the same daily arithmetic as the Monatskarte, WITHOUT a carry
	// (a range is not an account). The week view of the Leitungs-Dashboard
	// needs it because the chain itself is month-granular (#1417).
	GetRangeAggregate(ctx context.Context, staffID int64, from, to timezone.Date) (*RangeAggregate, error)
	// SetHolidayReader injects the public-holiday resolver (#1418 3a) —
	// wired in the factory after construction, like
	// WorkSessionService.SetStaffShiftRepo.
	SetHolidayReader(reader HolidayDatesReader)
	// SetAdjustmentReader injects the balance-adjustment reader (#1420) —
	// same after-construction wiring; nil keeps the pre-adjustment behavior
	// so existing unit fixtures stay valid.
	SetAdjustmentReader(reader monthAdjustmentReader)
	// SetSnapshotReader injects the frozen-month reader (#1417) — same
	// after-construction wiring; nil means no month is ever treated as frozen,
	// so existing unit fixtures stay valid.
	SetSnapshotReader(reader monthSnapshotReader)
}

// AdjustmentView is one Stundenkonto transaction as the Monatskarte shows it
// (#1420). MinutesDelta is signed (reductions negative).
type AdjustmentView struct {
	ID            int64         `json:"id"`
	Type          string        `json:"type"`
	MinutesDelta  int           `json:"minutes_delta"`
	EffectiveDate timezone.Date `json:"effective_date"`
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
	Date           timezone.Date `json:"date"`
	TargetMinutes  int           `json:"target_minutes"`
	CreditMinutes  int           `json:"credit_minutes"`
	ActualMinutes  int           `json:"actual_minutes"`
	BalanceMinutes int           `json:"balance_minutes"`
}

// DailyTarget is the contractual Soll of one calendar day, resolved against
// the schedule version that was valid ON that day (#1842).
type DailyTarget struct {
	Date          timezone.Date `json:"date"`
	TargetMinutes int           `json:"target_minutes"`
}

// The narrow dependency surfaces below are exactly what the month math
// needs; the full repositories satisfy them, and unit tests mock them
// without dragging in the complete repository interfaces.

// monthSettingsResolver is implemented by config.SettingsService.
type monthSettingsResolver interface {
	ResolveString(ctx context.Context, key string) (string, error)
}

// monthShiftReader is implemented by schedule.StaffShiftRepository.
type monthShiftReader interface {
	FindByStaffAndDateRange(ctx context.Context, staffID int64, start, end timezone.Date) ([]*scheduleModels.StaffShift, error)
}

// monthSessionReader is implemented by active.WorkSessionRepository.
type monthSessionReader interface {
	ListOverlappingByStaffID(ctx context.Context, staffID int64, from time.Time, to *time.Time) ([]*activeModels.WorkSession, error)
}

// monthBreakReader is implemented by active.WorkSessionBreakRepository.
type monthBreakReader interface {
	GetBySessionID(ctx context.Context, sessionID int64) ([]*activeModels.WorkSessionBreak, error)
	GetBySessionIDs(ctx context.Context, sessionIDs []int64) (map[int64][]*activeModels.WorkSessionBreak, error)
}

// monthAbsenceReader is implemented by active.StaffAbsenceRepository.
type monthAbsenceReader interface {
	GetByStaffAndDateRange(ctx context.Context, staffID int64, from, to timezone.Date) ([]*activeModels.StaffAbsence, error)
}

// monthAdjustmentReader is implemented by active.StaffBalanceAdjustmentRepository.
type monthAdjustmentReader interface {
	GetByStaffAndDateRange(ctx context.Context, staffID int64, from, to timezone.Date) ([]*activeModels.StaffBalanceAdjustment, error)
}

// monthSnapshotReader is implemented by
// active.StaffMonthBalanceSnapshotRepository (#1417).
type monthSnapshotReader interface {
	GetLatestClosedThrough(ctx context.Context, staffID int64, year, month int) (*activeModels.StaffMonthBalanceSnapshot, error)
}

// monthStaffReader is implemented by users.StaffRepository.
type monthStaffReader interface {
	FindByID(ctx context.Context, id any) (*userModels.Staff, error)
}

// monthScheduleReader is implemented by config.StaffWorkScheduleRepository.
type monthScheduleReader interface {
	FindByStaffIDsValidInRange(ctx context.Context, staffIDs []int64, from, to timezone.Date) ([]*configModels.StaffWorkSchedule, error)
	HasScheduleHistory(ctx context.Context, staffID int64) (bool, error)
}

// monthModelReader is implemented by config.WorkTimeModelRepository.
type monthModelReader interface {
	FindByID(ctx context.Context, id int64) (*configModels.WorkTimeModel, error)
}

// HolidayDatesReader is implemented by schedule.HolidayService. Public
// holidays zero the Soll of their day (#1418 3a). Exported because the
// factory injects it across service boundaries (month + session service).
type HolidayDatesReader interface {
	HolidayDates(ctx context.Context, from, to timezone.Date) (map[timezone.Date]bool, error)
}

type workTimeMonthService struct {
	sessionRepo    monthSessionReader
	breakRepo      monthBreakReader
	absenceRepo    monthAbsenceReader
	staffRepo      monthStaffReader
	scheduleRepo   monthScheduleReader
	workModelRepo  monthModelReader
	shiftRepo      monthShiftReader
	settings       monthSettingsResolver
	holidayReader  HolidayDatesReader
	adjustmentRepo monthAdjustmentReader
	snapshotRepo   monthSnapshotReader
	logger         *slog.Logger

	// todayFunc is a test hook; production uses timezone.TodayDate.
	todayFunc func() timezone.Date
	// nowFunc is a test hook; production uses time.Now.
	nowFunc func() time.Time
}

func NewWorkTimeMonthService(
	sessionRepo monthSessionReader,
	breakRepo monthBreakReader,
	absenceRepo monthAbsenceReader,
	staffRepo monthStaffReader,
	scheduleRepo monthScheduleReader,
	workModelRepo monthModelReader,
	shiftRepo monthShiftReader,
	settings monthSettingsResolver,
	logger *slog.Logger,
) WorkTimeMonthService {
	return &workTimeMonthService{
		sessionRepo:   sessionRepo,
		breakRepo:     breakRepo,
		absenceRepo:   absenceRepo,
		staffRepo:     staffRepo,
		scheduleRepo:  scheduleRepo,
		workModelRepo: workModelRepo,
		shiftRepo:     shiftRepo,
		settings:      settings,
		logger:        logger,
	}
}

// SetHolidayReader wires the public-holiday resolver (#1418 3a). A setter
// like SetStaffShiftRepo: nil keeps the pre-holiday behavior, so existing
// unit fixtures stay valid.
func (s *workTimeMonthService) SetHolidayReader(reader HolidayDatesReader) {
	s.holidayReader = reader
}

// SetAdjustmentReader wires the Stundenkonto transaction reader (#1420). A
// setter like SetHolidayReader: nil means no adjustments enter the balance,
// so existing unit fixtures stay valid.
func (s *workTimeMonthService) SetAdjustmentReader(reader monthAdjustmentReader) {
	s.adjustmentRepo = reader
}

// SetSnapshotReader wires the frozen-month reader (#1417). A setter like
// SetAdjustmentReader: nil means no month is ever frozen, so the carry chain
// behaves exactly as it did before the Monatsabschluss existed and existing
// unit fixtures stay valid.
func (s *workTimeMonthService) SetSnapshotReader(reader monthSnapshotReader) {
	s.snapshotRepo = reader
}

func (s *workTimeMonthService) today() timezone.Date {
	if s.todayFunc != nil {
		return s.todayFunc()
	}
	return timezone.TodayDate()
}

func (s *workTimeMonthService) now() time.Time {
	if s.nowFunc != nil {
		return s.nowFunc()
	}
	return time.Now()
}

// ---- month arithmetic ----------------------------------------------------

type monthKey struct {
	Year  int
	Month int
}

func monthOf(d timezone.Date) monthKey { return monthKey{Year: d.Year, Month: int(d.Month)} }

func (k monthKey) firstDay() timezone.Date { return timezone.NewDate(k.Year, time.Month(k.Month), 1) }

// lastDay relies on NewDate's time.Date normalization for month 13.
func (k monthKey) lastDay() timezone.Date {
	return timezone.NewDate(k.Year, time.Month(k.Month)+1, 1).AddDays(-1)
}

// addMonths shifts a key by n whole months, normalizing via time.Date.
func (k monthKey) addMonths(n int) monthKey {
	return monthOf(timezone.NewDate(k.Year, time.Month(k.Month+n), 1))
}

func (k monthKey) next() monthKey {
	if k.Month == 12 {
		return monthKey{Year: k.Year + 1, Month: 1}
	}
	return monthKey{Year: k.Year, Month: k.Month + 1}
}

func (k monthKey) before(o monthKey) bool {
	return k.Year < o.Year || (k.Year == o.Year && k.Month < o.Month)
}

// maxDailyTargetRangeDays bounds GetDailyProjection: the table never shows more
// than a month, and an unbounded range would let a caller walk years of
// calendar days per request.
const maxDailyTargetRangeDays = 366

// ErrInvalidTargetRange marks a GetDailyProjection range the caller got wrong:
// missing, inverted, or longer than the cap. The range comes straight from the
// query string, so handlers map this to HTTP 400 — an oversized window is the
// caller's mistake, not an internal failure.
var ErrInvalidTargetRange = errors.New("invalid target range")

// maxFutureMonths bounds how far ahead a Monatskarte may be requested. The
// carry chain walks every month from the account start up to the requested
// one, so the request itself sizes the work: without this bound a caller
// asking for 2100-12 makes the server aggregate — and query — every day since
// the account start, decades of them, for a card nobody can read.
//
// The bound is in the FUTURE direction only. A month before the account start
// is summarized standalone (one month, no chain), and the current month is the
// last one that holds real data; the year of headroom exists so month-by-month
// forward navigation keeps working against planned shifts and absences.
const maxFutureMonths = 12

// ErrMonthOutOfRange marks a Monatskarte request too far in the future. Like
// ErrInvalidTargetRange it comes from the query string, so handlers map it to
// HTTP 400.
var ErrMonthOutOfRange = errors.New("month out of range")

// maxCarryChainMonths bounds how far BACK the live carry chain may reach. The
// requested month sizes the future work (maxFutureMonths); the account start
// setting sizes the past work, and unlike the request it is not sanitized by a
// range check — a tenant admin can set operations.time_tracking_account_start_date
// to any valid date. A value years or decades back would make every
// current-month card (re)aggregate and (re)query every intervening day, and the
// open-month poll would trigger that decades-wide walk on a fixed interval.
//
// The chain therefore never starts earlier than this many months before the
// requested month, capping the per-request work at a constant regardless of the
// configured start. Ten years is far beyond any real Stundenkonto this product
// can hold (schools onboard within its lifetime), so a legitimate account is
// never affected; a start older than the bound is a misconfiguration and is
// rejected (ErrAccountStartTooOld) rather than silently clamped — clamping would
// drop the balance of every month before the floor and present a materially
// wrong opening/closing Stundenkonto as authoritative.
const maxCarryChainMonths = 120

// ErrAccountStartTooOld marks an account-start setting so far back that the live
// carry chain would exceed maxCarryChainMonths. Rather than clamp the chain to
// the floor — which silently discards the Soll/Ist/credits of every earlier
// month and yields a wrong opening and closing balance — the request fails so
// the misconfiguration surfaces and gets corrected. No real Stundenkonto reaches
// back this far, so a legitimate account never triggers it. Handlers map this to
// HTTP 500: it is a server-side configuration fault, not a caller mistake.
var ErrAccountStartTooOld = errors.New("account start predates the carry-chain bound")

func validateMonth(year, month int) error {
	if year < 2000 || year > 2100 || month < 1 || month > 12 {
		return fmt.Errorf("invalid month %d-%d", year, month)
	}
	return nil
}

// ---- aggregation ----------------------------------------------------------

// monthAggregates holds the per-month raw numbers before carry chaining.
type monthAggregates struct {
	target           int
	targetToDate     int
	actual           int
	creditedSick     int
	creditedVacation int
	creditedTraining int
	creditedOther    int
	sickDays         float64
	vacationDays     float64
	trainingDays     float64
	hasShifts        bool
	plannedShift     int
	adjustment       int
	adjustments      []*activeModels.StaffBalanceAdjustment
}

func (a *monthAggregates) creditedTotal() int {
	return a.creditedSick + a.creditedVacation + a.creditedTraining + a.creditedOther
}

func (a *monthAggregates) balance() int {
	return a.actual + a.creditedTotal() + a.adjustment - a.targetToDate
}

// dailyTargetResolver resolves the contractual Soll for single days: a
// date-valid staff_work_schedules row set wins, the assigned work-time model
// is the fallback — the same two-tier resolution the weekly summaries use.
type dailyTargetResolver struct {
	entries     []*configModels.StaffWorkSchedule
	staffAnchor *timezone.Date
	model       *configModels.WorkTimeModel
	modelAnchor timezone.Date
	holidays    map[timezone.Date]bool
}

func (r *dailyTargetResolver) targetFor(d timezone.Date) int {
	// Public holidays zero the Soll regardless of the schedule (§2 EntgFG:
	// the contractual hours of that day simply fall away, #1418 3a). This
	// also keeps absence credits at 0 on holidays — no double counting.
	if r.holidays[d] {
		return 0
	}
	if len(r.entries) > 0 {
		target, _ := configModels.DailyTargetFromSchedule(r.entries, r.staffAnchor, d)
		return target
	}
	if r.model != nil {
		target, _ := configModels.DailyTargetFromModel(r.model, r.modelAnchor, d)
		return target
	}
	return 0
}

func (s *workTimeMonthService) buildTargetResolver(ctx context.Context, staffID int64, from, to timezone.Date) (*dailyTargetResolver, error) {
	resolver := &dailyTargetResolver{}

	if s.holidayReader != nil {
		holidaySet, err := s.holidayReader.HolidayDates(ctx, from, to)
		if err != nil {
			return nil, fmt.Errorf("failed to load public holidays: %w", err)
		}
		resolver.holidays = holidaySet
	}

	staff, err := s.staffRepo.FindByID(ctx, staffID)
	if err != nil {
		return nil, fmt.Errorf("failed to load staff for month summary: %w", err)
	}
	resolver.staffAnchor = staff.RotationAnchorDate

	entries, err := s.scheduleRepo.FindByStaffIDsValidInRange(ctx, []int64{staffID}, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to load work schedules: %w", err)
	}
	resolver.entries = entries
	if len(entries) > 0 || staff.WorkTimeModelID == nil {
		return resolver, nil
	}

	// No snapshot is valid in this range. The assigned work-time model may only
	// stand in when the staff member has NO snapshot at all: if versions exist
	// but none covers these days, the range lies before the first one (or after
	// the schedule was emptied) and the Soll is genuinely zero. Applying the
	// current model there would charge Soll for days the schedule didn't exist
	// yet — and would make the answer depend on the query window, since a range
	// that also touches the first snapshot resolves the very same days to zero.
	hasHistory, err := s.scheduleRepo.HasScheduleHistory(ctx, staffID)
	if err != nil {
		return nil, fmt.Errorf("failed to check schedule history: %w", err)
	}
	if hasHistory {
		return resolver, nil
	}

	model, err := s.workModelRepo.FindByID(ctx, *staff.WorkTimeModelID)
	if err != nil {
		return nil, fmt.Errorf("failed to load work time model: %w", err)
	}
	resolver.model = model
	resolver.modelAnchor = model.RotationAnchorDate
	if staff.RotationAnchorDate != nil {
		resolver.modelAnchor = *staff.RotationAnchorDate
	}
	return resolver, nil
}

// computeAggregates builds the raw per-month numbers for every month from
// `first`'s month up to toKey with one set of range queries. `first` is a
// date, not a month: the account start may fall mid-month, and then the
// anchor month must only count from that day on. Days after `today`
// contribute nothing to targetToDate and credits, so the current month's
// balance is automatically pro-rated and future months are neutral.
func (s *workTimeMonthService) computeAggregates(ctx context.Context, staffID int64, first timezone.Date, toKey monthKey, today timezone.Date) (map[monthKey]*monthAggregates, error) {
	fromKey := monthOf(first)
	last := toKey.lastDay()

	aggregates := make(map[monthKey]*monthAggregates)
	for k := fromKey; !toKey.before(k); k = k.next() {
		aggregates[k] = &monthAggregates{}
	}

	resolver, err := s.buildTargetResolver(ctx, staffID, first, last)
	if err != nil {
		return nil, err
	}
	for d := first; !d.After(last); d = d.AddDays(1) {
		target := resolver.targetFor(d)
		agg := aggregates[monthOf(d)]
		agg.target += target
		if !d.After(today) {
			agg.targetToDate += target
		}
	}

	if err := s.addActualMinutes(ctx, staffID, first, last, today, aggregates); err != nil {
		return nil, err
	}
	if err := s.addAbsenceCredits(ctx, staffID, first, last, today, resolver, aggregates); err != nil {
		return nil, err
	}
	if err := s.addPlannedShifts(ctx, staffID, first, last, aggregates); err != nil {
		return nil, err
	}
	if err := s.addAdjustments(ctx, staffID, first, last, today, aggregates); err != nil {
		return nil, err
	}
	return aggregates, nil
}

// addAdjustments folds the Stundenkonto transactions (#1420) into their
// effective month. Future-dated adjustments are skipped like future targets
// and credits — a grant scheduled for next week must not pre-empt today's
// balance; the carry chain picks it up once the date arrives.
func (s *workTimeMonthService) addAdjustments(ctx context.Context, staffID int64, first, last, today timezone.Date, aggregates map[monthKey]*monthAggregates) error {
	if s.adjustmentRepo == nil {
		return nil
	}
	adjustments, err := s.adjustmentRepo.GetByStaffAndDateRange(ctx, staffID, first, last)
	if err != nil {
		return fmt.Errorf("failed to load balance adjustments: %w", err)
	}
	for _, adjustment := range adjustments {
		if adjustment.EffectiveDate.After(today) {
			continue
		}
		agg, ok := aggregates[monthOf(adjustment.EffectiveDate)]
		if !ok {
			continue
		}
		agg.adjustment += adjustment.MinutesDelta
		agg.adjustments = append(agg.adjustments, adjustment)
	}
	return nil
}

// addActualMinutes assigns each session's time to every Berlin calendar day it
// intersects. A work session is filed under its check-in date, but may validly
// run past midnight; counting it only in that filing month would lose the
// after-midnight part from the following day's balance.
func (s *workTimeMonthService) addActualMinutes(ctx context.Context, staffID int64, first, last, today timezone.Date, aggregates map[monthKey]*monthAggregates) error {
	end := last.AddDays(1).BerlinMidnight()
	sessions, err := s.sessionRepo.ListOverlappingByStaffID(ctx, staffID, first.BerlinMidnight(), &end)
	if err != nil {
		return fmt.Errorf("failed to load work sessions: %w", err)
	}
	now := s.now()
	breaksBySessionID, err := s.breaksBySessionID(ctx, sessions)
	if err != nil {
		return err
	}
	for _, session := range sessions {
		end := balanceSessionEnd(session, now, today)
		if !end.After(session.CheckInTime) {
			continue
		}

		for day, minutes := range netMinutesByDate(session, breaksBySessionID[session.ID], end, first, today) {
			if agg, ok := aggregates[monthOf(day)]; ok {
				agg.actual += minutes
			}
		}
	}
	return nil
}

func (s *workTimeMonthService) breaksBySessionID(ctx context.Context, sessions []*activeModels.WorkSession) (map[int64][]*activeModels.WorkSessionBreak, error) {
	ids := make([]int64, 0, len(sessions))
	for _, session := range sessions {
		ids = append(ids, session.ID)
	}
	if len(ids) == 0 {
		return map[int64][]*activeModels.WorkSessionBreak{}, nil
	}
	breaks, err := s.breakRepo.GetBySessionIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("failed to load work session breaks: %w", err)
	}
	return breaks, nil
}

// sessionEndUpTo clamps a session end at now. This prevents a future
// check-out from being credited before it happens while preserving valid work
// across Berlin midnight.
func sessionEndUpTo(session *activeModels.WorkSession, now time.Time) time.Time {
	end := now
	if session.CheckOutTime != nil && session.CheckOutTime.Before(end) {
		end = *session.CheckOutTime
	}
	return end
}

// maxOpenWorkSessionDuration is the shared live limit. The presence lookup in
// database/repositories/active reads the same constant, so a block that stops
// counting toward the balance stops marking its owner present as well.
const maxOpenWorkSessionDuration = activeModels.MaxOpenWorkSessionDuration

// BalanceSessionEnd applies the live balance limit using the Berlin day of
// now. Readers outside this package (notably the kiosk) use it so a running
// block cannot produce a different total from the monthly balance.
func BalanceSessionEnd(session *activeModels.WorkSession, now time.Time) time.Time {
	return balanceSessionEnd(session, now, timezone.DateFromTime(now))
}

func balanceSessionEnd(session *activeModels.WorkSession, now time.Time, today timezone.Date) time.Time {
	end := sessionEndUpTo(session, now)
	// A completed block is historical fact. Only still-open blocks (or a
	// malformed future check-out) need a live safety limit.
	if session.CheckOutTime != nil && !session.CheckOutTime.After(now) {
		return end
	}
	maxEnd := session.CheckInTime.Add(maxOpenWorkSessionDuration)
	if !end.After(maxEnd) {
		return end
	}
	if session.Date.Before(today.AddDays(-1)) {
		if staleEnd := session.Date.EndOfDay(); staleEnd.Before(end) {
			return staleEnd
		}
	}
	if !session.Date.Before(today.AddDays(-1)) && session.Date.Before(today) {
		return maxEnd
	}
	return end
}

// addAbsenceCredits credits sick/vacation/training days with the day's
// contractual target (Ansatz B, mirroring the frontend engine): half-day
// boundaries credit floor(target/2), overlapping absences credit a day only
// once (lowest absence ID wins), and only reported/approved absences count.
// Day counters are schedule-aware: a vacation day only counts when the day
// actually had a target (> 0), so part-timers aren't charged for days off.
func (s *workTimeMonthService) addAbsenceCredits(ctx context.Context, staffID int64, first, last, today timezone.Date, resolver *dailyTargetResolver, aggregates map[monthKey]*monthAggregates) error {
	absences, err := s.absenceRepo.GetByStaffAndDateRange(ctx, staffID, first, last)
	if err != nil {
		return fmt.Errorf("failed to load absences: %w", err)
	}
	through := last
	if today.Before(through) {
		through = today
	}
	walkCreditedAbsenceDays(absences, first, through, resolver.targetFor,
		func(d timezone.Date, absence *activeModels.StaffAbsence, credit int, fraction float64) {
			creditAbsenceDay(absence, credit, fraction, aggregates[monthOf(d)])
		})
	return nil
}

// absenceDayVisitor receives one effectively covered absence day together with
// the minutes it credits (already 0 for Freizeitausgleich, already halved on a
// half-day boundary) and the day fraction that credit represents.
type absenceDayVisitor func(d timezone.Date, absence *activeModels.StaffAbsence, credit int, fraction float64)

// walkCreditedAbsenceDays applies the ONE set of absence rules every reader
// prices days with — Monatskarte, Tageszeile, Stundenkonto: only
// reported/approved absences count, a day is consumed by the LOWEST absence ID
// covering it (so overlapping absences never pay twice), a day without Soll is
// worth nothing, a half-day boundary is worth half the Soll, and
// Freizeitausgleich consumes its day while crediting nothing (#1420 5b).
//
// `through` is the last creditable day: the Monatskarte passes today (a future
// day earns no credit yet), the reduction-capacity math passes the range end
// (a planned comp-time day must still reserve its Soll). Callers that used to
// carry their own copy of this walk drifted apart in exactly these details.
func walkCreditedAbsenceDays(
	absences []*activeModels.StaffAbsence,
	first, through timezone.Date,
	targetFor func(timezone.Date) int,
	visit absenceDayVisitor,
) {
	sort.Slice(absences, func(i, j int) bool { return absences[i].ID < absences[j].ID })
	credited := make(map[timezone.Date]bool)
	for _, absence := range absences {
		if absence.Status != activeModels.AbsenceStatusReported && absence.Status != activeModels.AbsenceStatusApproved {
			continue
		}
		startHalf, endHalf := effectiveAbsenceBoundaryHalves(absence)
		for d := absence.DateStart; !d.After(absence.DateEnd); d = d.AddDays(1) {
			if d.Before(first) || d.After(through) || credited[d] {
				continue
			}
			credited[d] = true
			target := targetFor(d)
			if target <= 0 {
				continue
			}
			fraction := 1.0
			credit := target
			if isHalfAbsenceDay(absence, d, startHalf, endHalf) {
				fraction = 0.5
				credit = target / 2
			}
			if absence.AbsenceType == activeModels.AbsenceTypeCompTime {
				// Freizeitausgleich (#1420 5b) credits NOTHING: the day keeps
				// its Soll, so the balance drops by the day's target. The day
				// still counts as consumed above, so an overlapping vacation
				// cannot re-credit it.
				credit = 0
			}
			visit(d, absence, credit, fraction)
		}
	}
}

// creditAbsenceDay books one credited absence day into its month's counters.
// The credit itself is already resolved by walkCreditedAbsenceDays; this only
// routes it to the Lohnart the month card reports.
func creditAbsenceDay(absence *activeModels.StaffAbsence, credit int, fraction float64, agg *monthAggregates) {
	if agg == nil {
		return
	}
	switch absence.AbsenceType {
	case activeModels.AbsenceTypeSick:
		agg.creditedSick += credit
		agg.sickDays += fraction
	case activeModels.AbsenceTypeVacation:
		agg.creditedVacation += credit
		agg.vacationDays += fraction
	case activeModels.AbsenceTypeTraining:
		// Fortbildung split (#1417 2b writers): a Lohnart "Fortbildung"
		// needs hours OR days separately from "Sonstige" — same credit
		// math, own counters.
		agg.creditedTraining += credit
		agg.trainingDays += fraction
	case activeModels.AbsenceTypeCompTime:
		// Nothing to credit (see walkCreditedAbsenceDays); the type still
		// needs its own case so it does not land in "Sonstige".
	default:
		agg.creditedOther += credit
	}
}

// effectiveAbsenceBoundaryHalves mirrors the absence service semantics: a
// bare half_day flag with no explicit boundary flags means both boundaries
// are half days.
func effectiveAbsenceBoundaryHalves(absence *activeModels.StaffAbsence) (bool, bool) {
	if absence.HalfDay && !absence.StartHalfDay && !absence.EndHalfDay {
		return true, true
	}
	return absence.StartHalfDay, absence.EndHalfDay
}

func isHalfAbsenceDay(absence *activeModels.StaffAbsence, d timezone.Date, startHalf, endHalf bool) bool {
	if absence.DateStart == absence.DateEnd {
		return d == absence.DateStart && (startHalf || endHalf)
	}
	return (d == absence.DateStart && startHalf) || (d == absence.DateEnd && endHalf)
}

func (s *workTimeMonthService) addPlannedShifts(ctx context.Context, staffID int64, first, last timezone.Date, aggregates map[monthKey]*monthAggregates) error {
	shifts, err := s.shiftRepo.FindByStaffAndDateRange(ctx, staffID, first, last)
	if err != nil {
		return fmt.Errorf("failed to load staff shifts: %w", err)
	}
	for _, shift := range shifts {
		agg, ok := aggregates[monthOf(shift.Date)]
		if !ok {
			continue
		}
		agg.hasShifts = true
		if !shift.Cancelled {
			agg.plannedShift += shiftNetMinutes(shift)
		}
	}
	return nil
}

// shiftNetMinutes is the planned working time of one shift: wall-clock span
// minus break, floored at 0. Local copy of the (unexported) helper in
// services/schedule — that package imports this one, so it cannot be shared
// without an import cycle.
func shiftNetMinutes(shift *scheduleModels.StaffShift) int {
	start := timezone.WallClock(shift.StartTime)
	end := timezone.WallClock(shift.EndTime)
	minutes := int(end.Sub(start)/time.Minute) - shift.BreakMinutes
	if minutes < 0 {
		return 0
	}
	return minutes
}

// ---- carry chain + summary ---------------------------------------------------

// chainAnchor determines where the live carry computation starts: the
// configured account start date, or January 1st of the summarized month's
// year as the stable fallback (the frontend defaults its Stundenkonto to
// Jan 1 the same way).
//
// The exact day matters — "Stundenkonto ab Datum" with 2026-06-15 means the
// account starts on the 15th, so June 1–14 must not enter any balance. The
// day is therefore preserved here and used as the lower bound of the anchor
// month, mirroring the frontend engine (resolveAccountStartDate).
//
// A settings read failure is returned, never swallowed: the account start is
// required input for the carry, and falling back to January would present a
// materially wrong Stundenkonto as authoritative. An unparsable value is a
// different case — it falls back with a warning, matching both the frontend
// and the fact that the settings API validates the date on write.
func (s *workTimeMonthService) chainAnchor(ctx context.Context, key monthKey) (timezone.Date, error) {
	return resolveAccountAnchor(ctx, s.settings, s.getLogger(), key)
}

// resolveAccountAnchor is the shared anchor resolution (see chainAnchor doc
// above). Package-level so the balance-adjustment service (#1420) can reject
// bookings dated before the account start — an adjustment before the anchor
// never enters any carry chain and would silently not move the Stundenkonto.
func resolveAccountAnchor(ctx context.Context, settings monthSettingsResolver, logger *slog.Logger, key monthKey) (timezone.Date, error) {
	fallback := timezone.NewDate(key.Year, time.January, 1)
	if settings == nil {
		return fallback, nil
	}
	value, err := settings.ResolveString(ctx, configModels.KeyTimeTrackingAccountStartDate)
	if err != nil {
		return timezone.Date{}, fmt.Errorf("failed to resolve account start date setting: %w", err)
	}
	if value == "" {
		return fallback, nil
	}
	start, err := timezone.ParseDate(value)
	if err != nil {
		logger.Warn("invalid account start date setting, using january fallback",
			"value", value,
			"error", err.Error(),
		)
		return fallback, nil
	}
	return start, nil
}

// GetMonthSummary computes the Monatskarte on read: the Übertrag is the sum
// of all month balances from the account-start anchor up to the previous
// month — live, so corrections in past months flow through automatically.
func (s *workTimeMonthService) GetMonthSummary(ctx context.Context, staffID int64, year, month int) (*MonthSummary, error) {
	if err := validateMonth(year, month); err != nil {
		return nil, err
	}
	key := monthKey{Year: year, Month: month}
	today := s.today()
	if monthOf(today).addMonths(maxFutureMonths).before(key) {
		return nil, fmt.Errorf("%w: %d-%02d is more than %d months ahead", ErrMonthOutOfRange, year, month, maxFutureMonths)
	}
	return s.getMonthSummaryThrough(ctx, staffID, key, today)
}

// GetMonthSummaryAtMonthEnd computes the card with the clamp pinned to the
// month's last day rather than today (#1417). See the interface doc: only the
// close path may use it, and only for months that are already over.
func (s *workTimeMonthService) GetMonthSummaryAtMonthEnd(ctx context.Context, staffID int64, year, month int) (*MonthSummary, error) {
	if err := validateMonth(year, month); err != nil {
		return nil, err
	}
	key := monthKey{Year: year, Month: month}
	return s.getMonthSummaryThrough(ctx, staffID, key, key.lastDay())
}

// GetRangeAggregate sums an arbitrary closed range from the SAME helpers the
// Monatskarte and the daily table use. It first clamps the whole range to the
// account anchor, then uses the same date-valid resolver for the Soll,
// getDailyActualMinutes for the Ist (day cap, now cap, running break), and
// getDailyBalanceDeltas for the balance (credits, adjustments, target). No new
// arithmetic, therefore no way for a week total to contradict the month card
// above it.
func (s *workTimeMonthService) GetRangeAggregate(ctx context.Context, staffID int64, from, to timezone.Date) (*RangeAggregate, error) {
	if from.IsZero() || to.IsZero() || to.Before(from) {
		return nil, errors.New("from and to must form a valid range")
	}
	anchor, err := s.chainAnchor(ctx, monthOf(to))
	if err != nil {
		return nil, err
	}
	aggregate := &RangeAggregate{}
	if to.Before(anchor) {
		return aggregate, nil
	}
	if from.Before(anchor) {
		from = anchor
	}
	resolver, targetAnchor, err := s.accountAwareTargets(ctx, staffID, from, to)
	if err != nil {
		return nil, err
	}
	for d := from; !d.After(to); d = d.AddDays(1) {
		if excludedByAccountStart(d, targetAnchor) {
			continue
		}
		aggregate.TargetMinutes += resolver.targetFor(d)
	}
	actual, err := s.getDailyActualMinutes(ctx, staffID, from, to)
	if err != nil {
		return nil, err
	}
	for _, minutes := range actual {
		aggregate.ActualMinutes += minutes
	}
	deltas, err := s.getDailyBalanceDeltas(ctx, staffID, from, to)
	if err != nil {
		return nil, err
	}
	for _, delta := range deltas {
		aggregate.BalanceMinutes += delta
	}
	return aggregate, nil
}

// GetClosingBalanceAsOf computes the carry-chain balance at the end of cutoff.
// It shares all target/session/absence/adjustment math with the Monatskarte, but
// pins the "today" clamp to cutoff so a historical reset cannot absorb activity
// that happened later in the same month.
func (s *workTimeMonthService) GetClosingBalanceAsOf(ctx context.Context, staffID int64, cutoff timezone.Date) (int, error) {
	if cutoff.IsZero() {
		return 0, errors.New("cutoff date is required")
	}
	key := monthOf(cutoff)
	summary, err := s.getMonthSummaryThrough(ctx, staffID, key, cutoff)
	if err != nil {
		return 0, err
	}
	return summary.ClosingBalanceMinutes, nil
}

// GetBalanceAdjustmentMinutes returns the signed ledger effect over a date
// range without folding in work sessions, targets, or absences from those days.
func (s *workTimeMonthService) GetBalanceAdjustmentMinutes(ctx context.Context, staffID int64, from, to timezone.Date) (int, error) {
	if from.IsZero() || to.IsZero() || to.Before(from) {
		return 0, errors.New("from and to must form a valid range")
	}
	if s.adjustmentRepo == nil {
		return 0, nil
	}
	adjustments, err := s.adjustmentRepo.GetByStaffAndDateRange(ctx, staffID, from, to)
	if err != nil {
		return 0, fmt.Errorf("failed to load balance adjustments: %w", err)
	}
	total := 0
	for _, adjustment := range adjustments {
		total += adjustment.MinutesDelta
	}
	return total, nil
}

// GetBalanceReductionCapacity treats adjustments as start-of-day ledger
// entries. It checks the opening balance on effectiveDate, every later debit
// already in the ledger, current balance for backdated entries, and every
// effective comp-time absence commitment through the supported future
// horizon. The caller holds the shared staff-balance lock, so these reads
// cannot race another balance-affecting write.
func (s *workTimeMonthService) GetBalanceReductionCapacity(ctx context.Context, staffID int64, effectiveDate timezone.Date) (int, error) {
	if staffID <= 0 || effectiveDate.IsZero() {
		return 0, errors.New("staff id and effective date are required")
	}

	today := s.today()
	last := monthOf(today).addMonths(maxFutureMonths).lastDay()
	if effectiveDate.After(last) {
		return 0, fmt.Errorf("effective date %s is outside the supported balance horizon", effectiveDate.String())
	}

	checkpointSet := map[timezone.Date]struct{}{effectiveDate: {}}
	if err := s.addAdjustmentReductionCheckpoints(ctx, staffID, effectiveDate, last, checkpointSet); err != nil {
		return 0, err
	}
	compTimeFrom := effectiveDate
	if tomorrow := today.AddDays(1); effectiveDate.After(today) {
		compTimeFrom = tomorrow
	}
	compTimeAbsences, err := s.addCompTimeReductionCheckpoints(ctx, staffID, compTimeFrom, last, checkpointSet)
	if err != nil {
		return 0, err
	}

	checkpoints := make([]timezone.Date, 0, len(checkpointSet))
	for checkpoint := range checkpointSet {
		checkpoints = append(checkpoints, checkpoint)
	}
	sort.Slice(checkpoints, func(i, j int) bool {
		return checkpoints[i].Before(checkpoints[j])
	})

	capacity, err := s.minimumOpeningReductionCapacity(ctx, staffID, checkpoints, compTimeAbsences, today)
	if err != nil {
		return 0, err
	}

	if !effectiveDate.After(today) {
		currentBalance, err := s.GetClosingBalanceAsOf(ctx, staffID, today)
		if err != nil {
			return 0, fmt.Errorf("failed to compute current balance for reduction validation: %w", err)
		}
		capacity = min(capacity, currentBalance)
		minimumHistoricalClosing, err := s.getMinimumDailyClosingBalance(ctx, staffID, effectiveDate, today)
		if err != nil {
			return 0, err
		}
		capacity = min(capacity, minimumHistoricalClosing)
	}

	return capacity, nil
}

func (s *workTimeMonthService) addAdjustmentReductionCheckpoints(
	ctx context.Context,
	staffID int64,
	effectiveDate, last timezone.Date,
	checkpoints map[timezone.Date]struct{},
) error {
	if s.adjustmentRepo == nil {
		return nil
	}
	adjustments, err := s.adjustmentRepo.GetByStaffAndDateRange(ctx, staffID, effectiveDate, last)
	if err != nil {
		return fmt.Errorf("failed to load balance adjustments for reduction validation: %w", err)
	}
	for _, adjustment := range adjustments {
		if adjustment.MinutesDelta < 0 && adjustment.EffectiveDate.After(effectiveDate) {
			checkpoints[adjustment.EffectiveDate] = struct{}{}
		}
	}
	return nil
}

func (s *workTimeMonthService) addCompTimeReductionCheckpoints(
	ctx context.Context,
	staffID int64,
	effectiveDate, last timezone.Date,
	checkpoints map[timezone.Date]struct{},
) ([]*activeModels.StaffAbsence, error) {
	if s.absenceRepo == nil {
		return nil, nil
	}
	absences, err := s.absenceRepo.GetByStaffAndDateRange(ctx, staffID, effectiveDate, last)
	if err != nil {
		return nil, fmt.Errorf("failed to load comp-time absences for reduction validation: %w", err)
	}
	compTimeAbsences := make([]*activeModels.StaffAbsence, 0, len(absences))
	for _, absence := range absences {
		if absence.AbsenceType != activeModels.AbsenceTypeCompTime ||
			(absence.Status != activeModels.AbsenceStatusReported && absence.Status != activeModels.AbsenceStatusApproved) {
			continue
		}
		compTimeAbsences = append(compTimeAbsences, absence)
		checkpoint := absence.DateStart
		if checkpoint.Before(effectiveDate) {
			checkpoint = effectiveDate
		}
		checkpoints[checkpoint] = struct{}{}
	}
	return compTimeAbsences, nil
}

func (s *workTimeMonthService) minimumOpeningReductionCapacity(
	ctx context.Context,
	staffID int64,
	checkpoints []timezone.Date,
	compTimeAbsences []*activeModels.StaffAbsence,
	today timezone.Date,
) (int, error) {
	capacity := int(^uint(0) >> 1)
	for _, checkpoint := range checkpoints {
		available, err := s.getOpeningBalanceOnDate(ctx, staffID, checkpoint, today, compTimeAbsences)
		if err != nil {
			return 0, err
		}
		commitment, err := s.getRemainingCompTimeCommitment(ctx, staffID, checkpoint, compTimeAbsences)
		if err != nil {
			return 0, err
		}
		capacity = min(capacity, available-commitment)
	}
	return capacity, nil
}

func (s *workTimeMonthService) getOpeningBalanceOnDate(
	ctx context.Context,
	staffID int64,
	date, today timezone.Date,
	compTimeAbsences []*activeModels.StaffAbsence,
) (int, error) {
	anchor, err := s.chainAnchor(ctx, monthOf(today))
	if err != nil {
		return 0, fmt.Errorf("failed to resolve account start for reduction validation: %w", err)
	}

	available := 0
	if date.After(today) {
		if !today.Before(anchor) {
			available, err = s.GetClosingBalanceAsOf(ctx, staffID, today)
			if err != nil {
				return 0, fmt.Errorf("failed to compute current balance for reduction validation: %w", err)
			}
		}
		futureAdjustments, err := s.GetBalanceAdjustmentMinutes(ctx, staffID, today.AddDays(1), date)
		if err != nil {
			return 0, fmt.Errorf("failed to compute adjustments through %s: %w", date.String(), err)
		}
		available += futureAdjustments
		priorCompTime, err := s.getCompTimeDeductionInRange(
			ctx,
			staffID,
			today.AddDays(1),
			date.AddDays(-1),
			compTimeAbsences,
		)
		if err != nil {
			return 0, err
		}
		return available - priorCompTime, nil
	}

	if cutoff := date.AddDays(-1); !cutoff.Before(anchor) {
		boundaries, err := s.frozenMonthBoundaries(ctx, staffID, date, date)
		if err != nil {
			return 0, fmt.Errorf("failed to load opening balance on %s: %w", date.String(), err)
		}
		if frozen, ok := boundaries[date]; ok {
			available = frozen
		} else {
			available, err = s.GetClosingBalanceAsOf(ctx, staffID, cutoff)
			if err != nil {
				return 0, fmt.Errorf("failed to compute opening balance on %s: %w", date.String(), err)
			}
		}
	}
	sameDayAdjustments, err := s.GetBalanceAdjustmentMinutes(ctx, staffID, date, date)
	if err != nil {
		return 0, fmt.Errorf("failed to compute adjustments on %s: %w", date.String(), err)
	}
	return available + sameDayAdjustments, nil
}

func (s *workTimeMonthService) getCompTimeDeductionInRange(
	ctx context.Context,
	staffID int64,
	from, to timezone.Date,
	absences []*activeModels.StaffAbsence,
) (int, error) {
	if to.Before(from) {
		return 0, nil
	}
	total := 0
	for _, absence := range absences {
		start := absence.DateStart
		if start.Before(from) {
			start = from
		}
		end := absence.DateEnd
		if end.After(to) {
			end = to
		}
		if end.Before(start) {
			continue
		}
		deduction, err := s.GetCompTimeDeductionMinutes(ctx, staffID, start, end, absence.HalfDay)
		if err != nil {
			return 0, fmt.Errorf("failed to compute comp-time commitment from %s: %w", start.String(), err)
		}
		total += deduction
	}
	return total, nil
}

// getMinimumDailyClosingBalance returns the lowest end-of-day balance in the
// historical range. A backdated debit lowers every closing balance from its
// effective date onward, so checking only that day's opening and today's
// closing can miss an intervening deficit that later work repaired.
func (s *workTimeMonthService) getMinimumDailyClosingBalance(
	ctx context.Context,
	staffID int64,
	from, to timezone.Date,
) (int, error) {
	anchor, err := s.chainAnchor(ctx, monthOf(to))
	if err != nil {
		return 0, fmt.Errorf("failed to resolve account start for daily reduction validation: %w", err)
	}

	balance := 0
	if cutoff := from.AddDays(-1); !cutoff.Before(anchor) {
		balance, err = s.GetClosingBalanceAsOf(ctx, staffID, cutoff)
		if err != nil {
			return 0, fmt.Errorf("failed to compute balance before %s: %w", from.String(), err)
		}
	}

	deltas, err := s.getDailyBalanceDeltas(ctx, staffID, from, to)
	if err != nil {
		return 0, err
	}
	// A frozen month boundary inside the range RESTARTS the chain at the
	// frozen closing balance (#1417). Accumulating live deltas straight
	// across it would report a minimum that is off by that month's drift, and
	// the payout/comp-time capacity checks in #1420 rely on this number.
	boundaries, err := s.frozenMonthBoundaries(ctx, staffID, from, to)
	if err != nil {
		return 0, err
	}
	minimum := int(^uint(0) >> 1)
	for d := from; !d.After(to); d = d.AddDays(1) {
		if opening, ok := boundaries[d]; ok && d.After(anchor) {
			balance = opening
		}
		balance += deltas[d]
		minimum = min(minimum, balance)
	}
	return minimum, nil
}

// frozenMonthBoundaries maps each month-start day inside [from, to] to the
// frozen closing balance of the preceding month, for the days where the carry
// chain restarts instead of continuing. `from` is a boundary only when it is
// itself the first day of a month.
func (s *workTimeMonthService) frozenMonthBoundaries(
	ctx context.Context,
	staffID int64,
	from, to timezone.Date,
) (map[timezone.Date]int, error) {
	boundaries := make(map[timezone.Date]int)
	if s.snapshotRepo == nil {
		return boundaries, nil
	}
	first := monthOf(from)
	if first.firstDay().Before(from) {
		first = first.next()
	}
	for k := first; !monthOf(to).before(k); k = k.next() {
		prev := k.addMonths(-1)
		snapshot, err := s.snapshotRepo.GetLatestClosedThrough(ctx, staffID, prev.Year, prev.Month)
		if err != nil {
			return nil, fmt.Errorf("failed to load month balance snapshot for %d-%02d: %w", prev.Year, prev.Month, err)
		}
		if snapshot == nil || snapshot.Year != prev.Year || snapshot.Month != prev.Month {
			continue
		}
		boundaries[k.firstDay()] = snapshot.ClosingBalanceMinutes
	}
	return boundaries, nil
}

// getDailyBalanceDeltas is deliberately snapshot-agnostic (#1417): it reports
// what happened on each day. A frozen month is not a per-day event, so folding
// it in here would double-count against the boundary reset in
// getMinimumDailyClosingBalance.
func (s *workTimeMonthService) getDailyBalanceDeltas(
	ctx context.Context,
	staffID int64,
	from, to timezone.Date,
) (map[timezone.Date]int, error) {
	resolver, err := s.buildTargetResolver(ctx, staffID, from, to)
	if err != nil {
		return nil, err
	}
	deltas := make(map[timezone.Date]int, from.DaysUntil(to)+1)
	for d := from; !d.After(to); d = d.AddDays(1) {
		deltas[d] = -resolver.targetFor(d)
	}
	if err := s.addDailyActualMinutes(ctx, staffID, from, to, deltas); err != nil {
		return nil, err
	}
	if err := s.addDailyAbsenceCredits(ctx, staffID, from, to, resolver, deltas); err != nil {
		return nil, err
	}
	if err := s.addDailyAdjustments(ctx, staffID, from, to, deltas); err != nil {
		return nil, err
	}
	anchor, err := s.chainAnchor(ctx, monthOf(from))
	if err != nil {
		return nil, err
	}
	for d := from; !d.After(to); d = d.AddDays(1) {
		if excludedByAccountStart(d, anchor) {
			deltas[d] = 0
		}
	}
	return deltas, nil
}

func (s *workTimeMonthService) addDailyActualMinutes(
	ctx context.Context,
	staffID int64,
	from, to timezone.Date,
	deltas map[timezone.Date]int,
) error {
	actualByDate, err := s.getDailyActualMinutes(ctx, staffID, from, to)
	if err != nil {
		return err
	}
	for date, minutes := range actualByDate {
		deltas[date] += minutes
	}
	return nil
}

func (s *workTimeMonthService) getDailyActualMinutes(
	ctx context.Context,
	staffID int64,
	from, to timezone.Date,
) (map[timezone.Date]int, error) {
	end := to.AddDays(1).BerlinMidnight()
	sessions, err := s.sessionRepo.ListOverlappingByStaffID(ctx, staffID, from.BerlinMidnight(), &end)
	if err != nil {
		return nil, fmt.Errorf("failed to load work sessions for daily balance calculation: %w", err)
	}
	actualByDate := make(map[timezone.Date]int)
	now := s.now()
	breaksBySessionID, err := s.breaksBySessionID(ctx, sessions)
	if err != nil {
		return nil, err
	}
	for _, session := range sessions {
		for day, minutes := range netMinutesByDate(session, breaksBySessionID[session.ID], balanceSessionEnd(session, now, timezone.TodayDate()), from, to) {
			actualByDate[day] += minutes
		}
	}
	return actualByDate, nil
}

func (s *workTimeMonthService) addDailyAbsenceCredits(
	ctx context.Context,
	staffID int64,
	from, to timezone.Date,
	resolver *dailyTargetResolver,
	deltas map[timezone.Date]int,
) error {
	credits, err := s.getDailyAbsenceCredits(ctx, staffID, from, to, to, resolver)
	if err != nil {
		return err
	}
	for d, credit := range credits {
		deltas[d] += credit
	}
	return nil
}

// getDailyAbsenceCredits is the per-day credit map behind both the daily
// deltas and the table's Gutschrift column (#2443). `through` is the last
// creditable day (see walkCreditedAbsenceDays).
func (s *workTimeMonthService) getDailyAbsenceCredits(
	ctx context.Context,
	staffID int64,
	from, to, through timezone.Date,
	resolver *dailyTargetResolver,
) (map[timezone.Date]int, error) {
	absences, err := s.absenceRepo.GetByStaffAndDateRange(ctx, staffID, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to load absences for daily credits: %w", err)
	}
	credits := make(map[timezone.Date]int)
	walkCreditedAbsenceDays(absences, from, through, resolver.targetFor,
		func(d timezone.Date, _ *activeModels.StaffAbsence, credit int, _ float64) {
			credits[d] += credit
		})
	return credits, nil
}

func (s *workTimeMonthService) addDailyAdjustments(
	ctx context.Context,
	staffID int64,
	from, to timezone.Date,
	deltas map[timezone.Date]int,
) error {
	if s.adjustmentRepo == nil {
		return nil
	}
	adjustments, err := s.adjustmentRepo.GetByStaffAndDateRange(ctx, staffID, from, to)
	if err != nil {
		return fmt.Errorf("failed to load adjustments for daily reduction validation: %w", err)
	}
	for _, adjustment := range adjustments {
		deltas[adjustment.EffectiveDate] += adjustment.MinutesDelta
	}
	return nil
}

func (s *workTimeMonthService) getRemainingCompTimeCommitment(
	ctx context.Context,
	staffID int64,
	from timezone.Date,
	absences []*activeModels.StaffAbsence,
) (int, error) {
	total := 0
	for _, absence := range absences {
		if from.Before(absence.DateStart) || from.After(absence.DateEnd) {
			continue
		}
		deduction, err := s.GetCompTimeDeductionMinutes(ctx, staffID, from, absence.DateEnd, absence.HalfDay)
		if err != nil {
			return 0, fmt.Errorf("failed to compute comp-time commitment from %s: %w", from.String(), err)
		}
		total += deduction
	}
	return total, nil
}

// GetCompTimeDeductionMinutes sums the missing daily target minutes a
// comp_time absence can remove from the Stundenkonto. Neither a full-day nor a
// half-day absence credits time, so recorded net work reduces the deduction by
// the same amount it contributes to the Monatskarte. Future days still reserve
// the full target.
func (s *workTimeMonthService) GetCompTimeDeductionMinutes(ctx context.Context, staffID int64, start, end timezone.Date, _ bool) (int, error) {
	if start.IsZero() || end.IsZero() || end.Before(start) {
		return 0, errors.New("start and end must form a valid range")
	}
	resolver, err := s.buildTargetResolver(ctx, staffID, start, end)
	if err != nil {
		return 0, err
	}
	actualByDate := make(map[timezone.Date]int)
	actualEnd := end
	if today := s.today(); actualEnd.After(today) {
		actualEnd = today
	}
	if !actualEnd.Before(start) {
		actualByDate, err = s.getDailyActualMinutes(ctx, staffID, start, actualEnd)
		if err != nil {
			return 0, err
		}
	}
	total := 0
	for d := start; !d.After(end); d = d.AddDays(1) {
		target := resolver.targetFor(d)
		if target <= 0 {
			continue
		}
		deduction := max(0, target-actualByDate[d])
		total += deduction
	}
	return total, nil
}

func (s *workTimeMonthService) getMonthSummaryThrough(ctx context.Context, staffID int64, key monthKey, through timezone.Date) (*MonthSummary, error) {
	anchor, err := s.chainAnchor(ctx, key)
	if err != nil {
		return nil, err
	}
	// The chain starts on the account start date; a month entirely before the
	// account start is summarized standalone (full month, no carry).
	startKey, first := monthOf(anchor), anchor
	if key.before(startKey) {
		startKey, first = key, key.firstDay()
	}

	// A frozen earlier month (#1417) short-circuits the chain: its closing
	// balance IS the opening balance here, so nothing before it is re-summed
	// and a retroactive correction there can no longer move this month.
	splice, err := s.spliceChainStart(ctx, staffID, key, anchor)
	if err != nil {
		return nil, err
	}
	carry := 0
	// Only an EARLIER frozen month shortens the chain. A splice that carries
	// nothing but the requested month's own snapshot (used for drift) must
	// leave the chain running from the account start.
	if splice != nil && splice.carryFrom != nil {
		startKey, first = splice.startKey, splice.startKey.firstDay()
		carry = splice.frozenCarry
	}

	// Bound how far back the chain reaches so a stale/misconfigured account
	// start can't make each poll aggregate and query decades of days. A
	// realistic account never predates the floor; when it does, the setting is
	// wrong, and we fail loudly instead of clamping — a clamp would drop the
	// balance of every month before the floor and quietly present a wrong
	// opening/closing Stundenkonto (#1842).
	if floorKey := key.addMonths(-maxCarryChainMonths); startKey.before(floorKey) {
		s.getLogger().Error("account start older than carry-chain bound, rejecting",
			"account_start", first.String(),
			"floor", floorKey.firstDay().String(),
			"max_carry_chain_months", maxCarryChainMonths,
		)
		return nil, fmt.Errorf("%w: %s is more than %d months before %d-%02d",
			ErrAccountStartTooOld, first.String(), maxCarryChainMonths, key.Year, key.Month)
	}

	aggregates, err := s.computeAggregates(ctx, staffID, first, key, through)
	if err != nil {
		return nil, err
	}

	for k := startKey; k.before(key); k = k.next() {
		carry += aggregates[k].balance()
	}

	agg := aggregates[key]
	summary := &MonthSummary{
		StaffID:                 staffID,
		Year:                    key.Year,
		Month:                   key.Month,
		CarryInMinutes:          carry,
		TargetMinutes:           agg.target,
		TargetMinutesToDate:     agg.targetToDate,
		ActualMinutes:           agg.actual,
		CreditedSickMinutes:     agg.creditedSick,
		CreditedVacationMinutes: agg.creditedVacation,
		CreditedTrainingMinutes: agg.creditedTraining,
		CreditedOtherMinutes:    agg.creditedOther,
		SickDays:                agg.sickDays,
		VacationDays:            agg.vacationDays,
		TrainingDays:            agg.trainingDays,
		BalanceMinutes:          agg.balance(),
	}
	if agg.hasShifts {
		planned := agg.plannedShift
		summary.PlannedShiftMinutes = &planned
	}
	summary.AdjustmentMinutes = agg.adjustment
	for _, adjustment := range agg.adjustments {
		summary.Adjustments = append(summary.Adjustments, AdjustmentView{
			ID:            adjustment.ID,
			Type:          adjustment.Type,
			MinutesDelta:  adjustment.MinutesDelta,
			EffectiveDate: adjustment.EffectiveDate,
			Note:          adjustment.Note,
			DecidedBy:     adjustment.DecidedBy,
			DecidedAt:     adjustment.DecidedAt,
		})
	}
	summary.ClosingBalanceMinutes = summary.CarryInMinutes + summary.BalanceMinutes
	if splice != nil {
		if splice.carryFrom != nil {
			summary.CarryInFrozen = true
			label := fmt.Sprintf("%04d-%02d", splice.carryFrom.Year, splice.carryFrom.Month)
			summary.CarryInFrozenFromMonth = &label
		}
		if own := splice.ownSnapshot; own != nil {
			summary.IsClosed = true
			closedAt, closedBy := own.ClosedAt, own.ClosedBy
			summary.ClosedAt, summary.ClosedBy = &closedAt, &closedBy
			summary.CloseReason = own.CloseReason
			frozen := own.ClosingBalanceMinutes
			summary.FrozenClosingBalanceMinutes = &frozen
			summary.DriftMinutes = summary.ClosingBalanceMinutes - frozen
		}
	}
	return summary, nil
}

// chainSplice is the outcome of the frozen-month lookup for one requested
// month: where the carry chain starts, what it starts with, and whether the
// requested month is itself frozen.
type chainSplice struct {
	// startKey is the first month the chain must still aggregate.
	startKey monthKey
	// frozenCarry is the opening balance for startKey.
	frozenCarry int
	// carryFrom names the frozen month that produced frozenCarry; nil when no
	// earlier month is frozen and the chain runs from the account start.
	carryFrom *monthKey
	// ownSnapshot is the requested month's own active snapshot, if it is
	// closed. It supplies the drift comparison, never the carry.
	ownSnapshot *activeModels.StaffMonthBalanceSnapshot
}

// spliceChainStart resolves how the carry chain for `key` must start (#1417).
// It returns nil when nothing is frozen and the caller should keep the
// account-anchor chain untouched.
//
// One read covers both questions: the newest active snapshot at or before
// `key` is either `key`'s own (then a second read finds the one before it) or
// already an earlier month (then it seeds the chain directly).
//
// A snapshot before the anchor month is ignored on purpose. The chain never
// starts before the account start, so such a row — left behind by a later
// change to the account-start setting — could only corrupt the card. The
// close path rejects creating one; this is the read-side half of that fence.
func (s *workTimeMonthService) spliceChainStart(
	ctx context.Context,
	staffID int64,
	key monthKey,
	anchor timezone.Date,
) (*chainSplice, error) {
	if s.snapshotRepo == nil {
		return nil, nil
	}
	anchorKey := monthOf(anchor)

	latest, err := s.snapshotRepo.GetLatestClosedThrough(ctx, staffID, key.Year, key.Month)
	if err != nil {
		return nil, fmt.Errorf("failed to load month balance snapshot: %w", err)
	}
	if latest == nil {
		return nil, nil
	}

	splice := &chainSplice{startKey: key}
	seed := latest
	if latest.Year == key.Year && latest.Month == key.Month {
		splice.ownSnapshot = latest
		prev := key.addMonths(-1)
		seed, err = s.snapshotRepo.GetLatestClosedThrough(ctx, staffID, prev.Year, prev.Month)
		if err != nil {
			return nil, fmt.Errorf("failed to load preceding month balance snapshot: %w", err)
		}
	}

	if seed != nil {
		seedKey := monthKey{Year: seed.Year, Month: seed.Month}
		if !seedKey.before(anchorKey) && seedKey.before(key) {
			splice.startKey = seedKey.next()
			splice.frozenCarry = seed.ClosingBalanceMinutes
			splice.carryFrom = &seedKey
		}
	}
	if splice.carryFrom == nil && splice.ownSnapshot == nil {
		return nil, nil
	}
	return splice, nil
}

// excludedByAccountStart reports whether d is inside the account-start month
// but before the start day. GetMonthSummary begins the anchor month's
// aggregation on the start date itself (`first = anchor`), so those days carry
// no Soll on the card; a month WHOLLY before the anchor is a different case —
// it is summarized standalone over its full length and stays fully counted.
func excludedByAccountStart(d, anchor timezone.Date) bool {
	return monthOf(d) == monthOf(anchor) && d.Before(anchor)
}

// accountAwareTargets loads the date-valid Soll resolver together with the
// account anchor. The anchor is absolute; the month key only picks the January
// fallback for an unset setting, and no day can precede January 1st of its own
// year — so an unset account start zeroes nothing.
func (s *workTimeMonthService) accountAwareTargets(ctx context.Context, staffID int64, from, to timezone.Date) (*dailyTargetResolver, timezone.Date, error) {
	resolver, err := s.buildTargetResolver(ctx, staffID, from, to)
	if err != nil {
		return nil, timezone.Date{}, err
	}
	anchor, err := s.chainAnchor(ctx, monthOf(from))
	if err != nil {
		return nil, timezone.Date{}, err
	}
	return resolver, anchor, nil
}

// GetDailyProjection prices every day in [from, to] with the same helpers the
// Monatskarte sums: the date-valid Soll (buildTargetResolver), the absence
// credit (walkCreditedAbsenceDays) and the recorded net work
// (getDailyActualMinutes). Days with nothing on them are returned as zeros
// rather than omitted: the caller must be able to tell "planned day off" from
// "range not covered".
//
// Days before a mid-month account start project as all-zero: the card starts
// accruing on the start date, so billing them in the table would contradict its
// "Summe Soll" (#1842). Future days carry their Soll but no balance — the card
// charges Soll only up to today.
func (s *workTimeMonthService) GetDailyProjection(ctx context.Context, staffID int64, from, to timezone.Date) ([]DailyProjection, error) {
	if from.IsZero() || to.IsZero() {
		return nil, fmt.Errorf("%w: from and to are required", ErrInvalidTargetRange)
	}
	if to.Before(from) {
		return nil, fmt.Errorf("%w: to must be on or after from", ErrInvalidTargetRange)
	}
	if from.DaysUntil(to) > maxDailyTargetRangeDays {
		return nil, fmt.Errorf("%w: range must not exceed %d days", ErrInvalidTargetRange, maxDailyTargetRangeDays)
	}

	resolver, anchor, err := s.accountAwareTargets(ctx, staffID, from, to)
	if err != nil {
		return nil, err
	}
	today := s.today()
	// Everything that prices a day stops at today, exactly like the card:
	// tomorrow's Soll is planned, not owed.
	through := to
	if today.Before(through) {
		through = today
	}
	targetFor := func(d timezone.Date) int {
		if excludedByAccountStart(d, anchor) {
			return 0
		}
		return resolver.targetFor(d)
	}

	credits := map[timezone.Date]int{}
	actual := map[timezone.Date]int{}
	if !through.Before(from) {
		if credits, err = s.getDailyAbsenceCredits(ctx, staffID, from, to, through, resolver); err != nil {
			return nil, err
		}
		if actual, err = s.getDailyActualMinutes(ctx, staffID, from, through); err != nil {
			return nil, err
		}
	}

	projection := make([]DailyProjection, 0, from.DaysUntil(to)+1)
	for d := from; !d.After(to); d = d.AddDays(1) {
		day := DailyProjection{Date: d, TargetMinutes: targetFor(d)}
		if excludedByAccountStart(d, anchor) || d.After(today) {
			projection = append(projection, day)
			continue
		}
		day.CreditMinutes = credits[d]
		day.ActualMinutes = actual[d]
		day.BalanceMinutes = day.ActualMinutes + day.CreditMinutes - day.TargetMinutes
		projection = append(projection, day)
	}
	return projection, nil
}

// GetDailyTargets resolves the contractual Soll for every day in [from, to]
// without loading absence or work-session data. It is the lightweight variant
// for overview consumers that only chart targets.
func (s *workTimeMonthService) GetDailyTargets(ctx context.Context, staffID int64, from, to timezone.Date) ([]DailyTarget, error) {
	if from.IsZero() || to.IsZero() {
		return nil, fmt.Errorf("%w: from and to are required", ErrInvalidTargetRange)
	}
	if to.Before(from) {
		return nil, fmt.Errorf("%w: to must be on or after from", ErrInvalidTargetRange)
	}
	if from.DaysUntil(to) > maxDailyTargetRangeDays {
		return nil, fmt.Errorf("%w: range must not exceed %d days", ErrInvalidTargetRange, maxDailyTargetRangeDays)
	}

	resolver, anchor, err := s.accountAwareTargets(ctx, staffID, from, to)
	if err != nil {
		return nil, err
	}
	targets := make([]DailyTarget, 0, from.DaysUntil(to)+1)
	for d := from; !d.After(to); d = d.AddDays(1) {
		target := resolver.targetFor(d)
		if excludedByAccountStart(d, anchor) {
			target = 0
		}
		targets = append(targets, DailyTarget{Date: d, TargetMinutes: target})
	}
	return targets, nil
}

func (s *workTimeMonthService) getLogger() *slog.Logger {
	if s.logger == nil {
		return slog.Default()
	}
	return s.logger
}
