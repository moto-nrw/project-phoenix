package migrations

// Coverage for the 1.15.166 silent departure-field cutover: lineages whose
// latest form-schema version still declares legacy departure fields
// (student.departure / student.bus / student.bus_days /
// student.pickup_status) get a NEW version with one
// student.allowed_departure_modes field, phases advance onto it, and old
// versions stay byte-identical so pinned requests keep rendering.

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type departureTestCondition struct {
	Source   string `json:"source"`
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    any    `json:"value"`
}

type departureTestField struct {
	Key         string                  `json:"key"`
	Label       string                  `json:"label"`
	Type        string                  `json:"type"`
	Required    bool                    `json:"required"`
	AppliesToCh bool                    `json:"applies_to_child"`
	SortOrder   int                     `json:"sort_order"`
	Target      string                  `json:"target"`
	VisibleWhen *departureTestCondition `json:"visible_when"`
}

func validateConvertedDepartureFields(fields []departureTestField) error {
	raw, err := json.Marshal(fields)
	if err != nil {
		return err
	}
	return validateConvertedDepartureSchema(departureSchemaRow{
		Name:             "test",
		Version:          1,
		CreatedBy:        1,
		CoreRequirements: `{"guardian_phone":true}`,
		LegalBlocks:      `[]`,
	}, string(raw))
}

func TestValidateConvertedDepartureSchemaRejectsInvalidUnchangedContent(t *testing.T) {
	t.Parallel()

	row := departureSchemaRow{Name: "test", Version: 0, CreatedBy: 1, CoreRequirements: `{"unknown":true}`, LegalBlocks: `[]`}
	err := validateConvertedDepartureSchema(row, `[{"key":"departure","label":"Heimweg","type":"weekday_multi_mode","applies_to_child":true,"target":"student.allowed_departure_modes"}]`)
	require.ErrorContains(t, err, `unknown core requirement "unknown"`)

	row.CoreRequirements = `{}`
	row.LegalBlocks = `[{"key":"agb","kind":"terms","enabled":true,"source":"standard"}]`
	err = validateConvertedDepartureSchema(row, `[{"key":"departure","label":"Heimweg","type":"weekday_multi_mode","applies_to_child":true,"target":"student.allowed_departure_modes"}]`)
	require.ErrorContains(t, err, "invalid kind, label, title, or required state")

	row.LegalBlocks = `[]`
	err = validateConvertedDepartureSchema(row, `[{"key":"Bad-Key","label":"Heimweg","type":"weekday_multi_mode","applies_to_child":true,"target":"student.allowed_departure_modes"}]`)
	require.ErrorContains(t, err, "lowercase letters, digits, or underscores")
}

func TestValidateConvertedDepartureSchemaRejectsInvalidSchemaMetadata(t *testing.T) {
	t.Parallel()

	fields := `[{"key":"departure","label":"Heimweg","type":"weekday_multi_mode","applies_to_child":true,"target":"student.allowed_departure_modes"}]`
	valid := departureSchemaRow{Name: "test", Version: 0, CreatedBy: 1, CoreRequirements: `{}`, LegalBlocks: `[]`}

	row := valid
	row.Name = ""
	require.EqualError(t, validateConvertedDepartureSchema(row, fields), "form schema name is required")

	row = valid
	row.Version = -1
	require.EqualError(t, validateConvertedDepartureSchema(row, fields), "form schema version must be positive")

	row = valid
	row.CreatedBy = 0
	require.EqualError(t, validateConvertedDepartureSchema(row, fields), "form schema created_by is required")
}

func insertDepartureTestSchema(t *testing.T, db *testpkg.DB, tenantID, createdBy int64, name string, version int, fieldsJSON string) int64 {
	t.Helper()
	var id int64
	err := db.QueryRowContext(context.Background(), `
		INSERT INTO enrollment.form_schemas
			(tenant_id, name, version, fields, core_requirements, legal_blocks, is_active, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?::jsonb, '{"guardian_phone":true}'::jsonb, '[]'::jsonb, true, ?, NOW(), NOW())
		RETURNING id
	`, tenantID, name, version, fieldsJSON, createdBy).Scan(&id)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(),
			`DELETE FROM enrollment.form_schemas WHERE tenant_id = ?`, tenantID)
	})
	return id
}

