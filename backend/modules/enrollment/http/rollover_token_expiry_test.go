package enrollmenthttp_test

import (
	"errors"
	"testing"
	"time"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentAPI "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestRolloverStatusToken_UsesConfiguredLifetime(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		mode string
		days int
	}{
		{capability.PhaseRolloverModeOptIn, 14},
		{capability.PhaseRolloverModeOptOut, 365},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			testpkg.OwnCtx(t)
			env := setupRolloverTest(t)
			ctx := testpkg.Ctx(t)
			source := seedApprovedChild(t, env, env.sourcePhase.ID, "Anna", "Beispiel", "expiry@example.test", "Lina", "Beispiel", 1)
			env.settings.intValues[settingKeyStatusTokenTTLDays] = tc.days
			before := time.Now()
			result, err := env.rolloverSvc.CreatePhaseFromSource(ctx, validRolloverRequest(env, tc.mode, true))
			after := time.Now()
			require.NoError(t, err)
			children, err := testpkg.TenantQuery2(t, ctx, env.db, env.repos.Enrollment().ChildrenByPhaseStatuses, result.Phase.ID,
				[]string{capability.ChildStatusPendingRenewal, capability.ChildStatusAutoRenewed})
			require.NoError(t, err)
			require.Len(t, children, 1)
			request, err := enrollmentAPI.ReadOwnerRequestForTest(ctx, env.repos.Enrollment(), children[0].RequestID)
			require.NoError(t, err)
			require.NotNil(t, request.StatusTokenExpires, "renewal status links must expire")
			lifetime := time.Duration(tc.days) * 24 * time.Hour
			require.WithinRange(t, *request.StatusTokenExpires, before.Add(lifetime), after.Add(lifetime))
			original, err := enrollmentAPI.ReadOwnerRequestForTest(ctx, env.repos.Enrollment(), source.RequestID)
			require.NoError(t, err)
			require.NotEqual(t, original.StatusToken, request.StatusToken)
		})
	}
}

func TestRolloverStatusToken_RejectsInvalidLifetimeAtomically(t *testing.T) {
	t.Parallel()
	failure := errors.New("token lifetime unavailable")
	for _, tc := range []struct {
		name string
		days int
		err  error
	}{
		{name: "read failure", err: failure},
		{name: "zero", days: 0},
		{name: "negative", days: -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := testpkg.OwnCtx(t)
			env := setupRolloverTest(t)
			seedApprovedChild(t, env, env.sourcePhase.ID, "Anna", "Beispiel", "expiry@example.test", "Lina", "Beispiel", 1)
			env.settings.intValues[settingKeyStatusTokenTTLDays] = tc.days
			env.settings.intErrors[settingKeyStatusTokenTTLDays] = tc.err
			result, err := env.rolloverSvc.CreatePhaseFromSource(ctx, validRolloverRequest(env, capability.PhaseRolloverModeOptIn, true))
			require.Error(t, err)
			require.Nil(t, result)
			if tc.err != nil {
				require.ErrorIs(t, err, tc.err)
			} else {
				require.ErrorContains(t, err, "enrollment.status_token_ttl_days must be positive")
			}
			phases, err := testpkg.TenantQuery0(t, ctx, env.db, env.repos.Enrollment().Phases)
			require.NoError(t, err)
			require.Len(t, phases, 1, "a lifetime failure must roll back the follow-up phase")
			require.Equal(t, env.sourcePhase.ID, phases[0].ID)
		})
	}
}
