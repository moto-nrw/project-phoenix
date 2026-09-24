package timetablehttp

// The attendance side of the instance list: which assigned children the care
// plan places in the OGS, and the per-instance student payload and counts
// derived from that verdict (#1747, #2360).

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// resolveCareDays derives, for the whole window at once, which assigned
// children the care plan actually places in the OGS on which day (#1747).
//
// The pre-pass asks for the rows whose verdict can still change something:
// still 'expected', or flipped to 'absent' by a broad day status that still
// owns them. A sick report lands on every expected row of the day, including
// the days the child was never booked into care, so restricting the pre-pass
// to 'expected' would leave exactly those rows unresolved and displayed as
// ordinary absences (#1747). The window is capped at 56 days, but the cost does
// not scale with it: weekly care plans are recurring, so the derivation loads
// them once and combines them with the window's exceptions in memory.
//
// An unwired service yields an empty map, which reads as "unknown" everywhere
// and leaves every count exactly as it was before this feature.
func (rs *Resource) resolveCareDays(
	ctx context.Context,
	instances []timetable.ScheduledInstance,
	from, to calendar.Date,
) (map[int64]map[calendar.Date]careplan.CareDayStatus, error) {
	empty := map[int64]map[calendar.Date]careplan.CareDayStatus{}
	if rs.CareDayService == nil || rs.TimetableData == nil || len(instances) == 0 {
		return empty, nil
	}

	instanceIDs := make([]int64, 0, len(instances))
	for _, inst := range instances {
		instanceIDs = append(instanceIDs, inst.ID)
	}

	rows, err := rs.TimetableData.ListCareDayCandidates(ctx, instanceIDs)
	if err != nil {
		return nil, fmt.Errorf("load care-day candidate students: %w", err)
	}

	seen := make(map[int64]bool, len(rows))
	studentIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		if !seen[row.StudentID] {
			seen[row.StudentID] = true
			studentIDs = append(studentIDs, row.StudentID)
		}
	}
	if len(studentIDs) == 0 {
		return empty, nil
	}

	return rs.CareDayService.ResolveForRange(ctx, studentIDs, from, to)
}

// instanceStudentCareDay picks the care-day verdict reported for one
// assignment row — the single source both the per-child payload and the
// counts read, so a child can never be listed as "Erwartet" while the header
// count leaves them out (#1747 review).
//
// The rules live in careplan.AttendanceRowCareDay, shared with the operation
// roster and the planned-now cards; this only looks up the plan verdict for the
// instance's date.
func instanceStudentCareDay(
	inst timetable.ScheduledInstance,
	row timetable.ScheduledParticipant,
	careDays map[int64]map[calendar.Date]careplan.CareDayStatus,
) careplan.CareDayStatus {
	return careplan.AttendanceRowCareDay(
		inst.Status == timetable.InstanceStatusCompleted, &careplan.CareDayAttendance{Expected: row.Status == timetable.SlotAttendanceExpected, NotScheduled: row.NotScheduled, ManuallyDecided: row.ManualStatusAt != nil, PlanOwnedAbsence: row.StudentStatusDayID != nil || row.PickupExceptionID != nil},
		careDays[row.StudentID][inst.Date],
	)
}

// instanceAttendanceSummary is the per-instance student payload plus the three
// counts derived from it. They are built together so the rows and the header
// counts can never disagree about the same child (#1747 review).
type instanceAttendanceSummary struct {
	studentIDs   []int64
	students     []instanceStudentSummary
	expected     int
	present      int
	notScheduled int
}

