package active

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

// Period values for the dashboard summary (#1417).
const (
	OverviewPeriodWeek  = "week"
	OverviewPeriodMonth = "month"
)

// ErrOverviewInvalid marks a caller mistake in an overview request — HTTP 400.
var ErrOverviewInvalid = errors.New("invalid overview request")

// DashboardPeriodTotals is the tenant-wide Soll/Ist/Delta of the selected
// period.
//
// DeltaMinutes is the sum of the per-staff month balances
// (actual + credited + adjustments − target_to_date), deliberately NOT
// ist − soll: crediting sick and vacation days against the day's Soll is the
// whole premise of the Stundenkonto ("Ansatz B"). A dashboard delta computed
// differently from the Monatskarte saldo would be exactly the drift between
// aggregate and detail view that this tranche exists to prevent.
type DashboardPeriodTotals struct {
	SollMinutes  int `json:"soll_minutes"`
	IstMinutes   int `json:"ist_minutes"`
	DeltaMinutes int `json:"delta_minutes"`
}

// DashboardSummary is the Leitungs-Dashboard aggregate (#1417 2a). Field names
// are fixed by the issue.
type DashboardSummary struct {
	ActiveStaffCount   int `json:"active_staff_count"`
	SickToday          int `json:"sick_today"`
	VacationToday      int `json:"vacation_today"`
	CurrentlyClockedIn int `json:"currently_clocked_in"`
	ExpectedClockedIn  int `json:"expected_clocked_in"`

	Period DashboardPeriodTotals `json:"period"`

	// SaldoSchoolTotalMinutes is the sum of the live Stundenkonto balances. It
	// is an account balance, not a period quantity, so it is identical for
	// period=week and period=month.
	SaldoSchoolTotalMinutes int `json:"saldo_school_total_minutes"`
	PendingRequestsCount    int `json:"pending_requests_count"`
}

