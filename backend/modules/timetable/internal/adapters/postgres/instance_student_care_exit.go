package postgres

import (
	"encoding/json"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

type careExitRosterSnapshot struct {
	TenantID           int64      `json:"tenant_id"`
	StudentID          int64      `json:"student_id"`
	InstanceID         int64      `json:"instance_id"`
	RoomID             *int64     `json:"room_id"`
	Status             string     `json:"status"`
	Substatus          *string    `json:"substatus"`
	Note               *string    `json:"note"`
	IsUnplanned        bool       `json:"is_unplanned"`
	NotScheduled       bool       `json:"not_scheduled"`
	ManualStatusAt     *time.Time `json:"manual_status_at"`
	StudentStatusDayID *int64     `json:"student_status_day_id"`
	PickupExceptionID  *int64     `json:"pickup_exception_id"`
}

const careExitRosterRecordset = `jsonb_to_recordset(?::jsonb) AS rm(
 tenant_id bigint, student_id bigint, instance_id bigint,
 room_id bigint, status text, substatus text, note text,
 is_unplanned boolean, not_scheduled boolean, manual_status_at timestamptz,
 student_status_day_id bigint, pickup_exception_id bigint
)`

func encodeCareExitRoster(removals []domain.InstanceStudent) (string, error) {
	rows := make([]careExitRosterSnapshot, 0, len(removals))
	for _, row := range removals {
		rows = append(rows, careExitRosterSnapshot{
			TenantID: row.TenantID, StudentID: row.StudentID, InstanceID: row.InstanceID,
			RoomID: row.RoomID, Status: row.Status, Substatus: row.Substatus, Note: row.Note,
			IsUnplanned: row.IsUnplanned, NotScheduled: row.NotScheduled, ManualStatusAt: row.ManualStatusAt,
			StudentStatusDayID: row.StudentStatusDayID, PickupExceptionID: row.PickupExceptionID,
		})
	}
	encoded, err := json.Marshal(rows)
	return string(encoded), err
}
