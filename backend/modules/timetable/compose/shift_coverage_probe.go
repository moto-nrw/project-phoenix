package compose

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/sliceutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

const (
	// A school-year series contains at most roughly 260 selected weekdays. The
	// defensive 366 bounds admit a full leap-year candidate set without
	// allowing an unbounded read through a malformed API request.
	maxShiftCoverageCandidateDates = 366
	maxShiftCoverageSpanDays       = 366
	maxShiftCoverageStaffIDs       = 500
	maxShiftCoverageComparisons    = 10_000
)

// errInconsistentSeriesValidity reports template schedules that do not share
// one validity envelope, the invariant every template write enforces.
var errInconsistentSeriesValidity = errors.New("template schedules have inconsistent validity bounds")

func invalidShiftCoverageProbe(detail string) error {
	return fmt.Errorf("%w: %s", timetable.ErrInvalidShiftCoverageQuery, detail)
}

func normalizeShiftCoverageProbe(probe timetable.ShiftCoverageProbe) ([]timezone.Date, []int64, error) {
	dates, err := normalizeShiftCoverageDates(probe.Dates)
	if err != nil {
		return nil, nil, err
	}
	if !timezone.NormalizeWallClock(probe.EndTime).After(timezone.NormalizeWallClock(probe.StartTime)) {
		return nil, nil, invalidShiftCoverageProbe("end time must be after start time")
	}
	staffIDs, err := normalizeShiftCoverageStaffIDs(probe.StaffIDs)
	if err != nil {
		return nil, nil, err
	}
	if len(dates) > maxShiftCoverageComparisons/len(staffIDs) {
		return nil, nil, invalidShiftCoverageProbe("too many date and staff combinations")
	}
	if err := validateShiftCoverageOptions(probe, len(dates)); err != nil {
		return nil, nil, err
	}
	if probe.ConcreteInstanceDate != nil && !containsCoverageDate(dates, *probe.ConcreteInstanceDate) {
		return nil, nil, invalidShiftCoverageProbe("concrete instance date must be one of the candidate dates")
	}
	return dates, staffIDs, nil
}

func normalizeShiftCoverageDates(input []timezone.Date) ([]timezone.Date, error) {
	if len(input) == 0 {
		return nil, invalidShiftCoverageProbe("dates are required")
	}
	for _, date := range input {
		if date.IsZero() {
			return nil, invalidShiftCoverageProbe("dates must be valid calendar dates")
		}
	}
	dates := sliceutil.Unique(input)
	if len(dates) > maxShiftCoverageCandidateDates {
		return nil, invalidShiftCoverageProbe("too many candidate dates")
	}
	sort.Slice(dates, func(i, j int) bool { return dates[i].Before(dates[j]) })
	if dates[0].DaysUntil(dates[len(dates)-1]) > maxShiftCoverageSpanDays {
		return nil, invalidShiftCoverageProbe("candidate dates span more than 366 days")
	}
	return dates, nil
}

func normalizeShiftCoverageStaffIDs(input []int64) ([]int64, error) {
	if len(input) == 0 {
		return nil, invalidShiftCoverageProbe("staff IDs are required")
	}
	for _, staffID := range input {
		if staffID <= 0 {
			return nil, invalidShiftCoverageProbe("staff IDs must be positive")
		}
	}
	staffIDs := sliceutil.Unique(input)
	if len(staffIDs) > maxShiftCoverageStaffIDs {
		return nil, invalidShiftCoverageProbe("too many staff IDs")
	}
	return staffIDs, nil
}

func validateShiftCoverageOptions(probe timetable.ShiftCoverageProbe, dateCount int) error {
	if err := validateConcreteShiftCoverageOptions(probe, dateCount); err != nil {
		return err
	}
	if err := validateSeriesShiftCoverageOptions(probe); err != nil {
		return err
	}
	return validateRecurrenceShiftCoverageOptions(probe)
}

