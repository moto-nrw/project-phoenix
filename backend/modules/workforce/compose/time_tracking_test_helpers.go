//go:build testhooks

package compose

import (
	"context"
	"errors"

	postgresNative "github.com/moto-nrw/project-phoenix/modules/workforce/internal/adapters/postgres"
	workforcePostgres "github.com/moto-nrw/project-phoenix/modules/workforce/internal/adapters/workforcepostgres"
	"github.com/uptrace/bun"
)

type (
	StaffAbsenceTypeAllowanceRepository       = workforcePostgres.StaffAbsenceTypeAllowanceRepository
	StaffAbsenceTypeAllowanceChangeRepository = workforcePostgres.StaffAbsenceTypeAllowanceChangeRepository
)

func NewStaffAbsenceTypeAllowanceRepository(db *bun.DB) *StaffAbsenceTypeAllowanceRepository {
	return workforcePostgres.NewStaffAbsenceTypeAllowanceRepository(staffAllowanceTestDatabase(db))
}

func NewStaffAbsenceTypeAllowanceChangeRepository(db *bun.DB) *StaffAbsenceTypeAllowanceChangeRepository {
	return workforcePostgres.NewStaffAbsenceTypeAllowanceChangeRepository(staffAllowanceTestDatabase(db))
}

func staffAllowanceTestDatabase(db *bun.DB) workforcePostgres.Database {
	return func(ctx context.Context) (bun.IDB, int64, error) {
		connection, scope, err := postgresNative.DatabaseRuntime(db)(ctx)
		if err != nil {
			return nil, 0, err
		}
		if scope.AcrossSchools {
			return nil, 0, errors.New("staff allowance: tenant is required")
		}
		return connection, scope.TenantID, nil
	}
}
