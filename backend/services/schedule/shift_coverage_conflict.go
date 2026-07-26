package schedule

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/sliceutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModel "github.com/moto-nrw/project-phoenix/models/users"
)

const (
	// A school-year series contains at most roughly 260 selected weekdays. The
	// defensive 366 bounds admit a full leap-year candidate set without
	// allowing an unbounded read through a malformed API request.
	maxShiftCoverageCandidateDates   = 366
	maxShiftCoverageSpanDays         = 366
	maxShiftCoverageStaffIDs         = 500
	maxShiftCoverageComparisons      = 10_000
	maxReturnedShiftCoverageWarnings = 100
)

// ErrInvalidShiftCoverageQuery identifies caller-controlled validation
// failures; the API maps every wrapped instance to one stable 400 response.
var ErrInvalidShiftCoverageQuery = errors.New("invalid shift coverage query")

// ShiftCoverageWarning is one advisory uncovered interval returned by the
// dedicated shift-coverage probe. It never blocks the subsequent write.
type ShiftCoverageWarning struct {
	StaffID            int64  `json:"staff_id"`
	StaffName          string `json:"staff_name"`
	Date               string `json:"date"`
	StartTime          string `json:"start_time"`
	EndTime            string `json:"end_time"`
	UncoveredStartTime string `json:"uncovered_start_time"`
	UncoveredEndTime   string `json:"uncovered_end_time"`
	Message            string `json:"message"`
}

type ShiftCoverageResult struct {
	Warnings          []ShiftCoverageWarning
	TotalWarningCount int
}

// ShiftCoverageQuery describes one single-occurrence or recurring-series
// availability probe. Dates are explicit candidate weekdays supplied by the
// caller; optional period/A-B filtering selects the actual occurrences.
type ShiftCoverageQuery struct {
	Dates                 []timezone.Date
	StartTime             time.Time
	EndTime               time.Time
	StaffIDs              []int64
	ExcludeInstanceID     *int64
	ConcreteInstanceDate  *timezone.Date
	ReplanActivityGroupID *int64
	CalendarPeriodID      *int64
	WeekPattern           *int
}

type StaffWithPersonBatchReader interface {
	FindWithPersonByIDs(ctx context.Context, ids []int64) (map[int64]*usersModel.Staff, error)
}

type CalendarPeriodByIDReader interface {
	FindByID(ctx context.Context, id any) (*scheduleModel.CalendarPeriod, error)
}

type ActivityGroupInstanceRangeReader interface {
	FindByActivityGroupAndDateRange(ctx context.Context, activityGroupID int64, from, to timezone.Date) ([]*scheduleModel.ActivityInstance, error)
}

type ActivityExceptionRangeReader interface {
	FindByActivityGroupAndDateRange(ctx context.Context, activityGroupID int64, from, to timezone.Date) ([]*scheduleModel.ActivityException, error)
}

type ActivityScheduleGroupReader interface {
	FindByGroupID(ctx context.Context, groupID int64) ([]*activitiesModel.Schedule, error)
}

type ShiftCoverageDependencies struct {
	Shifts          StaffShiftCoverageReader
	Instances       ActivityGroupInstanceRangeReader
	Exceptions      ActivityExceptionRangeReader
	Schedules       ActivityScheduleGroupReader
	InstanceStaff   InstanceStaffBatchReader
	Staff           StaffWithPersonBatchReader
	CalendarPeriods CalendarPeriodByIDReader
}

type StaffShiftCoverageReader interface {
	FindByStaffIDsAndDates(ctx context.Context, staffIDs []int64, dates []timezone.Date) ([]*scheduleModel.StaffShift, error)
	FindUsedCalendarWeeks(ctx context.Context, start, end timezone.Date) ([]timezone.Date, error)
}

