package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/services/users"
)

type caregiverIdentity struct{ *identityaccess.Module }

func (c caregiverIdentity) FindCaregiverAccount(ctx context.Context, accountID int64) (*users.CaregiverAccount, error) {
	account, err := c.FindAccountMetadata(ctx, accountID)
	if err != nil {
		return nil, err
	}
	return &users.CaregiverAccount{ID: account.ID, Email: account.Email}, nil
}
