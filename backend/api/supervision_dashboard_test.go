package api

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	settingsplatform "github.com/moto-nrw/project-phoenix/modules/settings"
)

// supervisionSettingsDouble is a func-field double for the settings resolver
// the supervision ports read. A nil field answers the zero value.
type supervisionSettingsDouble struct {
	ResolveStringFn func(ctx context.Context, key string) (string, error)
	ResolveBoolFn   func(ctx context.Context, key string) (bool, error)
	PrefetchFn      func(ctx context.Context, keys []string) (context.Context, error)
}

func (s *supervisionSettingsDouble) ResolveString(ctx context.Context, key string) (string, error) {
	if s.ResolveStringFn != nil {
		return s.ResolveStringFn(ctx, key)
	}
	return "", nil
}

func (s *supervisionSettingsDouble) ResolveBool(ctx context.Context, key string) (bool, error) {
	if s.ResolveBoolFn != nil {
		return s.ResolveBoolFn(ctx, key)
	}
	return false, nil
}

// Prefetch keeps the request context by default.
func (s *supervisionSettingsDouble) Prefetch(ctx context.Context, keys []string) (context.Context, error) {
	if s.PrefetchFn != nil {
		return s.PrefetchFn(ctx, keys)
	}
	return ctx, nil
}

func supervisionAdminContext() context.Context {
	principal, err := permissions.NewPrincipal(permissions.PrincipalInput{
		AccountID: 99,
		TenantID:  23,
		Scope:     string(permissions.ScopeTenant),
		Admin:     true,
	})
	if err != nil {
		panic(err)
	}
	return permissions.WithPrincipal(context.Background(), principal)
}

func TestSupervisionPortsRequireTheirSources(t *testing.T) {
	t.Parallel()

	settings := &supervisionSettingsDouble{}
	assert.Nil(t, newSupervisionAccess(nil, SupervisionCaller{}), "missing wiring fails composition, not the first request")
	assert.Nil(t, newSupervisionAccess(settings, nil))
	assert.NotNil(t, newSupervisionAccess(settings, SupervisionCaller{}))
	assert.Nil(t, newSupervisionSettings(nil))
	assert.NotNil(t, newSupervisionSettings(settings))
	assert.Nil(t, newSupervisionYard(nil))
}

func TestSupervisionCallerResolvesOverviewAndPermissions(t *testing.T) {
	t.Parallel()

	settings := &supervisionSettingsDouble{ResolveStringFn: func(_ context.Context, key string) (string, error) {
		if key != settingsplatform.KeyOperationalOverviewScope {
			return "", errors.New("unexpected settings key: " + key)
		}
		return settingsplatform.OverviewScopeAdmins, nil
	}}
	a := supervisionAccess{settings: settings}

	ctx := context.WithValue(supervisionAdminContext(), sessiontoken.CtxClaims, sessiontoken.AppClaims{ID: 99, IsAdmin: true})
	ctx = context.WithValue(ctx, sessiontoken.CtxPermissions, []string{permissions.SchedulesRead})
	caller, err := a.Caller(ctx)
	require.NoError(t, err)
	assert.Equal(t, decodeAs(t, caller, `{"AccountID":99,"TokenAdmin":true,"AdminScope":true,"OperationalOverview":true,"CanReadSchedules":true}`), caller)

	settings.ResolveStringFn = func(context.Context, string) (string, error) {
		return settingsplatform.OverviewScopeOwn, nil
	}
	caller, err = a.Caller(ctx)
	require.NoError(t, err)
	assert.True(t, caller.OperationalOverview, "admins always have the school-wide overview")

	settings.ResolveStringFn = func(context.Context, string) (string, error) { return "", errors.New("boom") }
	_, err = a.Caller(ctx)
	require.ErrorContains(t, err, "operational overview scope")

	plain := context.WithValue(context.Background(), sessiontoken.CtxPermissions, []string{permissions.UsersRead})
	settings.ResolveStringFn = func(context.Context, string) (string, error) { return settingsplatform.OverviewScopeOwn, nil }
	caller, err = a.Caller(plain)
	require.NoError(t, err)
	assert.Equal(t, decodeAs(t, caller, `{"CanReadStudents":true}`), caller)
}

