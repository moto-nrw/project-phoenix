package ports

import (
	"time"
)

type PickupExceptionProjection struct {
	ID            int64
	StudentID     int64
	ExceptionDate string
	ExcusedFrom   *time.Time
	ExcusedAuto   bool
}

type PickupExceptionFilter struct {
	IDs        []int64
	StudentIDs []int64
	Date       string
	From       string
}

type StudentStatusDayProjection struct {
	ID        int64
	StudentID int64
	Date      string
	Status    string
}

type StudentStatusDayFilter struct {
	IDs        []int64
	StudentIDs []int64
	Date       string
	From       string
	ActiveOnly bool
	LatestOnly bool
}