// DetectShiftCoverageWarnings checks every effective occurrence with a fixed
// number of batched reads. A ReplanWeek projection uses at most eight reads:
// period, the group's recurrence envelope, group-scoped instances,
// group-scoped exceptions, all concrete staff rows, all shifts, and all
// tenant week usage, and warning names.
func DetectShiftCoverage(ctx context.Context, deps ShiftCoverageDependencies, query ShiftCoverageQuery) (ShiftCoverageResult, error) {
	dates, staffIDs, err := normalizeShiftCoverageQuery(query)
	if err != nil {
		return ShiftCoverageResult{}, err
	}
	dates, err = resolveShiftCoverageDates(ctx, deps.CalendarPeriods, query, dates)
	if err != nil {
		return ShiftCoverageResult{}, err
	}
	if len(dates) == 0 {
		return emptyShiftCoverageResult(), nil
	}
	if query.ReplanActivityGroupID != nil {
		dates, err = resolveSeriesCoverageDates(ctx, deps.Schedules, *query.ReplanActivityGroupID, dates)
		if err != nil {
			return ShiftCoverageResult{}, err
		}
		if len(dates) == 0 {
			return emptyShiftCoverageResult(), nil
		}
	}
	coverageDays, effectiveStaffIDs, err := resolveEffectiveCoverageDays(
		ctx,
		deps,
		query,
		dates,
		staffIDs,
	)
	if err != nil {
		return ShiftCoverageResult{}, err
	}
	if len(effectiveStaffIDs) == 0 {
		return emptyShiftCoverageResult(), nil
	}
	shifts, usedWeeks, err := loadShiftCoverageData(ctx, deps.Shifts, dates, effectiveStaffIDs)
	if err != nil {
		return ShiftCoverageResult{}, err
	}
	if len(usedWeeks) == 0 {
		return emptyShiftCoverageResult(), nil
	}
	pending, total := collectPendingCoverageWarnings(dates, coverageDays, shifts, usedWeeks)
	if len(pending) == 0 {
		return emptyShiftCoverageResult(), nil
	}
	staff, loadErr := deps.Staff.FindWithPersonByIDs(ctx, effectiveStaffIDs)
	if loadErr != nil {
		return ShiftCoverageResult{}, fmt.Errorf("load staff names for coverage warnings: %w", loadErr)
	}
	return ShiftCoverageResult{Warnings: buildShiftCoverageWarnings(pending, staff), TotalWarningCount: total}, nil
}

// DetectShiftCoverageWarnings preserves the warning-only service contract used
// by existing non-HTTP callers. The API uses DetectShiftCoverage to expose the
// total count alongside the bounded detail list.
func DetectShiftCoverageWarnings(ctx context.Context, deps ShiftCoverageDependencies, query ShiftCoverageQuery) ([]ShiftCoverageWarning, error) {
	result, err := DetectShiftCoverage(ctx, deps, query)
	return result.Warnings, err
}

func emptyShiftCoverageResult() ShiftCoverageResult {
	return ShiftCoverageResult{Warnings: make([]ShiftCoverageWarning, 0)}
}

func resolveShiftCoverageDates(
	ctx context.Context,
	periods CalendarPeriodByIDReader,
	query ShiftCoverageQuery,
	dates []timezone.Date,
) ([]timezone.Date, error) {
	if query.CalendarPeriodID == nil {
		return dates, nil
	}
	period, err := periods.FindByID(ctx, *query.CalendarPeriodID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, invalidShiftCoverageQuery("calendar period does not exist for this tenant")
		}
		return nil, fmt.Errorf("load calendar period for shift coverage: %w", err)
	}
	if period == nil {
		return nil, invalidShiftCoverageQuery("calendar period does not exist for this tenant")
	}
	if !period.IsActive {
		return []timezone.Date{}, nil
	}
	filtered := filterShiftCoverageDates(dates, period, *query.WeekPattern)
	if query.ConcreteInstanceDate != nil && containsCoverageDate(dates, *query.ConcreteInstanceDate) &&
		!containsCoverageDate(filtered, *query.ConcreteInstanceDate) {
		filtered = append(filtered, *query.ConcreteInstanceDate)
		sort.Slice(filtered, func(i, j int) bool { return filtered[i].Before(filtered[j]) })
	}
	return filtered, nil
}

