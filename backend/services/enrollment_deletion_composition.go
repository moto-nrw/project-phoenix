package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/services/enrollment"
)

type enrollmentGuardianDirectory struct{ guardians peopledirectory.GuardianQuery }

func (d enrollmentGuardianDirectory) ListGuardiansByAccount(ctx context.Context, accountIDs []int64) ([]enrollment.DirectoryGuardian, error) {
	guardians, err := d.guardians.ListGuardiansByAccount(ctx, accountIDs)
	return toEnrollmentGuardians(guardians), err
}

func (d enrollmentGuardianDirectory) ListGuardiansByID(ctx context.Context, ids []int64) ([]enrollment.DirectoryGuardian, error) {
	guardians, err := d.guardians.ListGuardiansByID(ctx, ids)
	return toEnrollmentGuardians(guardians), err
}

func (d enrollmentGuardianDirectory) CountGuardianLinks(ctx context.Context, ids []int64) (map[int64]int, error) {
	return d.guardians.CountGuardianLinks(ctx, ids)
}

func toEnrollmentGuardians(guardians []peopledirectory.Guardian) []enrollment.DirectoryGuardian {
	result := make([]enrollment.DirectoryGuardian, 0, len(guardians))
	for _, guardian := range guardians {
		result = append(result, enrollment.DirectoryGuardian{ID: guardian.ID, AccountID: guardian.AccountID})
	}
	return result
}