func insertDepartureTestPhase(t *testing.T, db *testpkg.DB, tenantID, schemaID int64) int64 {
	t.Helper()
	var id int64
	err := db.QueryRowContext(context.Background(), `
		INSERT INTO enrollment.phases
			(tenant_id, name, service_start_date, service_end_date, is_active, form_schema_id, created_at, updated_at)
		VALUES (?, 'Halbjahr Test', CURRENT_DATE, CURRENT_DATE + 180, true, ?, NOW(), NOW())
		RETURNING id
	`, tenantID, schemaID).Scan(&id)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(),
			`DELETE FROM enrollment.phases WHERE tenant_id = ?`, tenantID)
	})
	return id
}

func loadSchemaFields(t *testing.T, db *testpkg.DB, id int64) []departureTestField {
	t.Helper()
	var raw string
	require.NoError(t, db.QueryRowContext(context.Background(),
		`SELECT fields::text FROM enrollment.form_schemas WHERE id = ?`, id).Scan(&raw))
	var fields []departureTestField
	require.NoError(t, json.Unmarshal([]byte(raw), &fields))
	return fields
}

func TestFormSchemasMigrateLegacyDeparture_PublishesConvertedVersionAndRepointsPhases(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	tenantID := time.Now().UnixNano()
	testpkg.EnsureTestTenant(t, db, tenantID)
	testpkg.OwnTenantRows(t, db, tenantID)
	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("departure-migration-%d@test.local", tenantID))

	legacyFields := `[
		{"key":"pickup_status","label":"Abholung","type":"weekday_boolean","required":true,"applies_to_child":true,"sort_order":0,"target":"student.pickup_status"},
		{"key":"allergies","label":"Allergien","type":"textarea","required":false,"applies_to_child":true,"sort_order":1,"target":"student.health_info"},
		{"key":"bus_days","label":"Buskind","type":"weekday_boolean","required":false,"applies_to_child":true,"sort_order":2,"target":"student.bus_days"}
	]`
	v1 := insertDepartureTestSchema(t, db, tenantID, account.ID, "Halbjahresformular", 1, legacyFields)
	phaseID := insertDepartureTestPhase(t, db, tenantID, v1)

	require.NoError(t, formSchemasMigrateLegacyDepartureUp(ctx, db))

	// A new v2 exists with the converted fields; v1 is untouched.
	var newID int64
	var newVersion int
	require.NoError(t, db.QueryRowContext(ctx, `
		SELECT id, version FROM enrollment.form_schemas
		WHERE tenant_id = ? AND name = 'Halbjahresformular'
		ORDER BY version DESC LIMIT 1
	`, tenantID).Scan(&newID, &newVersion))
	require.NotEqual(t, v1, newID)
	assert.Equal(t, 2, newVersion)

	converted := loadSchemaFields(t, db, newID)
	require.Len(t, converted, 2)
	assert.Equal(t, "student.allowed_departure_modes", converted[0].Target)
	assert.Equal(t, "weekday_multi_mode", converted[0].Type)
	assert.True(t, converted[0].Required, "required must carry over from the required legacy field")
	assert.True(t, converted[0].AppliesToCh)
	assert.Equal(t, 0, converted[0].SortOrder)
	assert.Equal(t, "student.health_info", converted[1].Target)
	assert.Equal(t, 1, converted[1].SortOrder)

	require.NoError(t, validateConvertedDepartureFields(converted))

	// The old version keeps its legacy fields for pinned requests.
	old := loadSchemaFields(t, db, v1)
	require.Len(t, old, 3)
	assert.Equal(t, "student.pickup_status", old[0].Target)

	// The phase advanced onto the converted version.
	var phaseSchemaID int64
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT form_schema_id FROM enrollment.phases WHERE id = ?`, phaseID).Scan(&phaseSchemaID))
	assert.Equal(t, newID, phaseSchemaID)

	// Idempotency: a second run publishes nothing new.
	require.NoError(t, formSchemasMigrateLegacyDepartureUp(ctx, db))
	var count int
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM enrollment.form_schemas WHERE tenant_id = ?`, tenantID).Scan(&count))
	assert.Equal(t, 2, count)
}

