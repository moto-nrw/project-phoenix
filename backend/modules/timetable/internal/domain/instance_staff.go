package domain

import (
	"errors"
	"time"
)

var ErrInstanceStaffNotFound = errors.New("instance staff assignment not found")

const InstanceStaffUniqueConstraint = "unique_instance_staff"

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

type InstanceStaffFields struct {
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
