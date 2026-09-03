package repositories

import (
	"context"

	enrollmentRepo "github.com/moto-nrw/project-phoenix/database/repositories/enrollment"
	parentRepo "github.com/moto-nrw/project-phoenix/database/repositories/parent"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// bindGuardianDirectories hands the People Directory to every legacy
// repository that used to read users.guardian_profiles or
// users.students_guardians through a foreign join (#2663). Each repository
// declares the narrow projection it needs; the adapters below map the
// owner's rows onto it.
func (f *Factory) bindGuardianDirectories(guardians peopledirectory.GuardianQuery) {
	if repo, ok := f.ParentChild.(*parentRepo.ChildRepository); ok {
		repo.BindGuardianDirectory(parentGuardianDirectory{guardians})
	}
	if repo, ok := f.ParentEnrollablePhase.(*parentRepo.EnrollablePhaseRepository); ok {
		repo.BindGuardianDirectory(parentGuardianDirectory{guardians})
	}
	if repo, ok := f.ParentEnrollmentRequest.(*parentRepo.EnrollmentRequestRepository); ok {
		repo.BindGuardianDirectory(parentGuardianDirectory{guardians})
	}
	if repo, ok := f.EnrollmentDeletion.(*enrollmentRepo.DeletionRepository); ok {
		repo.BindGuardianDirectory(enrollmentGuardianDirectory{guardians})
	}
}

type parentGuardianDirectory struct{ guardians peopledirectory.GuardianQuery }

func (d parentGuardianDirectory) ListGuardianLinksByAccount(ctx context.Context, accountID int64) ([]parentRepo.DirectoryGuardianLink, error) {
	links, err := d.guardians.ListGuardianLinksByAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	result := make([]parentRepo.DirectoryGuardianLink, 0, len(links))
	for _, link := range links {
		result = append(result, parentRepo.DirectoryGuardianLink{
			TenantID: link.TenantID, StudentID: link.StudentID, GuardianProfileID: link.GuardianProfileID, Permissions: link.Permissions,
		})
	}
	return result, nil
}

type enrollmentGuardianDirectory struct{ guardians peopledirectory.GuardianQuery }

func (d enrollmentGuardianDirectory) ListGuardiansByAccount(ctx context.Context, accountIDs []int64) ([]enrollmentRepo.DirectoryGuardian, error) {
	guardians, err := d.guardians.ListGuardiansByAccount(ctx, accountIDs)
	return toEnrollmentGuardians(guardians), err
}

func (d enrollmentGuardianDirectory) ListGuardiansByID(ctx context.Context, ids []int64) ([]enrollmentRepo.DirectoryGuardian, error) {
	guardians, err := d.guardians.ListGuardiansByID(ctx, ids)
	return toEnrollmentGuardians(guardians), err
}

func (d enrollmentGuardianDirectory) CountGuardianLinks(ctx context.Context, ids []int64) (map[int64]int, error) {
	return d.guardians.CountGuardianLinks(ctx, ids)
}

func toEnrollmentGuardians(guardians []peopledirectory.Guardian) []enrollmentRepo.DirectoryGuardian {
	result := make([]enrollmentRepo.DirectoryGuardian, 0, len(guardians))
	for _, guardian := range guardians {
		result = append(result, enrollmentRepo.DirectoryGuardian{ID: guardian.ID, AccountID: guardian.AccountID})
	}
	return result
}
