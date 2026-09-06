package repositories

import (
	"context"

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
