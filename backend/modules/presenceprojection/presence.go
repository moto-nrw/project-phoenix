package presenceprojection

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

// The presence cutover (#2762) split what used to be one row: the plan of a
// block and its participants stays with Timetable (schedule.activity_instances,
// schedule.instance_students), the execution and the observed attendance
// belong to Student Presence (active.activity_sessions,
// active.activity_session_attendance). The reads below join the two halves
// back together for the retained legacy consumers. A participant without an
// attendance row is expected; an instance without a session has not started.
//
// Every read takes the tenant explicitly and scopes both halves by it.

// participantAttendanceJoin joins the lazily written attendance row of a
// participant. The aliases are fixed: "instance_student" for the participant
// and "attendance" for its Student Presence row.
const participantAttendanceJoin = `LEFT JOIN active.activity_session_attendance AS "attendance"
	ON "attendance".tenant_id = "instance_student".tenant_id AND "attendance".instance_student_id = "instance_student".id`

// instanceSessionJoin joins the execution of an instance under the alias
// "session"; the instance carries the alias "activity_instance".
const instanceSessionJoin = `LEFT JOIN active.activity_sessions AS "session"
	ON "session".tenant_id = "activity_instance".tenant_id AND "session".schedule_instance_id = "activity_instance".id`

const (
	attendanceStatus       = `COALESCE("attendance".status, 'expected')`
	attendanceNotScheduled = `COALESCE("attendance".not_scheduled, FALSE)`
	attendanceUnplanned    = `COALESCE("attendance".is_unplanned, FALSE)`
	// executionStatus folds the execution back onto the planning status the
	// legacy readers compare against: cancelled wins, then the session, then
	// planned.
	executionStatus = `CASE WHEN "activity_instance".status = 'cancelled' THEN 'cancelled' ELSE COALESCE("session".status, 'planned') END`
)

// CountNonAbsentParticipants counts, per instance, the participants that are
// not recorded absent. Instances without such participants are absent from
// the map.
func CountNonAbsentParticipants(ctx context.Context, db bun.IDB, tenantID int64, instanceIDs []int64) (map[int64]int, error) {
	result := make(map[int64]int)
	if tenantID <= 0 {
		return nil, ErrInvalidTenantID
	}
	if len(instanceIDs) == 0 {
		return result, nil
	}
	var rows []struct {
		InstanceID int64 `bun:"instance_id"`
		Count      int   `bun:"count"`
	}
	err := db.NewSelect().TableExpr(`schedule.instance_students AS "instance_student"`).
		ColumnExpr(`"instance_student".instance_id`).ColumnExpr(`COUNT(*)::int AS count`).
		Join(participantAttendanceJoin).
		Where(`"instance_student".tenant_id = ?`, tenantID).
		Where(`"instance_student".instance_id IN (?)`, bun.List(instanceIDs)).
		Where(attendanceStatus+` <> 'absent'`).
		GroupExpr(`"instance_student".instance_id`).Scan(ctx, &rows)
	if err != nil {
		return nil, fmt.Errorf("presence projection: count non-absent participants: %w", err)
	}
	for _, row := range rows {
		result[row.InstanceID] = row.Count
	}
	return result, nil
}

