package active

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type auditStaffNamesFunc func(context.Context, []int64) (map[int64]string, error)

func (f auditStaffNamesFunc) StaffDisplayNames(ctx context.Context, ids []int64) (map[int64]string, error) {
	return f(ctx, ids)
}

func TestAuditNamesBatchesDistinctStaffAndActors(t *testing.T) {
	t.Parallel()
	staffID, actorID, missingID := int64(11), int64(22), int64(33)
	calls := 0
	svc := timeTrackingAuditLogService{staffNames: auditStaffNamesFunc(func(_ context.Context, ids []int64) (map[int64]string, error) {
		calls++
		assert.ElementsMatch(t, []int64{staffID, actorID, missingID}, ids)
		return map[int64]string{staffID: "Anna Beispiel", actorID: "Ben Muster"}, nil
	})}
	events, err := svc.decorate(context.Background(), []*TimeTrackingAuditEntry{
		{StaffID: &staffID, ActorStaffID: &actorID},
		{StaffID: &staffID, ActorStaffID: &staffID},
		{StaffID: &missingID, ActorIsSystem: true},
	})
	require.NoError(t, err)
	require.Len(t, events, 3)
	assert.Equal(t, 1, calls)
	assert.Equal(t, "Anna Beispiel", events[0].Staff.Name)
	assert.Equal(t, "Ben Muster", events[0].Actor.Name)
	assert.False(t, events[0].Actor.IsSelf)
	assert.True(t, events[1].Actor.IsSelf)
	assert.Empty(t, events[2].Staff.Name)
	assert.Equal(t, "System", events[2].Actor.Name)
	assert.True(t, events[2].Actor.IsSystem)
}

func TestAuditNamesSkipsLookupWithoutStaffIDs(t *testing.T) {
	t.Parallel()
	svc := timeTrackingAuditLogService{staffNames: auditStaffNamesFunc(func(context.Context, []int64) (map[int64]string, error) {
		t.Fatal("system-only events must not query the staff directory")
		return nil, nil
	})}
	events, err := svc.decorate(context.Background(), []*TimeTrackingAuditEntry{{ActorIsSystem: true}})
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, "System", events[0].Actor.Name)
}

func TestAuditNamesPreservesLookupFailure(t *testing.T) {
	t.Parallel()
	failure := errors.New("staff directory unavailable")
	id := int64(11)
	svc := timeTrackingAuditLogService{staffNames: auditStaffNamesFunc(func(context.Context, []int64) (map[int64]string, error) {
		return nil, failure
	})}
	events, err := svc.decorate(context.Background(), []*TimeTrackingAuditEntry{{StaffID: &id}})
	assert.Nil(t, events)
	assert.ErrorIs(t, err, failure)
	assert.EqualError(t, err, "failed to resolve names for audit log: staff directory unavailable")
}