func resolveSeriesCoverageDates(
	ctx context.Context,
	schedules ActivityScheduleGroupReader,
	activityGroupID int64,
	dates []timezone.Date,
) ([]timezone.Date, error) {
	if schedules == nil {
		return nil, errors.New("activity schedule reader is not wired")
	}
	rows, err := schedules.FindByGroupID(ctx, activityGroupID)
	if err != nil {
		return nil, fmt.Errorf("load series recurrence envelope: %w", err)
	}
	if len(rows) == 0 {
		return nil, invalidShiftCoverageQuery("activity group schedules do not exist for this tenant")
	}
	validFrom, validUntil, err := commonScheduleValidityEnvelope(rows)
	if err != nil {
		return nil, fmt.Errorf("load series recurrence envelope: %w", err)
	}
	envelope := &activitiesModel.Schedule{ValidFrom: validFrom, ValidUntil: validUntil}
	filtered := make([]timezone.Date, 0, len(dates))
	for _, date := range dates {
		if scheduleNotStartedOn(envelope, date) || scheduleEndedOn(envelope, date) {
			continue
		}
		filtered = append(filtered, date)
	}
	return filtered, nil
}

type effectiveShiftCoverageDay struct {
	staffIDs []int64
	start    time.Time
	end      time.Time
	active   bool
}

func resolveEffectiveCoverageDays(
	ctx context.Context,
	deps ShiftCoverageDependencies,
	query ShiftCoverageQuery,
	dates []timezone.Date,
	staffIDs []int64,
) (map[timezone.Date]effectiveShiftCoverageDay, []int64, error) {
	coverageDays := initializeEffectiveCoverageDays(query, dates, staffIDs)
	if query.ExcludeInstanceID != nil {
		return resolveConcreteEffectiveCoverageDays(ctx, deps.InstanceStaff, query, dates, staffIDs, coverageDays)
	}
	if query.ReplanActivityGroupID == nil {
		return coverageDays, staffIDs, nil
	}
	return resolveSeriesEffectiveCoverageDays(ctx, deps, query, dates, staffIDs, coverageDays)
}

func initializeEffectiveCoverageDays(
	query ShiftCoverageQuery,
	dates []timezone.Date,
	staffIDs []int64,
) map[timezone.Date]effectiveShiftCoverageDay {
	coverageDays := make(map[timezone.Date]effectiveShiftCoverageDay, len(dates))
	for _, date := range dates {
		coverageDays[date] = effectiveShiftCoverageDay{
			staffIDs: staffIDs,
			start:    query.StartTime,
			end:      query.EndTime,
			active:   true,
		}
	}
	return coverageDays
}

func resolveConcreteEffectiveCoverageDays(
	ctx context.Context,
	instanceStaff InstanceStaffBatchReader,
	query ShiftCoverageQuery,
	dates []timezone.Date,
	staffIDs []int64,
	coverageDays map[timezone.Date]effectiveShiftCoverageDay,
) (map[timezone.Date]effectiveShiftCoverageDay, []int64, error) {
	rows, err := instanceStaff.FindByInstanceIDs(ctx, []int64{*query.ExcludeInstanceID})
	if err != nil {
		return nil, nil, fmt.Errorf("load concrete staff deviations: %w", err)
	}
	concreteDate := dates[0]
	if query.ConcreteInstanceDate != nil {
		concreteDate = *query.ConcreteInstanceDate
	}
	day := coverageDays[concreteDate]
	day.staffIDs = effectiveCoverageStaffIDs(staffIDs, rows)
	coverageDays[concreteDate] = day
	return coverageDays, uniqueEffectiveCoverageStaff(coverageDays, dates), nil
}

func resolveSeriesEffectiveCoverageDays(
	ctx context.Context,
	deps ShiftCoverageDependencies,
	query ShiftCoverageQuery,
	dates []timezone.Date,
	staffIDs []int64,
	coverageDays map[timezone.Date]effectiveShiftCoverageDay,
) (map[timezone.Date]effectiveShiftCoverageDay, []int64, error) {
	activityGroupID := *query.ReplanActivityGroupID

	instances, err := deps.Instances.FindByActivityGroupAndDateRange(
		ctx,
		activityGroupID,
		dates[0],
		dates[len(dates)-1],
	)
	if err != nil {
		return nil, nil, fmt.Errorf("load concrete series occurrences: %w", err)
	}
	exceptions, err := deps.Exceptions.FindByActivityGroupAndDateRange(
		ctx,
		activityGroupID,
		dates[0],
		dates[len(dates)-1],
	)
	if err != nil {
		return nil, nil, fmt.Errorf("load concrete series exceptions: %w", err)
	}
	candidatesByDate, instanceIDs := preservedDeviationCandidates(instances, dates, activityGroupID)
	exceptionsByDate := effectiveSeriesExceptions(exceptions, dates, activityGroupID)
	rowsByInstance, err := loadSeriesCoverageStaffRows(ctx, deps.InstanceStaff, instanceIDs)
	if err != nil {
		return nil, nil, err
	}

	allEffective := make([]int64, 0, len(staffIDs))
	for _, date := range dates {
		day, projectErr := projectSeriesCoverageDay(seriesCoverageProjection{
			day:                 coverageDays[date],
			exception:           exceptionsByDate[date],
			instances:           instances,
			activityGroupID:     activityGroupID,
			date:                date,
			deviationCandidates: candidatesByDate[date],
			rowsByInstance:      rowsByInstance,
			staffIDs:            staffIDs,
		})
		if projectErr != nil {
			return nil, nil, projectErr
		}
		coverageDays[date] = day
		if day.active {
			allEffective = append(allEffective, day.staffIDs...)
		}
	}
	return coverageDays, sliceutil.Unique(allEffective), nil
}

