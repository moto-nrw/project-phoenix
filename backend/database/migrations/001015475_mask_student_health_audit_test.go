package migrations

import (
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStudentHealthAuditMigrationRedactsOnlyHealthValuesAcrossTenants(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := t.Context()
	cases := []struct {
		field              string
		oldValue, newValue *string
		wantOld, wantNew   string
	}{
		{"health_info", nil, testpkg.StrPtr("synthetic-value-a"), "", "Angabe hinzugefügt"},
		{"health_info", testpkg.StrPtr("synthetic-value-a"), testpkg.StrPtr("synthetic-value-b"), "Angabe vorhanden", "Angabe geändert"},
		{"health_info", testpkg.StrPtr("synthetic-value-a"), nil, "Angabe vorhanden", ""},
		{"health_info", testpkg.StrPtr("synthetic-value-a"), testpkg.StrPtr(""), "Angabe vorhanden", ""},
		{"health_info", testpkg.StrPtr(""), testpkg.StrPtr(" synthetic-value-a "), "", "Angabe hinzugefügt"},
		{"health_info", nil, nil, "", ""},
		{"extra_info", testpkg.StrPtr("synthetic-value-a"), testpkg.StrPtr("synthetic-value-b"), "synthetic-value-a", "synthetic-value-b"},
		{"departure_companion_note", testpkg.StrPtr("synthetic-value-a"), testpkg.StrPtr("synthetic-value-b"), "synthetic-value-a", "synthetic-value-b"},
	}
	type expectedRow struct {
		id                 int64
		metadata           string
		oldValue, newValue string
	}
	var expected []expectedRow
	for _, tenantID := range []int64{testpkg.Tenant(t), testpkg.NewTenantScope(t, db).TenantID} {
		student := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Synthetic", "Audit", "1a")
		for _, tc := range cases {
			var id int64
			require.NoError(t, db.NewRaw(`INSERT INTO audit.student_field_edits
    (tenant_id, student_id, edited_by, edited_by_name, field_name, old_value, new_value)
    VALUES (?, ?, 0, 'System', ?, ?, ?) RETURNING id`, tenantID, student.ID, tc.field, tc.oldValue, tc.newValue).Scan(ctx, &id))
			var metadata string
			require.NoError(t, db.NewRaw(`SELECT (to_jsonb(e) - 'old_value' - 'new_value')::text
    FROM audit.student_field_edits e WHERE id = ?`, id).Scan(ctx, &metadata))
			expected = append(expected, expectedRow{id, metadata, tc.wantOld, tc.wantNew})
		}
	}
	for range 2 {
		require.NoError(t, maskStudentHealthAuditUp(ctx, db))
		for _, row := range expected {
			var oldValue, newValue string
			var metadata string
			require.NoError(t, db.NewRaw(`SELECT COALESCE(old_value, ''), COALESCE(new_value, ''), (to_jsonb(e) - 'old_value' - 'new_value')::text
    FROM audit.student_field_edits e WHERE id = ?`, row.id).Scan(ctx, &oldValue, &newValue, &metadata))
			assert.Equal(t, row.oldValue, oldValue)
			assert.Equal(t, row.newValue, newValue)
			assert.JSONEq(t, row.metadata, metadata, "identity, tenant, student, actor and timestamp must stay unchanged")
		}
	}
	var nullValues int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM audit.student_field_edits
		WHERE field_name = 'health_info' AND (old_value IS NULL OR new_value IS NULL)`).Scan(ctx, &nullValues))
	assert.Zero(t, nullValues)
	var canUpdate, canTruncate bool
	require.NoError(t, db.NewRaw(`SELECT has_table_privilege('phoenix_tenant', 'audit.student_field_edits', 'UPDATE'),
  has_table_privilege('phoenix_tenant', 'audit.student_field_edits', 'TRUNCATE')`).Scan(ctx, &canUpdate, &canTruncate))
	assert.False(t, canUpdate)
	assert.False(t, canTruncate)
}

func TestStudentHealthAuditMigrationFailsAtomicallyAndCannotRestorePlaintext(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := t.Context()
	student := testpkg.CreateTestStudent(t, db, "Synthetic", "Failure", "1a")
	_, err := db.ExecContext(ctx, `INSERT INTO audit.student_field_edits
  (tenant_id, student_id, edited_by, edited_by_name, field_name, old_value, new_value)
  VALUES (?, ?, 0, 'System', 'health_info', 'synthetic-value-a', 'synthetic-value-c'),
         (?, ?, 0, 'System', 'health_info', 'synthetic-value-b', 'synthetic-value-d');
 `, testpkg.Tenant(t), student.ID, testpkg.Tenant(t), student.ID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `
  CREATE FUNCTION audit.reject_synthetic_health_update() RETURNS trigger LANGUAGE plpgsql AS $$
  BEGIN
   IF OLD.old_value = 'synthetic-value-b' THEN
    RAISE EXCEPTION 'synthetic migration failure';
   END IF;
   RETURN NEW;
  END;
  $$;
  CREATE TRIGGER reject_synthetic_health_update BEFORE UPDATE ON audit.student_field_edits
   FOR EACH ROW EXECUTE FUNCTION audit.reject_synthetic_health_update();
 `)
	require.NoError(t, err)
	require.ErrorContains(t, maskStudentHealthAuditUp(ctx, db), "synthetic migration failure")
	var unchanged int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM audit.student_field_edits
  WHERE student_id = ? AND old_value IN ('synthetic-value-a', 'synthetic-value-b')
   AND new_value IN ('synthetic-value-c', 'synthetic-value-d')`, student.ID).Scan(ctx, &unchanged))
	assert.Equal(t, 2, unchanged, "no partial redaction on failure")
	require.ErrorContains(t, maskStudentHealthAuditDown(ctx, db), "irreversible")
}
