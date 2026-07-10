package students

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
)

const (
	DayPlanningStatusAll              = "all"
	DayPlanningStatusComesToday       = "comes_today"
	DayPlanningStatusNotComingToday   = "not_coming_today"
	dayPlanningReasonSick             = "sick"
	dayPlanningReasonExcused          = "excused"
	dayPlanningReasonClassTrip        = "class_trip"
	dayPlanningReasonArrivalException = "arrival_exception"
	dayPlanningReasonPickupException  = "pickup_exception"
	dayPlanningReasonArrivalSchedule  = "arrival_schedule"
	dayPlanningReasonPickupSchedule   = "pickup_schedule"
	dayPlanningReasonTimetable        = "timetable"
	dayPlanningReasonUnplanned        = "unplanned_attendance"
	dayPlanningReasonNoPlan           = "no_plan"
)

func (rs *Resource) enrichWithDayPlanning(ctx context.Context, responses []StudentResponse, now time.Time, attendances map[int64]*activeService.AttendanceStatus) error {
	fullAccessIDs := collectFullAccessStudentIDs(responses)
	if len(fullAccessIDs) == 0 {
		return nil
	}

	arrivals := map[int64]*scheduleService.EffectiveArrivalTime{}
	if rs.ArrivalScheduleService != nil {
		var err error
		arrivals, err = rs.ArrivalScheduleService.GetBulkEffectiveArrivalTimesForDate(ctx, fullAccessIDs, timezone.DateFromTime(now))
		if err != nil {
			return err
		}
	}

	pickups := map[int64]*scheduleService.EffectivePickupTime{}
	if rs.PickupScheduleService != nil {
		var err error
		pickups, err = rs.PickupScheduleService.GetBulkEffectivePickupTimesForDate(ctx, fullAccessIDs, timezone.DateFromTime(now))
		if err != nil {
			return err
		}
	}

	timetableIDs := map[int64]struct{}{}
	if rs.InstanceService != nil {
		plannedIDs, err := rs.InstanceService.GetPlannedStudentIDsByDate(ctx, fullAccessIDs, timezone.DateFromTime(now))
		if err != nil {
			return err
		}
		for _, id := range plannedIDs {
			timetableIDs[id] = struct{}{}
		}
	}

	// Pending parent excused-absence requests covering today (#1845): shown as an
	// informational badge with the parent's note, without changing the planning
	// status — the child stays "expected" until staff decide the request.
	//
	// The badge exposes the parent's note and belongs to the users:update-gated
	// review queue (Änderungsanfragen). This enrichment also runs inside the
	// users:read list/detail/export handlers, so a read-only group supervisor
	// would otherwise see the note and approval marker without any right to the
	// queue or to decide the request. Only enrich when the caller actually holds
	// the review permission that gates the queue itself.
	pendingExcused := map[int64]*activeModels.ExcusedAbsenceRequest{}
	if rs.ExcusedRequestService != nil &&
		authorize.HasPermission(permissions.UsersUpdate, jwt.PermissionsFromCtx(ctx)) {
		var err error
		pendingExcused, err = rs.ExcusedRequestService.PendingByStudentForDate(ctx, timezone.DateFromTime(now))
		if err != nil {
			return err
		}
	}

	for i := range responses {
		if !responses[i].HasFullAccess {
			continue
		}
		status, reason, label := resolveDayPlanning(responses[i], arrivals[responses[i].ID], pickups[responses[i].ID], attendances[responses[i].ID], timetableIDs)
		responses[i].DayPlanningStatus = status
		responses[i].DayPlanningReason = reason
		responses[i].DayPlanningLabel = label
		if req, ok := pendingExcused[responses[i].ID]; ok {
			note := req.Note
			responses[i].PendingExcusedNote = &note
		}
	}

	return nil
}

func resolveDayPlanning(
	student StudentResponse,
	arrival *scheduleService.EffectiveArrivalTime,
	pickup *scheduleService.EffectivePickupTime,
	attendance *activeService.AttendanceStatus,
	timetableIDs map[int64]struct{},
) (string, string, string) {
	if hasActualAttendanceToday(attendance) {
		return DayPlanningStatusComesToday, dayPlanningReasonUnplanned, "ungeplant anwesend"
	}
	if student.Sick {
		return DayPlanningStatusNotComingToday, dayPlanningReasonSick, "krank gemeldet"
	}
	if student.ClassTrip {
		return DayPlanningStatusNotComingToday, dayPlanningReasonClassTrip, "Klassenfahrt"
	}
	if student.Excused {
		return DayPlanningStatusNotComingToday, dayPlanningReasonExcused, "entschuldigt"
	}
	if arrival != nil && arrival.ArrivalTime != nil {
		if arrival.IsException {
			return DayPlanningStatusComesToday, dayPlanningReasonArrivalException, "geplante Ankunft heute"
		}
		return DayPlanningStatusComesToday, dayPlanningReasonArrivalSchedule, "Ankunftsplan heute"
	}
	if pickup != nil && pickup.PickupTime != nil {
		if pickup.IsException {
			return DayPlanningStatusComesToday, dayPlanningReasonPickupException, "geplante Abholung heute"
		}
		return DayPlanningStatusComesToday, dayPlanningReasonPickupSchedule, "Abholplan heute"
	}
	if _, ok := timetableIDs[student.ID]; ok {
		return DayPlanningStatusComesToday, dayPlanningReasonTimetable, "Betreuungsplan heute"
	}
	if arrival != nil && arrival.IsException && arrival.ArrivalTime == nil {
		return DayPlanningStatusNotComingToday, dayPlanningReasonArrivalException, dayPlanningExceptionLabel(arrival.Notes)
	}
	if pickup != nil && pickup.IsException && pickup.PickupTime == nil {
		return DayPlanningStatusNotComingToday, dayPlanningReasonPickupException, dayPlanningExceptionLabel(pickup.Notes)
	}
	return DayPlanningStatusNotComingToday, dayPlanningReasonNoPlan, "kein Plan für heute"
}

func hasActualAttendanceToday(attendance *activeService.AttendanceStatus) bool {
	return attendance != nil && attendance.CheckInTime != nil && attendance.Status != "not_checked_in"
}

func attendanceMapFromSnapshot(snapshot *common.StudentDataSnapshot) map[int64]*activeService.AttendanceStatus {
	if snapshot == nil || snapshot.LocationSnapshot == nil || snapshot.LocationSnapshot.Attendances == nil {
		return map[int64]*activeService.AttendanceStatus{}
	}
	return snapshot.LocationSnapshot.Attendances
}

func dayPlanningExceptionLabel(notes string) string {
	if notes != "" {
		return notes
	}
	return "Tagesausnahme"
}

func applyDayPlanningFilter(responses []StudentResponse, dayStatus string) []StudentResponse {
	if dayStatus == "" || dayStatus == DayPlanningStatusAll {
		return responses
	}
	filtered := make([]StudentResponse, 0, len(responses))
	for _, response := range responses {
		if response.DayPlanningStatus == dayStatus {
			filtered = append(filtered, response)
		}
	}
	return filtered
}
