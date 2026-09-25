package services

import (
	"context"

	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

type enrollmentGuardianDirectory struct{ guardians peopledirectory.GuardianQuery }

func (d enrollmentGuardianDirectory) ListGuardiansByAccount(ctx context.Context, accountIDs []int64) ([]enrollmentCompose.DirectoryGuardian, error) {
	guardians, err := d.guardians.ListGuardiansByAccount(ctx, accountIDs)
	return toEnrollmentGuardians(guardians), err
}

func (d enrollmentGuardianDirectory) ListGuardiansByID(ctx context.Context, ids []int64) ([]enrollmentCompose.DirectoryGuardian, error) {
	guardians, err := d.guardians.ListGuardiansByID(ctx, ids)
	return toEnrollmentGuardians(guardians), err
}

func (d enrollmentGuardianDirectory) CountGuardianLinks(ctx context.Context, ids []int64) (map[int64]int, error) {
	return d.guardians.CountGuardianLinks(ctx, ids)
}

func toEnrollmentGuardians(guardians []peopledirectory.Guardian) []enrollmentCompose.DirectoryGuardian {
	result := make([]enrollmentCompose.DirectoryGuardian, 0, len(guardians))
	for _, guardian := range guardians {
		result = append(result, enrollmentCompose.DirectoryGuardian{ID: guardian.ID, AccountID: guardian.AccountID})
	}
	return result
}