// ListParallelPresence returns, for the given students, the other running
// blocks of the day they are still present in. Ordered by the other block's
// start time descending, then id descending.
func ListParallelPresence(ctx context.Context, db bun.IDB, tenantID, excludedInstanceID int64, date timezone.Date, studentIDs []int64) ([]scheduleModels.ParallelPresence, error) {
	if tenantID <= 0 {
		return nil, ErrInvalidTenantID
	}
	if len(studentIDs) == 0 {
		return []scheduleModels.ParallelPresence{}, nil
	}
	rows := []scheduleModels.ParallelPresence{}
	err := db.NewSelect().TableExpr(`schedule.instance_students AS "instance_student"`).
		ColumnExpr(`"instance_student".student_id, "activity_instance".id AS instance_id`).
		ColumnExpr(`"activity_instance".title, "activity_instance".start_time, "activity_instance".end_time`).
		Join(participantAttendanceJoin).
		Join(`INNER JOIN schedule.activity_instances AS "activity_instance" ON "activity_instance".id = "instance_student".instance_id AND "activity_instance".tenant_id = "instance_student".tenant_id`).
		Join(`INNER JOIN active.activity_sessions AS "session" ON "session".tenant_id = "activity_instance".tenant_id AND "session".schedule_instance_id = "activity_instance".id`).
		Where(`"instance_student".tenant_id = ?`, tenantID).
		Where(`"instance_student".instance_id != ?`, excludedInstanceID).
		Where(`"instance_student".student_id IN (?)`, bun.List(studentIDs)).
		Where(`"attendance".status = 'present'`).Where(`"attendance".checked_out_at IS NULL`).
		Where(`"activity_instance".date = ?`, date).Where(`"activity_instance".status <> 'cancelled'`).
		Where(`"session".status = 'active'`).
		OrderExpr(`"activity_instance".start_time DESC, "activity_instance".id DESC`).Scan(ctx, &rows)
	if err != nil {
		return nil, fmt.Errorf("presence projection: list parallel presence: %w", err)
	}
	return rows, nil
}

// ListPartialAbsenceBlocks lists the blocks of a child's day from a clock
// time on that an earlier pickup can still excuse: not cancelled, not ended,
// and with an attendance the care plan may still decide (no manual status,
// no non-booking, no day status, no other excusal than the automatic ones
// named). With enrolled, blocks the child's course enrollment covers but no
// roster lists yet are added.
func ListPartialAbsenceBlocks(ctx context.Context, db bun.IDB, tenantID, studentID int64, date timezone.Date, from time.Time, enrolled bool, autoExceptionIDs []int64) ([]scheduleModels.PartialAbsenceBlock, error) {
	if tenantID <= 0 {
		return nil, ErrInvalidTenantID
	}
	autoIDs := autoExceptionIDs
	if autoIDs == nil {
		autoIDs = []int64{}
	}
	rows := []scheduleModels.PartialAbsenceBlock{}
	clock := from.Format("15:04:05")
	err := db.NewSelect().TableExpr(`schedule.activity_instances AS "activity_instance"`).
		ColumnExpr(`"activity_instance".id, "activity_instance".title, "activity_instance".start_time, "activity_instance".end_time`).
		Join(instanceSessionJoin).
		Where(`"activity_instance".tenant_id = ?`, tenantID).Where(`"activity_instance".date = ?`, date).
		Where(`"activity_instance".start_time >= ?::time`, clock).
		Where(`"activity_instance".status <> 'cancelled'`).Where(`COALESCE("session".status, 'planned') <> 'completed'`).
		Where(`EXISTS (
	SELECT 1 FROM schedule.instance_students AS "instance_student"
	`+participantAttendanceJoin+`
	WHERE "instance_student".tenant_id = "activity_instance".tenant_id AND "instance_student".instance_id = "activity_instance".id
		AND "instance_student".student_id = ? AND "attendance".manual_status_at IS NULL
		AND NOT `+attendanceNotScheduled+` AND `+attendanceStatus+` IN ('expected', 'absent')
		AND "attendance".student_status_day_id IS NULL
		AND ("attendance".pickup_exception_id IS NULL OR "attendance".pickup_exception_id = ANY(?::BIGINT[])))`, studentID, pgdialect.Array(autoIDs)).
		OrderExpr(`"activity_instance".start_time ASC, "activity_instance".id ASC`).Scan(ctx, &rows)
	if err != nil {
		return nil, fmt.Errorf("presence projection: list partial absence blocks: %w", err)
	}
	if !enrolled {
		return rows, nil
	}
	unmaterialized := []scheduleModels.PartialAbsenceBlock{}
	err = db.NewSelect().TableExpr(`schedule.activity_instances AS "activity_instance"`).
		Distinct().ColumnExpr(`"activity_instance".id, "activity_instance".title, "activity_instance".start_time, "activity_instance".end_time`).
		Join(instanceSessionJoin).
		Join(`LEFT JOIN schedule.instance_students AS "instance_student" ON "instance_student".tenant_id = "activity_instance".tenant_id AND "instance_student".instance_id = "activity_instance".id AND "instance_student".student_id = ?`, studentID).
		Join(`INNER JOIN activities.student_enrollments AS "enrollment" ON "enrollment".tenant_id = "activity_instance".tenant_id AND "enrollment".student_id = ? AND "enrollment".activity_group_id = "activity_instance".activity_group_id`, studentID).
		Where(`"activity_instance".tenant_id = ?`, tenantID).Where(`"activity_instance".date = ?`, date).
		Where(`"activity_instance".start_time >= ?::time`, clock).
		Where(`"activity_instance".status <> 'cancelled'`).Where(`COALESCE("session".status, 'planned') <> 'completed'`).
		Where(`"instance_student".id IS NULL`).Where(`"enrollment".valid_from <= "activity_instance".date`).
		Where(`("enrollment".valid_until IS NULL OR "enrollment".valid_until > "activity_instance".date)`).
		Where(`("enrollment".calendar_period_id IS NULL OR "enrollment".calendar_period_id = "activity_instance".calendar_period_id)`).
		Where(`("enrollment".weekday IS NULL OR "enrollment".weekday = date_part('isodow', "activity_instance".date))`).
		Where(`(COALESCE(jsonb_array_length("enrollment".selected_weekdays), 0) = 0 OR "enrollment".selected_weekdays @> to_jsonb(ARRAY[date_part('isodow', "activity_instance".date)::integer]))`).
		OrderExpr(`"activity_instance".start_time ASC, "activity_instance".id ASC`).Scan(ctx, &unmaterialized)
	if err != nil {
		return nil, fmt.Errorf("presence projection: list partial absence enrollment blocks: %w", err)
	}
	return mergePartialAbsenceBlocks(rows, unmaterialized), nil
}

