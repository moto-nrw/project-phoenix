package repositories

import (
	"context"

	parentRepo "github.com/moto-nrw/project-phoenix/database/repositories/parent"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

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
