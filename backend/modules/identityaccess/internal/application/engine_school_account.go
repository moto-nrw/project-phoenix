package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

func (e engine) FindSchoolAccountByEmail(ctx context.Context, email string) (identityaccess.Account, error) {
	account, err := e.Service.FindSchoolAccountByEmail(ctx, email)
	return identityaccess.Account(account), MapError(err)
}
