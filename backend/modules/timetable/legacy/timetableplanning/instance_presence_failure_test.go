package timetableplanning_test

import (
	"context"
	"errors"
	"testing"

	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable/legacy/timetableplanning"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failedConflictPresence struct {
	timetableplanning.InstancePresence
	err   error
	reads int
}

type failedReopenSupervisions struct {
	timetableplanning.InstancePresence
	err   error
	reads int
}

func (p *failedReopenSupervisions) QueryGroupSupervisions(ctx context.Context, filter studentpresence.GroupSupervisionFilter) ([]studentpresence.GroupSupervision, error) {
	p.reads++
	if p.err != nil {
		return []studentpresence.GroupSupervision{{StaffID: filter.StaffIDs[0]}}, p.err
	}
	return p.InstancePresence.QueryGroupSupervisions(ctx, filter)
}

func TestInstanceReopenPreservesCompletedStateOnSupervisionFailure(t *testing.T) {
	t.Parallel()
	s := buildLifecycle(t)
	instance := seedInstance(t, s, true, false)
	started, err := s.svc.Start(s.ctx, instance.ID, 0)
	require.NoError(t, err)
	_, err = s.svc.Complete(s.ctx, instance.ID)
	require.NoError(t, err)
	injected := errors.New("staff supervisions unavailable")
	fault := &failedReopenSupervisions{InstancePresence: s.presence, err: injected}
	s.presence = fault
	broadcaster := testpkg.NewRecordingBroadcaster()
	s.svc = instanceServiceWithBroadcaster(s, broadcaster)
	result, err := s.svc.Reopen(s.ctx, instance.ID, 0, true)
	require.ErrorIs(t, err, injected)
	assert.Nil(t, result)
	assert.Equal(t, 1, fault.reads)
	assert.Empty(t, broadcaster.Calls())
	stored, err := s.repos.ActivityInstance.FindByID(s.ctx, instance.ID)
	require.NoError(t, err)
	assert.Equal(t, scheduleModels.InstanceStatusCompleted, stored.Status)
	group, err := s.factory.Active.GetActiveGroup(s.ctx, started.ActiveGroupID)
	require.NoError(t, err)
	assert.NotNil(t, group.EndTime)
	fault.err = nil
	result, err = s.svc.Reopen(s.ctx, instance.ID, 0, true)
	require.NoError(t, err)
	assert.Equal(t, scheduleModels.InstanceStatusActive, result.Instance.Status)
}

func (p *failedConflictPresence) ListVisits(ctx context.Context, filter studentpresence.VisitFilter) ([]studentpresence.Visit, error) {
	p.reads++
	if p.err != nil {
		return nil, p.err
	}
	return p.InstancePresence.ListVisits(ctx, filter)
}

func TestInstanceStartPropagatesPresenceFailureAndRetries(t *testing.T) {
	t.Parallel()
	s := buildLifecycle(t)
	instance := seedInstance(t, s, true, true)
	injected := errors.New("current visits unavailable")
	fault := &failedConflictPresence{InstancePresence: s.presence, err: injected}
	s.presence = fault
	broadcaster := testpkg.NewRecordingBroadcaster()
	s.svc = instanceServiceWithBroadcaster(s, broadcaster)
	result, err := s.svc.Start(s.ctx, instance.ID, 0)
	require.ErrorIs(t, err, injected)
	assert.Nil(t, result)
	require.Equal(t, 1, fault.reads)
	var classified *careplan.ScheduleError
	require.ErrorAs(t, err, &classified)
	assert.Empty(t, broadcaster.Calls())
	stored, err := s.repos.ActivityInstance.FindByID(s.ctx, instance.ID)
	require.NoError(t, err)
	assert.Equal(t, scheduleModels.InstanceStatusPlanned, stored.Status)
	assert.Nil(t, stored.ActiveGroupID)
	groups, err := s.repos.ActiveGroup.FindActiveByRoomID(s.ctx, s.roomID)
	require.NoError(t, err)
	assert.Empty(t, groups, "failed presence lookup must not create an active session")
	fault.err = nil
	result, err = s.svc.Start(s.ctx, instance.ID, 0)
	require.NoError(t, err)
	require.NotNil(t, result)
	groups, err = s.repos.ActiveGroup.FindActiveByRoomID(s.ctx, s.roomID)
	require.NoError(t, err)
	require.Len(t, groups, 1)
	assert.Equal(t, result.ActiveGroupID, groups[0].ID)
}
