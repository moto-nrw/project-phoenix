package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

func (s *Service) ListGuardianPortalContacts(ctx context.Context, guardianIDs, studentIDs []int64) ([]peopledirectory.GuardianPortalContact, error) {
	if len(guardianIDs) == 0 && len(studentIDs) == 0 {
		return []peopledirectory.GuardianPortalContact{}, nil
	}

	values, err := s.guardianRows.ListPortalContacts(ctx, guardianIDs, studentIDs)
	if err != nil {
		return nil, MapError(err)
	}
	result := make([]peopledirectory.GuardianPortalContact, 0, len(values))
	for _, value := range values {
		result = append(result, peopledirectory.GuardianPortalContact{
			GuardianProfileID: value.GuardianProfileID, TenantID: value.TenantID, AccountID: value.AccountID,
			FirstName: value.FirstName, LastName: value.LastName, Email: value.Email,
			PortalLocale: value.PortalLocale, StudentID: value.StudentID, PortalPermission: value.PortalPermission,
		})
	}
	return result, nil
}
