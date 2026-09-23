package presenceprojection

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/uptrace/bun"
)

// The retained DTOs of the legacy schedule repositories used to be one row
// each. Since the presence cutover (#2762) a DTO is the Timetable plan joined
// with the Student Presence execution (activity instances) or attendance
// (participants). The two list reads below serve those DTOs in one
// statement so the retained list endpoints keep their query budgets.

// LegacyInstanceFilter selects activity instances the way the retained
// repositories ask for them. Status uses the legacy vocabulary: planned is a
// plan nobody started, active and completed are session states, cancelled is
// a plan state.
type LegacyInstanceFilter struct {
	IDs                 []int64
	Date                *string
	Dates               []string
	FromDate            *string
	ToDate              *string
	ActivityGroupID     *int64
	ActivityGroupIDs    []int64
	ActiveGroupIDs      []int64
	Status              string
	IsSpontaneous       *bool
	IdempotencyKey      string
	MaterializedPlanned bool
	OrderByDateAndTime  bool
	Limit               int
	Offset              int
}

type legacyInstanceRow struct {
	ID                     int64         `bun:"id"`
	TenantID               int64         `bun:"tenant_id"`
	CreatedAt              time.Time     `bun:"created_at"`
	UpdatedAt              time.Time     `bun:"updated_at"`
	Date                   timezone.Date `bun:"date"`
	ActivityGroupID        *int64        `bun:"activity_group_id"`
	CalendarPeriodID       *int64        `bun:"calendar_period_id"`
	Title                  string        `bun:"title"`
	Description            *string       `bun:"description"`
	StartTime              time.Time     `bun:"start_time"`
	EndTime                time.Time     `bun:"end_time"`
	RoomID                 int64         `bun:"room_id"`
	RequiredStaff          *int          `bun:"required_staff"`
	PlanStatus             string        `bun:"plan_status"`
	ListKind               *string       `bun:"list_kind"`
	IsSpontaneous          bool          `bun:"is_spontaneous"`
	UnderstaffedAck        bool          `bun:"understaffed_ack"`
	UnderstaffedNote       *string       `bun:"understaffed_note"`
	CancelReason           *string       `bun:"cancel_reason"`
	Notes                  *string       `bun:"notes"`
	IdempotencyKey         *string       `bun:"idempotency_key"`
	IdempotencyFingerprint *string       `bun:"idempotency_fingerprint"`
	CreatedBy              *int64        `bun:"created_by"`
	SessionStatus          *string       `bun:"session_status"`
	ActiveGroupID          *int64        `bun:"session_active_group_id"`
	StartedBy              *int64        `bun:"session_started_by"`
	StartedAt              *time.Time    `bun:"session_started_at"`
	CompletedAt            *time.Time    `bun:"session_completed_at"`
	CompletedBy            *int64        `bun:"session_completed_by"`
	ReopenUntil            *time.Time    `bun:"session_reopen_until"`
	CompletionSnapshot     []byte        `bun:"session_completion_snapshot"`
}

const legacyInstanceColumns = `"activity_instance".id, "activity_instance".tenant_id, "activity_instance".created_at, "activity_instance".updated_at,
	"activity_instance".date, "activity_instance".activity_group_id, "activity_instance".calendar_period_id, "activity_instance".title,
	"activity_instance".description, "activity_instance".start_time, "activity_instance".end_time, "activity_instance".room_id,
	"activity_instance".required_staff, "activity_instance".status AS plan_status, "activity_instance".list_kind,
	"activity_instance".is_spontaneous, "activity_instance".understaffed_ack, "activity_instance".understaffed_note,
	"activity_instance".cancel_reason, "activity_instance".notes, "activity_instance".idempotency_key,
	"activity_instance".idempotency_fingerprint, "activity_instance".created_by,
	"session".status AS session_status, "session".active_group_id AS session_active_group_id,
	"session".started_by AS session_started_by, "session".started_at AS session_started_at,
	"session".completed_at AS session_completed_at, "session".completed_by AS session_completed_by,
	"session".reopen_until AS session_reopen_until, "session".completion_snapshot AS session_completion_snapshot`

