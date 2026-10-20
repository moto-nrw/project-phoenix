package timetracking

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
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
	staffRepo      OverviewStaffQuery
	sessionRepo    WorkSessionRecords
	breakRepo      WorkSessionBreakRecords
	absenceRepo    AbsenceRecords
	adjustmentRepo BalanceAdjustmentRecords
	quotaRepo      VacationQuotaRecords
	snapshotRepo   MonthSnapshots
	scheduleRepo   OverviewWorkSchedules
	workModelRepo  OverviewWorkTimeModels
	shiftRepo      StaffOverviewShifts
	settings       monthSettingsResolver
	holidayReader  HolidayDatesReader
	overrideReader TargetOverrideReader
	// openingRepo carries the vacation takeover rows (#2132); supplied by
	// WithOverviewVacationOpenings, nil in bare unit fixtures.
	openingRepo VacationOpeningRecords
	logger      *slog.Logger

	// todayFunc is a test hook; production uses timezone.TodayDate.
	todayFunc func() calendar.Date
}

func NewStaffOverviewService(
	staffRepo OverviewStaffQuery,
	sessionRepo WorkSessionRecords,
	breakRepo WorkSessionBreakRecords,
	absenceRepo AbsenceRecords,
	adjustmentRepo BalanceAdjustmentRecords,
	quotaRepo VacationQuotaRecords,
	snapshotRepo MonthSnapshots,
	scheduleRepo OverviewWorkSchedules,
	workModelRepo OverviewWorkTimeModels,
	shiftRepo StaffOverviewShifts,
	settings monthSettingsResolver,
	logger *slog.Logger,
	opts ...StaffOverviewOption,
) *staffOverviewService {
	service := &staffOverviewService{
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
	for _, opt := range opts {
		opt(service)
	}
	return service
}

func (s *staffOverviewService) getLogger() *slog.Logger {
	return loggerOrDefault(s.logger)
}

func (s *staffOverviewService) today() calendar.Date {
	if s.todayFunc != nil {
		return s.todayFunc()
	}
	return calendar.TodayDate()
}

// --- overview list ---------------------------------------------------------

func (s *staffOverviewService) GetTimeTrackingOverview(ctx context.Context, filters OverviewFilters) (*TimeTrackingOverview, error) {
	if err := validateOverviewFilters(filters); err != nil {
		return nil, err
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
	staffMembers = filterOverviewEmploymentType(staffMembers, filters.EmploymentType)
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
	vacation, err := s.prefetchOverviewVacation(ctx, staffIDs, key, today)
	if err != nil {
		return nil, err
	}

	for _, staff := range staffMembers {
		row, err := s.overviewRow(ctx, monthSvc, staff, key, vacation)
		if err != nil {
			return nil, err
		}
		if overviewSaldoInRange(row.BalanceMinutes, filters) {
			overview.Rows = append(overview.Rows, row)
		}
	}

	sortOverviewRows(overview.Rows, filters)
	return overview, nil
}

// filterOverviewEmploymentType keeps the staff members of the requested
// employment type; an empty type keeps everybody.
func filterOverviewEmploymentType(staffMembers []OverviewStaff, employmentType string) []OverviewStaff {
	if employmentType == "" {
		return staffMembers
	}
	return slices.DeleteFunc(staffMembers, func(staff OverviewStaff) bool {
		return staff.EmploymentType == nil || *staff.EmploymentType != employmentType
	})
}

// overviewSaldoInRange applies the saldo bounds, which only exist once the
// carry chain has run.
func overviewSaldoInRange(balanceMinutes int, filters OverviewFilters) bool {
	if filters.SaldoMin != nil && balanceMinutes < *filters.SaldoMin {
		return false
	}
	return filters.SaldoMax == nil || balanceMinutes <= *filters.SaldoMax
}

// validateOverviewFilters rejects unknown filter values before anything is
// read.
func validateOverviewFilters(filters OverviewFilters) error {
	if filters.EmploymentType != "" && !slices.Contains(validEmploymentTypes(), filters.EmploymentType) {
		return fmt.Errorf("%w: unknown employment_type %q", ErrOverviewInvalid, filters.EmploymentType)
	}
	if filters.SortBy != "" && !slices.Contains(ValidOverviewSortKeys, filters.SortBy) {
		return fmt.Errorf("%w: unknown sort key %q", ErrOverviewInvalid, filters.SortBy)
	}
	if filters.SaldoMin != nil && filters.SaldoMax != nil && *filters.SaldoMin > *filters.SaldoMax {
		return fmt.Errorf("%w: saldo_min must not exceed saldo_max", ErrOverviewInvalid)
	}
	return nil
}

// overviewVacation is the Resturlaub input of the overview rows: the quotas,
// the vacation absences up to the cutoff and the takeover rows.
type overviewVacation struct {
	through  calendar.Date
	quotas   map[int64]*StaffVacationQuota
	absences map[int64][]*StaffAbsence
	openings map[int64]*StaffVacationOpening
}

// prefetchOverviewVacation loads the Resturlaub inputs. Resturlaub is counted
// in the year of the SELECTED month, not of today: a December view opened in
// January must not show the new year's quota.
func (s *staffOverviewService) prefetchOverviewVacation(ctx context.Context, staffIDs []int64, key monthKey, today calendar.Date) (overviewVacation, error) {
	quotaValues, err := s.quotaRepo.ListStaffVacationQuotas(ctx, workforce.StaffVacationFilter{StaffIDs: staffIDs, Year: key.Year})
	quotas := quotasByStaff(quotaValues)
	err = rowReadError("get vacation quotas by staff IDs+year", err, nil)
	if err != nil {
		return overviewVacation{}, fmt.Errorf("failed to prefetch vacation quotas: %w", err)
	}
	// Loaded separately rather than reused from the month prefetch: that
	// window starts at the account anchor, which may lie after January 1st.
	// Current-month views keep the annual semantics of the quota endpoint.
	// Historical views stop at month end so later vacation does not rewrite
	// an earlier Resturlaub. One extra query, constant in N.
	vacationThrough := calendar.NewDate(key.Year, time.December, 31)
	if key.before(monthOf(today)) {
		vacationThrough = key.lastDay()
	}
	absenceValues, err := s.absenceRepo.ListStaffAbsences(ctx, workforce.StaffAbsenceFilter{StaffIDs: staffIDs, OverlapFrom: calendar.NewDate(key.Year, time.January, 1).String(), OverlapTo: vacationThrough.String(), Order: []workforce.StaffAbsenceOrder{{Field: workforce.StaffAbsenceOrderStaffID}, {Field: workforce.StaffAbsenceOrderDateStart}}})
	vacationAbsences := staffAbsencesByID(absenceValues)
	err = rowReadError("get absences by staff IDs and date range", err, nil)
	if err != nil {
		return overviewVacation{}, fmt.Errorf("failed to prefetch vacation absences: %w", err)
	}
	openings, err := s.prefetchVacationOpenings(ctx, staffIDs, key.Year)
	if err != nil {
		return overviewVacation{}, err
	}
	return overviewVacation{through: vacationThrough, quotas: quotas, absences: vacationAbsences, openings: openings}, nil
}

// overviewRow computes one staff member's row of the overview.
func (s *staffOverviewService) overviewRow(ctx context.Context, monthSvc WorkTimeMonthService, staff OverviewStaff, key monthKey, vacation overviewVacation) (TimeTrackingOverviewRow, error) {
	summary, err := monthSvc.GetMonthSummary(ctx, staff.ID, key.Year, key.Month)
	if err != nil {
		// A per-staff failure would otherwise leave a school total that is
		// silently short. Fail the request and name the staff member.
		return TimeTrackingOverviewRow{}, fmt.Errorf("failed to compute month summary for staff %d: %w", staff.ID, err)
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
			vacation.through,
			vacation.quotas[staff.ID],
			vacation.absences[staff.ID],
			vacation.openings[staff.ID],
		),
	}
	if staff.EmploymentType != nil {
		row.EmploymentType = *staff.EmploymentType
	}
	row.FirstName, row.LastName = staff.FirstName, staff.LastName
	row.Name = strings.TrimSpace(staff.FirstName + " " + staff.LastName)
	return row, nil
}

// remainingVacationDays reuses the absence service's arithmetic through the
// selected cutoff, including the default entitlement fallback and the
// reservation for requested/question rows.
func (s *staffOverviewService) remainingVacationDays(
	staffID int64,
	year int,
	through calendar.Date,
	quota *StaffVacationQuota,
	absences []*StaffAbsence,
	opening *StaffVacationOpening,
) float64 {
	entitled, carryover := defaultEntitledDays, 0.0
	if quota != nil {
		entitled, carryover = quota.EntitledDays, quota.CarryoverDays
	}
	return computeVacationQuotaSummaryThrough(staffID, year, entitled, carryover, through, absences, opening).RemainingDays
}

func (s *staffOverviewService) prefetchVacationOpenings(ctx context.Context, staffIDs []int64, year int) (map[int64]*StaffVacationOpening, error) {
	if s.openingRepo == nil {
		return map[int64]*StaffVacationOpening{}, nil
	}
	openingValues, err := s.openingRepo.ListStaffVacationOpenings(ctx, workforce.StaffVacationFilter{StaffIDs: staffIDs, Year: year})
	openings := openingsByStaff(openingValues)
	err = rowReadError("get vacation openings by staff IDs+year", err, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to prefetch vacation openings: %w", err)
	}
	return openings, nil
}

// resolveOverviewMonth turns the requested year/month into the month to
// compute. Both zero means "current month" so the parameter stays optional.
func resolveOverviewMonth(filters OverviewFilters, today calendar.Date) (monthKey, error) {
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
	key := filters.SortBy
	if key == "" {
		key = OverviewSortName
	}
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
		order = compareOrdered(a.BalanceMinutes, b.BalanceMinutes)
	case OverviewSortSoll:
		order = compareOrdered(a.SollMinutes, b.SollMinutes)
	case OverviewSortIst:
		order = compareOrdered(a.IstMinutes, b.IstMinutes)
	case OverviewSortVacation:
		order = compareOrdered(a.RemainingVacationDays, b.RemainingVacationDays)
	default:
		order = compareOrdered(
			strings.ToLower(a.LastName+" "+a.FirstName),
			strings.ToLower(b.LastName+" "+b.FirstName),
		)
	}
	return orderOr(order, compareOrdered(a.StaffID, b.StaffID))
}

func validEmploymentTypes() []string {
	return []string{
		EmploymentTypeFullTime,
		EmploymentTypePartTime,
		EmploymentTypeMinijob,
	}
}

// activeStaff is the set every KPI is intersected against: tenant-scoped and
// soft-delete-filtered. "Active" means not offboarded.
func (s *staffOverviewService) activeStaff(ctx context.Context) ([]OverviewStaff, error) {
	staffMembers, err := s.staffRepo.ListOverviewStaff(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list staff: %w", err)
	}
	return slices.DeleteFunc(staffMembers, func(member OverviewStaff) bool { return member.Guest }), nil
}
