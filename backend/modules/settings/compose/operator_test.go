package compose_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/settings"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/uptrace/bun"
)

// The operator's school settings capability over the retained settings
// service and a real tenant transaction (#2736). The HTTP mapping of these
// results is covered in modules/settings/inbound/operator.

const (
	sessionEndTimeKey  = "operations.session_end_time"
	schulhofEnabledKey = "checkout.schulhof_enabled"
	ogsDevicePINKey    = "security.ogs_device_pin"
	presenceModeBinary = "binary"
)

type schemaItem struct {
	Key      string `json:"key"`
	Writable bool   `json:"writable"`
}

type operatorSchema struct {
	Tabs []struct {
		Categories []struct {
			Items []schemaItem `json:"items"`
		} `json:"categories"`
	} `json:"tabs"`
}

// changedBy is the account a test's writes are recorded against.
func changedBy(t *testing.T, db *bun.DB) int64 {
	t.Helper()
	return testpkg.CreateTestAccount(t, db, "operator-settings").ID
}

func (s operatorSchema) items() []schemaItem {
	var items []schemaItem
	for _, tab := range s.Tabs {
		for _, category := range tab.Categories {
			items = append(items, category.Items...)
		}
	}
	return items
}

func TestOperatorSchoolSettings_SchemaShowsEverySharedSettingWritable(t *testing.T) {
	t.Parallel()
	_, module := testutil.SetupOperatorSettingsModule(t)
	schoolSettings := module.OperatorSchoolSettings(nil)

	raw, err := schoolSettings.Schema(testpkg.Ctx(t), testpkg.Tenant(t))
	require.NoError(t, err)

	var schema operatorSchema
	require.NoError(t, json.Unmarshal(raw, &schema))
	items := schema.items()
	require.NotEmpty(t, items, "operators see every registered setting")
	for _, item := range items {
		assert.True(t, item.Writable, "operator should have writable=true on %s", item.Key)
		assert.NotEqual(t, ogsDevicePINKey, item.Key, "the schema must not include the admin-only PIN")
	}
}

func TestOperatorSchoolSettings_CheckOperatorWritable(t *testing.T) {
	t.Parallel()
	_, module := testutil.SetupOperatorSettingsModule(t)
	schoolSettings := module.OperatorSchoolSettings(nil)

	require.NoError(t, schoolSettings.CheckOperatorWritable(sessionEndTimeKey))
	assert.Error(t, schoolSettings.CheckOperatorWritable(ogsDevicePINKey), "admin-only settings are refused")
	_, unknown := errors.AsType[*settings.DefinitionNotFoundError](schoolSettings.CheckOperatorWritable("nonexistent.key"))
	assert.True(t, unknown, "an unknown key reports a missing definition")
}

func TestOperatorSchoolSettings_SetRevealReset(t *testing.T) {
	t.Parallel()
	db, module := testutil.SetupOperatorSettingsModule(t)
	schoolSettings := module.OperatorSchoolSettings(nil)
	ctx, schoolID, actor := testpkg.Ctx(t), testpkg.Tenant(t), changedBy(t, db)

	initial, err := schoolSettings.Reveal(ctx, schoolID, sessionEndTimeKey)
	require.NoError(t, err)
	require.NotEqual(t, "18:30", initial, "the test value must differ from the default")

	require.NoError(t, schoolSettings.SetValue(ctx, schoolID, sessionEndTimeKey, "18:30", actor, false))
	value, err := schoolSettings.Reveal(ctx, schoolID, sessionEndTimeKey)
	require.NoError(t, err)
	assert.Equal(t, "18:30", value)

	require.NoError(t, schoolSettings.ResetValue(ctx, schoolID, sessionEndTimeKey, actor))
	value, err = schoolSettings.Reveal(ctx, schoolID, sessionEndTimeKey)
	require.NoError(t, err)
	assert.Equal(t, initial, value, "reset restores the registry default")
}

