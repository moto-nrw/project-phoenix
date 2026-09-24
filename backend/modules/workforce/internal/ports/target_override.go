package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
)

// TargetOverrideStore is the persistence port over
// config.staff_target_overrides. A missing row reports found=false.
type TargetOverrideStore interface {
	// ListStaffTargetOverrides returns the ranges of the queried staff
	// members ordered by staff and start date; From/To narrow to ranges
	// touching that window.
	ListStaffTargetOverrides(context.Context, domain.TargetOverrideQuery) ([]domain.StaffTargetOverride, domain.OperationStats, error)
	FindStaffTargetOverride(ctx context.Context, staffID, id int64) (domain.StaffTargetOverride, bool, domain.OperationStats, error)
	CreateStaffTargetOverride(context.Context, domain.StaffTargetOverride) (domain.StaffTargetOverride, domain.OperationStats, error)
	DeleteStaffTargetOverride(ctx context.Context, staffID, id int64) (bool, domain.OperationStats, error)
}

// StatutoryHolidays is the consumer-owned port over the School Calendar: the
// tenant's statutory holidays in [from, to] as a date set. A Sonderarbeitszeit
// never sets a target on one of them.
type StatutoryHolidays func(ctx context.Context, from, to string) (map[string]bool, error)