func mergePartialAbsenceBlocks(first, second []scheduleModels.PartialAbsenceBlock) []scheduleModels.PartialAbsenceBlock {
	byID := make(map[int64]scheduleModels.PartialAbsenceBlock, len(first)+len(second))
	for _, value := range append(first, second...) {
		byID[value.ID] = value
	}
	result := make([]scheduleModels.PartialAbsenceBlock, 0, len(byID))
	for _, value := range byID {
		result = append(result, value)
	}
	slices.SortFunc(result, func(a, b scheduleModels.PartialAbsenceBlock) int {
		if order := a.StartTime.Compare(b.StartTime); order != 0 {
			return order
		}
		return int(a.ID - b.ID)
	})
	return result
}

type scheduledInstanceRow struct {
	ParticipantID      int64         `bun:"is_id"`
	InstanceID         int64         `bun:"is_instance_id"`
	StudentID          int64         `bun:"is_student_id"`
	StudentRoomID      *int64        `bun:"is_room_id"`
	StudentCreatedAt   time.Time     `bun:"is_created_at"`
	StudentUpdatedAt   time.Time     `bun:"is_updated_at"`
	AttendanceStatus   string        `bun:"at_status"`
	Substatus          *string       `bun:"at_substatus"`
	Note               *string       `bun:"at_note"`
	CheckedInAt        *time.Time    `bun:"at_checked_in_at"`
	CheckedOutAt       *time.Time    `bun:"at_checked_out_at"`
	IsUnplanned        bool          `bun:"at_is_unplanned"`
	NotScheduled       bool          `bun:"at_not_scheduled"`
	ManualStatusAt     *time.Time    `bun:"at_manual_status_at"`
	StudentStatusDayID *int64        `bun:"at_student_status_day_id"`
	PickupExceptionID  *int64        `bun:"at_pickup_exception_id"`
	ActivityInstanceID int64         `bun:"ai_id"`
	Date               timezone.Date `bun:"ai_date"`
	ActivityGroupID    *int64        `bun:"ai_activity_group_id"`
	CalendarPeriodID   *int64        `bun:"ai_calendar_period_id"`
	Title              string        `bun:"ai_title"`
	Description        *string       `bun:"ai_description"`
	StartTime          time.Time     `bun:"ai_start_time"`
	EndTime            time.Time     `bun:"ai_end_time"`
	ActivityRoomID     int64         `bun:"ai_room_id"`
	ActivityStatus     string        `bun:"ai_status"`
	IsSpontaneous      bool          `bun:"ai_is_spontaneous"`
	Notes              *string       `bun:"ai_notes"`
	CreatedBy          *int64        `bun:"ai_created_by"`
	ActivityCreatedAt  time.Time     `bun:"ai_created_at"`
	ActivityUpdatedAt  time.Time     `bun:"ai_updated_at"`
	ActiveGroupID      *int64        `bun:"se_active_group_id"`
	StartedBy          *int64        `bun:"se_started_by"`
	StartedAt          *time.Time    `bun:"se_started_at"`
	CompletedAt        *time.Time    `bun:"se_completed_at"`
}