// TimeTrackingOverviewRow is one staff member's Soll/Ist/Saldo/Resturlaub.
// Gated behind time_tracking:manage, not users:read — it exposes per-person
// working-time data.
type TimeTrackingOverviewRow struct {
	StaffID   int64  `json:"staff_id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Name      string `json:"name"`
	// EmploymentType is "" when the staff member has none recorded.
	EmploymentType string `json:"employment_type"`
	SollMinutes    int    `json:"soll_minutes"`
	IstMinutes     int    `json:"ist_minutes"`
	// BalanceMinutes is the authoritative closing balance: frozen for an
	// actively closed month, live otherwise.
	BalanceMinutes        int     `json:"balance_minutes"`
	RemainingVacationDays float64 `json:"remaining_vacation_days"`
}

// TimeTrackingOverview is the cross-staff list. Rows is never nil.
type TimeTrackingOverview struct {
	Year  int                       `json:"year"`
	Month int                       `json:"month"`
	Rows  []TimeTrackingOverviewRow `json:"rows"`
}

// OverviewFilters narrows the cross-staff list.
//
// EmploymentType is an INPUT attribute and is applied before the computation,
// so the per-staff math never runs for excluded rows. SaldoMin/SaldoMax are
// OUTPUTS of the computation and can only be applied afterwards — the saldo
// does not exist until the carry chain has run. They therefore reduce payload
// size, never query cost.
// Year/Month select the month to compute. Both zero means the current month.
// A future month is rejected rather than served: its Soll would be the full
// month against zero Ist, i.e. a minus balance for work that has not been
// missed yet — the same reason a running month cannot be closed.
type OverviewFilters struct {
	EmploymentType string
	SaldoMin       *int
	SaldoMax       *int
	SortBy         string
	Descending     bool
	Year           int
	Month          int
}

// Sort keys accepted by the overview.
const (
	OverviewSortName     = "name"
	OverviewSortBalance  = "balance"
	OverviewSortSoll     = "soll"
	OverviewSortIst      = "ist"
	OverviewSortVacation = "vacation"
)

// ValidOverviewSortKeys lists the accepted sort keys.
var ValidOverviewSortKeys = []string{
	OverviewSortName,
	OverviewSortBalance,
	OverviewSortSoll,
	OverviewSortIst,
	OverviewSortVacation,
}

// RangeAggregate is the Soll/Ist/credit/adjustment sum over an arbitrary closed
// date range, computed with the same resolver and the same daily arithmetic as
// the Monatskarte. No carry: a range is not an account.
type RangeAggregate struct {
	TargetMinutes  int
	ActualMinutes  int
	BalanceMinutes int
}

// StaffOverviewService serves the tenant-wide time-tracking views (#1417 2a).
//
// It never reimplements the Stundenkonto math. It prefetches every input for
// all staff members with one query per source and then runs the UNCHANGED
// per-staff WorkTimeMonthService over in-memory readers (see
// work_time_month_prefetch.go). That keeps the list numerically identical to
// the /staff/{id} detail view — the whole point — at a query count that does
// not grow with the number of staff members.
type StaffOverviewService interface {
	GetDashboardSummary(ctx context.Context, period string) (*DashboardSummary, error)
	GetTimeTrackingOverview(ctx context.Context, filters OverviewFilters) (*TimeTrackingOverview, error)
	// GetMonthExportRows builds the cross-staff payroll rows (#1417 2b) over
	// the same prefetch + month math as the overview. month == 0 = whole year.
	GetMonthExportRows(ctx context.Context, year, month int) ([]MonthExportRow, error)
}

type staffOverviewService struct {
	staffRepo      userModels.StaffRepository
	sessionRepo    activeModels.WorkSessionRepository
	breakRepo      activeModels.WorkSessionBreakRepository
	absenceRepo    activeModels.StaffAbsenceRepository
	adjustmentRepo activeModels.StaffBalanceAdjustmentRepository
	quotaRepo      activeModels.StaffVacationQuotaRepository
	snapshotRepo   activeModels.StaffMonthBalanceSnapshotRepository
	scheduleRepo   configModels.StaffWorkScheduleRepository
	workModelRepo  configModels.WorkTimeModelRepository
	shiftRepo      scheduleModels.StaffShiftRepository
	settings       monthSettingsResolver
	holidayReader  HolidayDatesReader
	// openingRepo carries the vacation takeover rows (#2132); setter
	// injection (SetVacationOpeningReader), nil in bare unit fixtures.
	openingRepo activeModels.StaffVacationOpeningRepository
	logger      *slog.Logger

	// todayFunc is a test hook; production uses timezone.TodayDate.
	todayFunc func() timezone.Date
}

func NewStaffOverviewService(
	staffRepo userModels.StaffRepository,
	sessionRepo activeModels.WorkSessionRepository,
	breakRepo activeModels.WorkSessionBreakRepository,
	absenceRepo activeModels.StaffAbsenceRepository,
	adjustmentRepo activeModels.StaffBalanceAdjustmentRepository,
	quotaRepo activeModels.StaffVacationQuotaRepository,
	snapshotRepo activeModels.StaffMonthBalanceSnapshotRepository,
	scheduleRepo configModels.StaffWorkScheduleRepository,
	workModelRepo configModels.WorkTimeModelRepository,
	shiftRepo scheduleModels.StaffShiftRepository,
	settings monthSettingsResolver,
	logger *slog.Logger,
) *staffOverviewService {
	return &staffOverviewService{
		staffRepo:      staffRepo,
		sessionRepo:    sessionRepo,
		breakRepo:      breakRepo,
		absenceRepo:    absenceRepo,
		adjustmentRepo: adjustmentRepo,
		quotaRepo:      quotaRepo,
		snapshotRepo:   snapshotRepo,
		scheduleRepo:   scheduleRepo,
		workModelRepo:  workModelRepo,
		shiftRepo:      shiftRepo,
		settings:       settings,
		logger:         logger,
	}
}

// SetHolidayReader wires the non-working-day resolver, mirroring
// WorkTimeMonthService.SetHolidayReader.
func (s *staffOverviewService) SetHolidayReader(reader HolidayDatesReader) {
	s.holidayReader = reader
}

func (s *staffOverviewService) getLogger() *slog.Logger {
	return cmp.Or(s.logger, slog.Default())
}

func (s *staffOverviewService) today() timezone.Date {
	if s.todayFunc != nil {
		return s.todayFunc()
	}
	return timezone.TodayDate()
}

// --- prefetch --------------------------------------------------------------

// buildPrefetch loads every month-math input for the given staff members with
// one query per source. `lower` lets the caller widen the window below the
// account anchor (the ISO week can start in the previous month).
func (s *staffOverviewService) buildPrefetch(
	ctx context.Context,
	staffMembers []*userModels.Staff,
	lower *timezone.Date,
	through monthKey,
) (*monthPrefetch, error) {
	memo := newMemoSettingsResolver(s.settings)
	anchor, err := resolveAccountAnchor(ctx, memo, s.getLogger(), through)
	if err != nil {
		return nil, err
	}
	from, to := anchor, through.lastDay()
	// getMonthSummaryThrough summarizes a month before the configured account
	// start as a standalone month. Prefetch the same month instead of querying
	// the future anchor through an earlier month end.
	if through.before(monthOf(anchor)) {
		from = through.firstDay()
	}
	if lower != nil && lower.Before(from) {
		from = *lower
	}
	// Same bound the detail view enforces, checked BEFORE loading anything:
	// a misconfigured account start must fail like it does per staff member
	// instead of pulling decades of rows for everybody.
	if floorKey := through.addMonths(-maxCarryChainMonths); monthOf(from).before(floorKey) {
		return nil, fmt.Errorf("%w: %s is more than %d months before %d-%02d",
			ErrAccountStartTooOld, from.String(), maxCarryChainMonths, through.Year, through.Month)
	}

	staffIDs := make([]int64, 0, len(staffMembers))
	staffByID := make(map[int64]*userModels.Staff, len(staffMembers))
	modelIDs := make([]int64, 0, len(staffMembers))
	for _, staff := range staffMembers {
		staffIDs = append(staffIDs, staff.ID)
		staffByID[staff.ID] = staff
		if staff.WorkTimeModelID != nil && !slices.Contains(modelIDs, *staff.WorkTimeModelID) {
			modelIDs = append(modelIDs, *staff.WorkTimeModelID)
		}
	}

	prefetch := &monthPrefetch{
		from: from, to: to,
		staff:    staffByID,
		settings: memo,
	}

	end := to.AddDays(1).BerlinMidnight()
	if prefetch.sessions, err = s.sessionRepo.ListOverlappingByStaffIDs(ctx, staffIDs, from.BerlinMidnight(), &end); err != nil {
		return nil, fmt.Errorf("failed to prefetch work sessions: %w", err)
	}
	// Breaks are keyed by session ID, so they can only be loaded once the
	// sessions are known. The interval math needs both completed and running
	// breaks to assign a night block's pause to the correct calendar day.
	sessionIDs := make([]int64, 0)
	for _, sessions := range prefetch.sessions {
		for _, session := range sessions {
			sessionIDs = append(sessionIDs, session.ID)
		}
	}
	if prefetch.breaks, err = s.breakRepo.GetBySessionIDs(ctx, sessionIDs); err != nil {
		return nil, fmt.Errorf("failed to prefetch work-session breaks: %w", err)
	}
	if prefetch.absences, err = s.absenceRepo.GetByStaffIDsAndDateRange(ctx, staffIDs, from, to); err != nil {
		return nil, fmt.Errorf("failed to prefetch absences: %w", err)
	}
	if prefetch.adjustments, err = s.adjustmentRepo.GetByStaffIDsAndDateRange(ctx, staffIDs, from, to); err != nil {
		return nil, fmt.Errorf("failed to prefetch balance adjustments: %w", err)
	}
	if prefetch.shifts, err = s.shiftRepo.FindByStaffIDsAndDateRange(ctx, staffIDs, from, to); err != nil {
		return nil, fmt.Errorf("failed to prefetch staff shifts: %w", err)
	}
	scheduleEntries, err := s.scheduleRepo.FindByStaffIDsValidInRange(ctx, staffIDs, workforceDate(from), workforceDate(to))
	if err != nil {
		return nil, fmt.Errorf("failed to prefetch work schedules: %w", err)
	}
	prefetch.schedules = make(map[int64][]*configModels.StaffWorkSchedule, len(staffIDs))
	for _, entry := range scheduleEntries {
		prefetch.schedules[entry.StaffID] = append(prefetch.schedules[entry.StaffID], entry)
	}
	// Not derivable from the range-filtered schedules above: the question is
	// whether a staff member EVER had a schedule version, which is what
	// decides if the assigned work-time model may stand in.
	if prefetch.scheduleHist, err = s.scheduleRepo.FindStaffIDsWithScheduleHistory(ctx, staffIDs); err != nil {
		return nil, fmt.Errorf("failed to prefetch schedule history: %w", err)
	}
	prefetch.workTimeModels = make(map[int64]*configModels.WorkTimeModel, len(modelIDs))
	if len(modelIDs) > 0 {
		models, err := s.workModelRepo.FindByIDs(ctx, modelIDs)
		if err != nil {
			return nil, fmt.Errorf("failed to prefetch work time models: %w", err)
		}
		for _, model := range models {
			prefetch.workTimeModels[model.ID] = model
		}
	}
	if s.holidayReader != nil {
		if prefetch.holidays, err = s.holidayReader.HolidayDates(ctx, from, to); err != nil {
			return nil, fmt.Errorf("failed to prefetch non-working days: %w", err)
		}
	}
	if prefetch.snapshots, err = s.prefetchSnapshots(ctx, staffIDs); err != nil {
		return nil, err
	}
	return prefetch, nil
}

// prefetchSnapshots loads the frozen months of the given staff members.
//
// Deliberately unbounded in time: the splice asks for "the newest snapshot at
// or before month M" for several different M, so a month-bounded prefetch could
// not answer them all. At most one active row exists per staff member and
// month, so the set stays small. The generic filtered List is enough — no new
// repository method needed.
func (s *staffOverviewService) prefetchSnapshots(
	ctx context.Context,
	staffIDs []int64,
) (map[int64][]*activeModels.StaffMonthBalanceSnapshot, error) {
	result := make(map[int64][]*activeModels.StaffMonthBalanceSnapshot, len(staffIDs))
	if len(staffIDs) == 0 {
		return result, nil
	}
	ids := make([]any, 0, len(staffIDs))
	for _, id := range staffIDs {
		ids = append(ids, id)
	}
	options := modelBase.NewQueryOptions()
	options.Filter = options.Filter.In("staff_id", ids...).IsNull("reopened_at")
	snapshots, err := s.snapshotRepo.List(ctx, options)
	if err != nil {
		return nil, fmt.Errorf("failed to prefetch month balance snapshots: %w", err)
	}
	for _, snapshot := range snapshots {
		result[snapshot.StaffID] = append(result[snapshot.StaffID], snapshot)
	}
	return result, nil
}

// --- overview list ---------------------------------------------------------

func (s *staffOverviewService) GetTimeTrackingOverview(ctx context.Context, filters OverviewFilters) (*TimeTrackingOverview, error) {
	if filters.EmploymentType != "" && !slices.Contains(validEmploymentTypes(), filters.EmploymentType) {
		return nil, fmt.Errorf("%w: unknown employment_type %q", ErrOverviewInvalid, filters.EmploymentType)
	}
	if filters.SortBy != "" && !slices.Contains(ValidOverviewSortKeys, filters.SortBy) {
		return nil, fmt.Errorf("%w: unknown sort key %q", ErrOverviewInvalid, filters.SortBy)
	}
	if filters.SaldoMin != nil && filters.SaldoMax != nil && *filters.SaldoMin > *filters.SaldoMax {
		return nil, fmt.Errorf("%w: saldo_min must not exceed saldo_max", ErrOverviewInvalid)
	}

	today := s.today()
	key, err := resolveOverviewMonth(filters, today)
	if err != nil {
		return nil, err
	}
	overview := &TimeTrackingOverview{
		Year:  key.Year,
		Month: key.Month,
		Rows:  []TimeTrackingOverviewRow{},
	}

	staffMembers, err := s.activeStaff(ctx)
	if err != nil {
		return nil, err
	}
	if filters.EmploymentType != "" {
		staffMembers = slices.DeleteFunc(staffMembers, func(staff *userModels.Staff) bool {
			return staff.EmploymentType == nil || *staff.EmploymentType != filters.EmploymentType
		})
	}
	if len(staffMembers) == 0 {
		return overview, nil
	}

	prefetch, err := s.buildPrefetch(ctx, staffMembers, nil, key)
	if err != nil {
		return nil, err
	}
	monthSvc := newPrefetchedMonthService(prefetch, s.logger)

	staffIDs := make([]int64, 0, len(staffMembers))
	for _, staff := range staffMembers {
		staffIDs = append(staffIDs, staff.ID)
	}
	// Resturlaub is counted in the year of the SELECTED month, not of today:
	// a December view opened in January must not show the new year's quota.
	quotas, err := s.quotaRepo.GetByStaffIDsAndYear(ctx, staffIDs, key.Year)
	if err != nil {
		return nil, fmt.Errorf("failed to prefetch vacation quotas: %w", err)
	}
	// Loaded separately rather than reused from the month prefetch: that
	// window starts at the account anchor, which may lie after January 1st.
	// Current-month views keep the annual semantics of the quota endpoint.
	// Historical views stop at month end so later vacation does not rewrite
	// an earlier Resturlaub. One extra query, constant in N.
	vacationThrough := timezone.NewDate(key.Year, time.December, 31)
	if key.before(monthOf(today)) {
		vacationThrough = key.lastDay()
	}
	vacationAbsences, err := s.absenceRepo.GetByStaffIDsAndDateRange(ctx, staffIDs,
		timezone.NewDate(key.Year, time.January, 1),
		vacationThrough,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to prefetch vacation absences: %w", err)
	}
	openings, err := s.prefetchVacationOpenings(ctx, staffIDs, key.Year)
	if err != nil {
		return nil, err
	}

	for _, staff := range staffMembers {
		summary, err := monthSvc.GetMonthSummary(ctx, staff.ID, key.Year, key.Month)
		if err != nil {
			// A per-staff failure would otherwise leave a school total that is
			// silently short. Fail the request and name the staff member.
			return nil, fmt.Errorf("failed to compute month summary for staff %d: %w", staff.ID, err)
		}
		balanceMinutes := summary.ClosingBalanceMinutes
		if summary.IsClosed && summary.FrozenClosingBalanceMinutes != nil {
			balanceMinutes = *summary.FrozenClosingBalanceMinutes
		}
		row := TimeTrackingOverviewRow{
			StaffID:        staff.ID,
			SollMinutes:    summary.TargetMinutesToDate,
			IstMinutes:     summary.ActualMinutes,
			BalanceMinutes: balanceMinutes,
			RemainingVacationDays: s.remainingVacationDays(
				staff.ID,
				key.Year,
				vacationThrough,
				quotas[staff.ID],
				vacationAbsences[staff.ID],
				openings[staff.ID],
			),
		}
		if staff.EmploymentType != nil {
			row.EmploymentType = *staff.EmploymentType
		}
		if staff.Person != nil {
			row.FirstName, row.LastName = staff.Person.FirstName, staff.Person.LastName
			row.Name = strings.TrimSpace(staff.Person.FirstName + " " + staff.Person.LastName)
		}
		if filters.SaldoMin != nil && row.BalanceMinutes < *filters.SaldoMin {
			continue
		}
		if filters.SaldoMax != nil && row.BalanceMinutes > *filters.SaldoMax {
			continue
		}
		overview.Rows = append(overview.Rows, row)
	}

	sortOverviewRows(overview.Rows, filters)
	return overview, nil
}

// remainingVacationDays reuses the absence service's arithmetic through the
// selected cutoff, including the default entitlement fallback and the
// reservation for requested/question rows.
func (s *staffOverviewService) remainingVacationDays(
	staffID int64,
	year int,
	through timezone.Date,
	quota *activeModels.StaffVacationQuota,
	absences []*activeModels.StaffAbsence,
	opening *activeModels.StaffVacationOpening,
) float64 {
	entitled, carryover := defaultEntitledDays, 0.0
	if quota != nil {
		entitled, carryover = quota.EntitledDays, quota.CarryoverDays
	}
	return computeVacationQuotaSummaryThrough(staffID, year, entitled, carryover, through, absences, opening).RemainingDays
}

// SetVacationOpeningReader wires the vacation takeover reader (#2132).
// Setter injection so the 12-parameter constructor and its unit fixtures
// stay unchanged; nil means the overview ignores takeovers.
func (s *staffOverviewService) SetVacationOpeningReader(repo activeModels.StaffVacationOpeningRepository) {
	s.openingRepo = repo
}

func (s *staffOverviewService) prefetchVacationOpenings(ctx context.Context, staffIDs []int64, year int) (map[int64]*activeModels.StaffVacationOpening, error) {
	if s.openingRepo == nil {
		return map[int64]*activeModels.StaffVacationOpening{}, nil
	}
	openings, err := s.openingRepo.GetByStaffIDsAndYear(ctx, staffIDs, year)
	if err != nil {
		return nil, fmt.Errorf("failed to prefetch vacation openings: %w", err)
	}
	return openings, nil
}

// resolveOverviewMonth turns the requested year/month into the month to
// compute. Both zero means "current month" so the parameter stays optional.
func resolveOverviewMonth(filters OverviewFilters, today timezone.Date) (monthKey, error) {
	if filters.Year == 0 && filters.Month == 0 {
		return monthOf(today), nil
	}
	if filters.Year < 2000 || filters.Year > 2100 || filters.Month < 1 || filters.Month > 12 {
		return monthKey{}, fmt.Errorf("%w: year/month out of range", ErrOverviewInvalid)
	}
	key := monthKey{Year: filters.Year, Month: filters.Month}
	if monthOf(today).before(key) {
		return monthKey{}, fmt.Errorf("%w: month %04d-%02d lies in the future", ErrOverviewInvalid, key.Year, key.Month)
	}
	return key, nil
}

// sortOverviewRows sorts by the requested key with an explicit staff-ID
// tiebreak. ListAllWithPerson has no ORDER BY, so without the tiebreak two
// staff members with the same balance (0 is very common) would swap places
// between identical requests and the table would flicker.
func sortOverviewRows(rows []TimeTrackingOverviewRow, filters OverviewFilters) {
	key := cmp.Or(filters.SortBy, OverviewSortName)
	slices.SortStableFunc(rows, func(a, b TimeTrackingOverviewRow) int {
		order := compareOverviewRows(a, b, key)
		if filters.Descending {
			return -order
		}
		return order
	})
}

// compareOverviewRows is a total order: the staff-ID tiebreak means two
// distinct rows never compare equal.
func compareOverviewRows(a, b TimeTrackingOverviewRow, key string) int {
	var order int
	switch key {
	case OverviewSortBalance:
		order = cmp.Compare(a.BalanceMinutes, b.BalanceMinutes)
	case OverviewSortSoll:
		order = cmp.Compare(a.SollMinutes, b.SollMinutes)
	case OverviewSortIst:
		order = cmp.Compare(a.IstMinutes, b.IstMinutes)
	case OverviewSortVacation:
		order = cmp.Compare(a.RemainingVacationDays, b.RemainingVacationDays)
	default:
		order = cmp.Compare(
			strings.ToLower(a.LastName+" "+a.FirstName),
			strings.ToLower(b.LastName+" "+b.FirstName),
		)
	}
	return cmp.Or(order, cmp.Compare(a.StaffID, b.StaffID))
}

func validEmploymentTypes() []string {
	return []string{
		userModels.EmploymentTypeFullTime,
		userModels.EmploymentTypePartTime,
		userModels.EmploymentTypeMinijob,
	}
}

// activeStaff is the set every KPI is intersected against: tenant-scoped and
// soft-delete-filtered. "Active" means not offboarded.
func (s *staffOverviewService) activeStaff(ctx context.Context) ([]*userModels.Staff, error) {
	staffMembers, err := s.staffRepo.ListAllWithPerson(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list staff: %w", err)
	}
	return staffMembers, nil
}

// --- dashboard -------------------------------------------------------------

func (s *staffOverviewService) GetDashboardSummary(ctx context.Context, period string) (*DashboardSummary, error) {
	if period == "" {
		period = OverviewPeriodMonth
	}
	if period != OverviewPeriodWeek && period != OverviewPeriodMonth {
		return nil, fmt.Errorf("%w: period must be %s or %s", ErrOverviewInvalid, OverviewPeriodWeek, OverviewPeriodMonth)
	}

	today := s.today()
	key := monthOf(today)
	summary := &DashboardSummary{}

	staffMembers, err := s.activeStaff(ctx)
	if err != nil {
		return nil, err
	}
	summary.ActiveStaffCount = len(staffMembers)
	activeIDs := make(map[int64]bool, len(staffMembers))
	for _, staff := range staffMembers {
		activeIDs[staff.ID] = true
	}

	if err := s.addTodayCounters(ctx, summary, activeIDs, today); err != nil {
		return nil, err
	}
	pending, err := s.countPendingRequests(ctx)
	if err != nil {
		return nil, err
	}
	summary.PendingRequestsCount = pending

	if len(staffMembers) == 0 {
		return summary, nil
	}

	// period=week can reach into the previous month, so the prefetch window
	// has to start no later than that Monday.
	var lower *timezone.Date
	weekStart := calendarDate(configModels.MondayOf(workforceDate(today)))
	if period == OverviewPeriodWeek {
		lower = &weekStart
	}
	prefetch, err := s.buildPrefetch(ctx, staffMembers, lower, key)
	if err != nil {
		return nil, err
	}
	monthSvc := newPrefetchedMonthService(prefetch, s.logger)

	weekEnd := weekStart.AddDays(6)
	if weekEnd.After(today) {
		// Clamp at today so Soll and Ist are comparable, mirroring the month
		// card's target_to_date.
		weekEnd = today
	}
	for _, staff := range staffMembers {
		monthSummary, err := monthSvc.GetMonthSummary(ctx, staff.ID, key.Year, key.Month)
		if err != nil {
			return nil, fmt.Errorf("failed to compute month summary for staff %d: %w", staff.ID, err)
		}
		summary.SaldoSchoolTotalMinutes += monthSummary.ClosingBalanceMinutes

		if period == OverviewPeriodMonth {
			summary.Period.SollMinutes += monthSummary.TargetMinutesToDate
			summary.Period.IstMinutes += monthSummary.ActualMinutes
			summary.Period.DeltaMinutes += monthSummary.BalanceMinutes
			continue
		}
		week, err := monthSvc.GetRangeAggregate(ctx, staff.ID, weekStart, weekEnd)
		if err != nil {
			return nil, fmt.Errorf("failed to compute week aggregate for staff %d: %w", staff.ID, err)
		}
		summary.Period.SollMinutes += week.TargetMinutes
		summary.Period.IstMinutes += week.ActualMinutes
		summary.Period.DeltaMinutes += week.BalanceMinutes
	}
	return summary, nil
}

// addTodayCounters fills the presence and absence KPIs.
//
// Every set is intersected with the active-staff IDs: the presence, absence and
// shift lookups do not join users.staff, and a leaver's sessions and absences
// survive offboarding, so counting them raw would overstate each KPI.
func (s *staffOverviewService) addTodayCounters(
	ctx context.Context,
	summary *DashboardSummary,
	activeIDs map[int64]bool,
	today timezone.Date,
) error {
	presence, err := s.sessionRepo.GetTodayPresenceMap(ctx)
	if err != nil {
		return fmt.Errorf("failed to load presence map: %w", err)
	}
	clockedIn := make(map[int64]bool, len(presence))
	for staffID, status := range presence {
		// The map stores the session status for an open session and the
		// sentinel "checked_out" otherwise; match the open statuses
		// positively so a new status value cannot be miscounted as present.
		if !activeIDs[staffID] {
			continue
		}
		if status == activeModels.WorkSessionStatusPresent || status == activeModels.WorkSessionStatusHomeOffice {
			clockedIn[staffID] = true
		}
	}
	// A session opened before midnight and never closed is still someone at
	// work, but it is not in today's presence map.
	openSessions, err := s.sessionRepo.GetOpenSessions(ctx, today)
	if err != nil {
		return fmt.Errorf("failed to load open sessions: %w", err)
	}
	for _, session := range openSessions {
		if activeIDs[session.StaffID] {
			clockedIn[session.StaffID] = true
		}
	}
	summary.CurrentlyClockedIn = len(clockedIn)

	absences, err := s.absenceRepo.GetAbsenceMapForDate(ctx, today)
	if err != nil {
		return fmt.Errorf("failed to load absence map: %w", err)
	}
	// The map keeps ONE type per staff member by priority
	// (sick > training > vacation > comp_time > other), so somebody with
	// overlapping sick and vacation counts as sick only and the two counters
	// never double-count a person.
	for staffID, absenceType := range absences {
		if !activeIDs[staffID] {
			continue
		}
		switch absenceType {
		case activeModels.AbsenceTypeSick:
			summary.SickToday++
		case activeModels.AbsenceTypeVacation:
			summary.VacationToday++
		}
	}

	expected, err := s.expectedClockedIn(ctx, activeIDs, today)
	if err != nil {
		return err
	}
	summary.ExpectedClockedIn = expected
	return nil
}

// expectedClockedIn counts staff members whose planned shift covers this very
// moment. Shift rows carry wall-clock TIME columns, normalized by the repo.
func (s *staffOverviewService) expectedClockedIn(
	ctx context.Context,
	activeIDs map[int64]bool,
	today timezone.Date,
) (int, error) {
	shifts, err := s.shiftRepo.FindByDateRange(ctx, today, today)
	if err != nil {
		return 0, fmt.Errorf("failed to load today's shifts: %w", err)
	}
	nowWall := timezone.NormalizeWallClock(timezone.Now())
	expected := make(map[int64]bool)
	for _, shift := range shifts {
		if shift.Cancelled || !activeIDs[shift.StaffID] {
			continue
		}
		start, end := timezone.NormalizeWallClock(shift.StartTime), timezone.NormalizeWallClock(shift.EndTime)
		if !nowWall.Before(start) && !nowWall.After(end) {
			expected[shift.StaffID] = true
		}
	}
	return len(expected), nil
}

// countPendingRequests counts absences awaiting a decision — the same set
// ListPendingRequests returns, counted through the generic filter API so no
// rows are loaded.
func (s *staffOverviewService) countPendingRequests(ctx context.Context) (int, error) {
	options := modelBase.NewQueryOptions()
	options.Filter = options.Filter.In("status",
		activeModels.AbsenceStatusRequested,
		activeModels.AbsenceStatusQuestion,
	)
	count, err := s.absenceRepo.CountWithOptions(ctx, options)
	if err != nil {
		return 0, fmt.Errorf("failed to count pending absence requests: %w", err)
	}
	return count, nil
}