// ListLegacyInstances lists activity instances with their execution as the
// retained repositories read them.
func ListLegacyInstances(ctx context.Context, db bun.IDB, tenantID int64, filter LegacyInstanceFilter) ([]*scheduleModels.ActivityInstance, error) {
	if tenantID <= 0 {
		return nil, ErrInvalidTenantID
	}
	query := db.NewSelect().TableExpr(`schedule.activity_instances AS "activity_instance"`).
		ColumnExpr(legacyInstanceColumns).Join(instanceSessionJoin).
		Where(`"activity_instance".tenant_id = ?`, tenantID)
	query = legacyInstanceStateFilters(legacyInstanceKeyFilters(query, filter), filter)
	rows := []legacyInstanceRow{}
	if err := query.Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("presence projection: list legacy instances: %w", err)
	}
	result := make([]*scheduleModels.ActivityInstance, 0, len(rows))
	for _, row := range rows {
		result = append(result, legacyInstance(row))
	}
	return result, nil
}

// legacyInstanceKeyFilters narrows the read by the ids, dates and groups of
// the filter.
func legacyInstanceKeyFilters(query *bun.SelectQuery, filter LegacyInstanceFilter) *bun.SelectQuery {
	if len(filter.IDs) > 0 {
		query = query.Where(`"activity_instance".id IN (?)`, bun.List(filter.IDs))
	}
	if filter.Date != nil {
		query = query.Where(`"activity_instance".date = ?::date`, *filter.Date)
	}
	if len(filter.Dates) > 0 {
		query = query.Where(`"activity_instance".date IN (?)`, bun.List(filter.Dates))
	}
	if filter.FromDate != nil {
		query = query.Where(`"activity_instance".date >= ?::date`, *filter.FromDate)
	}
	if filter.ToDate != nil {
		query = query.Where(`"activity_instance".date <= ?::date`, *filter.ToDate)
	}
	if filter.ActivityGroupID != nil {
		query = query.Where(`"activity_instance".activity_group_id = ?`, *filter.ActivityGroupID)
	}
	if len(filter.ActivityGroupIDs) > 0 {
		query = query.Where(`"activity_instance".activity_group_id IN (?)`, bun.List(filter.ActivityGroupIDs))
	}
	if len(filter.ActiveGroupIDs) > 0 {
		query = query.Where(`"session".active_group_id IN (?)`, bun.List(filter.ActiveGroupIDs))
	}
	return query
}

// legacyInstanceStateFilters applies the legacy status semantics, the
// planning flags, the order and the page of the filter.
func legacyInstanceStateFilters(query *bun.SelectQuery, filter LegacyInstanceFilter) *bun.SelectQuery {
	switch filter.Status {
	case scheduleModels.InstanceStatusCancelled:
		query = query.Where(`"activity_instance".status = 'cancelled'`)
	case scheduleModels.InstanceStatusPlanned:
		query = query.Where(`"activity_instance".status <> 'cancelled'`).Where(`"session".id IS NULL`)
	case scheduleModels.InstanceStatusActive, scheduleModels.InstanceStatusCompleted:
		query = query.Where(`"activity_instance".status <> 'cancelled'`).Where(`"session".status = ?`, filter.Status)
	}
	if filter.IsSpontaneous != nil {
		query = query.Where(`"activity_instance".is_spontaneous = ?`, *filter.IsSpontaneous)
	}
	if filter.IdempotencyKey != "" {
		query = query.Where(`"activity_instance".idempotency_key = ?`, filter.IdempotencyKey)
	}
	if filter.MaterializedPlanned {
		query = query.Where(`"activity_instance".status <> 'cancelled'`).Where(`"session".id IS NULL`).
			Where(`"activity_instance".activity_group_id IS NOT NULL`).
			Where(`"activity_instance".calendar_period_id IS NOT NULL`).
			Where(`"activity_instance".is_spontaneous = FALSE`)
	}
	if filter.OrderByDateAndTime {
		query = query.OrderExpr(`"activity_instance".date ASC, "activity_instance".start_time ASC`)
	}
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit).Offset(filter.Offset)
	}
	return query
}

func legacyInstance(row legacyInstanceRow) *scheduleModels.ActivityInstance {
	instance := &scheduleModels.ActivityInstance{
		Date: scheduleModels.Date(row.Date), ActivityGroupID: row.ActivityGroupID, CalendarPeriodID: row.CalendarPeriodID,
		Title: row.Title, Description: row.Description, StartTime: row.StartTime, EndTime: row.EndTime, RoomID: row.RoomID,
		RequiredStaff: row.RequiredStaff, Status: row.PlanStatus, ListKind: row.ListKind, IsSpontaneous: row.IsSpontaneous,
		UnderstaffedAck: row.UnderstaffedAck, UnderstaffedNote: row.UnderstaffedNote, CancelReason: row.CancelReason,
		Notes: row.Notes, IdempotencyKey: row.IdempotencyKey, IdempotencyFingerprint: row.IdempotencyFingerprint, CreatedBy: row.CreatedBy,
	}
	if row.SessionStatus != nil && row.PlanStatus != scheduleModels.InstanceStatusCancelled {
		instance.Status = *row.SessionStatus
		instance.ActiveGroupID, instance.StartedBy, instance.StartedAt = row.ActiveGroupID, row.StartedBy, row.StartedAt
		instance.CompletedAt, instance.CompletedBy, instance.ReopenUntil = row.CompletedAt, row.CompletedBy, row.ReopenUntil
		if len(row.CompletionSnapshot) > 0 {
			instance.CompletionSnapshot = json.RawMessage(row.CompletionSnapshot)
		}
	}
	instance.ID, instance.CreatedAt, instance.UpdatedAt = row.ID, row.CreatedAt, row.UpdatedAt
	instance.SetTenantID(row.TenantID)
	return instance
}

