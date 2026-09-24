package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// CareDays is the consumer-owned port to Care Plan's care-day verdict
// (#1747, ADR 0005), bound at the composition root. Care Plan resolves the
// verdict and owns the rules that read it; the timetable only asks.
type CareDays interface {
	// ResolveForDate resolves the verdict of the children on one date.
	ResolveForDate(ctx context.Context, studentIDs []int64, date timezone.Date) (map[int64]timetable.CareDayStatus, error)
	// ResolveForRange resolves the verdicts of the children over an
	// inclusive window in one pass.
	ResolveForRange(ctx context.Context, studentIDs []int64, from, to timezone.Date) (map[int64]map[timezone.Date]timetable.CareDayStatus, error)
	// AttendanceRowCareDay applies Care Plan's row rule: the verdict a
	// planned participant row shows, given the block's completion and the
	// plan verdict of the day.
	AttendanceRowCareDay(instanceCompleted bool, row timetable.CareDayAttendance, planVerdict timetable.CareDayStatus) timetable.CareDayStatus
	// Expected reports whether the verdict keeps the child in the expected
	// count.
	Expected(status timetable.CareDayStatus) bool
	// ExemptFromAbsence reports whether ending a block may skip the
	// expected → absent stamp: only a non-booking qualifies.
	ExemptFromAbsence(status timetable.CareDayStatus) bool
}

// careDayAttendance reads the attendance provenance of a planned participant
// row for Care Plan's row rule.
func careDayAttendance(row *scheduleModels.InstanceStudent) timetable.CareDayAttendance {
	return timetable.CareDayAttendance{
		Expected:         row.Status == scheduleModels.AttendanceStatusExpected,
		NotScheduled:     row.NotScheduled,
		ManuallyDecided:  row.ManualStatusAt != nil,
		PlanOwnedAbsence: row.StudentStatusDayID != nil || row.PickupExceptionID != nil,
	}
}