// ListScheduledInstancesForStudent returns the child's planned blocks in the
// inclusive date range together with the attendance each one carries,
// ordered by date and start time. Rows that only record a non-booking the
// care plan froze are left out.
func ListScheduledInstancesForStudent(ctx context.Context, db bun.IDB, tenantID, studentID int64, from, to timezone.Date) ([]*scheduleModels.ScheduledInstanceRow, error) {
	if tenantID <= 0 {
		return nil, ErrInvalidTenantID
	}
	rows := []scheduledInstanceRow{}
	err := db.NewSelect().TableExpr(`schedule.instance_students AS "instance_student"`).
		ColumnExpr(`"instance_student".id AS is_id, "instance_student".instance_id AS is_instance_id, "instance_student".student_id AS is_student_id,
	"instance_student".room_id AS is_room_id, "instance_student".created_at AS is_created_at, "instance_student".updated_at AS is_updated_at`).
		ColumnExpr(attendanceStatus+` AS at_status, "attendance".substatus AS at_substatus, "attendance".note AS at_note,
	"attendance".checked_in_at AS at_checked_in_at, "attendance".checked_out_at AS at_checked_out_at,
	`+attendanceUnplanned+` AS at_is_unplanned, `+attendanceNotScheduled+` AS at_not_scheduled,
	"attendance".manual_status_at AS at_manual_status_at, "attendance".student_status_day_id AS at_student_status_day_id,
	"attendance".pickup_exception_id AS at_pickup_exception_id`).
		ColumnExpr(`"activity_instance".id AS ai_id, "activity_instance".date AS ai_date, "activity_instance".activity_group_id AS ai_activity_group_id,
	"activity_instance".calendar_period_id AS ai_calendar_period_id, "activity_instance".title AS ai_title,
	"activity_instance".description AS ai_description, "activity_instance".start_time AS ai_start_time,
	"activity_instance".end_time AS ai_end_time, "activity_instance".room_id AS ai_room_id,
	`+executionStatus+` AS ai_status, "activity_instance".is_spontaneous AS ai_is_spontaneous, "activity_instance".notes AS ai_notes,
	"activity_instance".created_by AS ai_created_by, "activity_instance".created_at AS ai_created_at, "activity_instance".updated_at AS ai_updated_at`).
		ColumnExpr(`"session".active_group_id AS se_active_group_id, "session".started_by AS se_started_by,
	"session".started_at AS se_started_at, "session".completed_at AS se_completed_at`).
		Join(participantAttendanceJoin).
		Join(`INNER JOIN schedule.activity_instances AS "activity_instance" ON "activity_instance".id = "instance_student".instance_id AND "activity_instance".tenant_id = "instance_student".tenant_id`).
		Join(instanceSessionJoin).
		Where(`"instance_student".tenant_id = ?`, tenantID).Where(`"instance_student".student_id = ?`, studentID).
		Where(`"activity_instance".date >= ?`, from).Where(`"activity_instance".date <= ?`, to).
		Where(`NOT (`+attendanceNotScheduled+` AND `+attendanceStatus+` = 'expected' AND "attendance".manual_status_at IS NULL)`).
		OrderExpr(`"activity_instance".date ASC, "activity_instance".start_time ASC`).Scan(ctx, &rows)
	if err != nil {
		return nil, fmt.Errorf("presence projection: list scheduled instances for student: %w", err)
	}
	result := make([]*scheduleModels.ScheduledInstanceRow, 0, len(rows))
	for _, row := range rows {
		instance := &scheduleModels.ActivityInstance{
			Date: scheduleModels.Date(row.Date), ActivityGroupID: row.ActivityGroupID, CalendarPeriodID: row.CalendarPeriodID, Title: row.Title,
			Description: row.Description, StartTime: row.StartTime, EndTime: row.EndTime, RoomID: row.ActivityRoomID,
			Status: row.ActivityStatus, ActiveGroupID: row.ActiveGroupID, IsSpontaneous: row.IsSpontaneous, Notes: row.Notes,
			CreatedBy: row.CreatedBy, StartedBy: row.StartedBy, StartedAt: row.StartedAt, CompletedAt: row.CompletedAt,
		}
		instance.ID, instance.CreatedAt, instance.UpdatedAt = row.ActivityInstanceID, row.ActivityCreatedAt, row.ActivityUpdatedAt
		instance.SetTenantID(tenantID)
		attendance := &scheduleModels.InstanceStudent{
			InstanceID: row.InstanceID, StudentID: row.StudentID, RoomID: row.StudentRoomID, Status: row.AttendanceStatus,
			Substatus: row.Substatus, Note: row.Note, CheckedInAt: row.CheckedInAt, CheckedOutAt: row.CheckedOutAt,
			IsUnplanned: row.IsUnplanned, NotScheduled: row.NotScheduled, ManualStatusAt: row.ManualStatusAt,
			StudentStatusDayID: row.StudentStatusDayID, PickupExceptionID: row.PickupExceptionID,
		}
		attendance.ID, attendance.CreatedAt, attendance.UpdatedAt = row.ParticipantID, row.StudentCreatedAt, row.StudentUpdatedAt
		attendance.SetTenantID(tenantID)
		result = append(result, &scheduleModels.ScheduledInstanceRow{Instance: instance, Attendance: attendance})
	}
	return result, nil
}

