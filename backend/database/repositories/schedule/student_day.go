package schedule

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/schedule"
)

// ScheduledInstanceRow pairs an activity instance with the caller's attendance
// row on that instance. The timetable /student/{id}/day and /week endpoints
// assemble their response from rows of this shape: the instance carries the
// when/where/title, the attendance carries the student-specific status.
//
// Both fields are non-nil when returned by
// FindInstancesWithAttendanceByStudentAndDateRange — the INNER JOIN guarantees
// it. The struct lives in the repo package, not in models/schedule, because
// it is a query-result shape, not a persistent entity.
type ScheduledInstanceRow struct {
	Instance   *schedule.ActivityInstance
	Attendance *schedule.InstanceStudent
}

// scheduledInstanceScan is the flat row bun scans into; the repo then lifts
// it into a pair of pointers. Keeping the column aliases explicit sidesteps
// bun's cross-schema Relation limitations (same rationale as visits.go
// FindByStudentAndTimeRange, where an ad-hoc struct works around the tag).
type scheduledInstanceScan struct {
	// instance_students columns
	IsID          int64      `bun:"is_id"`
	IsTenantID    int64      `bun:"is_tenant_id"`
	IsInstanceID  int64      `bun:"is_instance_id"`
	IsStudentID   int64      `bun:"is_student_id"`
	IsRoomID      *int64     `bun:"is_room_id"`
	IsStatus      string     `bun:"is_status"`
	IsSubstatus   *string    `bun:"is_substatus"`
	IsNote        *string    `bun:"is_note"`
	IsCheckedInAt *time.Time `bun:"is_checked_in_at"`
	IsCreatedAt   time.Time  `bun:"is_created_at"`
	IsUpdatedAt   time.Time  `bun:"is_updated_at"`

	// activity_instances columns
	AiID              int64      `bun:"ai_id"`
	AiTenantID        int64      `bun:"ai_tenant_id"`
	AiDate            time.Time  `bun:"ai_date"`
	AiActivityGroupID *int64     `bun:"ai_activity_group_id"`
	AiCalendarPeriod  *int64     `bun:"ai_calendar_period_id"`
	AiTitle           string     `bun:"ai_title"`
	AiDescription     *string    `bun:"ai_description"`
	AiStartTime       time.Time  `bun:"ai_start_time"`
	AiEndTime         time.Time  `bun:"ai_end_time"`
	AiRoomID          int64      `bun:"ai_room_id"`
	AiStatus          string     `bun:"ai_status"`
	AiActiveGroupID   *int64     `bun:"ai_active_group_id"`
	AiIsSpontaneous   bool       `bun:"ai_is_spontaneous"`
	AiNotes           *string    `bun:"ai_notes"`
	AiCreatedBy       *int64     `bun:"ai_created_by"`
	AiStartedBy       *int64     `bun:"ai_started_by"`
	AiStartedAt       *time.Time `bun:"ai_started_at"`
	AiCompletedAt     *time.Time `bun:"ai_completed_at"`
	AiCreatedAt       time.Time  `bun:"ai_created_at"`
	AiUpdatedAt       time.Time  `bun:"ai_updated_at"`
}

