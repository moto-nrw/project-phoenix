package schedule

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/uptrace/bun"
)

// scheduledInstanceScan is the flat row bun scans into; the repo then lifts
// it into a pair of pointers. Keeping the column aliases explicit sidesteps
// bun's cross-schema Relation limitations (same rationale as visits.go
// FindByStudentAndTimeRange, where an ad-hoc struct works around the tag).
type scheduledInstanceScan struct {
	// instance_students columns
	IsID                 int64      `bun:"is_id"`
	IsTenantID           int64      `bun:"is_tenant_id"`
	IsInstanceID         int64      `bun:"is_instance_id"`
	IsStudentID          int64      `bun:"is_student_id"`
	IsRoomID             *int64     `bun:"is_room_id"`
	IsStatus             string     `bun:"is_status"`
	IsSubstatus          *string    `bun:"is_substatus"`
	IsNote               *string    `bun:"is_note"`
	IsCheckedInAt        *time.Time `bun:"is_checked_in_at"`
	IsCheckedOutAt       *time.Time `bun:"is_checked_out_at"`
	IsUnplanned          bool       `bun:"is_unplanned"`
	IsNotScheduled       bool       `bun:"is_not_scheduled"`
	IsManualStatusAt     *time.Time `bun:"is_manual_status_at"`
	IsStudentStatusDayID *int64     `bun:"is_student_status_day_id"`
	IsCreatedAt          time.Time  `bun:"is_created_at"`
	IsUpdatedAt          time.Time  `bun:"is_updated_at"`

	// activity_instances columns
	AiID              int64         `bun:"ai_id"`
	AiTenantID        int64         `bun:"ai_tenant_id"`
	AiDate            timezone.Date `bun:"ai_date"`
	AiActivityGroupID *int64        `bun:"ai_activity_group_id"`
	AiCalendarPeriod  *int64        `bun:"ai_calendar_period_id"`
	AiTitle           string        `bun:"ai_title"`
	AiDescription     *string       `bun:"ai_description"`
	AiStartTime       time.Time     `bun:"ai_start_time"`
	AiEndTime         time.Time     `bun:"ai_end_time"`
	AiRoomID          int64         `bun:"ai_room_id"`
	AiStatus          string        `bun:"ai_status"`
	AiActiveGroupID   *int64        `bun:"ai_active_group_id"`
	AiIsSpontaneous   bool          `bun:"ai_is_spontaneous"`
	AiNotes           *string       `bun:"ai_notes"`
	AiCreatedBy       *int64        `bun:"ai_created_by"`
	AiStartedBy       *int64        `bun:"ai_started_by"`
	AiStartedAt       *time.Time    `bun:"ai_started_at"`
	AiCompletedAt     *time.Time    `bun:"ai_completed_at"`
	AiCreatedAt       time.Time     `bun:"ai_created_at"`
	AiUpdatedAt       time.Time     `bun:"ai_updated_at"`
}

