package api

import (
	"errors"
	"sync"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestConcurrentMFACodeRequestsRespectIdentityLimit(t *testing.T) {
	t.Parallel()
	for _, operator := range []bool{false, true} {
		t.Run(map[bool]string{false: "account", true: "operator"}[operator], func(t *testing.T) {
			testpkg.OwnTenant(t)
			db := testpkg.SetupTestDB(t)
			mail := testpkg.NewCapturingMailer()
			module, err := NewAuthTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), WithAuthTestMailer(mail))
			require.NoError(t, err)
			var start func() error
			if operator {
				identity := testpkg.CreateTestOperator(t, db)
				start = func() error {
					_, err := module.OperatorMFA.StartOperatorMFAChallenge(t.Context(), identity.ID, nil)
					return err
				}
			} else {
				account := testpkg.CreateTestAccount(t, db, "challenge-race")
				start = func() error {
					_, err := module.MFA.StartMFAChallenge(t.Context(), account.ID, testpkg.Tenant(t), identityaccess.MFAChallengeScopeTenant, nil)
					return err
				}
			}
			const workers = 8
			results := make(chan error, workers)
			var group sync.WaitGroup
			for range workers {
				group.Go(func() { results <- start() })
			}
			group.Wait()
			close(results)
			sent := 0
			for err := range results {
				if err == nil {
					sent++
				} else {
					require.True(t, errors.Is(err, identityaccess.ErrMFARateLimited))
				}
			}
			require.Equal(t, 3, sent)
			require.Equal(t, 3, len(mail.Messages()))
		})
	}
}
