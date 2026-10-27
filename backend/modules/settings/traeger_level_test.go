package settings_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/settings/records"
	api "github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// traegerSettingsRight is the Träger right "Einstellungen" the root maps
// records.TraegerRightSettings to (identityaccess.TraegerRightSettingsManage).
const traegerSettingsRight = "traeger_settings:manage"

type traegerLevelFixture struct {
	db                         *bun.DB
	module                     api.SettingsTestModule
	ctx                        context.Context
	traeger, otherTraeger      int64
	first, second, otherSchool int64
	account                    int64
}

func newTraegerLevelFixture(t *testing.T) traegerLevelFixture {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, err := api.NewSettingsTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	suffix := testpkg.UniqueTestTenantID(t)
	traeger := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: "Träger Nord", Slug: fmt.Sprintf("traeger-nord-%d", suffix), Active: true}).ID
	otherTraeger := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: "Träger Süd", Slug: fmt.Sprintf("traeger-sued-%d", suffix), Active: true}).ID
	return traegerLevelFixture{
		db: db, module: module, ctx: testpkg.WithPackageTenantRuntime(context.Background()),
		traeger: traeger, otherTraeger: otherTraeger,
		first:       testpkg.CreateTestSchoolInOrganization(t, db, traeger, "Schule Eins"),
		second:      testpkg.CreateTestSchoolInOrganization(t, db, traeger, "Schule Zwei"),
		otherSchool: testpkg.CreateTestSchoolInOrganization(t, db, otherTraeger, "Schule Süd"),
		account:     testpkg.CreateTestAccount(t, db, "traeger-einstellungen").ID,
	}
}

func (f traegerLevelFixture) resolve(t *testing.T, school int64, key string) bool {
	t.Helper()
	value, err := f.module.Settings.ResolveBoolForTenant(f.ctx, school, key)
	require.NoError(t, err)
	return value
}

func (f traegerLevelFixture) schemaItem(t *testing.T, school int64, key string) *records.ResolvedSetting {
	t.Helper()
	schema, err := testpkg.TenantQuery1(t, testpkg.ContextForTenant(f.ctx, school), f.db, f.module.Settings.GetSchema, []string{"config:read", "config:update", "config:manage"})
	require.NoError(t, err)
	for _, tab := range schema.Tabs {
		for _, category := range tab.Categories {
			for _, item := range category.Items {
				if item.Key == key {
					return item
				}
			}
		}
	}
	t.Fatalf("schema of school %d has no %s", school, key)
	return nil
}

func (f traegerLevelFixture) count(t *testing.T, query string, args ...any) int {
	t.Helper()
	var count int
	require.NoError(t, f.db.NewRaw(query, args...).Scan(context.Background(), &count))
	return count
}