// FindInstancesWithAttendanceByStudentAndDateRange implements
// schedule.InstanceStudentRepository.
func (r *InstanceStudentRepository) FindInstancesWithAttendanceByStudentAndDateRange(
	ctx context.Context, studentID int64, from, to timezone.Date,
) ([]*schedule.ScheduledInstanceRow, error) {
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
		ColumnExpr(`"instance_student".checked_out_at AS is_checked_out_at`).
		ColumnExpr(`"instance_student".is_unplanned AS is_unplanned`).
		ColumnExpr(`"instance_student".not_scheduled AS is_not_scheduled`).
		ColumnExpr(`"instance_student".manual_status_at AS is_manual_status_at`).
		ColumnExpr(`"instance_student".student_status_day_id AS is_student_status_day_id`).
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
		// Ending a block writes an absence for every genuinely expected child —
		// including the days somebody cancelled, which belong in the history as
		// absences — and stamps not_scheduled on the children the care plan did
		// not book at all that day (#1747). That frozen marker must not
		// resurface as expected attendance in the history, the export, or the
		// week view.
		//
		// The status half of the predicate is what lets a human override it: a
		// marked child who turned up anyway is 'present', a marked slot somebody
		// decided by hand is 'absent', and both stay visible. Only the untouched
		// marker is hidden. Reading the marker from its own column (rather than
		// inferring it from 'expected' on a completed instance) is what keeps
		// the other writers of `status` — the attendance PATCH, ApplyStatusDay —
		// from creating or destroying it by accident.
		//
		// manual_status_at is the third guard, and it is not redundant with the
		// status half: staff can set an unbooked slot back to 'expected' ("the
		// plan is wrong, this child is coming"), which lands on the exact shape
		// the marker claims. The PATCH clears not_scheduled on that write, so
		// live rows never reach here as both — but the day this predicate is the
		// only thing standing between a deliberate decision and an invisible row
		// is the day some other writer forgets that pairing. A hand-decided row
		// stays visible on its own evidence (#1747 review).
		Where(`NOT ("instance_student".not_scheduled AND "instance_student".status = ? AND "instance_student".manual_status_at IS NULL)`,
			schedule.AttendanceStatusExpected).
		OrderExpr(`"activity_instance".date ASC, "activity_instance".start_time ASC`)

	query = base.WithTenantFilter(ctx, query, aliasInstanceStudent)

	if err := query.Scan(ctx, &scans); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find instances with attendance by student and date range",
			Err: err,
		}
	}

	out := make([]*schedule.ScheduledInstanceRow, 0, len(scans))
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
			InstanceID:         s.IsInstanceID,
			StudentID:          s.IsStudentID,
			RoomID:             s.IsRoomID,
			Status:             s.IsStatus,
			Substatus:          s.IsSubstatus,
			Note:               s.IsNote,
			CheckedInAt:        s.IsCheckedInAt,
			CheckedOutAt:       s.IsCheckedOutAt,
			IsUnplanned:        s.IsUnplanned,
			NotScheduled:       s.IsNotScheduled,
			ManualStatusAt:     s.IsManualStatusAt,
			StudentStatusDayID: s.IsStudentStatusDayID,
		}
		att.ID = s.IsID
		att.CreatedAt = s.IsCreatedAt
		att.UpdatedAt = s.IsUpdatedAt
		att.SetTenantID(s.IsTenantID)

		out = append(out, &schedule.ScheduledInstanceRow{Instance: inst, Attendance: att})
	}
	return out, nil
}

// HasPlannedSlotsInRange implements schedule.InstanceStudentRepository.
//
// Deliberately NOT filtered by student: the attendance history needs the
// tenant-wide answer, and the walk-in exclusion keeps spontaneous drop-ins
// from counting as care-plan usage. Cancelled instances are excluded as well —
// instance_students rows survive a cancellation, and a booking on a
// cancelled-only occurrence is no evidence of a usable care plan.
func (r *InstanceStudentRepository) HasPlannedSlotsInRange(
	ctx context.Context, from, to timezone.Date,
) (bool, error) {
	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(modelTblInstanceStudent).
		Join(`INNER JOIN schedule.activity_instances AS "activity_instance" ON "activity_instance".id = "instance_student".instance_id AND "activity_instance".tenant_id = "instance_student".tenant_id`).
		Where(`"instance_student".is_unplanned = FALSE`).
		Where(`"activity_instance".status <> ?`, schedule.InstanceStatusCancelled).
		Where(`"activity_instance".date >= ?`, from).
		Where(`"activity_instance".date <= ?`, to)

	query = base.WithTenantFilter(ctx, query, aliasInstanceStudent)

	exists, err := query.Exists(ctx)
	if err != nil {
		return false, &modelBase.DatabaseError{
			Op:  "check planned slots in range",
			Err: err,
		}
	}
	return exists, nil
}

func (r *InstanceStudentRepository) FindPlannedStudentIDsByDate(ctx context.Context, studentIDs []int64, date timezone.Date) ([]int64, error) {
	if len(studentIDs) == 0 {
		return []int64{}, nil
	}

	var ids []int64
	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(`schedule.instance_students AS "instance_student"`).
		Join(`INNER JOIN schedule.activity_instances AS "activity_instance" ON "activity_instance".id = "instance_student".instance_id AND "activity_instance".tenant_id = "instance_student".tenant_id`).
		ColumnExpr(`DISTINCT "instance_student".student_id`).
		Where(`"instance_student".student_id IN (?)`, bun.List(studentIDs)).
		Where(`"activity_instance".date = ?`, date).
		Where(`"activity_instance".status <> ?`, schedule.InstanceStatusCancelled).
		OrderExpr(`"instance_student".student_id ASC`)

	query = base.WithTenantFilter(ctx, query, aliasInstanceStudent)

	if err := query.Scan(ctx, &ids); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find planned student ids by date",
			Err: err,
		}
	}

	return ids, nil
}
