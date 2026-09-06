package domain

import "time"

type CompletionAttendance struct {
	RowID              int64
	Status             string
	Substatus          *string
	Note               *string
	CheckedInAt        *time.Time
	CheckedOutAt       *time.Time
	NotScheduled       bool
	StudentStatusDayID *int64
	PickupExceptionID  *int64
}
