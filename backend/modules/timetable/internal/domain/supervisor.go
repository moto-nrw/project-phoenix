package domain

import (
	"errors"
	"time"
)

var ErrPlannedSupervisorNotFound = errors.New("planned supervisor not found")

type PlannedSupervisor struct {
	ID               int64
	TenantID         int64
	CreatedAt        time.Time
	UpdatedAt        time.Time
	StaffID          int64
	GroupID          int64
	IsPrimary        bool
	ValidFrom        string
	ValidUntil       *string
	CalendarPeriodID *int64
	Weekday          *int
}

type PlannedSupervisorFields struct {
	StaffID          int64
	GroupID          int64
	IsPrimary        bool
	ValidFrom        string
	ValidUntil       *string
	CalendarPeriodID *int64
	Weekday          *int
}

type PlannedSupervisorFilter struct {
	GroupIDs []int64
	StaffID  *int64
}

type PlannedSupervisionBlocker struct {
	ID           int64
	ActivityID   int64
	ActivityName string
	IsPrimary    bool
}
