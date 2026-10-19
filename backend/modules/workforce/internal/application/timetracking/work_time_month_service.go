package timetracking

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type workTimeMonthService struct {
	sessionRepo    monthSessionReader
	breakRepo      monthBreakReader
	absenceRepo    monthAbsenceReader
	staffRepo      StaffScheduleQuery
	scheduleRepo   monthScheduleReader
	workModelRepo  monthModelReader
	shiftRepo      monthShiftReader
	settings       monthSettingsResolver
	holidayReader  HolidayDatesReader
	overrideReader TargetOverrideReader
	adjustmentRepo monthAdjustmentReader
	snapshotRepo   monthSnapshotReader
	logger         *slog.Logger

	// todayFunc is a test hook; production uses timezone.TodayDate.
	todayFunc func() calendar.Date
	// nowFunc is a test hook; production uses time.Now.
	nowFunc func() time.Time
}

func NewWorkTimeMonthService(
	sessionRepo monthSessionReader,
	breakRepo monthBreakReader,
	absenceRepo monthAbsenceReader,
	staffRepo StaffScheduleQuery,
	scheduleRepo monthScheduleReader,
	workModelRepo monthModelReader,
	shiftRepo monthShiftReader,
	settings monthSettingsResolver,
	logger *slog.Logger,
	opts ...WorkTimeMonthOption,
) WorkTimeMonthService {
	service := &workTimeMonthService{
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
	for _, opt := range opts {
		opt(service)
	}
	return service
}

func (s *workTimeMonthService) today() calendar.Date {
	if s.todayFunc != nil {
		return s.todayFunc()
	}
	return calendar.TodayDate()
}

func (s *workTimeMonthService) now() time.Time {
	if s.nowFunc != nil {
		return s.nowFunc()
	}
	return time.Now()
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
// computeAggregates builds the raw per-month numbers for every month from
// `first`'s month up to toKey with one set of range queries. `first` is a
// date, not a month: the account start may fall mid-month, and then the
// anchor month must only count from that day on. Days after `today`
// contribute nothing to targetToDate and credits, so the current month's
// balance is automatically pro-rated and future months are neutral.
func (s *workTimeMonthService) computeAggregates(ctx context.Context, staffID int64, first calendar.Date, toKey monthKey, today calendar.Date) (map[monthKey]*monthAggregates, error) {
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
		target := resolver.TargetFor(d)
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

// maxOpenWorkSessionDuration is the shared live limit. The presence lookup in
// database/repositories/active reads the same constant, so a block that stops
// counting toward the balance stops marking its owner present as well.
const maxOpenWorkSessionDuration = workforce.MaxOpenWorkSessionDuration

// BalanceSessionEnd applies the live balance limit using the Berlin day of
// now. Readers outside this package (notably the kiosk) use it so a running
// block cannot produce a different total from the monthly balance.
func BalanceSessionEnd(session *WorkSession, now time.Time) time.Time {
	return balanceSessionEnd(session, now, calendar.DateFromTime(now))
}

func balanceSessionEnd(session *WorkSession, now time.Time, today calendar.Date) time.Time {
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

type absenceDayVisitor func(d calendar.Date, absence *StaffAbsence, credit int, fraction float64)

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
	absences []*StaffAbsence,
	first, through calendar.Date,
	targetFor func(calendar.Date) int,
	visit absenceDayVisitor,
) {
	sort.Slice(absences, func(i, j int) bool { return absences[i].ID < absences[j].ID })
	credited := make(map[calendar.Date]bool)
	for _, absence := range absences {
		if absence.Status != AbsenceStatusReported && absence.Status != AbsenceStatusApproved {
			continue
		}
		walkAbsenceDays(absence, first, through, credited, targetFor, visit)
	}
}

// walkAbsenceDays visits the days of one absence that no lower-ID absence has
// consumed yet, inside [first, through] and with a Soll.
func walkAbsenceDays(
	absence *StaffAbsence,
	first, through calendar.Date,
	credited map[calendar.Date]bool,
	targetFor func(calendar.Date) int,
	visit absenceDayVisitor,
) {
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
		credit, fraction := absenceDayCredit(absence, d, target, startHalf, endHalf)
		visit(d, absence, credit, fraction)
	}
}

// absenceDayCredit prices one consumed absence day with a Soll: the full Soll,
// or half of it on a half-day boundary.
func absenceDayCredit(absence *StaffAbsence, d calendar.Date, target int, startHalf, endHalf bool) (int, float64) {
	fraction := 1.0
	credit := target
	if isHalfAbsenceDay(absence, d, startHalf, endHalf) {
		fraction = 0.5
		credit = target / 2
	}
	if absence.AbsenceType == AbsenceTypeCompTime {
		// Freizeitausgleich (#1420 5b) credits NOTHING: the day keeps
		// its Soll, so the balance drops by the day's target. The day
		// still counts as consumed above, so an overlapping vacation
		// cannot re-credit it.
		credit = 0
	}
	return credit, fraction
}

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
func (s *workTimeMonthService) chainAnchor(ctx context.Context, key monthKey) (calendar.Date, error) {
	return resolveAccountAnchor(ctx, s.settings, s.getLogger(), key)
}

// resolveAccountAnchor is the shared anchor resolution (see chainAnchor doc
// above). Package-level so the balance-adjustment service (#1420) can reject
// bookings dated before the account start — an adjustment before the anchor
// never enters any carry chain and would silently not move the Stundenkonto.
func resolveAccountAnchor(ctx context.Context, settings monthSettingsResolver, logger *slog.Logger, key monthKey) (calendar.Date, error) {
	fallback := calendar.NewDate(key.Year, time.January, 1)
	if settings == nil {
		return fallback, nil
	}
	value, err := settings.AccountStartDate(ctx)
	if err != nil {
		return calendar.Date(""), fmt.Errorf("failed to resolve account start date setting: %w", err)
	}
	if value == "" {
		return fallback, nil
	}
	start, err := calendar.ParseDate(value)
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
func (s *workTimeMonthService) GetRangeAggregate(ctx context.Context, staffID int64, from, to calendar.Date) (*RangeAggregate, error) {
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
		aggregate.TargetMinutes += resolver.TargetFor(d)
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
func (s *workTimeMonthService) GetClosingBalanceAsOf(ctx context.Context, staffID int64, cutoff calendar.Date) (int, error) {
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
func (s *workTimeMonthService) GetBalanceAdjustmentMinutes(ctx context.Context, staffID int64, from, to calendar.Date) (int, error) {
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

// excludedByAccountStart reports whether d is inside the account-start month
// but before the start day. GetMonthSummary begins the anchor month's
// aggregation on the start date itself (`first = anchor`), so those days carry
// no Soll on the card; a month WHOLLY before the anchor is a different case —
// it is summarized standalone over its full length and stays fully counted.
func excludedByAccountStart(d, anchor calendar.Date) bool {
	return monthOf(d) == monthOf(anchor) && d.Before(anchor)
}

// accountAwareTargets loads the date-valid Soll resolver together with the
// account anchor. The anchor is absolute; the month key only picks the January
// fallback for an unset setting, and no day can precede January 1st of its own
// year — so an unset account start zeroes nothing.
func (s *workTimeMonthService) accountAwareTargets(ctx context.Context, staffID int64, from, to calendar.Date) (*dailyTargetResolver, calendar.Date, error) {
	resolver, err := s.buildTargetResolver(ctx, staffID, from, to)
	if err != nil {
		return nil, calendar.Date(""), err
	}
	anchor, err := s.chainAnchor(ctx, monthOf(from))
	if err != nil {
		return nil, calendar.Date(""), err
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
func (s *workTimeMonthService) GetDailyProjection(ctx context.Context, staffID int64, from, to calendar.Date) ([]DailyProjection, error) {
	if err := validateDailyRange(from, to); err != nil {
		return nil, err
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
	targetFor := func(d calendar.Date) int {
		if excludedByAccountStart(d, anchor) {
			return 0
		}
		return resolver.TargetFor(d)
	}

	credits, actual, err := s.projectionDayValues(ctx, staffID, from, to, through, resolver)
	if err != nil {
		return nil, err
	}

	projection := make([]DailyProjection, 0, from.DaysUntil(to)+1)
	for d := from; !d.After(to); d = d.AddDays(1) {
		day := DailyProjection{Date: d, TargetMinutes: targetFor(d), TargetSource: resolver.SourceFor(d)}
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

// validateDailyRange bounds the day-level reads to a closed range of at most
// maxDailyTargetRangeDays.
func validateDailyRange(from, to calendar.Date) error {
	if from.IsZero() || to.IsZero() {
		return fmt.Errorf("%w: from and to are required", ErrInvalidTargetRange)
	}
	if to.Before(from) {
		return fmt.Errorf("%w: to must be on or after from", ErrInvalidTargetRange)
	}
	if from.DaysUntil(to) > maxDailyTargetRangeDays {
		return fmt.Errorf("%w: range must not exceed %d days", ErrInvalidTargetRange, maxDailyTargetRangeDays)
	}
	return nil
}

// projectionDayValues loads the absence credits and the recorded net work of
// the priced days; nothing is priced when through lies before from.
func (s *workTimeMonthService) projectionDayValues(ctx context.Context, staffID int64, from, to, through calendar.Date, resolver *dailyTargetResolver) (map[calendar.Date]int, map[calendar.Date]int, error) {
	if through.Before(from) {
		return map[calendar.Date]int{}, map[calendar.Date]int{}, nil
	}
	credits, err := s.getDailyAbsenceCredits(ctx, staffID, from, to, through, resolver)
	if err != nil {
		return nil, nil, err
	}
	actual, err := s.getDailyActualMinutes(ctx, staffID, from, through)
	if err != nil {
		return nil, nil, err
	}
	return credits, actual, nil
}

// GetDailyTargets resolves the contractual Soll for every day in [from, to]
// without loading absence or work-session data. It is the lightweight variant
// for overview consumers that only chart targets.
func (s *workTimeMonthService) GetDailyTargets(ctx context.Context, staffID int64, from, to calendar.Date) ([]DailyTarget, error) {
	if err := validateDailyRange(from, to); err != nil {
		return nil, err
	}

	resolver, anchor, err := s.accountAwareTargets(ctx, staffID, from, to)
	if err != nil {
		return nil, err
	}
	targets := make([]DailyTarget, 0, from.DaysUntil(to)+1)
	for d := from; !d.After(to); d = d.AddDays(1) {
		target := resolver.TargetFor(d)
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