func loadSeriesCoverageStaffRows(
	ctx context.Context,
	instanceStaff InstanceStaffBatchReader,
	instanceIDs []int64,
) (map[int64][]*scheduleModel.InstanceStaff, error) {
	rowsByInstance := make(map[int64][]*scheduleModel.InstanceStaff, len(instanceIDs))
	if len(instanceIDs) == 0 {
		return rowsByInstance, nil
	}
	rows, err := instanceStaff.FindByInstanceIDs(ctx, instanceIDs)
	if err != nil {
		return nil, fmt.Errorf("load concrete series deviations: %w", err)
	}
	for _, row := range rows {
		if row != nil {
			rowsByInstance[row.InstanceID] = append(rowsByInstance[row.InstanceID], row)
		}
	}
	return rowsByInstance, nil
}

type seriesCoverageProjection struct {
	day                 effectiveShiftCoverageDay
	exception           *scheduleModel.ActivityException
	instances           []*scheduleModel.ActivityInstance
	activityGroupID     int64
	date                timezone.Date
	deviationCandidates []*scheduleModel.ActivityInstance
	rowsByInstance      map[int64][]*scheduleModel.InstanceStaff
	staffIDs            []int64
}

func projectSeriesCoverageDay(projection seriesCoverageProjection) (effectiveShiftCoverageDay, error) {
	day := applySeriesCoverageException(projection.day, projection.exception)
	if !day.active {
		return day, nil
	}
	if !timezone.WallClock(day.end).After(timezone.WallClock(day.start)) {
		return day, invalidShiftCoverageQuery("effective exception end time must be after start time")
	}
	if survivingInstanceBlocksSeriesCandidate(projection.instances, projection.activityGroupID, projection.date, day.start) {
		return inactiveCoverageDay(day), nil
	}
	if source := selectPreservedDeviationSource(projection.deviationCandidates, day.start); source != nil {
		day.staffIDs = effectiveSeriesCoverageStaffIDs(projection.staffIDs, projection.rowsByInstance[source.ID])
	}
	return day, nil
}

func applySeriesCoverageException(
	day effectiveShiftCoverageDay,
	exception *scheduleModel.ActivityException,
) effectiveShiftCoverageDay {
	if exception == nil {
		return day
	}
	if exception.IsCancellation() {
		return inactiveCoverageDay(day)
	}
	if exception.StartTime != nil {
		day.start = timezone.WallClock(*exception.StartTime)
	}
	if exception.EndTime != nil {
		day.end = timezone.WallClock(*exception.EndTime)
	}
	return day
}

func inactiveCoverageDay(day effectiveShiftCoverageDay) effectiveShiftCoverageDay {
	day.active = false
	day.staffIDs = nil
	return day
}