func TestFormSchemasMigrateLegacyDeparture_DropsLegacyWhenModernFieldExists(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	tenantID := time.Now().UnixNano()
	testpkg.EnsureTestTenant(t, db, tenantID)
	testpkg.OwnTenantRows(t, db, tenantID)
	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("departure-migration-%d@test.local", tenantID))

	mixedFields := `[
		{"key":"heimwege","label":"Erlaubte Heimwege","type":"weekday_multi_mode","required":true,"applies_to_child":true,"sort_order":0,"target":"student.allowed_departure_modes"},
		{"key":"bus","label":"Buskind","type":"weekday_boolean","required":false,"applies_to_child":true,"sort_order":1,"target":"student.bus"}
	]`
	insertDepartureTestSchema(t, db, tenantID, account.ID, "Gemischt", 1, mixedFields)

	require.NoError(t, formSchemasMigrateLegacyDepartureUp(ctx, db))

	var newID int64
	require.NoError(t, db.QueryRowContext(ctx, `
		SELECT id FROM enrollment.form_schemas
		WHERE tenant_id = ? AND name = 'Gemischt'
		ORDER BY version DESC LIMIT 1
	`, tenantID).Scan(&newID))

	converted := loadSchemaFields(t, db, newID)
	require.Len(t, converted, 1)
	assert.Equal(t, "student.allowed_departure_modes", converted[0].Target)
	assert.Equal(t, "heimwege", converted[0].Key, "existing modern field must survive unchanged")
}

func TestFormSchemasMigrateLegacyDeparture_ExistingModernFieldInheritsRequirednessAndBroadensVisibility(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	tenantID := time.Now().UnixNano()
	testpkg.EnsureTestTenant(t, db, tenantID)
	testpkg.OwnTenantRows(t, db, tenantID)
	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("departure-migration-%d@test.local", tenantID))

	mixedFields := `[
		{"key":"heimwege","label":"Erlaubte Heimwege","type":"weekday_multi_mode","required":false,"applies_to_child":true,"sort_order":0,"target":"student.allowed_departure_modes","visible_when":{"source":"grade_level","operator":"eq","value":1}},
		{"key":"bus","label":"Buskind","type":"weekday_boolean","required":true,"applies_to_child":true,"sort_order":1,"target":"student.bus"}
	]`
	insertDepartureTestSchema(t, db, tenantID, account.ID, "Gemischt Pflicht", 1, mixedFields)

	require.NoError(t, formSchemasMigrateLegacyDepartureUp(ctx, db))

	var newID int64
	require.NoError(t, db.QueryRowContext(ctx, `
		SELECT id FROM enrollment.form_schemas
		WHERE tenant_id = ? AND name = 'Gemischt Pflicht'
		ORDER BY version DESC LIMIT 1
	`, tenantID).Scan(&newID))

	converted := loadSchemaFields(t, db, newID)
	require.Len(t, converted, 1)
	assert.Equal(t, "heimwege", converted[0].Key)
	assert.True(t, converted[0].Required, "required legacy departure fields must keep the surviving modern field required")
	assert.Nil(t, converted[0].VisibleWhen, "mixed modern/legacy visibility must broaden so children who saw the legacy field still see the modern one")
}

func TestFormSchemasMigrateLegacyDeparture_PreservesSharedVisibilityWhenModernFieldExists(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	tenantID := time.Now().UnixNano()
	testpkg.EnsureTestTenant(t, db, tenantID)
	testpkg.OwnTenantRows(t, db, tenantID)
	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("departure-migration-%d@test.local", tenantID))

	mixedFields := `[
		{"key":"heimwege","label":"Erlaubte Heimwege","type":"weekday_multi_mode","required":false,"applies_to_child":true,"sort_order":0,"target":"student.allowed_departure_modes","visible_when":{"source":"grade_level","operator":"eq","value":1}},
		{"key":"bus","label":"Buskind","type":"weekday_boolean","required":true,"applies_to_child":true,"sort_order":1,"target":"student.bus","visible_when":{"source":"grade_level","operator":"eq","value":1}}
	]`
	insertDepartureTestSchema(t, db, tenantID, account.ID, "Gemischt Gleiche Sichtbarkeit", 1, mixedFields)

	require.NoError(t, formSchemasMigrateLegacyDepartureUp(ctx, db))

	var newID int64
	require.NoError(t, db.QueryRowContext(ctx, `
		SELECT id FROM enrollment.form_schemas
		WHERE tenant_id = ? AND name = 'Gemischt Gleiche Sichtbarkeit'
		ORDER BY version DESC LIMIT 1
	`, tenantID).Scan(&newID))

	converted := loadSchemaFields(t, db, newID)
	require.Len(t, converted, 1)
	assert.Equal(t, "heimwege", converted[0].Key)
	assert.True(t, converted[0].Required)
	require.NotNil(t, converted[0].VisibleWhen, "identical departure visibility should remain conditional")
	assert.Equal(t, "grade_level", converted[0].VisibleWhen.Source)
	assert.Equal(t, "eq", converted[0].VisibleWhen.Operator)
	assert.Equal(t, float64(1), converted[0].VisibleWhen.Value)
}