// HasPlannedSlotsInRange reports whether the tenant planned at least one
// child onto a not cancelled block in the inclusive range. Walk-ins do not
// count.
func HasPlannedSlotsInRange(ctx context.Context, db bun.IDB, tenantID int64, from, to timezone.Date) (bool, error) {
	if tenantID <= 0 {
		return false, ErrInvalidTenantID
	}
	exists, err := db.NewSelect().TableExpr(`schedule.instance_students AS "instance_student"`).
		Join(participantAttendanceJoin).
		Join(`INNER JOIN schedule.activity_instances AS "activity_instance" ON "activity_instance".id = "instance_student".instance_id AND "activity_instance".tenant_id = "instance_student".tenant_id`).
		Where(`"instance_student".tenant_id = ?`, tenantID).Where(`NOT `+attendanceUnplanned).
		Where(`"activity_instance".status <> 'cancelled'`).
		Where(`"activity_instance".date >= ?`, from).Where(`"activity_instance".date <= ?`, to).Exists(ctx)
	if err != nil {
		return false, fmt.Errorf("presence projection: check planned slots in range: %w", err)
	}
	return exists, nil
}

// OpenParticipant is a participant whose block presence is still open.
type OpenParticipant struct {
	ParticipantID int64 `bun:"participant_id"`
	StudentID     int64 `bun:"student_id"`
}

// ListOpenParticipants returns the participants of the given students with
// an open block presence (checked in, not checked out), ordered by
// participant id.
func ListOpenParticipants(ctx context.Context, db bun.IDB, tenantID int64, studentIDs []int64) ([]OpenParticipant, error) {
	if tenantID <= 0 {
		return nil, ErrInvalidTenantID
	}
	if len(studentIDs) == 0 {
		return []OpenParticipant{}, nil
	}
	rows := []OpenParticipant{}
	err := db.NewSelect().TableExpr(`schedule.instance_students AS "instance_student"`).
		ColumnExpr(`"instance_student".id AS participant_id, "instance_student".student_id`).
		Join(`INNER JOIN active.activity_session_attendance AS "attendance" ON "attendance".tenant_id = "instance_student".tenant_id AND "attendance".instance_student_id = "instance_student".id`).
		Where(`"instance_student".tenant_id = ?`, tenantID).
		Where(`"instance_student".student_id IN (?)`, bun.List(studentIDs)).
		Where(`"attendance".checked_in_at IS NOT NULL`).Where(`"attendance".checked_out_at IS NULL`).
		OrderExpr(`"instance_student".id`).Scan(ctx, &rows)
	if err != nil {
		return nil, fmt.Errorf("presence projection: list open participants: %w", err)
	}
	return rows, nil
}

