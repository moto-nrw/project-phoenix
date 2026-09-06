package timetable

import (
	"context"
	"time"
)

type InstanceStaff struct {
	ID            int64
	TenantID      int64
	CreatedAt     time.Time
	UpdatedAt     time.Time
	InstanceID    int64
	StaffID       int64
	RoomID        *int64
	IsPrimary     bool
	IsSubstitute  bool
	IsAbsent      bool
	AbsenceReason *string
	SickAbsenceID *int64
}

type InstanceStaffInput struct {
	InstanceID    int64
	StaffID       int64
	RoomID        *int64
	IsPrimary     bool
	IsSubstitute  bool
	IsAbsent      bool
	AbsenceReason *string
	SickAbsenceID *int64
}

type InstanceStaffFilter struct {
	IDs                       []int64
	InstanceIDs               []int64
	StaffIDs                  []int64
	SickAbsenceID             *int64
	Date                      *string
	FromDate                  *string
	ToDate                    *string
	OrderByCreated            bool
	OrderByInstanceAndCreated bool
	OrderByActivityTime       bool
	OrderByActivityDateTime   bool
	Limit                     int
	Offset                    int
}

type InstanceStaffQuery interface {
	FindInstanceStaff(context.Context, int64) (InstanceStaff, error)
	ListInstanceStaff(context.Context, InstanceStaffFilter) ([]InstanceStaff, error)
	CountNonAbsentInstanceStaff(context.Context, []int64) (map[int64]int, error)
}

type InstanceStaffCommand interface {
	CreateInstanceStaff(context.Context, InstanceStaffInput) (InstanceStaff, error)
	UpdateInstanceStaff(context.Context, int64, InstanceStaffInput) (InstanceStaff, error)
	PatchInstanceStaff(context.Context, int64, InstanceStaffInput, []string) (int64, error)
	DeleteInstanceStaff(context.Context, int64) error
	DeleteInstanceStaffByInstance(context.Context, int64) error
	DeleteUpcomingInstanceStaff(context.Context, int64, string) (int64, error)
}

type InstanceStaffCapability interface {
	InstanceStaffQuery
	InstanceStaffCommand
}