func TestOperatorSchoolSettings_SettingsErrors(t *testing.T) {
	t.Parallel()
	db, module := testutil.SetupOperatorSettingsModule(t)
	schoolSettings := module.OperatorSchoolSettings(nil)
	ctx, schoolID, actor := testpkg.Ctx(t), testpkg.Tenant(t), changedBy(t, db)

	_, err := schoolSettings.Reveal(ctx, schoolID, "nonexistent.key")
	settingsErr, ok := errors.AsType[*settings.SettingsError](err)
	require.True(t, ok, "got %v", err)
	_, notFound := errors.AsType[*settings.DefinitionNotFoundError](settingsErr.Unwrap())
	assert.True(t, notFound)

	err = schoolSettings.SetValue(ctx, schoolID, "operations.session_end_enabled", "not-a-boolean", actor, false)
	settingsErr, ok = errors.AsType[*settings.SettingsError](err)
	require.True(t, ok, "got %v", err)
	_, invalid := errors.AsType[*settings.InvalidValueError](settingsErr.Unwrap())
	assert.True(t, invalid)
}

func TestOperatorSchoolSettings_SetValueRunsHookAndBroadcastAfterCommit(t *testing.T) {
	t.Parallel()
	db, module := testutil.SetupOperatorSettingsModule(t)
	ctx, schoolID, actor := testpkg.Ctx(t), testpkg.Tenant(t), changedBy(t, db)

	var events []string
	var hookTenant int64
	var hookValue any
	var notifiedTenant int64
	var notifiedKey string
	schoolSettings := module.OperatorSchoolSettingsWith(testutil.OperatorSchoolSettingsOptions{
		Notify: func(_ context.Context, tenantID int64, key string) {
			events = append(events, "broadcast")
			notifiedTenant, notifiedKey = tenantID, key
		},
		OnValueSet: func(_ context.Context, tenantID int64, _ string, value any) (func(), error) {
			events = append(events, "hook")
			hookTenant, hookValue = tenantID, value
			return func() { events = append(events, "post-commit") }, nil
		},
	})

	require.NoError(t, schoolSettings.SetValue(ctx, schoolID, schulhofEnabledKey, true, actor, false))

	assert.Equal(t, []string{"hook", "post-commit", "broadcast"}, events)
	assert.Equal(t, schoolID, hookTenant, "the hook receives the school")
	assert.Equal(t, true, hookValue)
	assert.Equal(t, schoolID, notifiedTenant)
	assert.Equal(t, schulhofEnabledKey, notifiedKey)
}

func TestOperatorSchoolSettings_HookErrorRollsBackWithoutBroadcast(t *testing.T) {
	t.Parallel()
	db, module := testutil.SetupOperatorSettingsModule(t)
	ctx, schoolID, actor := testpkg.Ctx(t), testpkg.Tenant(t), changedBy(t, db)

	var broadcasts int
	schoolSettings := module.OperatorSchoolSettingsWith(testutil.OperatorSchoolSettingsOptions{
		Notify: func(context.Context, int64, string) { broadcasts++ },
		OnValueSet: func(context.Context, int64, string, any) (func(), error) {
			return nil, errors.New("hook rejected the change")
		},
	})
	before, err := schoolSettings.Reveal(ctx, schoolID, schulhofEnabledKey)
	require.NoError(t, err)
	require.NotEqual(t, true, before, "the test value must differ from the default")

	require.Error(t, schoolSettings.SetValue(ctx, schoolID, schulhofEnabledKey, true, actor, false))

	after, err := schoolSettings.Reveal(ctx, schoolID, schulhofEnabledKey)
	require.NoError(t, err)
	assert.Equal(t, before, after, "a failed hook rolls the write back")
	assert.Zero(t, broadcasts, "a rolled back write is not broadcast")
}