func validateConcreteShiftCoverageOptions(probe timetable.ShiftCoverageProbe, dateCount int) error {
	if probe.ExcludeInstanceID != nil && *probe.ExcludeInstanceID <= 0 {
		return invalidShiftCoverageProbe("exclude instance ID must be positive")
	}
	if probe.ConcreteInstanceDate != nil && probe.ConcreteInstanceDate.IsZero() {
		return invalidShiftCoverageProbe("concrete instance date must be valid")
	}
	if probe.ExcludeInstanceID == nil && probe.ConcreteInstanceDate != nil {
		return invalidShiftCoverageProbe("concrete instance date requires an exclude instance ID")
	}
	if probe.ExcludeInstanceID != nil && dateCount > 1 && probe.ConcreteInstanceDate == nil {
		return invalidShiftCoverageProbe("multi-date instance coverage requires the concrete instance date")
	}
	return nil
}

func validateSeriesShiftCoverageOptions(probe timetable.ShiftCoverageProbe) error {
	if probe.ReplanActivityGroupID != nil && *probe.ReplanActivityGroupID <= 0 {
		return invalidShiftCoverageProbe("activity group ID must be positive")
	}
	if probe.ReplanActivityGroupID != nil && probe.ExcludeInstanceID != nil {
		return invalidShiftCoverageProbe("activity group ID and exclude instance ID are mutually exclusive")
	}
	return nil
}

func validateRecurrenceShiftCoverageOptions(probe timetable.ShiftCoverageProbe) error {
	if (probe.CalendarPeriodID == nil) != (probe.WeekPattern == nil) {
		return invalidShiftCoverageProbe("calendar period ID and week pattern must be provided together")
	}
	if probe.CalendarPeriodID == nil {
		if probe.ReplanActivityGroupID != nil {
			return invalidShiftCoverageProbe("activity group ID requires recurrence metadata")
		}
		return nil
	}
	if *probe.CalendarPeriodID <= 0 {
		return invalidShiftCoverageProbe("calendar period ID must be positive")
	}
	if *probe.WeekPattern < 0 || *probe.WeekPattern > 2 {
		return invalidShiftCoverageProbe("week pattern must be 0, 1, or 2")
	}
	return nil
}

// resolveSeriesCoverageDates keeps the dates inside the series' recurrence
// envelope: valid_from is inclusive, valid_until exclusive, exactly as the
// materializer treats a template schedule.
func (d *conflictDetection) resolveSeriesCoverageDates(ctx context.Context, activityGroupID int64, dates []timezone.Date) ([]timezone.Date, error) {
	rows, err := d.deps.Schedules.FindByGroupID(ctx, activityGroupID)
	if err != nil {
		return nil, fmt.Errorf("load series recurrence envelope: %w", err)
	}
	if len(rows) == 0 {
		return nil, invalidShiftCoverageProbe("activity group schedules do not exist for this tenant")
	}
	validFrom, validUntil, err := seriesValidityEnvelope(rows)
	if err != nil {
		return nil, fmt.Errorf("load series recurrence envelope: %w", err)
	}
	filtered := make([]timezone.Date, 0, len(dates))
	for _, date := range dates {
		notStarted := validFrom != nil && validFrom.After(date)
		ended := validUntil != nil && !validUntil.After(date)
		if !notStarted && !ended {
			filtered = append(filtered, date)
		}
	}
	return filtered, nil
}

// seriesValidityEnvelope returns the validity bounds every schedule of the
// series shares. The template writes enforce one common envelope per segment;
// a row that disagrees is corrupt data and fails the probe.
func seriesValidityEnvelope(schedules []*activitiesModels.Schedule) (*timezone.Date, *timezone.Date, error) {
	if schedules[0] == nil {
		return nil, nil, fmt.Errorf("%w: nil schedule row", errInconsistentSeriesValidity)
	}
	validFrom, validUntil := scheduleDate(schedules[0].ValidFrom), scheduleDate(schedules[0].ValidUntil)
	if validFrom != nil && validUntil != nil && validFrom.After(*validUntil) {
		return nil, nil, fmt.Errorf("%w: segment valid_from %s is after valid_until %s",
			errInconsistentSeriesValidity, validFrom.String(), validUntil.String())
	}
	for _, schedule := range schedules[1:] {
		if schedule == nil {
			return nil, nil, fmt.Errorf("%w: nil schedule row", errInconsistentSeriesValidity)
		}
		if !sameOptionalDate(validFrom, scheduleDate(schedule.ValidFrom)) || !sameOptionalDate(validUntil, scheduleDate(schedule.ValidUntil)) {
			return nil, nil, fmt.Errorf("%w: schedule %d does not match the segment envelope", errInconsistentSeriesValidity, schedule.ID)
		}
	}
	return validFrom, validUntil, nil
}

