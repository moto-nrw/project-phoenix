package domain

import "time"

type CareExitRosterRow struct {
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