// LegacyParticipantFilter selects participants with their attendance the way
// the retained repositories ask for them. Status filters the attendance
// status; ExcludeFinished drops participants of cancelled or ended blocks.
type LegacyParticipantFilter struct {
	IDs                        []int64
	InstanceIDs                []int64
	StudentIDs                 []int64
	Status                     *string
	Date                       *string
	FromDate                   *string
	ToDate                     *string
	CurrentTime                *string
	ExcludeFinished            bool
	OrderByCreated             bool
	OrderByInstanceStudent     bool
	OrderByStudentActivityTime bool
	OrderByActivityDateTime    bool
	Limit                      int
	Offset                     int
}

type legacyParticipantRow struct {
	ID                 int64      `bun:"id"`
	TenantID           int64      `bun:"tenant_id"`
	CreatedAt          time.Time  `bun:"created_at"`
	UpdatedAt          time.Time  `bun:"updated_at"`
	InstanceID         int64      `bun:"instance_id"`
	StudentID          int64      `bun:"student_id"`
	RoomID             *int64     `bun:"room_id"`
	Status             string     `bun:"at_status"`
	Substatus          *string    `bun:"at_substatus"`
	Note               *string    `bun:"at_note"`
	CheckedInAt        *time.Time `bun:"at_checked_in_at"`
	CheckedOutAt       *time.Time `bun:"at_checked_out_at"`
	IsUnplanned        bool       `bun:"at_is_unplanned"`
	NotScheduled       bool       `bun:"at_not_scheduled"`
	ManualStatusAt     *time.Time `bun:"at_manual_status_at"`
	StudentStatusDayID *int64     `bun:"at_student_status_day_id"`
	PickupExceptionID  *int64     `bun:"at_pickup_exception_id"`
	AttendanceUpdated  *time.Time `bun:"at_updated_at"`
}

const legacyParticipantColumns = `"instance_student".id, "instance_student".tenant_id, "instance_student".created_at, "instance_student".updated_at,
	"instance_student".instance_id, "instance_student".student_id, "instance_student".room_id,
	` + attendanceStatus + ` AS at_status, "attendance".substatus AS at_substatus, "attendance".note AS at_note,
	"attendance".checked_in_at AS at_checked_in_at, "attendance".checked_out_at AS at_checked_out_at,
	` + attendanceUnplanned + ` AS at_is_unplanned, ` + attendanceNotScheduled + ` AS at_not_scheduled,
	"attendance".manual_status_at AS at_manual_status_at, "attendance".student_status_day_id AS at_student_status_day_id,
	"attendance".pickup_exception_id AS at_pickup_exception_id, "attendance".updated_at AS at_updated_at`

func legacyParticipantNeedsInstance(filter LegacyParticipantFilter) bool {
	return filter.Date != nil || filter.FromDate != nil || filter.ToDate != nil || filter.CurrentTime != nil ||
		filter.ExcludeFinished || filter.OrderByStudentActivityTime || filter.OrderByActivityDateTime
}

// ListLegacyParticipants lists participants with their attendance as the
// retained repositories read them.
func ListLegacyParticipants(ctx context.Context, db bun.IDB, tenantID int64, filter LegacyParticipantFilter) ([]*scheduleModels.InstanceStudent, error) {
	if tenantID <= 0 {
		return nil, ErrInvalidTenantID
	}
	query := db.NewSelect().TableExpr(`schedule.instance_students AS "instance_student"`).
		ColumnExpr(legacyParticipantColumns).Join(participantAttendanceJoin).
		Where(`"instance_student".tenant_id = ?`, tenantID)
	if legacyParticipantNeedsInstance(filter) {
		query = query.Join(`INNER JOIN schedule.activity_instances AS "activity_instance" ON "activity_instance".id = "instance_student".instance_id AND "activity_instance".tenant_id = "instance_student".tenant_id`).
			Join(instanceSessionJoin)
	}
	query = legacyParticipantOrder(legacyParticipantInstanceFilters(legacyParticipantKeyFilters(query, filter), filter), filter)
	rows := []legacyParticipantRow{}
	if err := query.Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("presence projection: list legacy participants: %w", err)
	}
	result := make([]*scheduleModels.InstanceStudent, 0, len(rows))
	for _, row := range rows {
		result = append(result, legacyParticipant(row))
	}
	return result, nil
}