// preservedDeviationCandidates selects only the materialized occurrences that
// ReplanWeek may snapshot for the edited template. The range read is tenant
// scoped by the repository; filtering the requested dates and group here keeps
// the operation to one query without introducing another repository shape.
func preservedDeviationCandidates(
	instances []*scheduleModel.ActivityInstance,
	dates []timezone.Date,
	activityGroupID int64,
) (map[timezone.Date][]*scheduleModel.ActivityInstance, []int64) {
	requested := make(map[timezone.Date]struct{}, len(dates))
	for _, date := range dates {
		requested[date] = struct{}{}
	}
	candidates := make(map[timezone.Date][]*scheduleModel.ActivityInstance)
	instanceIDs := make([]int64, 0)
	for _, instance := range instances {
		if instance == nil || instance.ActivityGroupID == nil || *instance.ActivityGroupID != activityGroupID ||
			instance.Status != scheduleModel.InstanceStatusPlanned || instance.IsSpontaneous {
			continue
		}
		if _, selected := requested[instance.Date]; !selected {
			continue
		}
		candidates[instance.Date] = append(candidates[instance.Date], instance)
		instanceIDs = append(instanceIDs, instance.ID)
	}
	return candidates, instanceIDs
}

func effectiveSeriesExceptions(
	exceptions []*scheduleModel.ActivityException,
	dates []timezone.Date,
	activityGroupID int64,
) map[timezone.Date]*scheduleModel.ActivityException {
	requested := make(map[timezone.Date]struct{}, len(dates))
	for _, date := range dates {
		requested[date] = struct{}{}
	}
	byDate := make(map[timezone.Date]*scheduleModel.ActivityException)
	for _, exception := range exceptions {
		if exception == nil || exception.ActivityGroupID != activityGroupID {
			continue
		}
		if _, selected := requested[exception.ExceptionDate]; selected {
			byDate[exception.ExceptionDate] = exception
		}
	}
	return byDate
}

// ReplanWeek deletes only planned, non-spontaneous instances. Any surviving
// instance with the regenerated candidate's key remains in the materializer's
// existing-index and suppresses a replacement occurrence. Such a historical,
// cancelled, active, completed, or spontaneous row is not changed by the
// proposed series edit, so it has no new coverage warning.
func survivingInstanceBlocksSeriesCandidate(
	instances []*scheduleModel.ActivityInstance,
	activityGroupID int64,
	date timezone.Date,
	effectiveStart time.Time,
) bool {
	effectiveStart = timezone.WallClock(effectiveStart)
	for _, instance := range instances {
		if instance == nil || instance.ActivityGroupID == nil || *instance.ActivityGroupID != activityGroupID || instance.Date != date {
			continue
		}
		deletedByReplan := instance.Status == scheduleModel.InstanceStatusPlanned && !instance.IsSpontaneous
		if !deletedByReplan && timezone.WallClock(instance.StartTime).Equal(effectiveStart) {
			return true
		}
	}
	return false
}

// selectPreservedDeviationSource mirrors matchRegeneratedInstance. When a
// template has one occurrence on a day, its deviation follows a changed start
// time. On a multi-slot day, only the old slot with the same start time may
// attach to the regenerated block; otherwise a deleted slot's deviation would
// be attributed to the wrong occurrence.
func selectPreservedDeviationSource(instances []*scheduleModel.ActivityInstance, proposedStart time.Time) *scheduleModel.ActivityInstance {
	if len(instances) == 1 {
		return instances[0]
	}
	proposedStart = timezone.WallClock(proposedStart)
	for _, instance := range instances {
		if timezone.WallClock(instance.StartTime).Equal(proposedStart) {
			return instance
		}
	}
	return nil
}

// effectiveSeriesCoverageStaffIDs projects the roster ReplanWeek will create
// after it reapplies #1871 deviations. Surviving planned absences remain
// absent, active substitutes are restored only up to that surviving absence
// count, and a former substitute promoted into the proposed base roster is a
// normal present assignment.
func effectiveSeriesCoverageStaffIDs(proposed []int64, prior []*scheduleModel.InstanceStaff) []int64 {
	proposedSet := coverageStaffIDSet(proposed)
	absent := survivingSeriesAbsences(prior, proposedSet)
	effective := presentProposedSeriesStaff(proposed, absent)
	effective = append(effective, restoredSeriesSubstitutes(prior, proposedSet, len(absent))...)
	return sliceutil.Unique(effective)
}

func coverageStaffIDSet(staffIDs []int64) map[int64]struct{} {
	staffSet := make(map[int64]struct{}, len(staffIDs))
	for _, staffID := range staffIDs {
		staffSet[staffID] = struct{}{}
	}
	return staffSet
}

