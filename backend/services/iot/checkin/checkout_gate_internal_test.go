// Package checkin internal tests for the daily-checkout / checkout-gate policy.
// These are pure policy tests that don't need a database; they build a
// CheckinService with mock collaborators and exercise the extracted gate logic
// (moved here from api/iot/checkin when the policy was extracted into the
// service). They live in the internal package so they can set the service's
// unexported collaborator fields and override the package-local timeNow clock.
package checkin

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/constants"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/education"
	facilityModels "github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/config/configtest"
	educationSvc "github.com/moto-nrw/project-phoenix/services/education"
	facilitiesSvc "github.com/moto-nrw/project-phoenix/services/facilities"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
)

// =============================================================================
// Test doubles
// =============================================================================

// overrideTimeNow pins the package-level timeNow clock to fixed and returns a
// restore function that should be deferred by the caller.
func overrideTimeNow(fixed time.Time) func() {
	orig := timeNow
	timeNow = func() time.Time { return fixed }
	return func() { timeNow = orig }
}

// mockPickupScheduleService is a minimal mock for testing IsAfterCheckoutTimeGate.
type mockPickupScheduleService struct {
	scheduleSvc.PickupScheduleService
	effectivePickupTime *scheduleSvc.EffectivePickupTime
	err                 error
}

func (m *mockPickupScheduleService) GetEffectivePickupTimeForDate(_ context.Context, _ int64, _ timezone.Date) (*scheduleSvc.EffectivePickupTime, error) {
	return m.effectivePickupTime, m.err
}

// mockEducationService is a minimal mock for testing ShouldShowDailyCheckoutWithGroup.
type mockEducationService struct {
	educationSvc.Service
	group *education.Group
	err   error
}

func (m *mockEducationService) GetGroup(_ context.Context, _ int64) (*education.Group, error) {
	return m.group, m.err
}

// mockFacilitiesService is a minimal mock for testing the Schulhof branch of
// ShouldShowDailyCheckoutWithGroup. Only FindRoomByName is exercised — it is
// the single call FindCanonicalSchulhofRoom makes.
type mockFacilitiesService struct {
	facilitiesSvc.Service
	room *facilityModels.Room
	err  error
}

func (m *mockFacilitiesService) FindRoomByName(_ context.Context, _ string) (*facilityModels.Room, error) {
	return m.room, m.err
}

// newSchulhofFacilities builds a facilities mock returning the canonical
// Schulhof room (correct name, marked as a system room) under roomID.
func newSchulhofFacilities(roomID int64) *mockFacilitiesService {
	return &mockFacilitiesService{
		room: &facilityModels.Room{
			Model:    base.Model{ID: roomID},
			Name:     constants.SchulhofRoomName,
			IsSystem: true,
		},
	}
}

// newMockSettingsService builds a configtest.Mock reproducing the behavior of
// the former hand-rolled mockSettingsService stub: ResolveString/Resolve return
// a "not found" error for missing keys, while ResolveBool/ResolveInt return the
// zero value with no error for missing keys.
func newMockSettingsService(values map[string]string, boolValues map[string]bool, intValues map[string]int) *configtest.Mock {
	resolveString := func(_ context.Context, key string) (string, error) {
		if v, ok := values[key]; ok {
			return v, nil
		}
		return "", fmt.Errorf("not found")
	}
	resolveBool := func(_ context.Context, key string) (bool, error) {
		if boolValues != nil {
			if v, ok := boolValues[key]; ok {
				return v, nil
			}
		}
		return false, nil
	}
	resolveInt := func(_ context.Context, key string) (int, error) {
		if intValues != nil {
			if v, ok := intValues[key]; ok {
				return v, nil
			}
		}
		return 0, nil
	}
	return &configtest.Mock{
		ResolveFn: func(_ context.Context, key string) (any, error) {
			if v, ok := values[key]; ok {
				return v, nil
			}
			return nil, fmt.Errorf("not found")
		},
		ResolveStringFn: resolveString,
		ResolveStringForTenantFn: func(ctx context.Context, _ int64, key string) (string, error) {
			return resolveString(ctx, key)
		},
		ResolveBoolFn: resolveBool,
		ResolveBoolForTenantFn: func(ctx context.Context, _ int64, key string) (bool, error) {
			return resolveBool(ctx, key)
		},
		ResolveIntFn: resolveInt,
		ResolveIntForTenantFn: func(ctx context.Context, _ int64, key string) (int, error) {
			return resolveInt(ctx, key)
		},
		HasTenantOverrideFn: func(_ context.Context, key string) (bool, error) {
			_, exists := values[key]
			return exists, nil
		},
	}
}

