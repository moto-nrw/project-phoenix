package owners

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
)

type caregiverIdentity struct{ *identityaccess.Module }

func (c caregiverIdentity) FindCaregiverAccount(ctx context.Context, accountID int64) (*peoplePorts.CaregiverAccount, error) {
	account, err := c.FindAccountMetadata(ctx, accountID)
	if err != nil {
		return nil, err
	}
	return &peoplePorts.CaregiverAccount{ID: account.ID, Email: account.Email}, nil
}
