package active

import (
	"context"
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
// current month's balance doesn't count days that haven't happened yet.
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
	CreditedOtherMinutes    int     `json:"credited_other_minutes"`
	SickDays                float64 `json:"sick_days"`
	VacationDays            float64 `json:"vacation_days"`

	// PlannedShiftMinutes is the Dienstplan coverage (display only, never
	// saldo-relevant); nil = no shifts maintained in this month.
	PlannedShiftMinutes *int `json:"planned_shift_minutes,omitempty"`

	// BalanceMinutes is this month's own saldo:
	// actual + credited − target_to_date.
	BalanceMinutes int `json:"balance_minutes"`
	// ClosingBalanceMinutes = CarryInMinutes + BalanceMinutes; for the
	// current month this is the live Stundenkonto as of today.
	ClosingBalanceMinutes int `json:"closing_balance_minutes"`
}

// WorkTimeMonthService owns the Monatskarte math. It lives outside
// workSessionService on purpose: monthly aggregation is read-model logic
// with its own dependency set.
type WorkTimeMonthService interface {
	GetMonthSummary(ctx context.Context, staffID int64, year, month int) (*MonthSummary, error)
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
	GetHistoryByStaffID(ctx context.Context, staffID int64, from, to timezone.Date) ([]*activeModels.WorkSession, error)
}

// monthAbsenceReader is implemented by active.StaffAbsenceRepository.
type monthAbsenceReader interface {
	GetByStaffAndDateRange(ctx context.Context, staffID int64, from, to timezone.Date) ([]*activeModels.StaffAbsence, error)
}

// monthStaffReader is implemented by users.StaffRepository.
type monthStaffReader interface {
	FindByID(ctx context.Context, id any) (*userModels.Staff, error)
}

// monthScheduleReader is implemented by config.StaffWorkScheduleRepository.
type monthScheduleReader interface {
	FindByStaffIDsValidInRange(ctx context.Context, staffIDs []int64, from, to timezone.Date) ([]*configModels.StaffWorkSchedule, error)
}

// monthModelReader is implemented by config.WorkTimeModelRepository.
type monthModelReader interface {
	FindByID(ctx context.Context, id int64) (*configModels.WorkTimeModel, error)
}

type workTimeMonthService struct {
	sessionRepo   monthSessionReader
	absenceRepo   monthAbsenceReader
	staffRepo     monthStaffReader
	scheduleRepo  monthScheduleReader
	workModelRepo monthModelReader
	shiftRepo     monthShiftReader
	settings      monthSettingsResolver
	logger        *slog.Logger

	// todayFunc is a test hook; production uses timezone.TodayDate.
	todayFunc func() timezone.Date
}

func NewWorkTimeMonthService(
	sessionRepo monthSessionReader,
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
		absenceRepo:   absenceRepo,
		staffRepo:     staffRepo,
		scheduleRepo:  scheduleRepo,
		workModelRepo: workModelRepo,
		shiftRepo:     shiftRepo,
		settings:      settings,
		logger:        logger,
	}
}

func (s *workTimeMonthService) today() timezone.Date {
	if s.todayFunc != nil {
		return s.todayFunc()
	}
	return timezone.TodayDate()
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

func (k monthKey) next() monthKey {
	if k.Month == 12 {
		return monthKey{Year: k.Year + 1, Month: 1}
	}
	return monthKey{Year: k.Year, Month: k.Month + 1}
}

func (k monthKey) before(o monthKey) bool {
	return k.Year < o.Year || (k.Year == o.Year && k.Month < o.Month)
}

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
	creditedOther    int
	sickDays         float64
	vacationDays     float64
	hasShifts        bool
	plannedShift     int
}

func (a *monthAggregates) creditedTotal() int {
	return a.creditedSick + a.creditedVacation + a.creditedOther
}

func (a *monthAggregates) balance() int {
	return a.actual + a.creditedTotal() - a.targetToDate
}

// dailyTargetResolver resolves the contractual Soll for single days: a
// date-valid staff_work_schedules row set wins, the assigned work-time model
// is the fallback — the same two-tier resolution the weekly summaries use.
type dailyTargetResolver struct {
	entries     []*configModels.StaffWorkSchedule
	staffAnchor *timezone.Date
	model       *configModels.WorkTimeModel
	modelAnchor timezone.Date
}

