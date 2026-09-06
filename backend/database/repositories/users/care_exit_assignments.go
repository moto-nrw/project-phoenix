package users

import (
	"context"
	"time"
)

// CareExitAssignments is the consumer-owned Timetable port. The care-exit
// transaction also records and restores the Care Plan removal ledger.
type CareExitAssignments interface {
	ListOpenStudentAssignments(context.Context, []int64) ([]int64, error)
	LatestStudentAssignmentAttendanceDate(context.Context, int64) (*string, error)
	CloseOpenStudentAssignments(context.Context, []int64, time.Time) (int64, error)
	LockOpenStudentAssignments(context.Context, []int64) error
	ReconnectCareExitAssignmentPickupExceptions(context.Context, []int64, []int64, []CareExitRemoval) error
}
