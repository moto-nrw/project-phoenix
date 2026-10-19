package application

import "context"

func (e engine) DeleteExpiredGuardianInvitations(ctx context.Context) (int, error) {
	deleted, err := e.signInRetention.DeleteExpiredGuardianInvitations(e.attach(ctx))
	return deleted, MapError(err)
}

func (e engine) DeleteExpiredCatererInvitations(ctx context.Context) (int, error) {
	deleted, err := e.signInRetention.DeleteExpiredCatererInvitations(e.attach(ctx))
	return deleted, MapError(err)
}

func (e engine) DeleteExpiredMFAEmailChallenges(ctx context.Context) (int, error) {
	deleted, err := e.signInRetention.DeleteExpiredMFAEmailChallenges(e.attach(ctx))
	return deleted, MapError(err)
}

func (e engine) DeleteExpiredPasskeySessions(ctx context.Context) (int, error) {
	deleted, err := e.signInRetention.DeleteExpiredPasskeySessions(e.attach(ctx))
	return deleted, MapError(err)
}
