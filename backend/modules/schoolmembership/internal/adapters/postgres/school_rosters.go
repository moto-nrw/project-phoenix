package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/adapters/postgres/calendar"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

// NewSchoolRosters reads on the caller's administrative transaction.
func NewSchoolRosters(transaction func(context.Context) (any, bool), admin func(context.Context) bool) *schoolRosters {
	return &schoolRosters{transaction: transaction, admin: admin}
}

type schoolRosters struct {
	transaction func(context.Context) (any, bool)
	admin       func(context.Context) bool
}

// activeStudentMembershipsSQL takes the school set, then the capture day of
// the billing rule, so roster and Stichtagszahl cannot drift apart.
const activeStudentMembershipsSQL = `SELECT tenant_id, student_profile_id, id
FROM users.student_school_memberships
WHERE tenant_id = ANY(?) AND ` + activelyManagedPredicate

// activeStaffIDsSQL takes the school set, then the day twice.
const activeStaffIDsSQL = `SELECT staff.tenant_id, staff.id
FROM users.staff_school_memberships AS staff
WHERE staff.tenant_id = ANY(?) AND staff.deleted_at IS NULL AND NOT EXISTS (
  SELECT 1 FROM users.external_caregivers AS external_caregiver
  WHERE external_caregiver.tenant_id = staff.tenant_id AND external_caregiver.staff_id = staff.id
    AND (external_caregiver.start_date IS NULL OR external_caregiver.start_date <= ?::date)
    AND (external_caregiver.end_date IS NULL OR external_caregiver.end_date >= ?::date))`

func (r schoolRosters) database(ctx context.Context) (bun.IDB, error) {
	transaction, ok := r.transaction(ctx)
	if !ok {
		return nil, errors.New("school membership: transaction is required")
	}
	if !r.admin(ctx) {
		return nil, errors.New("school membership: school rosters need the administrative transaction")
	}
	switch tx := transaction.(type) {
	case bun.Tx:
		return tx, nil
	case *bun.Tx:
		if tx != nil {
			return *tx, nil
		}
	}
	return nil, fmt.Errorf("school membership postgres: unsupported transaction %T", transaction)
}

func (r schoolRosters) ActiveStudentMembershipsByTenant(ctx context.Context, schools []int64, at time.Time) (map[int64]map[int64]int64, error) {
	db, err := r.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		TenantID         int64 `bun:"tenant_id"`
		StudentProfileID int64 `bun:"student_profile_id"`
		ID               int64 `bun:"id"`
	}
	if err := db.NewRaw(activeStudentMembershipsSQL, pgdialect.Array(schools), calendar.DateFromTime(at)).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("school membership postgres: active student memberships: %w", err)
	}
	result := make(map[int64]map[int64]int64, len(schools))
	for _, row := range rows {
		if result[row.TenantID] == nil {
			result[row.TenantID] = map[int64]int64{}
		}
		result[row.TenantID][row.StudentProfileID] = row.ID
	}
	return result, nil
}

func (r schoolRosters) ActiveStaffIDsByTenant(ctx context.Context, schools []int64, at time.Time) (map[int64][]int64, error) {
	db, err := r.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		TenantID int64 `bun:"tenant_id"`
		ID       int64 `bun:"id"`
	}
	day := calendar.DateFromTime(at)
	if err := db.NewRaw(activeStaffIDsSQL, pgdialect.Array(schools), day, day).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("school membership postgres: active staff: %w", err)
	}
	result := make(map[int64][]int64, len(schools))
	for _, row := range rows {
		result[row.TenantID] = append(result[row.TenantID], row.ID)
	}
	return result, nil
}
