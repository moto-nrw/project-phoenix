package domain

import (
	"errors"
	"time"
)

var (
	ErrAppointmentNotFound          = errors.New("appointment not found")
	ErrAppointmentLifecycleConflict = errors.New("appointment changed by a concurrent lifecycle transition")
)

type Date string

type Appointment struct {
	ID                 int64
	TenantID           int64
	CreatedAt          time.Time
	UpdatedAt          time.Time
	OrganizerStaffID   int64
	Title              string
	Description        *string
	Location           *string
	StartDate          Date
	EndDate            Date
	StartTime          time.Time
	EndTime            time.Time
	AllDay             bool
	DeliveryMode       string
	OverviewVisibility string
	CancelledAt        *time.Time
	DeletedAt          *time.Time
	NotifyGuardians    bool
	Revision           int
}

type AppointmentFields struct {
	OrganizerStaffID   int64
	Title              string
	Description        *string
	Location           *string
	StartDate          Date
	EndDate            Date
	StartTime          time.Time
	EndTime            time.Time
	AllDay             bool
	DeliveryMode       string
	OverviewVisibility string
	NotifyGuardians    bool
}

type AppointmentTarget struct {
	ID            int64
	TenantID      int64
	CreatedAt     time.Time
	UpdatedAt     time.Time
	AppointmentID int64
	TargetType    string
	TargetID      *int64
	TargetValue   *string
}

type AppointmentTargetFields struct {
	TargetType  string
	TargetID    *int64
	TargetValue *string
}

type OperationStats struct {
	Queries           int64
	Rows              int64
	StatementDuration time.Duration
}

func (s *OperationStats) Add(other OperationStats) {
	s.Queries += other.Queries
	s.Rows += other.Rows
	s.StatementDuration += other.StatementDuration
}