func (r *dailyTargetResolver) targetFor(d timezone.Date) int {
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

// computeAggregates builds the raw per-month numbers for every month in
// [fromKey, toKey] with one set of range queries. Days after `today`
// contribute nothing to targetToDate and credits, so the current month's
// balance is automatically pro-rated and future months are neutral.
func (s *workTimeMonthService) computeAggregates(ctx context.Context, staffID int64, fromKey, toKey monthKey, today timezone.Date) (map[monthKey]*monthAggregates, error) {
	first, last := fromKey.firstDay(), toKey.lastDay()

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

	if err := s.addActualMinutes(ctx, staffID, first, last, aggregates); err != nil {
		return nil, err
	}
	if err := s.addAbsenceCredits(ctx, staffID, first, last, today, resolver, aggregates); err != nil {
		return nil, err
	}
	if err := s.addPlannedShifts(ctx, staffID, first, last, aggregates); err != nil {
		return nil, err
	}
	return aggregates, nil
}

func (s *workTimeMonthService) addActualMinutes(ctx context.Context, staffID int64, first, last timezone.Date, aggregates map[monthKey]*monthAggregates) error {
	sessions, err := s.sessionRepo.GetHistoryByStaffID(ctx, staffID, first, last)
	if err != nil {
		return fmt.Errorf("failed to load work sessions: %w", err)
	}
	now := time.Now()
	for _, session := range sessions {
		if agg, ok := aggregates[monthOf(session.Date)]; ok {
			agg.actual += netMinutes(session, now)
		}
	}
	return nil
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
	sort.Slice(absences, func(i, j int) bool { return absences[i].ID < absences[j].ID })

	credited := make(map[timezone.Date]bool)
	for _, absence := range absences {
		if absence.Status != activeModels.AbsenceStatusReported && absence.Status != activeModels.AbsenceStatusApproved {
			continue
		}
		s.creditAbsenceDays(absence, first, last, today, resolver, aggregates, credited)
	}
	return nil
}

func (s *workTimeMonthService) creditAbsenceDays(absence *activeModels.StaffAbsence, first, last, today timezone.Date, resolver *dailyTargetResolver, aggregates map[monthKey]*monthAggregates, credited map[timezone.Date]bool) {
	startHalf, endHalf := effectiveAbsenceBoundaryHalves(absence)
	for d := absence.DateStart; !d.After(absence.DateEnd); d = d.AddDays(1) {
		if d.Before(first) || d.After(last) || d.After(today) || credited[d] {
			continue
		}
		credited[d] = true
		target := resolver.targetFor(d)
		if target <= 0 {
			continue
		}
		fraction := 1.0
		credit := target
		if isHalfAbsenceDay(absence, d, startHalf, endHalf) {
			fraction = 0.5
			credit = target / 2
		}
		agg := aggregates[monthOf(d)]
		switch absence.AbsenceType {
		case activeModels.AbsenceTypeSick:
			agg.creditedSick += credit
			agg.sickDays += fraction
		case activeModels.AbsenceTypeVacation:
			agg.creditedVacation += credit
			agg.vacationDays += fraction
		default:
			agg.creditedOther += credit
		}
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
// configured account start month, or January of the summarized month's year
// as the stable fallback (the frontend defaults its Stundenkonto to Jan 1
// the same way).
func (s *workTimeMonthService) chainAnchor(ctx context.Context, key monthKey) monthKey {
	fallback := monthKey{Year: key.Year, Month: 1}
	if s.settings == nil {
		return fallback
	}
	value, err := s.settings.ResolveString(ctx, configModels.KeyTimeTrackingAccountStartDate)
	if err != nil || value == "" {
		return fallback
	}
	start, err := timezone.ParseDate(value)
	if err != nil {
		s.getLogger().Warn("invalid account start date setting, using january fallback",
			"value", value,
			"error", err.Error(),
		)
		return fallback
	}
	return monthOf(start)
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

	startKey := key
	if anchor := s.chainAnchor(ctx, key); anchor.before(key) {
		startKey = anchor
	}

	aggregates, err := s.computeAggregates(ctx, staffID, startKey, key, today)
	if err != nil {
		return nil, err
	}

	carry := 0
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
		CreditedOtherMinutes:    agg.creditedOther,
		SickDays:                agg.sickDays,
		VacationDays:            agg.vacationDays,
		BalanceMinutes:          agg.balance(),
	}
	if agg.hasShifts {
		planned := agg.plannedShift
		summary.PlannedShiftMinutes = &planned
	}
	summary.ClosingBalanceMinutes = summary.CarryInMinutes + summary.BalanceMinutes
	return summary, nil
}

func (s *workTimeMonthService) getLogger() *slog.Logger {
	if s.logger == nil {
		return slog.Default()
	}
	return s.logger
}
