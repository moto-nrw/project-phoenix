package timetable

import (
	"context"
	"time"
)

// PlannedSupervisor is the owner view of one activities.supervisors row.
// Calendar dates stay as YYYY-MM-DD strings at the module boundary.
type PlannedSupervisor struct {
	ID               int64     `json:"id"`
	TenantID         int64     `json:"tenant_id"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	StaffID          int64     `json:"staff_id"`
	GroupID          int64     `json:"group_id"`
	IsPrimary        bool      `json:"is_primary"`
	ValidFrom        string    `json:"valid_from"`
	ValidUntil       *string   `json:"valid_until,omitempty"`
	CalendarPeriodID *int64    `json:"calendar_period_id,omitempty"`
	Weekday          *int      `json:"weekday,omitempty"`
}

type PlannedSupervisorInput struct {
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
	ID           int64  `json:"id"`
	ActivityID   int64  `json:"activity_id"`
	ActivityName string `json:"activity_name"`
	IsPrimary    bool   `json:"is_primary"`
}

type PlannedSupervisorQuery interface {
	FindPlannedSupervisor(context.Context, int64) (PlannedSupervisor, error)
	ListPlannedSupervisors(context.Context, PlannedSupervisorFilter) ([]PlannedSupervisor, error)
	ListPlannedSupervisionBlockers(context.Context, int64) ([]PlannedSupervisionBlocker, error)
}

type PlannedSupervisorCommand interface {
	CreatePlannedSupervisor(context.Context, PlannedSupervisorInput) (PlannedSupervisor, error)
	UpdatePlannedSupervisor(context.Context, int64, PlannedSupervisorInput) (PlannedSupervisor, error)
	DeletePlannedSupervisor(context.Context, int64) error
	SetPrimaryPlannedSupervisor(context.Context, int64) error
	DeletePlannedSupervisorsByStaff(context.Context, int64) (int64, error)
	CapActivePlannedSupervisors(context.Context, int64, string) (int64, error)
	SetPlannedSupervisorValidUntil(context.Context, int64, string) error
	CloseOpenPlannedSupervisors(context.Context, int64, *int64, string) error
}

type PlannedSupervisorCapability interface {
	PlannedSupervisorQuery
	PlannedSupervisorCommand
}
