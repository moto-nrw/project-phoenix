package compose

import "context"

func (e engine) SeedGuardianAccount(ctx context.Context, email, passwordHash string) (int64, bool, error) {
	id, reused, err := e.service.SeedGuardianAccount(ctx, email, passwordHash)
	return id, reused, authenticationError(err)
}