func TestOperatorSchoolSettings_ResetReplaysHookOnlyForPhotoFlag(t *testing.T) {
	t.Parallel()
	db, module := testutil.SetupOperatorSettingsModule(t)
	ctx, schoolID, actor := testpkg.Ctx(t), testpkg.Tenant(t), changedBy(t, db)

	var hookKeys []string
	var hookValues []any
	schoolSettings := module.OperatorSchoolSettingsWith(testutil.OperatorSchoolSettingsOptions{
		OnValueSet: func(_ context.Context, _ int64, key string, value any) (func(), error) {
			hookKeys = append(hookKeys, key)
			hookValues = append(hookValues, value)
			return nil, nil
		},
	})
	require.NoError(t, schoolSettings.SetValue(ctx, schoolID, schulhofEnabledKey, true, actor, false))
	require.NoError(t, schoolSettings.SetValue(ctx, schoolID, settings.KeyStudentPhotosEnabled, true, actor, false))
	hookKeys, hookValues = nil, nil

	require.NoError(t, schoolSettings.ResetValue(ctx, schoolID, schulhofEnabledKey, actor))
	assert.Empty(t, hookKeys, "a non-photo reset does not replay the hook")

	require.NoError(t, schoolSettings.ResetValue(ctx, schoolID, settings.KeyStudentPhotosEnabled, actor))
	assert.Equal(t, []string{settings.KeyStudentPhotosEnabled}, hookKeys)
	assert.Equal(t, []any{false}, hookValues, "the photo reset replays the hook with the registry default")
}

func TestOperatorSchoolSettings_PresenceModeGuard(t *testing.T) {
	t.Parallel()
	db, module := testutil.SetupOperatorSettingsModule(t)
	schoolSettings := module.OperatorSchoolSettings(nil)
	ctx, schoolID, actor := testpkg.Ctx(t), testpkg.Tenant(t), changedBy(t, db)

	student := testpkg.CreateTestStudent(t, db, "Guard", "Compose", "9c")
	staff := testpkg.CreateTestStaff(t, db, "Guard", "Compose")
	device := testpkg.CreateTestDevice(t, db, "guard-compose-device")
	testpkg.CreateTestAttendance(t, db, student.ID, staff.ID, device.ID, time.Now().Add(-time.Hour), nil)

	err := schoolSettings.SetValue(ctx, schoolID, settings.KeyPresenceMode, presenceModeBinary, actor, false)
	require.ErrorIs(t, err, settings.ErrPresenceModeSwitchBlocked)
	value, err := schoolSettings.Reveal(ctx, schoolID, settings.KeyPresenceMode)
	require.NoError(t, err)
	assert.Equal(t, settings.PresenceModeDetailed, value, "the blocked switch keeps the mode")

	require.NoError(t, schoolSettings.SetValue(ctx, schoolID, settings.KeyPresenceMode, presenceModeBinary, actor, true),
		"force bypasses the guard")
	value, err = schoolSettings.Reveal(ctx, schoolID, settings.KeyPresenceMode)
	require.NoError(t, err)
	assert.Equal(t, presenceModeBinary, value)
}

func TestOperatorSchoolSettings_BookingAuthorityImpact(t *testing.T) {
	t.Parallel()
	_, module := testutil.SetupOperatorSettingsModule(t)

	impact, err := module.OperatorSchoolSettings(nil).BookingAuthorityImpact(testpkg.Ctx(t), testpkg.Tenant(t))
	require.NoError(t, err)
	require.NotNil(t, impact)
	assert.Empty(t, impact.BlockingChildren, "a school without children has nothing blocking")
}

func TestOperatorSchoolSettings_BookingAuthorityImpactUnavailableWithoutCarePlan(t *testing.T) {
	t.Parallel()
	_, module := testutil.SetupOperatorSettingsModule(t)
	schoolSettings := module.OperatorSchoolSettingsWith(testutil.OperatorSchoolSettingsOptions{WithoutCarePlan: true})

	impact, err := schoolSettings.BookingAuthorityImpact(testpkg.Ctx(t), testpkg.Tenant(t))
	require.ErrorIs(t, err, settings.ErrBookingAuthorityImpactUnavailable)
	assert.Nil(t, impact)
}
