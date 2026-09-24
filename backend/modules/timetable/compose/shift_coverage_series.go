package compose

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/sliceutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// Series projection of the shift-coverage probe: which staff will ReplanWeek
// actually plan on each occurrence of an edited template, once it has
// reapplied the exceptions and the #1871 deviations of the occurrences it
// replaces?

func (d *conflictDetection) resolveSeriesEffectiveCoverageDays(
	ctx context.Context,
	probe timetable.ShiftCoverageProbe,
	dates []timezone.Date,
	staffIDs []int64,
	coverageDays map[timezone.Date]effectiveShiftCoverageDay,
) (map[timezone.Date]effectiveShiftCoverageDay, []int64, error) {
	activityGroupID := *probe.ReplanActivityGroupID
	from, to := scheduleModels.Date(dates[0]), scheduleModels.Date(dates[len(dates)-1])
	instances, err := d.deps.Instances.FindByActivityGroupAndDateRange(ctx, activityGroupID, from, to)
	if err != nil {
		return nil, nil, fmt.Errorf("load concrete series occurrences: %w", err)
	}
	exceptions, err := d.deps.Exceptions.FindByActivityGroupAndDateRange(ctx, activityGroupID, from, to)
	if err != nil {
		return nil, nil, fmt.Errorf("load concrete series exceptions: %w", err)
	}
	candidatesByDate, instanceIDs := preservedDeviationCandidates(instances, dates, activityGroupID)
	exceptionsByDate := effectiveSeriesExceptions(exceptions, dates, activityGroupID)
	rowsByInstance, err := d.loadSeriesCoverageStaffRows(ctx, instanceIDs)
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

func (d *conflictDetection) loadSeriesCoverageStaffRows(ctx context.Context, instanceIDs []int64) (map[int64][]*scheduleModels.InstanceStaff, error) {
	rowsByInstance := make(map[int64][]*scheduleModels.InstanceStaff, len(instanceIDs))
	if len(instanceIDs) == 0 {
		return rowsByInstance, nil
	}
	rows, err := d.deps.InstanceStaff.FindByInstanceIDs(ctx, instanceIDs)
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
	exception           *scheduleModels.ActivityException
	instances           []*scheduleModels.ActivityInstance
	activityGroupID     int64
	date                timezone.Date
	deviationCandidates []*scheduleModels.ActivityInstance
	rowsByInstance      map[int64][]*scheduleModels.InstanceStaff
	staffIDs            []int64
}

func projectSeriesCoverageDay(projection seriesCoverageProjection) (effectiveShiftCoverageDay, error) {
	day := applySeriesCoverageException(projection.day, projection.exception)
	if !day.active {
		return day, nil
	}
	if !timezone.NormalizeWallClock(day.end).After(timezone.NormalizeWallClock(day.start)) {
		return day, invalidShiftCoverageProbe("effective exception end time must be after start time")
	}
	if survivingInstanceBlocksSeriesCandidate(projection.instances, projection.activityGroupID, projection.date, day.start) {
		return inactiveCoverageDay(day), nil
	}
	if source := selectPreservedDeviationSource(projection.deviationCandidates, day.start); source != nil {
		day.staffIDs = effectiveSeriesCoverageStaffIDs(projection.staffIDs, projection.rowsByInstance[source.ID])
	}
	return day, nil
}

func applySeriesCoverageException(day effectiveShiftCoverageDay, exception *scheduleModels.ActivityException) effectiveShiftCoverageDay {
	if exception == nil {
		return day
	}
	if exception.IsCancellation() {
		return inactiveCoverageDay(day)
	}
	if exception.StartTime != nil {
		day.start = timezone.NormalizeWallClock(*exception.StartTime)
	}
	if exception.EndTime != nil {
		day.end = timezone.NormalizeWallClock(*exception.EndTime)
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
// scoped by the repository; filtering the requested dates and group here
// keeps the operation to one query.
func preservedDeviationCandidates(
	instances []*scheduleModels.ActivityInstance,
	dates []timezone.Date,
	activityGroupID int64,
) (map[timezone.Date][]*scheduleModels.ActivityInstance, []int64) {
	requested := dateSet(dates)
	candidates := make(map[timezone.Date][]*scheduleModels.ActivityInstance)
	instanceIDs := make([]int64, 0)
	for _, instance := range instances {
		if !replannedSeriesOccurrence(instance, activityGroupID) {
			continue
		}
		date := timezone.Date(instance.Date)
		if _, selected := requested[date]; !selected {
			continue
		}
		candidates[date] = append(candidates[date], instance)
		instanceIDs = append(instanceIDs, instance.ID)
	}
	return candidates, instanceIDs
}

// replannedSeriesOccurrence reports whether ReplanWeek deletes and
// regenerates the occurrence: a planned, non-spontaneous row of the group.
func replannedSeriesOccurrence(instance *scheduleModels.ActivityInstance, activityGroupID int64) bool {
	return instance != nil && instance.ActivityGroupID != nil && *instance.ActivityGroupID == activityGroupID &&
		instance.Status == scheduleModels.InstanceStatusPlanned && !instance.IsSpontaneous
}

func effectiveSeriesExceptions(
	exceptions []*scheduleModels.ActivityException,
	dates []timezone.Date,
	activityGroupID int64,
) map[timezone.Date]*scheduleModels.ActivityException {
	requested := dateSet(dates)
	byDate := make(map[timezone.Date]*scheduleModels.ActivityException)
	for _, exception := range exceptions {
		if exception == nil || exception.ActivityGroupID != activityGroupID {
			continue
		}
		date := timezone.Date(exception.ExceptionDate)
		if _, selected := requested[date]; selected {
			byDate[date] = exception
		}
	}
	return byDate
}

func dateSet(dates []timezone.Date) map[timezone.Date]struct{} {
	set := make(map[timezone.Date]struct{}, len(dates))
	for _, date := range dates {
		set[date] = struct{}{}
	}
	return set
}

// survivingInstanceBlocksSeriesCandidate: ReplanWeek deletes only planned,
// non-spontaneous instances. Any surviving instance with the regenerated
// candidate's key remains in the materializer's existing-index and
// suppresses a replacement occurrence. Such a historical, cancelled, active,
// completed or spontaneous row is not changed by the proposed series edit, so
// it has no new coverage warning.
func survivingInstanceBlocksSeriesCandidate(
	instances []*scheduleModels.ActivityInstance,
	activityGroupID int64,
	date timezone.Date,
	effectiveStart time.Time,
) bool {
	effectiveStart = timezone.NormalizeWallClock(effectiveStart)
	for _, instance := range instances {
		if instance == nil || instance.ActivityGroupID == nil || *instance.ActivityGroupID != activityGroupID || timezone.Date(instance.Date) != date {
			continue
		}
		deletedByReplan := instance.Status == scheduleModels.InstanceStatusPlanned && !instance.IsSpontaneous
		if !deletedByReplan && timezone.NormalizeWallClock(instance.StartTime).Equal(effectiveStart) {
			return true
		}
	}
	return false
}

// selectPreservedDeviationSource mirrors the materializer's
// matchRegeneratedInstance. When a template has one occurrence on a day, its
// deviation follows a changed start time. On a multi-slot day only the old
// slot with the same start time may attach to the regenerated block;
// otherwise a deleted slot's deviation would be attributed to the wrong
// occurrence.
func selectPreservedDeviationSource(instances []*scheduleModels.ActivityInstance, proposedStart time.Time) *scheduleModels.ActivityInstance {
	if len(instances) == 1 {
		return instances[0]
	}
	proposedStart = timezone.NormalizeWallClock(proposedStart)
	for _, instance := range instances {
		if timezone.NormalizeWallClock(instance.StartTime).Equal(proposedStart) {
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
func effectiveSeriesCoverageStaffIDs(proposed []int64, prior []*scheduleModels.InstanceStaff) []int64 {
	proposedSet := idSet(proposed)
	absent := survivingSeriesAbsences(prior, proposedSet)
	effective := make([]int64, 0, len(proposed))
	for _, staffID := range proposed {
		if _, isAbsent := absent[staffID]; !isAbsent {
			effective = append(effective, staffID)
		}
	}
	effective = append(effective, restoredSeriesSubstitutes(prior, proposedSet, len(absent))...)
	return sliceutil.Unique(effective)
}

func survivingSeriesAbsences(prior []*scheduleModels.InstanceStaff, proposedSet map[int64]struct{}) map[int64]struct{} {
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

func restoredSeriesSubstitutes(prior []*scheduleModels.InstanceStaff, proposedSet map[int64]struct{}, limit int) []int64 {
	restored := make([]int64, 0, limit)
	for _, row := range prior {
		if len(restored) >= limit {
			break
		}
		if row == nil || !row.IsSubstitute || row.IsAbsent {
			continue
		}
		if _, promotedToBaseRoster := proposedSet[row.StaffID]; !promotedToBaseRoster {
			restored = append(restored, row.StaffID)
		}
	}
	return restored
}