// legacyParticipantKeyFilters narrows the read by the participant, block
// and student ids and by the attendance status.
func legacyParticipantKeyFilters(query *bun.SelectQuery, filter LegacyParticipantFilter) *bun.SelectQuery {
	if len(filter.IDs) > 0 {
		query = query.Where(`"instance_student".id IN (?)`, bun.List(filter.IDs))
	}
	if len(filter.InstanceIDs) > 0 {
		query = query.Where(`"instance_student".instance_id IN (?)`, bun.List(filter.InstanceIDs))
	}
	if len(filter.StudentIDs) > 0 {
		query = query.Where(`"instance_student".student_id IN (?)`, bun.List(filter.StudentIDs))
	}
	if filter.Status != nil {
		query = query.Where(attendanceStatus+` = ?`, *filter.Status)
	}
	return query
}

// legacyParticipantInstanceFilters narrows the read by the block's date,
// clock and execution.
func legacyParticipantInstanceFilters(query *bun.SelectQuery, filter LegacyParticipantFilter) *bun.SelectQuery {
	if filter.Date != nil {
		query = query.Where(`"activity_instance".date = ?::date`, *filter.Date)
	}
	if filter.FromDate != nil {
		query = query.Where(`"activity_instance".date >= ?::date`, *filter.FromDate)
	}
	if filter.ToDate != nil {
		query = query.Where(`"activity_instance".date <= ?::date`, *filter.ToDate)
	}
	if filter.CurrentTime != nil {
		query = query.Where(`"activity_instance".status <> 'cancelled'`).Where(`COALESCE("session".status, 'planned') <> 'completed'`).
			Where(`"activity_instance".start_time <= ?::time`, *filter.CurrentTime).
			Where(`"activity_instance".end_time > ?::time`, *filter.CurrentTime)
	}
	if filter.ExcludeFinished {
		query = query.Where(`"activity_instance".status <> 'cancelled'`).Where(`COALESCE("session".status, 'planned') <> 'completed'`)
	}
	return query
}

func legacyParticipantOrder(query *bun.SelectQuery, filter LegacyParticipantFilter) *bun.SelectQuery {
	switch {
	case filter.OrderByCreated:
		query = query.OrderExpr(`"instance_student".created_at ASC, "instance_student".id ASC`)
	case filter.OrderByInstanceStudent:
		query = query.OrderExpr(`"instance_student".instance_id ASC, "instance_student".student_id ASC`)
	case filter.OrderByStudentActivityTime:
		query = query.OrderExpr(`"instance_student".student_id ASC, "activity_instance".start_time ASC, "activity_instance".id ASC`)
	case filter.OrderByActivityDateTime:
		query = query.OrderExpr(`"activity_instance".date ASC, "activity_instance".start_time ASC, "instance_student".id ASC`)
	}
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit).Offset(filter.Offset)
	}
	return query
}

// legacyParticipant maps one joined row to the retained DTO. The row's
// updated_at is the later of the plan's and the attendance's.
func legacyParticipant(row legacyParticipantRow) *scheduleModels.InstanceStudent {
	participant := &scheduleModels.InstanceStudent{
		InstanceID: row.InstanceID, StudentID: row.StudentID, RoomID: row.RoomID, Status: row.Status,
		Substatus: row.Substatus, Note: row.Note, CheckedInAt: row.CheckedInAt, CheckedOutAt: row.CheckedOutAt,
		IsUnplanned: row.IsUnplanned, NotScheduled: row.NotScheduled, ManualStatusAt: row.ManualStatusAt,
		StudentStatusDayID: row.StudentStatusDayID, PickupExceptionID: row.PickupExceptionID,
	}
	participant.ID, participant.CreatedAt, participant.UpdatedAt = row.ID, row.CreatedAt, row.UpdatedAt
	if row.AttendanceUpdated != nil && row.AttendanceUpdated.After(participant.UpdatedAt) {
		participant.UpdatedAt = *row.AttendanceUpdated
	}
	participant.SetTenantID(row.TenantID)
	return participant
}