// TestTraegerValueReachesEverySchoolOfItsTraegerOnly is the acceptance test of
// #4652: the Träger value works in all its schools, a deviation only in the
// one school, another Träger stays untouched, a new school inherits the value
// without copying, and every change stands in the change log of its level.
func TestTraegerValueReachesEverySchoolOfItsTraegerOnly(t *testing.T) {
	t.Parallel()
	f := newTraegerLevelFixture(t)
	key := records.KeyParentNewsEnabled
	definition := records.GetDefinition(key)
	require.Equal(t, records.LevelBoth, definition.Levels)
	require.Equal(t, true, definition.Default)

	require.NoError(t, f.module.Traeger.SetValue(f.ctx, f.traeger, f.account, []string{traegerSettingsRight}, key, false))
	assert.False(t, f.resolve(t, f.first, key), "the Träger value reaches the first school")
	assert.False(t, f.resolve(t, f.second, key), "the Träger value reaches the second school")
	assert.True(t, f.resolve(t, f.otherSchool, key), "another Träger keeps the registry default")

	require.NoError(t, testpkg.WithinTenantContext(t, testpkg.ContextForTenant(f.ctx, f.second), f.db, f.second, func(ctx context.Context) error {
		return f.module.Settings.SetValue(ctx, key, true, &f.account, nil)
	}))
	assert.False(t, f.resolve(t, f.first, key), "the deviation stays in its school")
	assert.True(t, f.resolve(t, f.second, key), "the school deviates from its Träger")

	newSchool := testpkg.CreateTestSchoolInOrganization(t, f.db, f.traeger, "Neue Schule")
	assert.False(t, f.resolve(t, newSchool, key), "a new school of the Träger inherits its value")
	assert.Zero(t, f.count(t, `SELECT COUNT(*) FROM config.setting_values WHERE tenant_id = ?`, newSchool), "nothing is copied into the new school")

	batch, err := f.module.Settings.ResolveManyForTenants(f.ctx, []int64{f.first, f.second, newSchool, f.otherSchool}, []string{key})
	require.NoError(t, err)
	for school, want := range map[int64]bool{f.first: false, f.second: true, newSchool: false, f.otherSchool: true} {
		got, boolErr := batch[school].Bool(key)
		require.NoError(t, boolErr)
		assert.Equal(t, want, got, "batch read of school %d resolves the same levels", school)
	}

	first := f.schemaItem(t, f.first, key)
	assert.Equal(t, records.SourceTraeger, first.Source, "the school sees that the value comes from the Träger")
	assert.NotNil(t, first.TraegerChangedAt)
	assert.True(t, first.IsDefault, "the school has nothing of its own to reset")
	second := f.schemaItem(t, f.second, key)
	assert.Equal(t, records.SourceSchool, second.Source)
	assert.NotNil(t, second.TraegerChangedAt, "the school sees the Träger value beneath its deviation")
	assert.False(t, second.IsDefault)

	require.NoError(t, f.module.Traeger.ResetValue(f.ctx, f.traeger, f.account, []string{traegerSettingsRight}, key))
	assert.True(t, f.resolve(t, f.first, key), "without the Träger value the registry default applies again")
	assert.True(t, f.resolve(t, f.second, key), "the deviation stays after the Träger reset")
	assert.Equal(t, records.SourceDefault, f.schemaItem(t, f.first, key).Source)

	assert.Equal(t, 2, f.count(t, `SELECT COUNT(*) FROM config.traeger_setting_audit WHERE organization_id = ? AND setting_key = ? AND changed_by = ?`, f.traeger, key, f.account),
		"set and reset stand in the Träger change log")
	assert.Equal(t, 1, f.count(t, `SELECT COUNT(*) FROM config.traeger_setting_audit WHERE organization_id = ? AND old_value IS NOT NULL AND new_value IS NULL AND action = 'reset'`, f.traeger))
	assert.Zero(t, f.count(t, `SELECT COUNT(*) FROM config.traeger_setting_audit WHERE organization_id = ?`, f.otherTraeger))
	assert.Equal(t, 1, f.count(t, `SELECT COUNT(*) FROM config.setting_audit WHERE tenant_id = ? AND setting_key = ?`, f.second, key), "the deviation stands in the school's change log")
	assert.Zero(t, f.count(t, `SELECT COUNT(*) FROM config.setting_audit WHERE tenant_id = ? AND setting_key = ?`, f.first, key), "schools get no copy of the Träger change")
}

// TestTraegerSettingsCheckTheRightAndTheLevel: a session without the right of
// the setting gets a permission error, a school-only setting does not exist
// at the Träger, and an invalid value is rejected without a change.
func TestTraegerSettingsCheckTheRightAndTheLevel(t *testing.T) {
	t.Parallel()
	f := newTraegerLevelFixture(t)
	err := f.module.Traeger.SetValue(f.ctx, f.traeger, f.account, []string{"traeger_overview:read"}, records.KeyParentNewsEnabled, false)
	assert.ErrorIs(t, err, records.ErrPermissionDenied)
	assert.Equal(t, "forbidden", f.module.Traeger.ClassifyError(err))
	err = f.module.Traeger.SetValue(f.ctx, f.traeger, f.account, []string{traegerSettingsRight}, records.KeyMFAMode, "off")
	assert.ErrorIs(t, err, records.ErrDefinitionNotFound, "a school-only setting has no Träger value")
	err = f.module.Traeger.SetValue(f.ctx, f.traeger, f.account, []string{traegerSettingsRight}, records.KeyParentNewsEnabled, "nein")
	assert.ErrorIs(t, err, records.ErrInvalidValue)
	assert.Zero(t, f.count(t, `SELECT COUNT(*) FROM config.traeger_setting_values WHERE organization_id = ?`, f.traeger))
	assert.Zero(t, f.count(t, `SELECT COUNT(*) FROM config.traeger_setting_audit WHERE organization_id = ?`, f.traeger))
}