func survivingSeriesAbsences(
	prior []*scheduleModel.InstanceStaff,
	proposedSet map[int64]struct{},
) map[int64]struct{} {
	absent := make(map[int64]struct{})
	for _, row := range prior {
		if row == nil || row.IsSubstitute || !row.IsAbsent {
			continue
		}
		if _, survives := proposedSet[row.StaffID]; survives {
			absent[row.StaffID] = struct{}{}
		}
	}
	return absent
}

func presentProposedSeriesStaff(proposed []int64, absent map[int64]struct{}) []int64 {
	effective := make([]int64, 0, len(proposed))
	for _, staffID := range proposed {
		if _, isAbsent := absent[staffID]; !isAbsent {
			effective = append(effective, staffID)
		}
	}
	return effective
}

func restoredSeriesSubstitutes(
	prior []*scheduleModel.InstanceStaff,
	proposedSet map[int64]struct{},
	limit int,
) []int64 {
	restored := make([]int64, 0, limit)
	for _, row := range prior {
		if len(restored) >= limit {
			break
		}
		if isRestorableSeriesSubstitute(row, proposedSet) {
			restored = append(restored, row.StaffID)
		}
	}
	return restored
}

func isRestorableSeriesSubstitute(row *scheduleModel.InstanceStaff, proposedSet map[int64]struct{}) bool {
	if row == nil || !row.IsSubstitute || row.IsAbsent {
		return false
	}
	_, promotedToBaseRoster := proposedSet[row.StaffID]
	return !promotedToBaseRoster
}

func uniqueEffectiveCoverageStaff(
	coverageDays map[timezone.Date]effectiveShiftCoverageDay,
	dates []timezone.Date,
) []int64 {
	all := make([]int64, 0)
	for _, date := range dates {
		day := coverageDays[date]
		if day.active {
			all = append(all, day.staffIDs...)
		}
	}
	return sliceutil.Unique(all)
}

func containsCoverageDate(dates []timezone.Date, target timezone.Date) bool {
	for _, date := range dates {
		if date == target {
			return true
		}
	}
	return false
}

func loadShiftCoverageData(ctx context.Context, shifts StaffShiftCoverageReader, dates []timezone.Date, staffIDs []int64) ([]*scheduleModel.StaffShift, map[timezone.Date]bool, error) {
	firstWeekFrom, _ := containingCalendarWeek(dates[0])
	_, lastWeekTo := containingCalendarWeek(dates[len(dates)-1])
	rows, err := shifts.FindByStaffIDsAndDates(ctx, staffIDs, dates)
	if err != nil {
		return nil, nil, fmt.Errorf("load staff shifts for coverage: %w", err)
	}
	weeks, err := shifts.FindUsedCalendarWeeks(ctx, firstWeekFrom, lastWeekTo)
	if err != nil {
		return nil, nil, fmt.Errorf("load used staff-shift weeks for coverage: %w", err)
	}
	return rows, indexCalendarWeeks(weeks), nil
}

type pendingShiftCoverageWarning struct {
	staffID int64
	date    timezone.Date
	start   time.Time
	end     time.Time
	gap     ShiftCoverageInterval
}

func collectPendingCoverageWarnings(
	dates []timezone.Date,
	coverageDays map[timezone.Date]effectiveShiftCoverageDay,
	shifts []*scheduleModel.StaffShift,
	usedWeeks map[timezone.Date]bool,
) ([]pendingShiftCoverageWarning, int) {
	shiftIndex := indexShifts(shifts)
	pending := make([]pendingShiftCoverageWarning, 0, maxReturnedShiftCoverageWarnings)
	total := 0
	for _, date := range dates {
		day := coverageDays[date]
		if !day.active {
			continue
		}
		weekFrom, _ := containingCalendarWeek(date)
		if !usedWeeks[weekFrom] {
			continue
		}
		for _, staffID := range day.staffIDs {
			gaps := uncoveredShiftIntervals(day.start, day.end, shiftIndex[staffDateKey{staffID: staffID, date: date}])
			for _, gap := range gaps {
				total++
				if len(pending) >= maxReturnedShiftCoverageWarnings {
					continue
				}
				pending = append(pending, pendingShiftCoverageWarning{
					staffID: staffID,
					date:    date,
					start:   day.start,
					end:     day.end,
					gap:     gap,
				})
			}
		}
	}
	return pending, total
}

