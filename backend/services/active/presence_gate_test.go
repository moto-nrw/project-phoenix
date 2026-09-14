package active

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Unit tests for the presence-mode gates on active.Service — specifically
// GetPresenceMode, and the short-circuits in CreateVisit / EndVisit /
// EndDailySessions when the tenant is in binary mode. These are package-
// internal tests so we can stub `s.settings` directly without driving a
// full integration flow.

// stubSettingsResolver implements the minimal SettingsResolver surface
// the service uses. Real SettingsService is heavier than we need here.
type stubSettingsResolver struct {
	// Key → (value, error) — unset key returns "" with no error, mimicking
	// how ResolveString behaves for a missing override.
	stringValues map[string]string
	stringErr    error
	intValues    map[string]int
}

// The stub still scripts answers per registry key, so the tables below read
// the same as when the service asked for settings by key.
func (s *stubSettingsResolver) resolveString(key string) (string, error) {
	if s.stringErr != nil {
		return "", s.stringErr
	}
	return s.stringValues[key], nil
}

func (s *stubSettingsResolver) PresenceMode(context.Context) (string, error) {
	return s.resolveString(configModel.KeyPresenceMode)
}

func (s *stubSettingsResolver) SickClearMode(context.Context) (string, error) {
	return s.resolveString(configModel.KeySickClearMode)
}

func (s *stubSettingsResolver) ExcusedClearMode(context.Context) (string, error) {
	return s.resolveString(configModel.KeyExcusedClearMode)
}

func (s *stubSettingsResolver) AttendanceEditScope(context.Context) (string, error) {
	return s.resolveString(configModel.KeyAttendanceEditScope)
}

func (s *stubSettingsResolver) OperationalOverviewScope(context.Context) (string, error) {
	return s.resolveString(configModel.KeyOperationalOverviewScope)
}

func (s *stubSettingsResolver) SessionInactivityTimeoutMinutes(context.Context) (int, error) {
	return s.intValues[configModel.KeySessionInactivityTimeoutMin], nil
}

func TestResolvePresenceModeRejectsFailuresAndInvalidValues(t *testing.T) {
	t.Parallel()
	injected := errors.New("settings read failed")
	for _, tc := range []struct {
		name     string
		resolver SettingsResolver
		want     string
		wantErr  bool
	}{
		{name: "missing wiring", wantErr: true},
		{name: "read error", resolver: &stubSettingsResolver{stringErr: injected}, wantErr: true},
		{name: "empty", resolver: &stubSettingsResolver{}, wantErr: true},
		{name: "invalid", resolver: &stubSettingsResolver{stringValues: map[string]string{configModel.KeyPresenceMode: "manual"}}, wantErr: true},
		{name: "detailed", resolver: &stubSettingsResolver{stringValues: map[string]string{configModel.KeyPresenceMode: PresenceModeDetailed}}, want: PresenceModeDetailed},
		{name: "binary", resolver: &stubSettingsResolver{stringValues: map[string]string{configModel.KeyPresenceMode: PresenceModeBinary}}, want: PresenceModeBinary},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := &service{settings: tc.resolver}
			mode, err := svc.GetPresenceMode(context.Background())
			if tc.wantErr {
				require.Error(t, err)
				assert.Empty(t, mode)
				if tc.name == "read error" {
					require.ErrorIs(t, err, injected)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, mode)
		})
	}
}

func TestService_CreateVisit_BinaryMode_IsNoOp(t *testing.T) {
	t.Parallel()

	// Binary mode short-circuits before any repo call. A nil visitRepo
	// proves the short-circuit — if the gate leaks, this would panic. Visit
	// must pass Validate() (StudentID/ActiveGroupID/EntryTime), since the
	// gate is placed after validation to preserve caller-contract errors.
	s := &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal, Logger: slog.Default()}, settings: &stubSettingsResolver{
		stringValues: map[string]string{"operations.presence_mode": "binary"},
	},
	}
	visit := &studentpresence.Visit{
		StudentID:     1,
		ActiveGroupID: 2,
		EntryTime:     time.Now(),
	}

	err := s.CreateVisit(context.Background(), visit)
	require.NoError(t, err, "binary mode must return nil without touching repos")
}

func TestService_EndVisit_BinaryMode_IsNoOp(t *testing.T) {
	t.Parallel()

	// Same contract for EndVisit — stale visit IDs from before a mode
	// switch hit the no-op path instead of a missing-row error.
	s := &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal, Logger: slog.Default()}, settings: &stubSettingsResolver{
		stringValues: map[string]string{"operations.presence_mode": "binary"},
	},
	}

	err := s.EndVisit(context.Background(), 12345)
	require.NoError(t, err, "binary mode must skip the visit end path cleanly")
}

func TestService_EndDailySessions_BinaryMode_ReturnsEmptySuccess(t *testing.T) {
	t.Parallel()

	s := &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal, Logger: slog.Default()}, settings: &stubSettingsResolver{
		stringValues: map[string]string{"operations.presence_mode": "binary"},
	},
	}

	result, err := s.EndDailySessions(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
	assert.Zero(t, result.VisitsEnded, "binary mode has no visits to end")
	assert.Empty(t, result.Errors)
}
