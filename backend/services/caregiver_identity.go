package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
)

type caregiverIdentity struct{ *identityaccess.Module }

func (c caregiverIdentity) FindCaregiverAccount(ctx context.Context, accountID int64) (*peopleCompose.CaregiverAccount, error) {
	account, err := c.FindAccountMetadata(ctx, accountID)
	if err != nil {
		return nil, err
	}
	return &peopleCompose.CaregiverAccount{ID: account.ID, Email: account.Email}, nil
}
