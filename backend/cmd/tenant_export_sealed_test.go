package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// The return of a school carries the health field of a child opened with
// the key ring of the installation, never its ciphertext (#3838, #6328),
// and its settings without their secrets (#3838).
func TestTenantExportOpensHealthAndProjectsSettingsWithTheRootBindings(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	school := testpkg.Tenant(t)
	student := testpkg.CreateTestStudentForTenant(t, db, school, "Synthetic", "Health", "1a")
	health := "Synthetische Erdnussallergie"
	testpkg.SetTestStudentHealth(t, db, student.ID, &health)

	_, err := db.NewRaw(`INSERT INTO config.setting_values (tenant_id, setting_key, value) VALUES (?0, 'security.ogs_device_pin', '"synthetic-pin-4711"'), (?0, 'sftp.host', '"sftp.example.invalid"')`, school).Exec(t.Context())
	require.NoError(t, err)

	sealed, err := tenantExportSealedFields()
	require.NoError(t, err)
	database := testpkg.NewDatabase(t, db)
	exporter, err := newTenantExporter(database, sealed, tenantExportProjectedFields(), false)
	require.NoError(t, err)
	operator := testpkg.CreateTestOperator(t, db)
	out := filepath.Join(t.TempDir(), "export")
	require.NoError(t, exporter.Export(tenant.WithUnitOfWork(t.Context(), database.UnitOfWork()),
		organizationtenancy.TenantExportOptions{SchoolID: school, OperatorID: operator.ID, Output: out, Uploads: t.TempDir()}))
	require.FileExists(t, filepath.Join(out, "COMPLETE"))
	settings := map[string]string{}
	for _, row := range readExportRows(t, filepath.Join(out, "config.setting_values.jsonl")) {
		require.NotContains(t, row, "value", "the raw container never leaves")
		var key string
		require.NoError(t, json.Unmarshal(row["setting_key"], &key))
		settings[key] = string(row["exported_value"])
	}
	require.Equal(t, map[string]string{"security.ogs_device_pin": "null", "sftp.host": `"sftp.example.invalid"`}, settings)

	var opened []string
	for _, row := range readExportRows(t, filepath.Join(out, "users.student_care_profiles.jsonl")) {
		require.NotContains(t, row, "health_data", "the ciphertext never leaves")
		var value *string
		require.NoError(t, json.Unmarshal(row["health_info"], &value))
		if value != nil {
			opened = append(opened, *value)
		}
	}
	require.Equal(t, []string{health}, opened)
}

func readExportRows(t *testing.T, path string) []map[string]json.RawMessage {
	t.Helper()
	file, err := os.Open(path)
	require.NoError(t, err)
	defer func() { _ = file.Close() }()
	var rows []map[string]json.RawMessage
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var row map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(scanner.Bytes(), &row))
		rows = append(rows, row)
	}
	require.NoError(t, scanner.Err())
	return rows
}

// The Träger package runs on the migrated schema with the root bindings and
// names the Träger's schools, but carries no school-held row (#3838).
func TestTenantExportOrganizationPackageOnTheMigratedSchema(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	school := testpkg.Tenant(t)
	testpkg.CreateTestStudentForTenant(t, db, school, "Synthetic", "Traeger", "1a")
	var organization int64
	require.NoError(t, db.NewRaw(`SELECT organization_id FROM platform.schools WHERE id = ?`, school).Scan(t.Context(), &organization))
	other, _ := testpkg.CreateTestTenant(t, db)
	var otherOrganization int64
	require.NoError(t, db.NewRaw(`SELECT organization_id FROM platform.schools WHERE id = ?`, other).Scan(t.Context(), &otherOrganization))
	require.NotEqual(t, organization, otherOrganization)
	// A Träger mail has organization_id instead of tenant_id; no school
	// package carries it, so the Träger package must (#3838). No tenant owns
	// these rows, so the test removes them; their events go with the mail.
	t.Cleanup(func() {
		_, err := db.NewRaw(`DELETE FROM platform.email_outbox WHERE organization_id IN (?, ?)`, organization, otherOrganization).Exec(context.Background())
		require.NoError(t, err)
	})
	var mail int64
	require.NoError(t, db.NewRaw(`INSERT INTO platform.email_outbox (organization_id, kind, recipient, payload)
		VALUES (?, 'holiday_registration_received', '{}'::jsonb, '{}'::jsonb) RETURNING id`, organization).Scan(t.Context(), &mail))
	_, err := db.NewRaw(`INSERT INTO platform.email_outbox (organization_id, kind, recipient, payload)
		VALUES (?, 'holiday_registration_received', '{}'::jsonb, '{}'::jsonb)`, otherOrganization).Exec(t.Context())
	require.NoError(t, err)
	_, err = db.NewRaw(`INSERT INTO platform.email_delivery_events (outbox_id, provider, provider_event_id, state, occurred_at)
		VALUES (?, 'synthetic', 'synthetic-event', 'delivered', now())`, mail).Exec(t.Context())
	require.NoError(t, err)

	sealed, err := tenantExportSealedFields()
	require.NoError(t, err)
	// The root binds a Träger package with its PAIN.008 files and its
	// operator audit (ADR 0187).
	database := testpkg.NewDatabase(t, db)
	exporter, err := newTenantExporter(database, sealed, tenantExportProjectedFields(), true)
	require.NoError(t, err)
	operator := testpkg.CreateTestOperator(t, db)
	out := filepath.Join(t.TempDir(), "traeger")
	require.NoError(t, exporter.Export(tenant.WithUnitOfWork(t.Context(), database.UnitOfWork()),
		organizationtenancy.TenantExportOptions{OrganizationID: organization, OperatorID: operator.ID, Output: out, Uploads: t.TempDir()}))
	require.FileExists(t, filepath.Join(out, "COMPLETE"))

	var schools []int64
	for _, row := range readExportRows(t, filepath.Join(out, "platform.schools.jsonl")) {
		var id, owner int64
		require.NoError(t, json.Unmarshal(row["id"], &id))
		require.NoError(t, json.Unmarshal(row["organization_id"], &owner))
		require.Equal(t, organization, owner)
		schools = append(schools, id)
	}
	require.Contains(t, schools, school)
	require.NoFileExists(t, filepath.Join(out, "users.persons.jsonl"), "school-held rows go in the school package")

	mails := readExportRows(t, filepath.Join(out, "platform.email_outbox.jsonl"))
	require.Len(t, mails, 1, "only the mail of this Träger")
	require.Equal(t, strconv.FormatInt(mail, 10), string(mails[0]["id"]))
	events := readExportRows(t, filepath.Join(out, "platform.email_delivery_events.jsonl"))
	require.Len(t, events, 1)
	require.Equal(t, strconv.FormatInt(mail, 10), string(events[0]["outbox_id"]))
}