// summarizeInstanceStudents groups one instance's attendance rows by the
// care-day verdict instanceStudentCareDay reports for each of them.
func summarizeInstanceStudents(
	inst timetable.ScheduledInstance,
	studentRows []timetable.ScheduledParticipant,
	careDays map[int64]map[calendar.Date]careplan.CareDayStatus,
	pickupCutoffs map[int64]time.Time,
) instanceAttendanceSummary {
	out := instanceAttendanceSummary{
		studentIDs: make([]int64, 0, len(studentRows)),
		students:   make([]instanceStudentSummary, 0, len(studentRows)),
	}
	for _, row := range studentRows {
		out.studentIDs = append(out.studentIDs, row.StudentID)
		var checkedInAt *string
		if row.CheckedInAt != nil {
			formatted := row.CheckedInAt.UTC().Format("2006-01-02T15:04:05Z07:00")
			checkedInAt = &formatted
		}
		careDayStatus := instanceStudentCareDay(inst, row, careDays)
		// The marker means "stays expected, leaves early" — a row a full-day
		// status or manual absence flipped must not carry a pickup time. The
		// care-day verdict must agree: a timed auto-excusal exception can
		// coexist with a timeless "Kommt heute nicht" exception, which cancels
		// the whole day while the pre-cutoff row stays expected — that child is
		// not picked up early, they are not coming at all.
		var earlyPickup *string
		if row.Status == timetable.SlotAttendanceExpected && careDayStatus.Expected() {
			earlyPickup = earlyPickupWithin(inst, pickupCutoffs, row.StudentID)
		}
		out.students = append(out.students, instanceStudentSummary{
			StudentID:       row.StudentID,
			Status:          row.Status,
			Substatus:       row.Substatus,
			Note:            row.Note,
			CheckedInAt:     checkedInAt,
			CareDayStatus:   careDayStatus,
			EarlyPickupTime: earlyPickup,
		})
		switch row.Status {
		case timetable.SlotAttendanceExpected:
			// Assigned but not in care today: counted separately, and left out
			// of the staffing maths — planning for children who are not there
			// that day inflates the Betreuungsschlüssel (#1747). The row carries
			// the same verdict, so the slide-over can group it exactly the way
			// this count does.
			if !careDayStatus.Expected() {
				out.notScheduled++
				continue
			}
			out.expected++
		case timetable.SlotAttendancePresent:
			out.present++
		case timetable.SlotAttendanceAbsent:
			// A broad day status wrote this absence onto a day the care plan
			// never booked — the block has not ended yet, so nothing has undone
			// it. Group it where the verdict says it belongs instead of showing
			// a school an absence from care that was never owed (#1747).
			// instanceStudentCareDay hands out this verdict for no other absent
			// row, so a manual absence is untouched.
			if careDayStatus == careplan.CareDayNotScheduled {
				out.notScheduled++
			}
		}
	}
	return out
}

// earlyPickupWithin reports the child's pickup cutoff as HH:MM when it falls
// strictly inside the block's time window — the overlap case that stays
// expected but must be made visible (#2360). Wall-clock comparison only;
// cutoffs at or before the start belong to fully-excused blocks, cutoffs at
// or after the end do not affect the block.
func earlyPickupWithin(
	inst timetable.ScheduledInstance,
	pickupCutoffs map[int64]time.Time,
	studentID int64,
) *string {
	cutoff, ok := pickupCutoffs[studentID]
	if !ok {
		return nil
	}
	start := calendar.NormalizeWallClock(inst.StartTime)
	end := calendar.NormalizeWallClock(inst.EndTime)
	if cutoff.After(start) && cutoff.Before(end) {
		formatted := cutoff.Format("15:04")
		return &formatted
	}
	return nil
}

// summarizeInstanceStaff maps one instance's staff rows onto the list payload
// and counts the absent ones.
func summarizeInstanceStaff(staffRows []timetable.InstanceStaff) ([]instanceStaffSummary, int) {
	staff := make([]instanceStaffSummary, 0, len(staffRows))
	absentCount := 0
	for _, row := range staffRows {
		if row.IsAbsent {
			absentCount++
		}
		staff = append(staff, instanceStaffSummary{
			StaffID:       row.StaffID,
			IsPrimary:     row.IsPrimary,
			IsAbsent:      row.IsAbsent,
			IsSubstitute:  row.IsSubstitute,
			IsSickAbsence: row.SickAbsenceID != nil,
			AbsenceReason: row.AbsenceReason,
		})
	}
	return staff, absentCount
}