// newMockErrorSettingsService builds a configtest.Mock reproducing the former
// mockErrorSettingsService stub: identical to an empty mockSettingsService,
// except HasTenantOverride always errors.
func newMockErrorSettingsService() *configtest.Mock {
	m := newMockSettingsService(nil, nil, nil)
	m.HasTenantOverrideFn = func(_ context.Context, _ string) (bool, error) {
		return false, fmt.Errorf("db connection failed")
	}
	return m
}

// =============================================================================
// studentDailyCheckoutTime TESTS
// =============================================================================

// Deliberately NOT parallel: the test reaches process-global state (env
// variables, viper keys, the settings registry, os.Stdout) that the whole
// test binary shares.
func TestStudentDailyCheckoutTime_NoConfig_ReturnsNil(t *testing.T) {
	// Clear any existing env var
	_ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME")

	s := &CheckinService{}
	checkoutTime, err := s.studentDailyCheckoutTime(context.Background())
	require.NoError(t, err)

	// No time configured — daily checkout is always available
	assert.Nil(t, checkoutTime, "should return nil when no checkout time is configured")
}

func TestStudentDailyCheckoutTime_CustomValid(t *testing.T) {
	require.NoError(t, os.Setenv("STUDENT_DAILY_CHECKOUT_TIME", "14:30"))
	defer func() { _ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME") }()

	s := &CheckinService{}
	checkoutTime, err := s.studentDailyCheckoutTime(context.Background())
	require.NoError(t, err)
	require.NotNil(t, checkoutTime)

	assert.Equal(t, 14, checkoutTime.Hour())
	assert.Equal(t, 30, checkoutTime.Minute())
}

func TestStudentDailyCheckoutTime_InvalidFormat(t *testing.T) {
	require.NoError(t, os.Setenv("STUDENT_DAILY_CHECKOUT_TIME", "invalid"))
	defer func() { _ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME") }()

	s := &CheckinService{}
	_, err := s.studentDailyCheckoutTime(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid checkout time format")
}

func TestStudentDailyCheckoutTime_InvalidHour(t *testing.T) {
	require.NoError(t, os.Setenv("STUDENT_DAILY_CHECKOUT_TIME", "25:00"))
	defer func() { _ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME") }()

	s := &CheckinService{}
	_, err := s.studentDailyCheckoutTime(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid hour")
}

func TestStudentDailyCheckoutTime_InvalidMinute(t *testing.T) {
	require.NoError(t, os.Setenv("STUDENT_DAILY_CHECKOUT_TIME", "12:99"))
	defer func() { _ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME") }()

	s := &CheckinService{}
	_, err := s.studentDailyCheckoutTime(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid minute")
}

func TestStudentDailyCheckoutTime_NegativeHour(t *testing.T) {
	require.NoError(t, os.Setenv("STUDENT_DAILY_CHECKOUT_TIME", "-1:00"))
	defer func() { _ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME") }()

	s := &CheckinService{}
	_, err := s.studentDailyCheckoutTime(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid hour")
}

func TestStudentDailyCheckoutTime_NegativeMinute(t *testing.T) {
	require.NoError(t, os.Setenv("STUDENT_DAILY_CHECKOUT_TIME", "12:-5"))
	defer func() { _ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME") }()

	s := &CheckinService{}
	_, err := s.studentDailyCheckoutTime(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid minute")
}

func TestStudentDailyCheckoutTime_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		envVar  string
		wantH   int
		wantM   int
		wantErr bool
	}{
		{"midnight", "00:00", 0, 0, false},
		{"end of day", "23:59", 23, 59, false},
		{"noon", "12:00", 12, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, os.Setenv("STUDENT_DAILY_CHECKOUT_TIME", tt.envVar))
			defer func() { _ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME") }()

			s := &CheckinService{}
			checkoutTime, err := s.studentDailyCheckoutTime(context.Background())
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, checkoutTime)
				assert.Equal(t, tt.wantH, checkoutTime.Hour())
				assert.Equal(t, tt.wantM, checkoutTime.Minute())
			}
		})
	}
}

// Deliberately NOT parallel: the test reaches process-global state (env
// variables, viper keys, the settings registry, os.Stdout) that the whole
// test binary shares.
func TestStudentDailyCheckoutTime_UsesSettingsService(t *testing.T) {
	// Clear env var so only the settings service provides the value
	_ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME")

	s := &CheckinService{
		settings: newMockSettingsService(map[string]string{
			"operations.student_daily_checkout_time": "16:45",
		}, nil, nil),
	}

	checkoutTime, err := s.studentDailyCheckoutTime(context.Background())
	require.NoError(t, err)
	require.NotNil(t, checkoutTime)
	assert.Equal(t, 16, checkoutTime.Hour())
	assert.Equal(t, 45, checkoutTime.Minute())
}

func TestStudentDailyCheckoutTime_SettingsServiceFallsBackToEnv(t *testing.T) {
	require.NoError(t, os.Setenv("STUDENT_DAILY_CHECKOUT_TIME", "13:15"))
	defer func() { _ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME") }()

	// Settings service returns empty — should fall back to env var
	s := &CheckinService{
		settings: newMockSettingsService(map[string]string{}, nil, nil),
	}

	checkoutTime, err := s.studentDailyCheckoutTime(context.Background())
	require.NoError(t, err)
	require.NotNil(t, checkoutTime)
	assert.Equal(t, 13, checkoutTime.Hour())
	assert.Equal(t, 15, checkoutTime.Minute())
}

func TestStudentDailyCheckoutTime_NilSettingsServiceUsesEnv(t *testing.T) {
	require.NoError(t, os.Setenv("STUDENT_DAILY_CHECKOUT_TIME", "17:00"))
	defer func() { _ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME") }()

	s := &CheckinService{
		settings: nil,
	}

	checkoutTime, err := s.studentDailyCheckoutTime(context.Background())
	require.NoError(t, err)
	require.NotNil(t, checkoutTime)
	assert.Equal(t, 17, checkoutTime.Hour())
	assert.Equal(t, 0, checkoutTime.Minute())
}

// Deliberately NOT parallel: the test reaches process-global state (env
// variables, viper keys, the settings registry, os.Stdout) that the whole
// test binary shares.
func TestStudentDailyCheckoutTime_NoConfigAnywhere_ReturnsNil(t *testing.T) {
	_ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME")

	// Settings service exists but has no override
	s := &CheckinService{
		settings: newMockSettingsService(map[string]string{}, nil, nil),
	}

	checkoutTime, err := s.studentDailyCheckoutTime(context.Background())
	require.NoError(t, err)
	assert.Nil(t, checkoutTime, "should return nil when no time is configured anywhere")
}

// Deliberately NOT parallel: the test reaches process-global state (env
// variables, viper keys, the settings registry, os.Stdout) that the whole
// test binary shares.
func TestStudentDailyCheckoutTime_HasTenantOverrideError(t *testing.T) {
	_ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME")

	s := &CheckinService{
		settings: newMockErrorSettingsService(),
	}

	checkoutTime, err := s.studentDailyCheckoutTime(context.Background())
	require.NoError(t, err)
	// Should fall back to nil (no time configured) since env var is also unset
	assert.Nil(t, checkoutTime, "should return nil when HasTenantOverride errors and no env var")
}

func TestStudentDailyCheckoutTime_HasTenantOverrideError_FallsBackToEnv(t *testing.T) {
	require.NoError(t, os.Setenv("STUDENT_DAILY_CHECKOUT_TIME", "15:00"))
	defer func() { _ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME") }()

	s := &CheckinService{
		settings: newMockErrorSettingsService(),
	}

	checkoutTime, err := s.studentDailyCheckoutTime(context.Background())
	require.NoError(t, err)
	require.NotNil(t, checkoutTime)
	assert.Equal(t, 15, checkoutTime.Hour())
	assert.Equal(t, 0, checkoutTime.Minute())
}

// =============================================================================
// ShouldUpgradeToDailyCheckout TESTS
// =============================================================================

func TestShouldUpgradeToDailyCheckout_NotCheckedOutAction(t *testing.T) {
	t.Parallel()

	s := &CheckinService{}
	// Pass a valid student to avoid nil dereference on student.GroupID
	student := &users.Student{Model: base.Model{ID: 1}}
	result := s.ShouldUpgradeToDailyCheckout(context.Background(), "checked_in", student, nil)
	assert.False(t, result)
}

func TestShouldUpgradeToDailyCheckout_StudentNoGroupID(t *testing.T) {
	t.Parallel()

	s := &CheckinService{}
	student := &users.Student{Model: base.Model{ID: 1}}
	result := s.ShouldUpgradeToDailyCheckout(context.Background(), "checked_out", student, nil)
	assert.False(t, result)
}

func TestShouldUpgradeToDailyCheckout_NilCurrentVisit(t *testing.T) {
	t.Parallel()

	s := &CheckinService{}
	groupID := int64(1)
	student := &users.Student{Model: base.Model{ID: 1}, GroupID: &groupID}
	result := s.ShouldUpgradeToDailyCheckout(context.Background(), "checked_out", student, nil)
	assert.False(t, result)
}

func TestShouldUpgradeToDailyCheckout_NilActiveGroup(t *testing.T) {
	t.Parallel()

	s := &CheckinService{}
	groupID := int64(1)
	student := &users.Student{Model: base.Model{ID: 1}, GroupID: &groupID}
	visit := &active.Visit{}
	result := s.ShouldUpgradeToDailyCheckout(context.Background(), "checked_out", student, visit)
	assert.False(t, result)
}

// Deliberately NOT parallel: the test reaches process-global state (env
// variables, viper keys, the settings registry, os.Stdout) that the whole
// test binary shares.
func TestShouldUpgradeToDailyCheckout_CheckedOut_NoTimeGate(t *testing.T) {
	// No checkout time configured → time check should pass
	_ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME")

	s := &CheckinService{
		settings:  newMockSettingsService(map[string]string{}, nil, nil),
		education: &mockEducationService{group: &education.Group{Model: base.Model{ID: 1}}},
	}
	groupID := int64(1)
	student := &users.Student{Model: base.Model{ID: 1}, GroupID: &groupID}
	visit := &active.Visit{ActiveGroup: &active.Group{RoomID: 1}}
	result := s.ShouldUpgradeToDailyCheckout(context.Background(), "checked_out", student, visit)
	assert.True(t, result, "Should upgrade when no time gate and group has no room")
}

// =============================================================================
// ShouldShowDailyCheckoutWithGroup TESTS (direct calls to test defensive guards)
// =============================================================================

func TestShouldShowDailyCheckoutWithGroup_NilGroupID(t *testing.T) {
	t.Parallel()

	s := &CheckinService{}
	student := &users.Student{Model: base.Model{ID: 1}} // GroupID is nil
	visit := &active.Visit{ActiveGroup: &active.Group{RoomID: 1}}
	result := s.ShouldShowDailyCheckoutWithGroup(context.Background(), student, visit)
	assert.False(t, result)
}

func TestShouldShowDailyCheckoutWithGroup_NilCurrentVisit(t *testing.T) {
	t.Parallel()

	s := &CheckinService{}
	groupID := int64(1)
	student := &users.Student{Model: base.Model{ID: 1}, GroupID: &groupID}
	result := s.ShouldShowDailyCheckoutWithGroup(context.Background(), student, nil)
	assert.False(t, result)
}

func TestShouldShowDailyCheckoutWithGroup_NilActiveGroup(t *testing.T) {
	t.Parallel()

	s := &CheckinService{}
	groupID := int64(1)
	student := &users.Student{Model: base.Model{ID: 1}, GroupID: &groupID}
	visit := &active.Visit{} // ActiveGroup is nil
	result := s.ShouldShowDailyCheckoutWithGroup(context.Background(), student, visit)
	assert.False(t, result)
}

func TestShouldShowDailyCheckoutWithGroup_BeforeCheckoutTime(t *testing.T) {
	// Set checkout time far in the future so we're always before it
	require.NoError(t, os.Setenv("STUDENT_DAILY_CHECKOUT_TIME", "23:59"))
	defer func() { _ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME") }()

	s := &CheckinService{}
	groupID := int64(1)
	student := &users.Student{Model: base.Model{ID: 1}, GroupID: &groupID}
	visit := &active.Visit{ActiveGroup: &active.Group{RoomID: 1}}
	result := s.ShouldShowDailyCheckoutWithGroup(context.Background(), student, visit)
	assert.False(t, result, "Should return false before daily checkout time")
}

// Deliberately NOT parallel: the test reaches process-global state (env
// variables, viper keys, the settings registry, os.Stdout) that the whole
// test binary shares.
func TestShouldShowDailyCheckoutWithGroup_NilCheckoutTime_AlwaysAvailable(t *testing.T) {
	// No env var, no settings override → nil checkout time → time check skipped
	_ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME")

	// Education group has no room → daily checkout available from any room
	s := &CheckinService{
		settings:  newMockSettingsService(map[string]string{}, nil, nil),
		education: &mockEducationService{group: &education.Group{Model: base.Model{ID: 1}}},
	}
	groupID := int64(1)
	student := &users.Student{Model: base.Model{ID: 1}, GroupID: &groupID}
	visit := &active.Visit{ActiveGroup: &active.Group{RoomID: 1}}
	result := s.ShouldShowDailyCheckoutWithGroup(context.Background(), student, visit)
	assert.True(t, result, "Should return true when no checkout time is configured and group has no room")
}

// Deliberately NOT parallel: the test reaches process-global state (env
// variables, viper keys, the settings registry, os.Stdout) that the whole
// test binary shares.
func TestShouldShowDailyCheckoutWithGroup_NilCheckoutTime_MatchingRoom(t *testing.T) {
	_ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME")

	roomID := int64(42)
	s := &CheckinService{
		settings:  newMockSettingsService(map[string]string{}, nil, nil),
		education: &mockEducationService{group: &education.Group{Model: base.Model{ID: 1}, RoomID: &roomID}},
	}
	groupID := int64(1)
	student := &users.Student{Model: base.Model{ID: 1}, GroupID: &groupID}
	visit := &active.Visit{ActiveGroup: &active.Group{RoomID: 42}}
	result := s.ShouldShowDailyCheckoutWithGroup(context.Background(), student, visit)
	assert.True(t, result, "Should return true when rooms match and no time gate")
}

// Deliberately NOT parallel: the test reaches process-global state (env
// variables, viper keys, the settings registry, os.Stdout) that the whole
// test binary shares.
func TestShouldShowDailyCheckoutWithGroup_NilCheckoutTime_DifferentRoom(t *testing.T) {
	_ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME")

	roomID := int64(42)
	s := &CheckinService{
		settings:  newMockSettingsService(map[string]string{}, nil, nil),
		education: &mockEducationService{group: &education.Group{Model: base.Model{ID: 1}, RoomID: &roomID}},
	}
	groupID := int64(1)
	student := &users.Student{Model: base.Model{ID: 1}, GroupID: &groupID}
	visit := &active.Visit{ActiveGroup: &active.Group{RoomID: 99}}
	result := s.ShouldShowDailyCheckoutWithGroup(context.Background(), student, visit)
	assert.False(t, result, "Should return false when student is in wrong room")
}

func TestShouldShowDailyCheckoutWithGroup_GetCheckoutTimeError(t *testing.T) {
	// Set an invalid time format to trigger a parse error
	require.NoError(t, os.Setenv("STUDENT_DAILY_CHECKOUT_TIME", "not-a-time"))
	defer func() { _ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME") }()

	s := &CheckinService{}
	groupID := int64(1)
	student := &users.Student{Model: base.Model{ID: 1}, GroupID: &groupID}
	visit := &active.Visit{ActiveGroup: &active.Group{RoomID: 1}}
	result := s.ShouldShowDailyCheckoutWithGroup(context.Background(), student, visit)
	assert.False(t, result, "Should return false when checkout time parse fails")
}

// Deliberately NOT parallel: the test reaches process-global state (env
// variables, viper keys, the settings registry, os.Stdout) that the whole
// test binary shares.
func TestShouldShowDailyCheckoutWithGroup_EducationServiceError(t *testing.T) {
	_ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME")

	s := &CheckinService{
		settings:  newMockSettingsService(map[string]string{}, nil, nil),
		education: &mockEducationService{err: fmt.Errorf("db error")},
	}
	groupID := int64(1)
	student := &users.Student{Model: base.Model{ID: 1}, GroupID: &groupID}
	visit := &active.Visit{ActiveGroup: &active.Group{RoomID: 1}}
	result := s.ShouldShowDailyCheckoutWithGroup(context.Background(), student, visit)
	assert.False(t, result, "Should return false when education service errors")
}

// =============================================================================
// Schulhof daily-checkout TESTS (#2377)
//
// A child leaving from the schoolyard must be OFFERED "nach Hause" without
// being sent home automatically: PyrePortal renders its destination modal only
// for action "checked_out", so the availability flag and the action upgrade
// must disagree at the Schulhof.
// =============================================================================

// newSchulhofScenario builds the GS-Barnstorf shape: the student's group owns a
// room, and the visit being closed happened in the Schulhof room instead.
func newSchulhofScenario(t *testing.T) (*CheckinService, *users.Student, *active.Visit) {
	t.Helper()
	_ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME")

	const groupRoomID, schulhofRoomID = int64(42), int64(7)
	groupRoom := groupRoomID

	s := &CheckinService{
		settings:   newMockSettingsService(map[string]string{}, nil, nil),
		education:  &mockEducationService{group: &education.Group{Model: base.Model{ID: 1}, RoomID: &groupRoom}},
		facilities: newSchulhofFacilities(schulhofRoomID),
	}
	groupID := int64(1)
	student := &users.Student{Model: base.Model{ID: 1}, GroupID: &groupID}
	visit := &active.Visit{ActiveGroup: &active.Group{RoomID: schulhofRoomID}}
	return s, student, visit
}

// Deliberately NOT parallel: the test reaches process-global state (env
// variables, viper keys, the settings registry, os.Stdout) that the whole
// test binary shares.
func TestShouldShowDailyCheckoutWithGroup_SchulhofRoom_Offered(t *testing.T) {
	s, student, visit := newSchulhofScenario(t)

	result := s.ShouldShowDailyCheckoutWithGroup(context.Background(), student, visit)
	assert.True(t, result, "nach Hause must be offered when checking out from the Schulhof")
}

// Deliberately NOT parallel: the test reaches process-global state (env
// variables, viper keys, the settings registry, os.Stdout) that the whole
// test binary shares.
func TestShouldUpgradeToDailyCheckout_SchulhofRoom_NoAutoUpgrade(t *testing.T) {
	s, student, visit := newSchulhofScenario(t)

	result := s.ShouldUpgradeToDailyCheckout(context.Background(), "checked_out", student, visit)
	assert.False(t, result,
		"the Schulhof must not auto-send the child home — the action has to stay checked_out so PyrePortal renders the destination modal")
}

func TestShouldShowDailyCheckoutWithGroup_SchulhofRoom_BeforeCheckoutTime(t *testing.T) {
	s, student, visit := newSchulhofScenario(t)
	// A configured time in the future must still suppress the offer: the
	// Schulhof relaxes the ROOM gate, never the time gate.
	require.NoError(t, os.Setenv("STUDENT_DAILY_CHECKOUT_TIME", "23:59"))
	defer func() { _ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME") }()

	result := s.ShouldShowDailyCheckoutWithGroup(context.Background(), student, visit)
	assert.False(t, result, "the time gate still applies at the Schulhof")
}

// Deliberately NOT parallel: the test reaches process-global state (env
// variables, viper keys, the settings registry, os.Stdout) that the whole
// test binary shares.
func TestShouldShowDailyCheckoutWithGroup_OrdinaryRoom_NotOffered(t *testing.T) {
	s, student, _ := newSchulhofScenario(t)
	// Same school, but the child left an ordinary room that is neither their
	// group room nor the yard.
	visit := &active.Visit{ActiveGroup: &active.Group{RoomID: 99}}

	result := s.ShouldShowDailyCheckoutWithGroup(context.Background(), student, visit)
	assert.False(t, result, "an ordinary room must not offer nach Hause")
}

// Deliberately NOT parallel: the test reaches process-global state (env
// variables, viper keys, the settings registry, os.Stdout) that the whole
// test binary shares.
func TestShouldShowDailyCheckoutWithGroup_NoSchulhofRoomProvisioned(t *testing.T) {
	s, student, visit := newSchulhofScenario(t)
	// A school that never enabled the yard has no Schulhof room; the lookup
	// failing must not make the scan fail.
	s.facilities = &mockFacilitiesService{
		err: &facilitiesSvc.FacilitiesError{Op: "find room by name", Err: facilitiesSvc.ErrRoomNotFound},
	}

	result := s.ShouldShowDailyCheckoutWithGroup(context.Background(), student, visit)
	assert.False(t, result, "a missing Schulhof room means not offered, not an error")
}

// Deliberately NOT parallel: the test reaches process-global state (env
// variables, viper keys, the settings registry, os.Stdout) that the whole
// test binary shares.
func TestShouldShowDailyCheckoutWithGroup_NonCanonicalSchulhofRoom(t *testing.T) {
	s, student, visit := newSchulhofScenario(t)
	// An unprotected room that merely matches case-insensitively must not be
	// adopted as the yard (FindCanonicalSchulhofRoom rejects it).
	s.facilities = &mockFacilitiesService{
		room: &facilityModels.Room{
			Model:    base.Model{ID: 7},
			Name:     constants.SchulhofRoomName,
			IsSystem: false,
		},
	}

	result := s.ShouldShowDailyCheckoutWithGroup(context.Background(), student, visit)
	assert.False(t, result, "a non-system room named Schulhof must not unlock nach Hause")
}

// Deliberately NOT parallel: the test reaches process-global state (env
// variables, viper keys, the settings registry, os.Stdout) that the whole
// test binary shares.
func TestShouldShowDailyCheckoutWithGroup_NilFacilitiesService(t *testing.T) {
	s, student, visit := newSchulhofScenario(t)
	s.facilities = nil

	result := s.ShouldShowDailyCheckoutWithGroup(context.Background(), student, visit)
	assert.False(t, result, "a nil facilities service must not panic the checkout gate")
}

// Deliberately NOT parallel: the test reaches process-global state (env
// variables, viper keys, the settings registry, os.Stdout) that the whole
// test binary shares.
func TestShouldUpgradeToDailyCheckout_OwnGroupRoom_StillUpgrades(t *testing.T) {
	s, student, _ := newSchulhofScenario(t)
	// Leaving the child's OWN group room keeps the pre-existing automatic
	// daily checkout — this fix must not change that path.
	visit := &active.Visit{ActiveGroup: &active.Group{RoomID: 42}}

	result := s.ShouldUpgradeToDailyCheckout(context.Background(), "checked_out", student, visit)
	assert.True(t, result, "the own-group-room auto-upgrade is unchanged")
}

// =============================================================================
// IsAfterCheckoutTimeGate TESTS
// =============================================================================

// Deliberately NOT parallel: the test reaches process-global state (env
// variables, viper keys, the settings registry, os.Stdout) that the whole
// test binary shares.
func TestIsAfterCheckoutTimeGate_PerStudentDisabled_FallsBackToGlobal(t *testing.T) {
	// Per-student disabled (default) → should use global checkout time
	_ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME")

	s := &CheckinService{
		settings: newMockSettingsService(map[string]string{}, nil, nil),
	}
	student := &users.Student{Model: base.Model{ID: 1}}

	// No global time configured → always available
	result := s.IsAfterCheckoutTimeGate(context.Background(), student)
	assert.True(t, result, "should return true when per-student disabled and no global time")
}

func TestIsAfterCheckoutTimeGate_PerStudentDisabled_GlobalTimeInFuture(t *testing.T) {
	// Set a global time far in the future
	require.NoError(t, os.Setenv("STUDENT_DAILY_CHECKOUT_TIME", "23:59"))
	defer func() { _ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME") }()

	s := &CheckinService{
		settings: newMockSettingsService(map[string]string{}, nil, nil),
	}
	student := &users.Student{Model: base.Model{ID: 1}}

	result := s.IsAfterCheckoutTimeGate(context.Background(), student)
	assert.False(t, result, "should return false when per-student disabled and global time is in future")
}

// Deliberately NOT parallel: the test reaches process-global state (env
// variables, viper keys, the settings registry, os.Stdout) that the whole
// test binary shares.
func TestIsAfterCheckoutTimeGate_PerStudentEnabled_BeforeDelta(t *testing.T) {
	_ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME")

	// Pin "now" to a deterministic midday so the +2h pickup offset never
	// wraps past midnight. The function projects pickup onto today's date,
	// so a wrapped hour would otherwise land in the past and flip the
	// assertion when CI runs late in the day.
	fixedNow := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	restore := overrideTimeNow(fixedNow)
	defer restore()

	// Pickup time 2 hours from fixed now, delta 15 min → too early.
	pickupTime := time.Date(2000, 1, 1, fixedNow.Hour()+2, 0, 0, 0, fixedNow.Location())

	s := &CheckinService{
		settings: newMockSettingsService(
			map[string]string{},
			map[string]bool{"operations.per_student_checkout_enabled": true},
			map[string]int{"operations.per_student_checkout_delta_minutes": 15},
		),
		pickup: &mockPickupScheduleService{
			effectivePickupTime: &scheduleSvc.EffectivePickupTime{PickupTime: &pickupTime},
		},
	}
	student := &users.Student{Model: base.Model{ID: 1}}

	result := s.IsAfterCheckoutTimeGate(context.Background(), student)
	assert.False(t, result, "should return false when current time is before pickup_time - delta")
}

// Deliberately NOT parallel: the test reaches process-global state (env
// variables, viper keys, the settings registry, os.Stdout) that the whole
// test binary shares.
func TestIsAfterCheckoutTimeGate_PerStudentEnabled_AfterDelta(t *testing.T) {
	_ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME")

	// Pickup time 5 minutes from now, delta 15 min → already past threshold
	now := time.Now()
	pickupTime := time.Date(2000, 1, 1, now.Hour(), now.Minute()+5, 0, 0, now.Location())

	s := &CheckinService{
		settings: newMockSettingsService(
			map[string]string{},
			map[string]bool{"operations.per_student_checkout_enabled": true},
			map[string]int{"operations.per_student_checkout_delta_minutes": 15},
		),
		pickup: &mockPickupScheduleService{
			effectivePickupTime: &scheduleSvc.EffectivePickupTime{PickupTime: &pickupTime},
		},
	}
	student := &users.Student{Model: base.Model{ID: 1}}

	result := s.IsAfterCheckoutTimeGate(context.Background(), student)
	assert.True(t, result, "should return true when current time is after pickup_time - delta")
}

// Deliberately NOT parallel: the test reaches process-global state (env
// variables, viper keys, the settings registry, os.Stdout) that the whole
// test binary shares.
func TestIsAfterCheckoutTimeGate_PerStudentEnabled_NoPickupTime_FallsBackToGlobal(t *testing.T) {
	_ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME")

	// Per-student enabled but student has no pickup time → fall back to global (no global = always available)
	s := &CheckinService{
		settings: newMockSettingsService(
			map[string]string{},
			map[string]bool{"operations.per_student_checkout_enabled": true},
			nil,
		),
		pickup: &mockPickupScheduleService{
			effectivePickupTime: &scheduleSvc.EffectivePickupTime{PickupTime: nil},
		},
	}
	student := &users.Student{Model: base.Model{ID: 1}}

	result := s.IsAfterCheckoutTimeGate(context.Background(), student)
	assert.True(t, result, "should fall back to global (no global = always available) when student has no pickup time")
}

// Deliberately NOT parallel: the test reaches process-global state (env
// variables, viper keys, the settings registry, os.Stdout) that the whole
// test binary shares.
func TestIsAfterCheckoutTimeGate_PerStudentEnabled_PickupServiceError_FallsBackToGlobal(t *testing.T) {
	_ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME")

	s := &CheckinService{
		settings: newMockSettingsService(
			map[string]string{},
			map[string]bool{"operations.per_student_checkout_enabled": true},
			nil,
		),
		pickup: &mockPickupScheduleService{
			err: fmt.Errorf("db error"),
		},
	}
	student := &users.Student{Model: base.Model{ID: 1}}

	result := s.IsAfterCheckoutTimeGate(context.Background(), student)
	assert.True(t, result, "should fall back to global when pickup service errors")
}

// Deliberately NOT parallel: the test reaches process-global state (env
// variables, viper keys, the settings registry, os.Stdout) that the whole
// test binary shares.
func TestIsAfterCheckoutTimeGate_PerStudentEnabled_NilPickupService_FallsBackToGlobal(t *testing.T) {
	_ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME")

	s := &CheckinService{
		settings: newMockSettingsService(
			map[string]string{},
			map[string]bool{"operations.per_student_checkout_enabled": true},
			nil,
		),
		// pickup is nil
	}
	student := &users.Student{Model: base.Model{ID: 1}}

	result := s.IsAfterCheckoutTimeGate(context.Background(), student)
	assert.True(t, result, "should fall back to global when PickupScheduleService is nil")
}

// Deliberately NOT parallel: the test reaches process-global state (env
// variables, viper keys, the settings registry, os.Stdout) that the whole
// test binary shares.
func TestIsAfterCheckoutTimeGate_PerStudentEnabled_DeltaZero(t *testing.T) {
	_ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME")

	// Pickup time 1 minute from now, delta 0 → not yet
	now := time.Now()
	pickupTime := time.Date(2000, 1, 1, now.Hour(), now.Minute()+1, 0, 0, now.Location())

	s := &CheckinService{
		settings: newMockSettingsService(
			map[string]string{},
			map[string]bool{"operations.per_student_checkout_enabled": true},
			map[string]int{"operations.per_student_checkout_delta_minutes": 0},
		),
		pickup: &mockPickupScheduleService{
			effectivePickupTime: &scheduleSvc.EffectivePickupTime{PickupTime: &pickupTime},
		},
	}
	student := &users.Student{Model: base.Model{ID: 1}}

	result := s.IsAfterCheckoutTimeGate(context.Background(), student)
	assert.False(t, result, "should return false when delta is 0 and current time is before pickup time")
}

// Deliberately NOT parallel: the test reaches process-global state (env
// variables, viper keys, the settings registry, os.Stdout) that the whole
// test binary shares.
func TestIsAfterCheckoutTimeGate_NilSettingsService(t *testing.T) {
	_ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME")

	s := &CheckinService{}
	student := &users.Student{Model: base.Model{ID: 1}}

	// No settings service → perStudentEnabled defaults to false → no global time → always available
	result := s.IsAfterCheckoutTimeGate(context.Background(), student)
	assert.True(t, result, "should return true when SettingsService is nil and no global time")
}