func buildShiftCoverageWarnings(
	pending []pendingShiftCoverageWarning,
	staff map[int64]*usersModel.Staff,
) []ShiftCoverageWarning {
	warnings := make([]ShiftCoverageWarning, 0, len(pending))
	for _, item := range pending {
		name := coverageStaffName(staff[item.staffID], item.staffID)
		start := timezone.WallClock(item.start).Format("15:04")
		end := timezone.WallClock(item.end).Format("15:04")
		gapStart := timezone.WallClock(item.gap.StartTime).Format("15:04")
		gapEnd := timezone.WallClock(item.gap.EndTime).Format("15:04")
		warnings = append(warnings, ShiftCoverageWarning{
			StaffID:            item.staffID,
			StaffName:          name,
			Date:               item.date.String(),
			StartTime:          start,
			EndTime:            end,
			UncoveredStartTime: gapStart,
			UncoveredEndTime:   gapEnd,
			Message: fmt.Sprintf(
				"%s ist am %s für den Termin %s–%s nicht durchgehend im Dienstplan abgedeckt (offen: %s–%s).",
				name,
				item.date.Format(germanDateLayout),
				start,
				end,
				gapStart,
				gapEnd,
			),
		})
	}
	return warnings
}

func normalizeShiftCoverageQuery(query ShiftCoverageQuery) ([]timezone.Date, []int64, error) {
	dates, err := normalizeShiftCoverageDates(query.Dates)
	if err != nil {
		return nil, nil, err
	}
	if !timezone.WallClock(query.EndTime).After(timezone.WallClock(query.StartTime)) {
		return nil, nil, invalidShiftCoverageQuery("end time must be after start time")
	}
	staffIDs, err := normalizeShiftCoverageStaffIDs(query.StaffIDs)
	if err != nil {
		return nil, nil, err
	}
	if len(dates) > maxShiftCoverageComparisons/len(staffIDs) {
		return nil, nil, invalidShiftCoverageQuery("too many date and staff combinations")
	}
	if err := validateShiftCoverageOptions(query, len(dates)); err != nil {
		return nil, nil, err
	}
	if query.ConcreteInstanceDate != nil && !containsCoverageDate(dates, *query.ConcreteInstanceDate) {
		return nil, nil, invalidShiftCoverageQuery("concrete instance date must be one of the candidate dates")
	}
	return dates, staffIDs, nil
}

func normalizeShiftCoverageDates(input []timezone.Date) ([]timezone.Date, error) {
	if len(input) == 0 {
		return nil, invalidShiftCoverageQuery("dates are required")
	}
	for _, date := range input {
		if date.IsZero() {
			return nil, invalidShiftCoverageQuery("dates must be valid calendar dates")
		}
	}
	dates := sliceutil.Unique(input)
	if len(dates) > maxShiftCoverageCandidateDates {
		return nil, invalidShiftCoverageQuery("too many candidate dates")
	}
	sort.Slice(dates, func(i, j int) bool { return dates[i].Before(dates[j]) })
	if dates[0].DaysUntil(dates[len(dates)-1]) > maxShiftCoverageSpanDays {
		return nil, invalidShiftCoverageQuery("candidate dates span more than 366 days")
	}
	return dates, nil
}

func normalizeShiftCoverageStaffIDs(input []int64) ([]int64, error) {
	if len(input) == 0 {
		return nil, invalidShiftCoverageQuery("staff IDs are required")
	}
	for _, staffID := range input {
		if staffID <= 0 {
			return nil, invalidShiftCoverageQuery("staff IDs must be positive")
		}
	}
	staffIDs := sliceutil.Unique(input)
	if len(staffIDs) > maxShiftCoverageStaffIDs {
		return nil, invalidShiftCoverageQuery("too many staff IDs")
	}
	return staffIDs, nil
}

func validateShiftCoverageOptions(query ShiftCoverageQuery, dateCount int) error {
	if err := validateConcreteShiftCoverageOptions(query, dateCount); err != nil {
		return err
	}
	if err := validateSeriesShiftCoverageOptions(query); err != nil {
		return err
	}
	return validateRecurrenceShiftCoverageOptions(query)
}

