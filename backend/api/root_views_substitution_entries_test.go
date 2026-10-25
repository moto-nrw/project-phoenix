package api

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	education "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type substitutionEntryStub struct {
	name  string
	calls *[]string
}

func (s substitutionEntryStub) Overview(context.Context, education.Caller, education.OverviewQuery) (*education.OverviewResult, error) {
	*s.calls = append(*s.calls, s.name+".overview")
	return &education.OverviewResult{}, nil
}

func (s substitutionEntryStub) Assign(context.Context, education.Caller, education.Assignment) (*education.AssignmentResult, error) {
	*s.calls = append(*s.calls, s.name+".assign")
	return &education.AssignmentResult{}, nil
}

func (s substitutionEntryStub) End(context.Context, education.Caller, education.EndRequest) error {
	*s.calls = append(*s.calls, s.name+".end")
	return nil
}

func TestSubstitutionEntriesRouteTheScheduleTargetToTheWorkflow(t *testing.T) {
	t.Parallel()
	var calls []string
	entries := SubstitutionEntries{Groups: substitutionEntryStub{"groups", &calls}, Schedule: substitutionEntryStub{"schedule", &calls}}
	ctx, caller := context.Background(), education.Caller{}
	day := calendar.NewDate(2026, 10, 26)

	_, err := entries.Overview(ctx, caller, education.OverviewQuery{GroupID: 61})
	require.NoError(t, err)
	_, err = entries.Overview(ctx, caller, education.OverviewQuery{ScheduleFrom: &day, ScheduleTo: &day})
	require.NoError(t, err)
	_, err = entries.Overview(ctx, caller, education.OverviewQuery{IncludeScheduleTargets: true})
	require.NoError(t, err)
	_, err = entries.Assign(ctx, caller, education.Assignment{Type: education.TargetGroupHandover})
	require.NoError(t, err)
	_, err = entries.Assign(ctx, caller, education.Assignment{Type: education.TargetScheduleSubstitution})
	require.NoError(t, err)
	require.NoError(t, entries.End(ctx, caller, education.EndRequest{Type: education.TargetGroupHandover, ID: 71}))
	require.NoError(t, entries.End(ctx, caller, education.EndRequest{Type: education.TargetScheduleSubstitution, ID: 71}))

	assert.Equal(t, []string{
		"groups.overview", "schedule.overview", "schedule.overview",
		"groups.assign", "schedule.assign", "groups.end", "schedule.end",
	}, calls)
}