func TestFormSchemasMigrateLegacyDeparture_DeduplicatesModernDepartureFieldsBeforeValidation(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	tenantID := time.Now().UnixNano()
	testpkg.EnsureTestTenant(t, db, tenantID)
	testpkg.OwnTenantRows(t, db, tenantID)
	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("departure-migration-%d@test.local", tenantID))

	legacyDuplicateFields := `[
		{"key":"heimwege_a","label":"Erlaubte Heimwege","type":"weekday_multi_mode","required":false,"applies_to_child":true,"sort_order":0,"target":"student.allowed_departure_modes"},
		{"key":"bus","label":"Buskind","type":"weekday_boolean","required":true,"applies_to_child":true,"sort_order":1,"target":"student.bus"},
		{"key":"heimwege_b","label":"Erlaubte Heimwege Kopie","type":"weekday_multi_mode","required":false,"applies_to_child":true,"sort_order":2,"target":"student.allowed_departure_modes"}
	]`
	insertDepartureTestSchema(t, db, tenantID, account.ID, "Doppelte Heimwege", 1, legacyDuplicateFields)

	require.NoError(t, formSchemasMigrateLegacyDepartureUp(ctx, db))

	var newID int64
	var newVersion int
	require.NoError(t, db.QueryRowContext(ctx, `
		SELECT id, version FROM enrollment.form_schemas
		WHERE tenant_id = ? AND name = 'Doppelte Heimwege'
		ORDER BY version DESC LIMIT 1
	`, tenantID).Scan(&newID, &newVersion))

	converted := loadSchemaFields(t, db, newID)
	require.Len(t, converted, 1)
	assert.Equal(t, "heimwege_a", converted[0].Key, "the first modern departure field should survive")
	assert.Equal(t, "student.allowed_departure_modes", converted[0].Target)
	assert.True(t, converted[0].Required, "requiredness must merge from the legacy field")
	assert.Nil(t, converted[0].VisibleWhen)

	require.NoError(t, validateConvertedDepartureFields(converted), "converted schema must not contain duplicate modern departure targets")
}

func TestFormSchemasMigrateLegacyDeparture_ClearsDifferingLegacyVisibilityOnReplacement(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	tenantID := time.Now().UnixNano()
	testpkg.EnsureTestTenant(t, db, tenantID)
	testpkg.OwnTenantRows(t, db, tenantID)
	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("departure-migration-%d@test.local", tenantID))

	legacyFields := `[
		{"key":"care_kind","label":"Betreuung","type":"select","required":true,"applies_to_child":true,"sort_order":0,"options":[{"value":"ogs","label":"OGS"},{"value":"eight_to_one","label":"8-1"}]},
		{"key":"pickup_status","label":"Abholung","type":"weekday_boolean","required":true,"applies_to_child":true,"sort_order":1,"target":"student.pickup_status","visible_when":{"source":"field","field":"care_kind","operator":"eq","value":"ogs"}},
		{"key":"bus_days","label":"Buskind","type":"weekday_boolean","required":false,"applies_to_child":true,"sort_order":2,"target":"student.bus_days","visible_when":{"source":"grade_level","operator":"eq","value":1}}
	]`
	insertDepartureTestSchema(t, db, tenantID, account.ID, "Sichtbar Bedingt", 1, legacyFields)

	require.NoError(t, formSchemasMigrateLegacyDepartureUp(ctx, db))

	var newID int64
	require.NoError(t, db.QueryRowContext(ctx, `
		SELECT id FROM enrollment.form_schemas
		WHERE tenant_id = ? AND name = 'Sichtbar Bedingt'
		ORDER BY version DESC LIMIT 1
	`, tenantID).Scan(&newID))

	converted := loadSchemaFields(t, db, newID)
	require.Len(t, converted, 2)
	assert.Equal(t, "care_kind", converted[0].Key)
	assert.Equal(t, "student.allowed_departure_modes", converted[1].Target)
	assert.Nil(t, converted[1].VisibleWhen, "replacement field must be unconditional when removed legacy fields had different visibility")
	assert.True(t, converted[1].Required)
}

