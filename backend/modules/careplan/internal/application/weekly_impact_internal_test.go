package application

import (
	"context"
	"testing"

	"github.com/moto-ogs/project-phoenix/modules/careplan/carerequests"
	"github.com/stretchr/testify/require"
)

type fixedWeeklyImpact string

func (f fixedWeeklyImpact) WeeklyImpactToken(context.Context, carerequests.Request) (string, error) {
	return string(f), nil
}

// #6843: only a decision that names the diff it saw is pinned to it.
func TestVerifyWeeklyImpact(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	request := weeklyCareRequest()
	shown, stale := "shown", "stale"
	require.NoError(t, verifyWeeklyImpact(ctx, request, carerequests.DecideInput{}), "bulk and conflict decisions carry no token")
	require.NoError(t, verifyWeeklyImpact(ctx, request, carerequests.DecideInput{ExpectedImpactToken: &shown, WeeklyImpact: fixedWeeklyImpact(shown)}))
	require.ErrorIs(t, verifyWeeklyImpact(ctx, request, carerequests.DecideInput{ExpectedImpactToken: &stale, WeeklyImpact: fixedWeeklyImpact(shown)}), carerequests.ErrWeeklyPlanChanged)
	require.Error(t, verifyWeeklyImpact(ctx, request, carerequests.DecideInput{ExpectedImpactToken: &shown}), "a token without a check fails instead of approving unpinned")
}