// FindInstancesWithAttendanceByStudentAndDateRange returns one row per
// (instance_student, activity_instance) pair for the student in the inclusive
// date range, sorted by date then start time. Tenant-scoped via the caller's
// context. Single query — no N+1.
//
// Use this for the timetable per-student day/week aggregation. Instances
// where the student has NO attendance row (e.g. a spontaneous instance they
// dropped into without being enrolled) are NOT returned here — the handler
// layer enriches those via the visits-side lookup.
func (r *InstanceStudentRepository) FindInstancesWithAttendanceByStudentAndDateRange(
	ctx context.Context, studentID int64, from, to time.Time,
) ([]*ScheduledInstanceRow, error) {
	var scans []scheduledInstanceScan

	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(`schedule.instance_students AS "instance_student"`).
		Join(`INNER JOIN schedule.activity_instances AS "activity_instance" ON "activity_instance".id = "instance_student".instance_id`).
		ColumnExpr(`"instance_student".id AS is_id`).
		ColumnExpr(`"instance_student".tenant_id AS is_tenant_id`).
		ColumnExpr(`"instance_student".instance_id AS is_instance_id`).
		ColumnExpr(`"instance_student".student_id AS is_student_id`).
		ColumnExpr(`"instance_student".room_id AS is_room_id`).
		ColumnExpr(`"instance_student".status AS is_status`).
		ColumnExpr(`"instance_student".substatus AS is_substatus`).
		ColumnExpr(`"instance_student".note AS is_note`).
		ColumnExpr(`"instance_student".checked_in_at AS is_checked_in_at`).
		ColumnExpr(`"instance_student".created_at AS is_created_at`).
		ColumnExpr(`"instance_student".updated_at AS is_updated_at`).
		ColumnExpr(`"activity_instance".id AS ai_id`).
		ColumnExpr(`"activity_instance".tenant_id AS ai_tenant_id`).
		ColumnExpr(`"activity_instance".date AS ai_date`).
		ColumnExpr(`"activity_instance".activity_group_id AS ai_activity_group_id`).
		ColumnExpr(`"activity_instance".calendar_period_id AS ai_calendar_period_id`).
		ColumnExpr(`"activity_instance".title AS ai_title`).
		ColumnExpr(`"activity_instance".description AS ai_description`).
		ColumnExpr(`"activity_instance".start_time AS ai_start_time`).
		ColumnExpr(`"activity_instance".end_time AS ai_end_time`).
		ColumnExpr(`"activity_instance".room_id AS ai_room_id`).
		ColumnExpr(`"activity_instance".status AS ai_status`).
		ColumnExpr(`"activity_instance".active_group_id AS ai_active_group_id`).
		ColumnExpr(`"activity_instance".is_spontaneous AS ai_is_spontaneous`).
		ColumnExpr(`"activity_instance".notes AS ai_notes`).
		ColumnExpr(`"activity_instance".created_by AS ai_created_by`).
		ColumnExpr(`"activity_instance".started_by AS ai_started_by`).
		ColumnExpr(`"activity_instance".started_at AS ai_started_at`).
		ColumnExpr(`"activity_instance".completed_at AS ai_completed_at`).
		ColumnExpr(`"activity_instance".created_at AS ai_created_at`).
		ColumnExpr(`"activity_instance".updated_at AS ai_updated_at`).
		Where(`"instance_student".student_id = ?`, studentID).
		Where(`"activity_instance".date >= ?`, from).
		Where(`"activity_instance".date <= ?`, to).
		OrderExpr(`"activity_instance".date ASC, "activity_instance".start_time ASC`)

	if where, val, ok := base.TenantWhere(ctx, aliasInstanceStudent); ok {
		query = query.Where(where, val)
	}

	if err := query.Scan(ctx, &scans); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find instances with attendance by student and date range",
			Err: err,
		}
	}

	out := make([]*ScheduledInstanceRow, 0, len(scans))
	for i := range scans {
		s := scans[i]
		inst := &schedule.ActivityInstance{
			Date:             s.AiDate,
			ActivityGroupID:  s.AiActivityGroupID,
			CalendarPeriodID: s.AiCalendarPeriod,
			Title:            s.AiTitle,
			Description:      s.AiDescription,
			StartTime:        s.AiStartTime,
			EndTime:          s.AiEndTime,
			RoomID:           s.AiRoomID,
			Status:           s.AiStatus,
			ActiveGroupID:    s.AiActiveGroupID,
			IsSpontaneous:    s.AiIsSpontaneous,
			Notes:            s.AiNotes,
			CreatedBy:        s.AiCreatedBy,
			StartedBy:        s.AiStartedBy,
			StartedAt:        s.AiStartedAt,
			CompletedAt:      s.AiCompletedAt,
		}
		inst.ID = s.AiID
		inst.CreatedAt = s.AiCreatedAt
		inst.UpdatedAt = s.AiUpdatedAt
		inst.SetTenantID(s.AiTenantID)

		att := &schedule.InstanceStudent{
			InstanceID:  s.IsInstanceID,
			StudentID:   s.IsStudentID,
			RoomID:      s.IsRoomID,
			Status:      s.IsStatus,
			Substatus:   s.IsSubstatus,
			Note:        s.IsNote,
			CheckedInAt: s.IsCheckedInAt,
		}
		att.ID = s.IsID
		att.CreatedAt = s.IsCreatedAt
		att.UpdatedAt = s.IsUpdatedAt
		att.SetTenantID(s.IsTenantID)

		out = append(out, &ScheduledInstanceRow{Instance: inst, Attendance: att})
	}
	return out, nil
}