// TestTraegerSchemaShowsOnlyTraegerSettingsWithTheirSource: the Träger sees
// every Träger-level setting, its effective value and where it comes from,
// writable only with the right, and nothing of another Träger.
func TestTraegerSchemaShowsOnlyTraegerSettingsWithTheirSource(t *testing.T) {
	t.Parallel()
	f := newTraegerLevelFixture(t)
	require.NoError(t, f.module.Traeger.SetValue(f.ctx, f.otherTraeger, f.account, []string{traegerSettingsRight}, records.KeyParentNewsEnabled, false))
	require.NoError(t, f.module.Traeger.SetValue(f.ctx, f.traeger, f.account, []string{traegerSettingsRight}, records.KeyEnrollmentLegalAGBText, "AGB des Trägers"))

	items := func(rights []string) map[string]*records.ResolvedSetting {
		raw, err := f.module.Traeger.Schema(f.ctx, f.traeger, rights)
		require.NoError(t, err)
		schema, ok := raw.(*records.SettingsSchema)
		require.True(t, ok)
		result := map[string]*records.ResolvedSetting{}
		for _, tab := range schema.Tabs {
			for _, category := range tab.Categories {
				for _, item := range category.Items {
					result[item.Key] = item
				}
			}
		}
		return result
	}
	writer := items([]string{"traeger_overview:read", traegerSettingsRight})
	for key, item := range writer {
		assert.True(t, records.GetDefinition(key).Levels.AtTraeger(), "%s is a Träger-level setting", key)
		assert.True(t, item.Writable, "%s is writable with the right", key)
	}
	require.Contains(t, writer, records.KeyEnrollmentLegalAGBText)
	assert.NotContains(t, writer, records.KeyMFAMode)
	assert.Equal(t, "AGB des Trägers", writer[records.KeyEnrollmentLegalAGBText].Value)
	assert.Equal(t, records.SourceTraeger, writer[records.KeyEnrollmentLegalAGBText].Source)
	assert.Equal(t, true, writer[records.KeyParentNewsEnabled].Value, "the value of another Träger does not show")
	assert.Equal(t, records.SourceDefault, writer[records.KeyParentNewsEnabled].Source)
	for key, item := range items([]string{"traeger_overview:read"}) {
		assert.False(t, item.Writable, "%s is read-only without the right", key)
	}
}

// TestTraegerOnlySettingsRefuseSchoolOverrides: a school can neither write a
// Träger-only setting nor see a stale override of it.
func TestTraegerOnlySettingsRefuseSchoolOverrides(t *testing.T) {
	t.Parallel()
	registry := records.NewRegistry()
	registry.Register(records.Definition{Key: "traeger.only", Type: records.FieldBoolean, Default: false, Tab: "operations", Category: "test", Levels: records.LevelTraeger})
	snapshot, err := records.NewLayeredSettingsSnapshot(registry, 7, []string{"traeger.only"},
		[]*records.SettingValue{{TenantID: 7, SettingKey: "traeger.only", Value: []byte("false")}},
		[]*records.TraegerSettingValue{{OrganizationID: 3, SettingKey: "traeger.only", Value: []byte("true")}})
	require.NoError(t, err)
	value, err := snapshot.Bool("traeger.only")
	require.NoError(t, err)
	assert.True(t, value, "a school override of a Träger-only setting is ignored")
	source, _, _, err := snapshot.Layer("traeger.only")
	require.NoError(t, err)
	assert.Equal(t, records.SourceTraeger, source)
}

// TestTraegerChangeDropsChangesOlderThanTheRetention: the Träger change log
// keeps three years like the school's; a change drops older entries of its
// own Träger only.
func TestTraegerChangeDropsChangesOlderThanTheRetention(t *testing.T) {
	t.Parallel()
	f := newTraegerLevelFixture(t)
	for _, organization := range []int64{f.traeger, f.otherTraeger} {
		_, err := f.db.ExecContext(context.Background(), `INSERT INTO config.traeger_setting_audit (organization_id, setting_key, new_value, action, changed_at)
			VALUES (?, ?, 'true', 'set', NOW() - INTERVAL '3 years 1 day'), (?, ?, 'true', 'set', NOW() - INTERVAL '2 years')`,
			organization, records.KeyParentNewsEnabled, organization, records.KeyParentNewsEnabled)
		require.NoError(t, err)
	}
	require.NoError(t, f.module.Traeger.SetValue(f.ctx, f.traeger, f.account, []string{traegerSettingsRight}, records.KeyParentNewsEnabled, false))
	assert.Equal(t, 2, f.count(t, `SELECT COUNT(*) FROM config.traeger_setting_audit WHERE organization_id = ?`, f.traeger), "the expired entry goes, the recent one and the new one stay")
	assert.Equal(t, 2, f.count(t, `SELECT COUNT(*) FROM config.traeger_setting_audit WHERE organization_id = ?`, f.otherTraeger), "another Träger keeps its entries")
}
