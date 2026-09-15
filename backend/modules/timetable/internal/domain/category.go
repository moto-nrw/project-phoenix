package domain

import (
	"errors"
	"time"
)

const CategoryNameActiveIndex = "idx_categories_tenant_name_active"

var (
	ErrCategoryNotFound        = errors.New("category not found")
	ErrCategoryNameConflict    = errors.New("category name conflict")
	ErrUnknownCategoryIDs      = errors.New("unknown category IDs")
	ErrSystemCategoryProtected = errors.New("system category protected")
	ErrSystemCategoryName      = errors.New("system category name reserved")
	ErrCategoryArchived        = errors.New("category archived")
)

type Category struct {
	ID          int64
	TenantID    int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Name        string
	Description string
	Color       string
	IsSystem    bool
	ShiftTypeID *int64
	ArchivedAt  *time.Time
}

func (c Category) IsArchived() bool { return c.ArchivedAt != nil }

type CategoryFields struct {
	Name        string
	Description string
	Color       string
	IsSystem    bool
}

type CareExitEnrollment struct {
	ID                       int64
	TenantID                 int64
	StudentID                int64
	ActivityGroupID          int64
	ValidFrom                string
	ValidUntil               *string
	CalendarPeriodID         *int64
	EnrollmentRequestChildID *int64
	SelectedWeekdays         []int
	AttendanceStatus         *string
	Weekday                  *int
}

type CareExitEnrollmentCap struct {
	TenantID           int64
	StudentID          int64
	ID                 int64
	PreviousValidUntil *string
}

type CareExitEnrollmentChanges struct {
	Deleted []CareExitEnrollment
	Capped  []CareExitEnrollmentCap
}

type CareExitEnrollmentRemoval struct {
	CareExitEnrollment
	WasDeleted         bool
	PreviousValidUntil *string
}

type OperationStats struct {
	Queries                      int64
	Rows                         int64
	DuplicatePreventionConflicts int64
	StatementDuration            time.Duration
}

func (s *OperationStats) Add(other OperationStats) {
	s.Queries += other.Queries
	s.Rows += other.Rows
	s.DuplicatePreventionConflicts += other.DuplicatePreventionConflicts
	s.StatementDuration += other.StatementDuration
}
