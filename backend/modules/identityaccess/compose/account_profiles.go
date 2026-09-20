package compose

import "context"

func (e engine) FindAccountProfile(ctx context.Context, accountID int64) (bio, settings string, found bool, err error) {
	value, found, err := e.profiles.FindAccountProfile(ctx, accountID)
	return value.Bio, value.Settings, found, mapError(err)
}

func (e engine) SetAccountBio(ctx context.Context, accountID int64, bio string) error {
	return mapError(e.profiles.SetAccountBio(ctx, accountID, bio))
}
