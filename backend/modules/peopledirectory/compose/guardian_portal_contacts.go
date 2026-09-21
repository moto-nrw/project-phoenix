package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

func (e engine) ListGuardianPortalContacts(ctx context.Context, guardianIDs, studentIDs []int64) ([]peopledirectory.GuardianPortalContact, error) {
	values, err := e.guardians.ListPortalContacts(ctx, guardianIDs, studentIDs)
	if err != nil {
		return nil, mapError(err)
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