func validateConcreteShiftCoverageOptions(query ShiftCoverageQuery, dateCount int) error {
	if query.ExcludeInstanceID != nil && *query.ExcludeInstanceID <= 0 {
		return invalidShiftCoverageQuery("exclude instance ID must be positive")
	}
	if query.ConcreteInstanceDate != nil && query.ConcreteInstanceDate.IsZero() {
		return invalidShiftCoverageQuery("concrete instance date must be valid")
	}
	if query.ExcludeInstanceID == nil && query.ConcreteInstanceDate != nil {
		return invalidShiftCoverageQuery("concrete instance date requires an exclude instance ID")
	}
	if query.ExcludeInstanceID != nil && dateCount > 1 && query.ConcreteInstanceDate == nil {
		return invalidShiftCoverageQuery("multi-date instance coverage requires the concrete instance date")
	}
	return nil
}

func validateSeriesShiftCoverageOptions(query ShiftCoverageQuery) error {
	if query.ReplanActivityGroupID != nil && *query.ReplanActivityGroupID <= 0 {
		return invalidShiftCoverageQuery("activity group ID must be positive")
	}
	if query.ReplanActivityGroupID != nil && query.ExcludeInstanceID != nil {
		return invalidShiftCoverageQuery("activity group ID and exclude instance ID are mutually exclusive")
	}
	return nil
}

func validateRecurrenceShiftCoverageOptions(query ShiftCoverageQuery) error {
	if (query.CalendarPeriodID == nil) != (query.WeekPattern == nil) {
		return invalidShiftCoverageQuery("calendar period ID and week pattern must be provided together")
	}
	if query.CalendarPeriodID == nil {
		if query.ReplanActivityGroupID != nil {
			return invalidShiftCoverageQuery("activity group ID requires recurrence metadata")
		}
		return nil
	}
	if *query.CalendarPeriodID <= 0 {
		return invalidShiftCoverageQuery("calendar period ID must be positive")
	}
	if *query.WeekPattern < 0 || *query.WeekPattern > 2 {
		return invalidShiftCoverageQuery("week pattern must be 0, 1, or 2")
	}
	return nil
}

func invalidShiftCoverageQuery(detail string) error {
	return fmt.Errorf("%w: %s", ErrInvalidShiftCoverageQuery, detail)
}

func filterShiftCoverageDates(dates []timezone.Date, period *scheduleModel.CalendarPeriod, weekPattern int) []timezone.Date {
	filtered := make([]timezone.Date, 0, len(dates))
	for _, date := range dates {
		if period.ContainsDay(date) && ShouldMaterializeWeekPattern(weekPattern, date, period) {
			filtered = append(filtered, date)
		}
	}
	return filtered
}

func effectiveCoverageStaffIDs(submitted []int64, prior []*scheduleModel.InstanceStaff) []int64 {
	priorByStaff := make(map[int64]*scheduleModel.InstanceStaff, len(prior))
	for _, row := range prior {
		if row != nil {
			priorByStaff[row.StaffID] = row
		}
	}
	if len(submitted) != len(priorByStaff) {
		return submitted
	}
	for _, staffID := range submitted {
		if _, exists := priorByStaff[staffID]; !exists {
			return submitted
		}
	}

	// Membership is unchanged: preserve the concrete daily state. IsAbsent
	// wins even for a substitute row; only non-absent rows are effective.
	effective := make([]int64, 0, len(submitted))
	for _, staffID := range submitted {
		row := priorByStaff[staffID]
		if row == nil || row.IsAbsent {
			continue
		}
		effective = append(effective, staffID)
	}
	return effective
}

func containingCalendarWeek(date timezone.Date) (timezone.Date, timezone.Date) {
	// time.Weekday is Sunday=0. Rotate it so Monday=0 and move back to that
	// Monday. Dienstplan comparison accepts all seven ISO weekdays even though
	// the current weekly grid renders Monday-Friday.
	daysSinceMonday := (int(date.Weekday()) + 6) % 7
	monday := date.AddDays(-daysSinceMonday)
	return monday, monday.AddDays(6)
}

func coverageStaffName(staff *usersModel.Staff, id int64) string {
	if staff != nil && staff.Person != nil {
		if name := strings.TrimSpace(staff.Person.FirstName + " " + staff.Person.LastName); name != "" {
			return name
		}
	}
	return fmt.Sprintf("Mitarbeiter #%d", id)
}
