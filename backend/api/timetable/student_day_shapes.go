package timetable

import (
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
)

// SlotResponse describes an arrival or pickup slot for a single day.
//
// Source is always one of SlotSource* below. ExpectedTime is nil when
// source=none, or when source=exception and the exception expresses absence
// (no time). Reason is populated only for exceptions that carry one.
type SlotResponse struct {
	ExpectedTime *string `json:"expected_time"`
	Source       string  `json:"source"`
	Reason       *string `json:"reason,omitempty"`
}

// SlotSource constants for SlotResponse.Source. Aliased to the schedule
// service so the wire strings and the shared ResolveSlotSource precedence rule
// stay in lockstep — one source of truth.
const (
	SlotSourceSchedule  = scheduleSvc.SlotSourceSchedule
	SlotSourceException = scheduleSvc.SlotSourceException
	SlotSourceNone      = scheduleSvc.SlotSourceNone
)

// AttendanceDayResponse is the per-student attendance payload on a day
// instance. Status / substatus / note / checked_in_at come from
// schedule.instance_students; is_unplanned marks a persisted visit without a
// booking. Legacy retained visits can still be synthesized during migration.
type AttendanceDayResponse struct {
	Status       string  `json:"status"`
	Substatus    *string `json:"substatus"`
	Note         *string `json:"note"`
	CheckedInAt  *string `json:"checked_in_at"`
	CheckedOutAt *string `json:"checked_out_at"`
	IsUnplanned  bool    `json:"is_unplanned"`
}

// InstanceDayResponse is one scheduled instance as it appears in a student's
// day view. active_group_id is nil for planned/cancelled instances.
type InstanceDayResponse struct {
	ID            int64                 `json:"id"`
	Title         string                `json:"title"`
	StartTime     string                `json:"start_time"`
	EndTime       string                `json:"end_time"`
	RoomID        int64                 `json:"room_id"`
	Status        string                `json:"status"`
	ActiveGroupID *int64                `json:"active_group_id"`
	Attendance    AttendanceDayResponse `json:"attendance"`
}

// StudentDayResponse is the body returned by GET /student/{id}/day.
type StudentDayResponse struct {
	StudentID int64                 `json:"student_id"`
	Date      string                `json:"date"`
	Weekday   int                   `json:"weekday"`
	Arrival   SlotResponse          `json:"arrival"`
	Instances []InstanceDayResponse `json:"instances"`
	Pickup    SlotResponse          `json:"pickup"`
}

// StudentWeekResponse is the body returned by GET /student/{id}/week. Days
// is always inclusive-inclusive across the requested range, one entry per
// date (including dates with no instances).
type StudentWeekResponse struct {
	StudentID int64                `json:"student_id"`
	From      string               `json:"from"`
	To        string               `json:"to"`
	Days      []StudentDayResponse `json:"days"`
}

// --- Mappers ---------------------------------------------------------------

// mapEnrolledInstance lifts a repo-side (instance, attendance) pair into the
// wire shape. The attendance row is authoritative — status/substatus/note/
// checked_in_at come from it directly. A persisted unplanned visit also has an
// instance_students row, so the row's is_unplanned flag stays authoritative.
func mapEnrolledInstance(row *scheduleModel.ScheduledInstanceRow) InstanceDayResponse {
	att := row.Attendance
	resp := InstanceDayResponse{
		ID:            row.Instance.ID,
		Title:         row.Instance.Title,
		StartTime:     row.Instance.StartTime.Format("15:04"),
		EndTime:       row.Instance.EndTime.Format("15:04"),
		RoomID:        row.Instance.RoomID,
		Status:        row.Instance.Status,
		ActiveGroupID: row.Instance.ActiveGroupID,
		Attendance: AttendanceDayResponse{
			Status:       att.Status,
			Substatus:    att.Substatus,
			Note:         att.Note,
			CheckedInAt:  formatOptionalRFC3339(att.CheckedInAt),
			CheckedOutAt: formatOptionalRFC3339(att.CheckedOutAt),
			IsUnplanned:  att.IsUnplanned,
		},
	}
	return resp
}

// mapUnplannedInstance builds a response entry for a visit without an
// instance_students row. Status is synthesized as "present" (the visit
// proves they were there), substatus/note stay nil, checked_in_at comes
// from the visit's entry_time.
func mapUnplannedInstance(inst *scheduleModel.ActivityInstance, visit *activeModel.Visit) InstanceDayResponse {
	checkedIn := formatOptionalRFC3339(&visit.EntryTime)
	return InstanceDayResponse{
		ID:            inst.ID,
		Title:         inst.Title,
		StartTime:     inst.StartTime.Format("15:04"),
		EndTime:       inst.EndTime.Format("15:04"),
		RoomID:        inst.RoomID,
		Status:        inst.Status,
		ActiveGroupID: inst.ActiveGroupID,
		Attendance: AttendanceDayResponse{
			Status:       scheduleModel.AttendanceStatusPresent,
			Substatus:    nil,
			Note:         nil,
			CheckedInAt:  checkedIn,
			CheckedOutAt: formatOptionalRFC3339(visit.ExitTime),
			IsUnplanned:  true,
		},
	}
}

// mapArrivalScheduleSlot maps a recurring weekly schedule to the slot shape.
func mapArrivalScheduleSlot(s *scheduleModel.StudentArrivalSchedule) SlotResponse {
	t := s.ExpectedArrival.Format("15:04")
	return SlotResponse{
		ExpectedTime: &t,
		Source:       SlotSourceSchedule,
	}
}

// mapArrivalExceptionSlot maps a date-specific exception. A nil time means
// absence on this date; we surface source=exception with ExpectedTime=nil.
func mapArrivalExceptionSlot(e *scheduleModel.StudentArrivalException) SlotResponse {
	resp := SlotResponse{Source: SlotSourceException}
	if e.ExpectedArrival != nil {
		t := e.ExpectedArrival.Format("15:04")
		resp.ExpectedTime = &t
	}
	if e.Reason != nil && *e.Reason != "" {
		r := *e.Reason
		resp.Reason = &r
	}
	return resp
}

// mapPickupScheduleSlot mirrors mapArrivalScheduleSlot for pickup.
func mapPickupScheduleSlot(s *scheduleModel.StudentPickupSchedule) SlotResponse {
	t := s.PickupTime.Format("15:04")
	return SlotResponse{
		ExpectedTime: &t,
		Source:       SlotSourceSchedule,
	}
}

// mapPickupExceptionSlot mirrors mapArrivalExceptionSlot for pickup.
func mapPickupExceptionSlot(e *scheduleModel.StudentPickupException) SlotResponse {
	resp := SlotResponse{Source: SlotSourceException}
	if e.PickupTime != nil {
		t := e.PickupTime.Format("15:04")
		resp.ExpectedTime = &t
	}
	if e.Reason != nil && *e.Reason != "" {
		r := *e.Reason
		resp.Reason = &r
	}
	return resp
}

// formatOptionalRFC3339 renders an optional timestamp in Berlin-local time
// using RFC3339. Returns nil when the input is nil — the JSON field stays
// null in that case. Berlin-local so the frontend does not need to round-
// trip an offset conversion for display.
func formatOptionalRFC3339(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.In(timezone.Berlin).Format(time.RFC3339)
	return &s
}
