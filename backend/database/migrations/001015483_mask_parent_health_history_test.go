package migrations

import (
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParentHealthHistoryMigrationRedactsOnlyHealthAcrossTenants(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := t.Context()
	account := testpkg.CreateTestAccount(t, db, "synthetic-parent")
	cases := []struct {
		target, field, status      string
		oldValue                   *string
		newValue, wantOld, wantNew string
	}{
		{"student", "health_info", "auto_applied", nil, `"synthetic-value-a"`, `""`, `"Angabe hinzugefügt"`},
		{"student", "health_info", "approved", testpkg.StrPtr(`"synthetic-value-a"`), `"synthetic-value-b"`, `"Angabe vorhanden"`, `"Angabe geändert"`},
		{"student", "health_info", "rejected", testpkg.StrPtr(`"synthetic-value-a"`), `null`, `"Angabe vorhanden"`, `""`},
		{"student", "health_info", "pending", testpkg.StrPtr(`null`), `""`, `""`, `""`},
		{"student", "health_info", "care_ended", testpkg.StrPtr(`"synthetic-value-a"`), `""`, `"Angabe vorhanden"`, `""`},
		{"student", "health_info", "auto_applied", testpkg.StrPtr(`""`), `" synthetic-value-a "`, `""`, `"Angabe hinzugefügt"`},
		{"student", "school_class", "approved", testpkg.StrPtr(`"synthetic-value-a"`), `"synthetic-value-b"`, `"synthetic-value-a"`, `"synthetic-value-b"`},
		{"guardian_profile", "health_info", "auto_applied", testpkg.StrPtr(`"synthetic-value-a"`), `"synthetic-value-b"`, `"synthetic-value-a"`, `"synthetic-value-b"`},
	}
	type expectedRow struct {
		id                           int64
		metadata, oldValue, newValue string
	}
	var expected []expectedRow
	for _, tenantID := range []int64{testpkg.Tenant(t), testpkg.NewTenantScope(t, db).TenantID} {
		student := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Synthetic", "History", "1a")
		for _, tc := range cases {
			var id int64
			require.NoError(t, db.NewRaw(`INSERT INTO users.student_data_change_requests
    (tenant_id, student_id, submitted_by, target, field_key, status, old_value, new_value,
     reviewed_by, reviewed_at, applied_at, review_reason, target_ref_id)
    VALUES (?, ?, ?, ?, ?, ?, ?::jsonb, ?::jsonb, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 'synthetic-reason', 7)
    RETURNING id`, tenantID, student.ID, account.ID, tc.target, tc.field, tc.status, tc.oldValue, tc.newValue, account.ID).Scan(ctx, &id))
			var metadata string
			require.NoError(t, db.NewRaw(`SELECT (to_jsonb(r) - 'old_value' - 'new_value')::text
    FROM users.student_data_change_requests r WHERE id = ?`, id).Scan(ctx, &metadata))
			expected = append(expected, expectedRow{id, metadata, tc.wantOld, tc.wantNew})
		}
	}
	for range 2 {
		require.NoError(t, maskParentHealthHistoryUp(ctx, db))
		for _, row := range expected {
			var oldValue, newValue, metadata string
			require.NoError(t, db.NewRaw(`SELECT COALESCE(old_value, 'null'::jsonb)::text, new_value::text,
    (to_jsonb(r) - 'old_value' - 'new_value')::text
    FROM users.student_data_change_requests r WHERE id = ?`, row.id).Scan(ctx, &oldValue, &newValue, &metadata))
			assert.JSONEq(t, row.oldValue, oldValue)
			assert.JSONEq(t, row.newValue, newValue)
			assert.JSONEq(t, row.metadata, metadata, "all attribution, status and timestamps stay unchanged")
		}
	}
}

func TestParentHealthHistoryMigrationFailsAtomicallyAndCannotRestorePlaintext(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := t.Context()
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	_, err := db.ExecContext(ctx, `INSERT INTO users.student_data_change_requests
  (tenant_id, student_id, submitted_by, target, field_key, status, old_value, new_value)
  VALUES (?, ?, ?, 'student', 'health_info', 'auto_applied', '"synthetic-value-a"', '"synthetic-value-c"'),
         (?, ?, ?, 'student', 'health_info', 'auto_applied', '"synthetic-value-b"', '"synthetic-value-d"')`,
		chain.TenantID, chain.StudentID, chain.AccountID, chain.TenantID, chain.StudentID, chain.AccountID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `CREATE FUNCTION users.reject_synthetic_parent_health_update() RETURNS trigger LANGUAGE plpgsql AS $$
  BEGIN
   IF OLD.old_value = '"synthetic-value-b"'::jsonb THEN
    RAISE EXCEPTION 'synthetic migration failure';
   END IF;
   RETURN NEW;
  END;
  $$;
  CREATE TRIGGER reject_synthetic_parent_health_update BEFORE UPDATE ON users.student_data_change_requests
   FOR EACH ROW EXECUTE FUNCTION users.reject_synthetic_parent_health_update()`)
	require.NoError(t, err)
	require.ErrorContains(t, maskParentHealthHistoryUp(ctx, db), "synthetic migration failure")
	var unchanged int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM users.student_data_change_requests WHERE student_id = ?
  AND old_value IN ('"synthetic-value-a"'::jsonb, '"synthetic-value-b"'::jsonb)
  AND new_value IN ('"synthetic-value-c"'::jsonb, '"synthetic-value-d"'::jsonb)`, chain.StudentID).Scan(ctx, &unchanged))
	assert.Equal(t, 2, unchanged, "no partial redaction on failure")
	require.ErrorContains(t, maskParentHealthHistoryDown(ctx, db), "irreversible")
}