func scheduleDate(date *activitiesModels.Date) *timezone.Date {
	if date == nil {
		return nil
	}
	value := timezone.Date(*date)
	return &value
}

func sameOptionalDate(left, right *timezone.Date) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

type effectiveShiftCoverageDay struct {
	staffIDs []int64
	start    time.Time
	end      time.Time
	active   bool
}

func (d *conflictDetection) resolveEffectiveCoverageDays(
	ctx context.Context,
	probe timetable.ShiftCoverageProbe,
	dates []timezone.Date,
	staffIDs []int64,
) (map[timezone.Date]effectiveShiftCoverageDay, []int64, error) {
	coverageDays := make(map[timezone.Date]effectiveShiftCoverageDay, len(dates))
	for _, date := range dates {
		coverageDays[date] = effectiveShiftCoverageDay{staffIDs: staffIDs, start: probe.StartTime, end: probe.EndTime, active: true}
	}
	if probe.ExcludeInstanceID != nil {
		return d.resolveConcreteEffectiveCoverageDays(ctx, probe, dates, staffIDs, coverageDays)
	}
	if probe.ReplanActivityGroupID == nil {
		return coverageDays, staffIDs, nil
	}
	return d.resolveSeriesEffectiveCoverageDays(ctx, probe, dates, staffIDs, coverageDays)
}

func (d *conflictDetection) resolveConcreteEffectiveCoverageDays(
	ctx context.Context,
	probe timetable.ShiftCoverageProbe,
	dates []timezone.Date,
	staffIDs []int64,
	coverageDays map[timezone.Date]effectiveShiftCoverageDay,
) (map[timezone.Date]effectiveShiftCoverageDay, []int64, error) {
	rows, err := d.deps.InstanceStaff.FindByInstanceIDs(ctx, []int64{*probe.ExcludeInstanceID})
	if err != nil {
		return nil, nil, fmt.Errorf("load concrete staff deviations: %w", err)
	}
	concreteDate := dates[0]
	if probe.ConcreteInstanceDate != nil {
		concreteDate = *probe.ConcreteInstanceDate
	}
	day := coverageDays[concreteDate]
	day.staffIDs = effectiveCoverageStaffIDs(staffIDs, rows)
	coverageDays[concreteDate] = day
	return coverageDays, uniqueEffectiveCoverageStaff(coverageDays, dates), nil
}

// effectiveCoverageStaffIDs keeps the concrete daily state when the submitted
// membership is unchanged: IsAbsent wins even for a substitute row, and only
// non-absent rows are effective. A changed membership is taken as submitted.
func effectiveCoverageStaffIDs(submitted []int64, prior []*scheduleModels.InstanceStaff) []int64 {
	priorByStaff := make(map[int64]*scheduleModels.InstanceStaff, len(prior))
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
	effective := make([]int64, 0, len(submitted))
	for _, staffID := range submitted {
		if row := priorByStaff[staffID]; row != nil && !row.IsAbsent {
			effective = append(effective, staffID)
		}
	}
	return effective
}

func uniqueEffectiveCoverageStaff(coverageDays map[timezone.Date]effectiveShiftCoverageDay, dates []timezone.Date) []int64 {
	all := make([]int64, 0)
	for _, date := range dates {
		if day := coverageDays[date]; day.active {
			all = append(all, day.staffIDs...)
		}
	}
	return sliceutil.Unique(all)
}