func TestFormSchemasMigrateLegacyDeparture_ClearsDanglingVisibilityDependencies(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	tenantID := time.Now().UnixNano()
	testpkg.EnsureTestTenant(t, db, tenantID)
	testpkg.OwnTenantRows(t, db, tenantID)
	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("departure-migration-%d@test.local", tenantID))

	legacyFields := `[
		{"key":"pickup_status","label":"Abholung","type":"weekday_boolean","required":true,"applies_to_child":true,"sort_order":0,"target":"student.pickup_status"},
		{"key":"pickup_note","label":"Abholhinweis","type":"textarea","required":false,"applies_to_child":true,"sort_order":1,"target":"student.extra_info","visible_when":{"source":"field","field":"pickup_status","operator":"eq","value":true}}
	]`
	insertDepartureTestSchema(t, db, tenantID, account.ID, "Abhängigkeit", 1, legacyFields)

	require.NoError(t, formSchemasMigrateLegacyDepartureUp(ctx, db))

	var newID int64
	var newVersion int
	require.NoError(t, db.QueryRowContext(ctx, `
		SELECT id, version FROM enrollment.form_schemas
		WHERE tenant_id = ? AND name = 'Abhängigkeit'
		ORDER BY version DESC LIMIT 1
	`, tenantID).Scan(&newID, &newVersion))

	converted := loadSchemaFields(t, db, newID)
	require.Len(t, converted, 2)
	assert.Equal(t, "student.allowed_departure_modes", converted[0].Target)
	assert.Equal(t, "pickup_note", converted[1].Key)
	assert.Nil(t, converted[1].VisibleWhen, "dependent field must not point at the removed legacy pickup_status key")

	require.NoError(t, validateConvertedDepartureFields(converted))
}

func TestFormSchemasMigrateLegacyDeparture_RepointsLegacyPhaseToCleanLatest(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	tenantID := time.Now().UnixNano()
	testpkg.EnsureTestTenant(t, db, tenantID)
	testpkg.OwnTenantRows(t, db, tenantID)
	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("departure-migration-%d@test.local", tenantID))

	legacyFields := `[
		{"key":"pickup_status","label":"Abholung","type":"weekday_boolean","required":true,"applies_to_child":true,"sort_order":0,"target":"student.pickup_status"}
	]`
	cleanFields := `[
		{"key":"heimwege","label":"Erlaubte Heimwege","type":"weekday_multi_mode","required":true,"applies_to_child":true,"sort_order":0,"target":"student.allowed_departure_modes"}
	]`
	v1 := insertDepartureTestSchema(t, db, tenantID, account.ID, "Schon Sauber", 1, legacyFields)
	v2 := insertDepartureTestSchema(t, db, tenantID, account.ID, "Schon Sauber", 2, cleanFields)
	phaseID := insertDepartureTestPhase(t, db, tenantID, v1)

	require.NoError(t, formSchemasMigrateLegacyDepartureUp(ctx, db))

	var count int
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM enrollment.form_schemas WHERE tenant_id = ? AND name = 'Schon Sauber'`, tenantID).Scan(&count))
	assert.Equal(t, 2, count, "clean latest lineage must not receive a duplicate version")

	var phaseSchemaID int64
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT form_schema_id FROM enrollment.phases WHERE id = ?`, phaseID).Scan(&phaseSchemaID))
	assert.Equal(t, v2, phaseSchemaID, "phase pinned to legacy v1 must advance to clean latest v2")

	require.NoError(t, formSchemasMigrateLegacyDepartureUp(ctx, db))
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM enrollment.form_schemas WHERE tenant_id = ? AND name = 'Schon Sauber'`, tenantID).Scan(&count))
	assert.Equal(t, 2, count, "clean-latest repoint path must stay idempotent")
}

func TestFormSchemasMigrateLegacyDeparture_SkipsCleanLineages(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	tenantID := time.Now().UnixNano()
	testpkg.EnsureTestTenant(t, db, tenantID)
	testpkg.OwnTenantRows(t, db, tenantID)
	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("departure-migration-%d@test.local", tenantID))

	cleanFields := `[
		{"key":"heimwege","label":"Erlaubte Heimwege","type":"weekday_multi_mode","required":false,"applies_to_child":true,"sort_order":0,"target":"student.allowed_departure_modes"}
	]`
	insertDepartureTestSchema(t, db, tenantID, account.ID, "Sauber", 1, cleanFields)

	require.NoError(t, formSchemasMigrateLegacyDepartureUp(ctx, db))

	var count int
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM enrollment.form_schemas WHERE tenant_id = ?`, tenantID).Scan(&count))
	assert.Equal(t, 1, count, "clean lineage must not receive a new version")
}
