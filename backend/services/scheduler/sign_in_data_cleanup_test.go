package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeSignInDataCleaner struct {
	calls           []string
	operatorCutoffs []time.Time
}

func (f *fakeSignInDataCleaner) DeleteExpiredGuardianInvitations(context.Context) (int, error) {
	f.calls = append(f.calls, "guardian invitations")
	return 1, nil
}

func (f *fakeSignInDataCleaner) DeleteExpiredMFAEmailChallenges(context.Context) (int, error) {
	f.calls = append(f.calls, "mfa email challenges")
	return 2, nil
}

func (f *fakeSignInDataCleaner) DeleteExpiredPasskeySessions(context.Context) (int, error) {
	f.calls = append(f.calls, "passkey sessions")
	return 3, nil
}

func (f *fakeSignInDataCleaner) DeleteExpiredOperatorSessions(_ context.Context, now time.Time) (int, error) {
	f.calls = append(f.calls, "operator sessions")
	f.operatorCutoffs = append(f.operatorCutoffs, now)
	return 4, nil
}

// The hourly token cleanup deletes the short-lived sign-in data of #3954,
// and the expired operator sessions as soon as they expire, like account
// sessions.
func TestTokenCleanupRunsTheSignInDataRetention(t *testing.T) {
	t.Parallel()
	cleaner := &fakeSignInDataCleaner{}

	jobs := signInDataCleanupJobs(cleaner)
	before := time.Now()
	deleted := 0
	for _, job := range jobs {
		count, err := job.Run(context.Background())
		require.NoError(t, err)
		deleted += count
	}

	assert.Equal(t, []string{"guardian invitations", "mfa email challenges", "passkey sessions", "operator sessions"}, cleaner.calls)
	assert.Equal(t, 10, deleted)
	require.Len(t, cleaner.operatorCutoffs, 1)
	assert.WithinDuration(t, before, cleaner.operatorCutoffs[0], time.Minute, "operator sessions go at their expiry")
}

func TestTokenCleanupRegistersTheSignInDataRetention(t *testing.T) {
	t.Parallel()
	deps := minimalWorkerDependencies(t)
	cleaner := &fakeOperatorInvitationCleaner{}
	deps.OperatorInvitationCleanup = cleaner

	scheduler := newScheduler(deps)

	descriptions := make([]string, 0, len(scheduler.cleanupJobs))
	for _, job := range scheduler.cleanupJobs {
		descriptions = append(descriptions, job.Description)
	}
	assert.Subset(t, descriptions, []string{
		"Operator invitation token cleanup",
		"Guardian invitation cleanup", "MFA email code cleanup", "Passkey session cleanup", "Operator session cleanup",
	})
}