// LatestAttendedBlockDate returns the date of the last block the child was
// checked in to, or nil when there is none.
func LatestAttendedBlockDate(ctx context.Context, db bun.IDB, tenantID, studentID int64) (*timezone.Date, error) {
	if tenantID <= 0 {
		return nil, ErrInvalidTenantID
	}
	var day *timezone.Date
	err := db.NewRaw(`SELECT MAX("activity_instance".date)
	FROM schedule.instance_students AS "instance_student"
	JOIN active.activity_session_attendance AS "attendance"
	  ON "attendance".tenant_id = "instance_student".tenant_id AND "attendance".instance_student_id = "instance_student".id
	JOIN schedule.activity_instances AS "activity_instance"
	  ON "activity_instance".tenant_id = "instance_student".tenant_id AND "activity_instance".id = "instance_student".instance_id
	WHERE "instance_student".tenant_id = ? AND "instance_student".student_id = ? AND "attendance".checked_in_at IS NOT NULL`,
		tenantID, studentID).Scan(ctx, &day)
	if err != nil {
		return nil, fmt.Errorf("presence projection: latest attended block date: %w", err)
	}
	if day == nil || day.IsZero() {
		return nil, nil
	}
	return day, nil
}

// CourseInstanceRow counts the occurrences of one course in a period.
type CourseInstanceRow struct {
	CourseID           int64  `bun:"course_id"`
	Name               string `bun:"name"`
	CategoryName       string `bun:"category_name"`
	MaxParticipants    int    `bun:"max_participants"`
	HeldInstances      int    `bun:"held_instances"`
	CancelledInstances int    `bun:"cancelled_instances"`
}

// CourseParticipationRow aggregates one child's attendance in one course.
type CourseParticipationRow struct {
	CourseID    int64 `bun:"course_id"`
	StudentID   int64 `bun:"student_id"`
	PresentDays int   `bun:"present_days"`
	AbsentDays  int   `bun:"absent_days"`
	OpenDays    int   `bun:"open_days"`
}

// courseKeyExpr folds every segment a template split produced back onto the
// original row: the original keeps series_root_id NULL and is its own root.
const courseKeyExpr = `COALESCE("template".series_root_id, "template".id)`

// heldInstance is true for an occurrence that took place: it started or
// ended, or its day is over and it was not cancelled. The bind parameter is
// the report date the caller captured once.
const heldInstance = `("activity_instance".status <> 'cancelled' AND ("session".status IN ('active', 'completed') OR ("session".status IS NULL AND "activity_instance".date < ?)))`

// CourseInstances counts the occurrences of every course in [from, to],
// separating the ones that happened from the cancelled ones. Only
// Betreuungsplan templates are courses.
func CourseInstances(ctx context.Context, db bun.IDB, tenantID int64, from, to, today timezone.Date) ([]CourseInstanceRow, error) {
	if tenantID <= 0 {
		return nil, ErrInvalidTenantID
	}
	rows := []CourseInstanceRow{}
	err := db.NewSelect().TableExpr(`schedule.activity_instances AS "activity_instance"`).
		Join(instanceSessionJoin).
		Join(`JOIN activities.groups AS "template" ON "template".id = "activity_instance".activity_group_id AND "template".tenant_id = "activity_instance".tenant_id`).
		Join(`JOIN activities.groups AS "root" ON "root".id = `+courseKeyExpr+` AND "root".tenant_id = "activity_instance".tenant_id`).
		Join(`LEFT JOIN activities.categories AS "category" ON "category".id = "root".category_id`).
		ColumnExpr(courseKeyExpr+` AS course_id`).ColumnExpr(`"root".name AS name`).
		ColumnExpr(`COALESCE("category".name, '') AS category_name`).
		ColumnExpr(`COALESCE("root".max_participants, 0) AS max_participants`).
		ColumnExpr(`COUNT(*) FILTER (WHERE `+heldInstance+`)::int AS held_instances`, today).
		ColumnExpr(`COUNT(*) FILTER (WHERE "activity_instance".status = 'cancelled')::int AS cancelled_instances`).
		Where(`"activity_instance".tenant_id = ?`, tenantID).
		Where(`"activity_instance".date >= ? AND "activity_instance".date <= ?`, from, to).
		Where(`"template".is_template`).Where(`"root".is_template`).
		GroupExpr(courseKeyExpr+`, "root".name, "root".max_participants, "category".name`).Scan(ctx, &rows)
	if err != nil {
		return nil, fmt.Errorf("presence projection: course instances: %w", err)
	}
	return rows, nil
}

