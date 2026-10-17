package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

func (e engine) FindAccountMetadata(ctx context.Context, accountID int64) (identityaccess.AccountMetadata, error) {
	value, err := e.profiles.FindAccountMetadata(ctx, accountID)
	return identityaccess.AccountMetadata(value), MapError(err)
}

func (e engine) SetAccountUsername(ctx context.Context, accountID int64, username string) error {
	return MapError(e.profiles.SetAccountUsername(ctx, accountID, username))
}

func (e engine) SetAccountAvatar(ctx context.Context, accountID int64, avatar string) error {
	return MapError(e.profiles.SetAccountAvatar(ctx, accountID, avatar))
}

func (e engine) FindAccountProfile(ctx context.Context, accountID int64) (bio, settings string, found bool, err error) {
	value, found, err := e.profiles.FindAccountProfile(ctx, accountID)
	return value.Bio, value.Settings, found, MapError(err)
}

func (e engine) SetAccountBio(ctx context.Context, accountID int64, bio string) error {
	return MapError(e.profiles.SetAccountBio(ctx, accountID, bio))
}
