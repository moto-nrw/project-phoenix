package schedule_test

import (
	"context"
	"errors"
	"testing"

	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failedConflictPresence struct {
	scheduleSvc.InstancePresence
	err   error
	reads int
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
	var classified *scheduleSvc.ScheduleError
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