// enrolledOnInstanceDate keeps only the attendance rows the child's course
// enrollment covers on that date, plus walk-ins and rows for courses the
// child has no enrollment for at all.
const enrolledOnInstanceDate = `(
		` + attendanceUnplanned + `
		OR EXISTS (
			SELECT 1 FROM activities.student_enrollments AS "enrollment"
			WHERE "enrollment".tenant_id = "instance_student".tenant_id
				AND "enrollment".student_id = "instance_student".student_id
				AND "enrollment".activity_group_id = "activity_instance".activity_group_id
				AND "enrollment".valid_from <= "activity_instance".date
				AND ("enrollment".valid_until IS NULL OR "enrollment".valid_until > "activity_instance".date)
				AND ("enrollment".calendar_period_id IS NULL
					OR "enrollment".calendar_period_id = "activity_instance".calendar_period_id)
				AND ("enrollment".weekday IS NULL
					OR "enrollment".weekday = date_part('isodow', "activity_instance".date))
				AND (COALESCE(jsonb_array_length("enrollment".selected_weekdays), 0) = 0
					OR "enrollment".selected_weekdays @> to_jsonb(ARRAY[date_part('isodow', "activity_instance".date)::integer]))
		)
		OR NOT EXISTS (
			SELECT 1 FROM activities.student_enrollments AS "enrollment"
			WHERE "enrollment".tenant_id = "instance_student".tenant_id
				AND "enrollment".student_id = "instance_student".student_id
				AND "enrollment".activity_group_id = "activity_instance".activity_group_id
		)
	)`

// CourseParticipation aggregates the attendance of every child per course
// over the occurrences CourseInstances counts as held.
func CourseParticipation(ctx context.Context, db bun.IDB, tenantID int64, from, to, today timezone.Date) ([]CourseParticipationRow, error) {
	if tenantID <= 0 {
		return nil, ErrInvalidTenantID
	}
	rows := []CourseParticipationRow{}
	err := db.NewSelect().TableExpr(`schedule.instance_students AS "instance_student"`).
		Join(participantAttendanceJoin).
		Join(`JOIN schedule.activity_instances AS "activity_instance" ON "activity_instance".id = "instance_student".instance_id AND "activity_instance".tenant_id = "instance_student".tenant_id`).
		Join(instanceSessionJoin).
		Join(`JOIN activities.groups AS "template" ON "template".id = "activity_instance".activity_group_id AND "template".tenant_id = "activity_instance".tenant_id`).
		ColumnExpr(courseKeyExpr+` AS course_id`).ColumnExpr(`"instance_student".student_id AS student_id`).
		ColumnExpr(`COUNT(*) FILTER (WHERE `+attendanceStatus+` = 'present')::int AS present_days`).
		ColumnExpr(`COUNT(*) FILTER (WHERE `+attendanceStatus+` = 'absent')::int AS absent_days`).
		ColumnExpr(`COUNT(*) FILTER (WHERE `+attendanceStatus+` = 'expected')::int AS open_days`).
		Where(`"instance_student".tenant_id = ?`, tenantID).
		Where(`"activity_instance".date >= ? AND "activity_instance".date <= ?`, from, to).
		Where(heldInstance, today).
		Where(`NOT (`+attendanceNotScheduled+` AND `+attendanceStatus+` = 'expected')`).
		Where(enrolledOnInstanceDate).Where(`"template".is_template`).
		GroupExpr(courseKeyExpr+`, "instance_student".student_id`).Scan(ctx, &rows)
	if err != nil {
		return nil, fmt.Errorf("presence projection: course participation: %w", err)
	}
	return rows, nil
}