func TestSupervisionSettingsBranches(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	mock := &supervisionSettingsDouble{
		ResolveBoolFn:   func(context.Context, string) (bool, error) { return true, nil },
		ResolveStringFn: func(context.Context, string) (string, error) { return "", errors.New("boom") },
	}
	s := supervisionSettings{settings: mock}

	_, err := s.TrackingIndicatorLabels(ctx)
	require.Error(t, err)

	mock.ResolveStringFn = func(context.Context, string) (string, error) { return "  ", nil }
	labels, err := s.TrackingIndicatorLabels(ctx)
	require.NoError(t, err)
	assert.Empty(t, labels)

	mock.ResolveStringFn = func(_ context.Context, key string) (string, error) {
		if key == settingsplatform.KeyTrackingIndicator1 {
			return "Hausaufgaben", nil
		}
		return "", nil
	}
	labels, err = s.TrackingIndicatorLabels(ctx)
	require.NoError(t, err)
	assert.Equal(t, []string{"Hausaufgaben"}, labels)

	mock.ResolveBoolFn = func(_ context.Context, key string) (bool, error) {
		return key != settingsplatform.KeyTrackingIndicatorsEnabled, nil
	}
	labels, err = s.TrackingIndicatorLabels(ctx)
	require.NoError(t, err)
	assert.Nil(t, labels, "disabled indicators carry no labels")

	// The switch alone decides since #3730; no string setting is consulted.
	mock.ResolveStringFn = func(context.Context, string) (string, error) { return "", errors.New("settings unavailable") }
	enabled, err := s.SpontaneousActivitiesEnabled(ctx)
	require.NoError(t, err)
	assert.True(t, enabled)

	mock.ResolveBoolFn = func(_ context.Context, key string) (bool, error) {
		return key != settingsplatform.KeyWebSpontaneousActivities, nil
	}
	enabled, err = s.SpontaneousActivitiesEnabled(ctx)
	require.NoError(t, err)
	assert.False(t, enabled, "a school with the switch off starts no spontaneous activities")

	mock.ResolveBoolFn = func(context.Context, string) (bool, error) { return false, errors.New("boom") }
	_, err = s.SpontaneousActivitiesEnabled(ctx)
	require.ErrorContains(t, err, "spontaneous activities")

	type prefetched struct{}
	var preparedKeys []string
	mock.PrefetchFn = func(ctx context.Context, keys []string) (context.Context, error) {
		preparedKeys = keys
		return context.WithValue(ctx, prefetched{}, true), nil
	}
	prepared, err := s.Prepare(ctx)
	require.NoError(t, err)
	assert.Equal(t, true, prepared.Value(prefetched{}), "the request continues with the prefetched context")
	assert.Contains(t, preparedKeys, settingsplatform.KeyWeekendFollowsFriday)
	mock.PrefetchFn = func(ctx context.Context, _ []string) (context.Context, error) {
		return ctx, errors.New("settings down")
	}
	_, err = s.Prepare(ctx)
	require.EqualError(t, err, "settings down")
}

// supervisionSchulhof answers one Facilities Schulhof status; the embedded
// service panics on any other call. Its status type comes from the service's
// own signature, which the root's tests may not import.
type supervisionSchulhof[S any] struct {
	facilitiesCompose.SchulhofService
	status func(staffID int64) (S, error)
}

func (f supervisionSchulhof[S]) GetSchulhofStatus(_ context.Context, staffID int64) (S, error) {
	return f.status(staffID)
}

func supervisionSchulhofReturning[S any](t *testing.T, wantStaffID int64, status S) supervisionSchulhof[S] {
	return supervisionSchulhof[S]{status: func(staffID int64) (S, error) {
		assert.Equal(t, wantStaffID, staffID)
		return status, nil
	}}
}

func schulhofStatusOf[I, S any](func(I, context.Context, int64) (S, error)) (zero S) { return zero }

func TestSupervisionYardPreservesWireShape(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	noStatus := schulhofStatusOf(facilitiesCompose.SchulhofService.GetSchulhofStatus)
	retained := decodeAs(t, noStatus, `{
		"exists": true, "room_id": 21, "room_name": "Schulhof", "activity_group_id": 3, "active_group_id": 8,
		"is_user_supervising": true, "supervision_id": 4, "supervisor_count": 1, "student_count": 12,
		"supervisors": [{"id": 4, "staff_id": 7, "name": "Erika Muster", "is_current_user": true}]
	}`)

	status, err := newSupervisionYard(supervisionSchulhofReturning(t, 77, retained)).Status(ctx, 77)
	require.NoError(t, err)
	assertSupervisionSameJSON(t, retained, status)

	empty := decodeAs(t, noStatus, `{}`)
	status, err = newSupervisionYard(supervisionSchulhofReturning(t, 77, empty)).Status(ctx, 77)
	require.NoError(t, err)
	assertSupervisionSameJSON(t, empty, status)

	status, err = newSupervisionYard(supervisionSchulhofReturning(t, 77, noStatus)).Status(ctx, 77)
	require.NoError(t, err)
	assert.Nil(t, status)
}

// assertSupervisionSameJSON is the wire-parity golden: the projection's own
// record must serialise byte-identically to the retained row it mirrors.
func assertSupervisionSameJSON(t *testing.T, retained, projected any) {
	t.Helper()
	want, err := json.Marshal(retained)
	require.NoError(t, err)
	got, err := json.Marshal(projected)
	require.NoError(t, err)
	assert.Equal(t, string(want), string(got))
}
