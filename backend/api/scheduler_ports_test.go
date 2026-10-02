package api

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The Worker reads its settings keys as plain strings (#2746). This pins every
// one of them to the Settings Platform registry: registered, and no secret in
// the minute snapshot. The Worker refuses to start on the same guard.
func TestSchedulerSettingKeysPassTheRegistryGuard(t *testing.T) {
	t.Parallel()

	require.NoError(t, verifySchedulerSettingKeys())
}

func TestPreloadSettingKeyGuardRefusesUnknownAndSecretKeys(t *testing.T) {
	t.Parallel()

	err := verifyPreloadSettingKeys([]string{"operations.session_end_time", "scheduler.no_such_setting", "security.ogs_device_pin"})

	require.Error(t, err)
	assert.ErrorContains(t, err, `setting "scheduler.no_such_setting" is not registered`)
	assert.ErrorContains(t, err, `setting "security.ogs_device_pin" is a secret`)
	assert.NotContains(t, err.Error(), "operations.session_end_time")
}

type retainedDay string

type retainedStatus string

func TestSchedulerPortsConvertIntoTheRetainedStringTypes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	day, err := callOnDay(func(_ context.Context, value retainedDay) (string, error) {
		return string(value), nil
	}, ctx, calendar.NewDate(2026, time.April, 20))
	require.NoError(t, err)
	assert.Equal(t, "2026-04-20", day)

	var transitioned [2]retainedStatus
	moved, err := transitionStudentStatus(func(_ context.Context, _ int64, expected, next retainedStatus) (bool, error) {
		transitioned = [2]retainedStatus{expected, next}
		return true, nil
	}, ctx, 4711, "pending", "active")
	require.NoError(t, err)
	assert.True(t, moved)
	assert.Equal(t, [2]retainedStatus{"pending", "active"}, transitioned)

	var recorded [2]retainedStatus
	require.NoError(t, recordStudentStatusChange(func(_ context.Context, _ int64, before, after retainedStatus) error {
		recorded = [2]retainedStatus{before, after}
		return nil
	}, ctx, 4711, "active", "inactive"))
	assert.Equal(t, [2]retainedStatus{"active", "inactive"}, recorded)
}

type boundSnapshotKey struct{}

// bindableSnapshot stands in for the Settings Platform's tenant snapshot.
type bindableSnapshot string

func (s bindableSnapshot) Bind(ctx context.Context) context.Context {
	return context.WithValue(ctx, boundSnapshotKey{}, string(s))
}

func TestSchedulerSettingsPortBindsOnlyItsOwnSnapshots(t *testing.T) {
	t.Parallel()
	port := schedulerSettingsPort{}

	ctx, err := port.BindSettingsSnapshot(context.Background(), bindableSnapshot("school-a"))
	require.NoError(t, err)
	assert.Equal(t, "school-a", ctx.Value(boundSnapshotKey{}))

	_, err = port.BindSettingsSnapshot(context.Background(), "not a snapshot")
	require.Error(t, err, "an unbindable snapshot must fail the tick instead of reading the database")
}
