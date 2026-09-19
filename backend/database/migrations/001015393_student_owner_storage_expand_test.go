package migrations

// Migration tests stay internal: the active architecture policy forbids an
// external module-behavior-test from importing a migration adapter. Exercise
// the registered deployment entry points below without widening that policy.

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

const studentOwnerStorageExpandName = "001015393"

var studentOwnerStorageTargets = []string{
	"users.student_profiles",
	"users.student_school_memberships",
	"users.student_care_profiles",
}

// studentOwnerStorageUnowned are the users.students columns Expand deliberately
// leaves without a target. The guardian_* values are reconciled into
// users.guardian_profiles / users.guardian_phone_numbers by the backfill, and
// the sick*/excused* flags are superseded by active.student_status_days.
var studentOwnerStorageUnowned = []string{
	"guardian_name", "guardian_contact", "guardian_email", "guardian_phone",
	"sick", "sick_since", "excused", "excused_since",
}

// studentOwnerStorageNewColumns are target columns with no users.students
// source: the surrogate keys, the bookkeeping timestamps every owner table
// carries, and the membership's own soft-deletion marker.
var studentOwnerStorageNewColumns = []string{
	"id", "tenant_id", "created_at", "updated_at",
	"student_profile_id", "membership_id", "deleted_at",
}

func TestStudentOwnerStorageExpandColumnMapping(t *testing.T) {
	t.Parallel()
	db := setupIsolatedStudentStorageBeforeCutover(t)
	for _, tc := range []struct {
		target  string
		columns []string
	}{
		{"users.student_profiles", []string{
			"id", "tenant_id", "person_id", "address_street", "address_city", "address_postal_code",
			"extra_info", "photo_path", "photo_consent_given_at", "photo_consent_given_by",
			"agb_accepted_at", "data_processing_accepted_at", "email_contact_accepted_at",
			"created_at", "updated_at",
		}},
		{"users.student_school_memberships", []string{
			"id", "tenant_id", "student_profile_id", "school_class", "group_id", "status",
			"enrolled_from", "enrolled_until", "created_at", "updated_at", "deleted_at",
		}},
		{"users.student_care_profiles", []string{
			"membership_id", "tenant_id", "supervisor_notes", "health_info", "pickup_status",
			"departure_days", "allowed_departure_modes", "departure_companion_note",
			"pickup_days", "bus_days", "created_at", "updated_at",
		}},
	} {
		t.Run(tc.target, func(t *testing.T) {
			schema, table, _ := strings.Cut(tc.target, ".")
			var columns []string
			require.NoError(t, db.NewRaw(`SELECT column_name FROM information_schema.columns
				WHERE table_schema = ? AND table_name = ? ORDER BY ordinal_position`, schema, table).Scan(t.Context(), &columns))
			require.ElementsMatch(t, tc.columns, columns)
			// Every moved field keeps the SQL type, nullability and default of its
			// users.students source column.
			for _, column := range tc.columns {
				if slices.Contains(studentOwnerStorageNewColumns, column) {
					continue
				}
				var same bool
				require.NoError(t, db.NewRaw(`SELECT old.data_type = target.data_type
					AND old.is_nullable = target.is_nullable
					AND old.column_default IS NOT DISTINCT FROM target.column_default
					FROM information_schema.columns old JOIN information_schema.columns target USING (column_name)
					WHERE old.table_schema = 'users' AND old.table_name = 'students'
					AND target.table_schema = ? AND target.table_name = ? AND old.column_name = ?`,
					schema, table, column).Scan(t.Context(), &same))
				require.True(t, same, "mapping of %s", column)
			}
		})
	}
	// Every old column either has exactly one owner among the targets or is on
	// the deliberately unowned list — nothing falls through silently.
	var unowned []string
	require.NoError(t, db.NewRaw(`SELECT column_name FROM information_schema.columns
		WHERE table_schema = 'users' AND table_name = 'students'
		AND column_name NOT IN ('id', 'tenant_id', 'created_at', 'updated_at')
		AND column_name NOT IN (SELECT column_name FROM information_schema.columns
			WHERE (table_schema, table_name) IN (('users', 'student_profiles'),
				('users', 'student_school_memberships'), ('users', 'student_care_profiles')))
		ORDER BY column_name`).Scan(t.Context(), &unowned))
	require.ElementsMatch(t, studentOwnerStorageUnowned, unowned)
	// ...and no moved column is duplicated across two targets.
	var duplicated []string
	require.NoError(t, db.NewRaw(`SELECT column_name FROM information_schema.columns
		WHERE (table_schema, table_name) IN (('users', 'student_profiles'),
			('users', 'student_school_memberships'), ('users', 'student_care_profiles'))
		AND column_name NOT IN ('id', 'tenant_id', 'created_at', 'updated_at')
		GROUP BY column_name HAVING count(*) > 1 ORDER BY column_name`).Scan(t.Context(), &duplicated))
	require.Empty(t, duplicated)
	// The link columns are NOT NULL bigints on both dependent targets.
	var links int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM information_schema.columns
		WHERE data_type = 'bigint' AND is_nullable = 'NO'
		AND ((table_schema, table_name, column_name) = ('users', 'student_school_memberships', 'student_profile_id')
			OR (table_schema, table_name, column_name) = ('users', 'student_care_profiles', 'membership_id'))`).Scan(t.Context(), &links))
	require.Equal(t, 2, links)
}

// TestStudentOwnerStorageExpandConstraintDefinitions pins the exact constraint
// set of each target by name and definition. The behavioural tests below prove
// what each one rejects; this one proves none was quietly added, dropped or
// widened — a composite tenant FK degraded to a single-column one still passes
// every rejection test written against one tenant.
func TestStudentOwnerStorageExpandConstraintDefinitions(t *testing.T) {
	t.Parallel()
	db := setupIsolatedStudentStorageBeforeCutover(t)
	for table, expected := range map[string]map[string]string{
		"users.student_profiles": {
			"student_profiles_pkey":                        "PRIMARY KEY (id)",
			"uq_student_profiles_tenant_id":                "UNIQUE (tenant_id, id)",
			"uq_student_profiles_person":                   "UNIQUE (tenant_id, person_id)",
			"student_profiles_tenant_id_fkey":              "FOREIGN KEY (tenant_id) REFERENCES platform.schools(id)",
			"fk_student_profiles_person":                   "FOREIGN KEY (tenant_id, person_id) REFERENCES users.persons(tenant_id, id) ON DELETE CASCADE",
			"student_profiles_photo_consent_given_by_fkey": "FOREIGN KEY (photo_consent_given_by) REFERENCES auth.accounts(id) ON DELETE SET NULL",
		},
		"users.student_school_memberships": {
			"student_school_memberships_pkey":           "PRIMARY KEY (id)",
			"uq_student_school_memberships_tenant_id":   "UNIQUE (tenant_id, id)",
			"student_school_memberships_tenant_id_fkey": "FOREIGN KEY (tenant_id) REFERENCES platform.schools(id)",
			"fk_student_school_memberships_profile":     "FOREIGN KEY (tenant_id, student_profile_id) REFERENCES users.student_profiles(tenant_id, id) ON DELETE CASCADE",
			"fk_student_school_memberships_group":       "FOREIGN KEY (tenant_id, group_id) REFERENCES education.groups(tenant_id, id) ON DELETE SET NULL (group_id)",
			"chk_student_school_memberships_status":     "CHECK ((status = ANY (ARRAY['pending'::text, 'active'::text, 'inactive'::text, 'alumnus'::text])))",
		},
		"users.student_care_profiles": {
			"student_care_profiles_pkey":                          "PRIMARY KEY (membership_id)",
			"uq_student_care_profiles_tenant_id":                  "UNIQUE (tenant_id, membership_id)",
			"student_care_profiles_tenant_id_fkey":                "FOREIGN KEY (tenant_id) REFERENCES platform.schools(id)",
			"fk_student_care_profiles_membership":                 "FOREIGN KEY (tenant_id, membership_id) REFERENCES users.student_school_memberships(tenant_id, id) ON DELETE CASCADE",
			"check_student_care_profiles_departure_days":          "CHECK (users.is_valid_departure_days(departure_days))",
			"check_student_care_profiles_allowed_departure_modes": "CHECK (users.is_valid_allowed_departure_modes(allowed_departure_modes))",
			"check_student_care_profiles_pickup_days":             "CHECK (users.is_valid_pickup_days(pickup_days))",
			"check_student_care_profiles_bus_days":                "CHECK (users.is_valid_bus_days(bus_days))",
		},
	} {
		t.Run(table, func(t *testing.T) {
			// pg_get_constraintdef omits schemas that the session search_path
			// already covers. Pin it to pg_catalog so every referenced table
			// is spelled out and a table moving schema cannot pass unnoticed,
			// while built-in type casts stay unqualified.
			tx, err := db.BeginTx(t.Context(), nil)
			require.NoError(t, err)
			defer func() { _ = tx.Rollback() }()
			_, err = tx.ExecContext(t.Context(), `SET LOCAL search_path = pg_catalog`)
			require.NoError(t, err)
			var names, definitions []string
			require.NoError(t, tx.NewRaw(`SELECT conname, pg_get_constraintdef(oid)
				FROM pg_constraint WHERE conrelid = ?::regclass ORDER BY conname`, table).
				Scan(t.Context(), &names, &definitions))
			actual := make(map[string]string, len(names))
			for i, name := range names {
				actual[name] = definitions[i]
			}
			require.Equal(t, expected, actual)
		})
	}
}

type studentOwnerExpandFixture struct {
	tenant, person, group, account int64
}

func createStudentOwnerExpandFixture(t *testing.T, db *testpkg.DB) studentOwnerExpandFixture {
	t.Helper()
	f := studentOwnerExpandFixture{tenant: testpkg.UniqueTestTenantID(t)}
	testpkg.EnsureTestTenant(t, db, f.tenant)
	f.person = testpkg.CreateTestPersonForTenant(t, db, f.tenant, "Student", "Expand").ID
	f.group = testpkg.CreateTestEducationGroupForTenant(t, db, f.tenant, "Student Expand").ID
	f.account = testpkg.CreateTestAccount(t, db, "student-expand").ID
	return f
}

// insertChain creates one profile → membership → care-profile chain as
// superuser and returns the profile and membership IDs.
func (f studentOwnerExpandFixture) insertChain(t *testing.T, db *testpkg.DB) (int64, int64) {
	t.Helper()
	var profile, membership int64
	require.NoError(t, db.NewRaw(`INSERT INTO users.student_profiles (tenant_id, person_id)
		VALUES (?, ?) RETURNING id`, f.tenant, f.person).Scan(t.Context(), &profile))
	require.NoError(t, db.NewRaw(`INSERT INTO users.student_school_memberships (tenant_id, student_profile_id, school_class)
		VALUES (?, ?, '2a') RETURNING id`, f.tenant, profile).Scan(t.Context(), &membership))
	_, err := db.ExecContext(t.Context(), `INSERT INTO users.student_care_profiles (tenant_id, membership_id) VALUES (?, ?)`,
		f.tenant, membership)
	require.NoError(t, err)
	return profile, membership
}

func TestStudentOwnerStorageExpandTenantIsolation(t *testing.T) {
	t.Parallel()
	db := setupIsolatedStudentStorageBeforeCutover(t)
	fixtures := []studentOwnerExpandFixture{
		createStudentOwnerExpandFixture(t, db), createStudentOwnerExpandFixture(t, db),
	}
	// Each level is built by the level above, so the parent IDs are only known
	// once that subtest has inserted its rows under the tenant role.
	profiles, memberships := make([]int64, 2), make([]int64, 2)
	for _, tc := range []struct {
		table, insert, retarget string
		// args builds the insert arguments for fixture i stamped with tenant.
		args func(i int, tenant int64) []any
		// parent identifies fixture i's row for the tenant-retarget update.
		parent func(i int) int64
		// collect records the inserted IDs the next level hangs off; nil on the
		// leaf table, which has no surrogate id and no dependents.
		collect func(i int, id int64)
	}{
		{
			table:    "users.student_profiles",
			insert:   "INSERT INTO users.student_profiles (tenant_id, person_id) VALUES (?, ?)",
			retarget: "UPDATE users.student_profiles SET tenant_id = ? WHERE person_id = ?",
			args:     func(i int, tenant int64) []any { return []any{tenant, fixtures[i].person} },
			parent:   func(i int) int64 { return fixtures[i].person },
			collect:  func(i int, id int64) { profiles[i] = id },
		},
		{
			table:    "users.student_school_memberships",
			insert:   "INSERT INTO users.student_school_memberships (tenant_id, student_profile_id, school_class) VALUES (?, ?, '2a')",
			retarget: "UPDATE users.student_school_memberships SET tenant_id = ? WHERE student_profile_id = ?",
			args:     func(i int, tenant int64) []any { return []any{tenant, profiles[i]} },
			parent:   func(i int) int64 { return profiles[i] },
			collect:  func(i int, id int64) { memberships[i] = id },
		},
		{
			table:    "users.student_care_profiles",
			insert:   "INSERT INTO users.student_care_profiles (tenant_id, membership_id) VALUES (?, ?)",
			retarget: "UPDATE users.student_care_profiles SET tenant_id = ? WHERE membership_id = ?",
			args:     func(i int, tenant int64) []any { return []any{tenant, memberships[i]} },
			parent:   func(i int) int64 { return memberships[i] },
		},
	} {
		t.Run(tc.table, func(t *testing.T) {
			for i, own := range fixtures {
				require.NoError(t, testpkg.WithTenantTx(t, t.Context(), db, own.tenant, func(ctx context.Context, tx testpkg.Tx) error {
					_, err := tx.ExecContext(ctx, tc.insert, tc.args(i, own.tenant)...)
					return err
				}))
			}
			for i, own := range fixtures {
				other := fixtures[1-i]
				var tenants []int64
				require.NoError(t, testpkg.WithTenantTx(t, t.Context(), db, own.tenant, func(ctx context.Context, tx testpkg.Tx) error {
					return tx.NewRaw("SELECT tenant_id FROM "+tc.table).Scan(ctx, &tenants)
				}))
				require.Equal(t, []int64{own.tenant}, tenants, "unfiltered reads must remain tenant scoped")
				for _, query := range []string{
					"UPDATE " + tc.table + " SET updated_at = NOW() WHERE tenant_id = ?",
					"DELETE FROM " + tc.table + " WHERE tenant_id = ?",
				} {
					var affected int64
					require.NoError(t, testpkg.WithTenantTx(t, t.Context(), db, own.tenant, func(ctx context.Context, tx testpkg.Tx) error {
						result, err := tx.ExecContext(ctx, query, other.tenant)
						if err != nil {
							return err
						}
						affected, err = result.RowsAffected()
						return err
					}))
					require.Zero(t, affected)
				}
				err := testpkg.WithTenantTx(t, t.Context(), db, own.tenant, func(ctx context.Context, tx testpkg.Tx) error {
					_, err := tx.ExecContext(ctx, tc.insert, tc.args(i, other.tenant)...)
					return err
				})
				requirePresenceSQLState(t, err, "42501")
				err = testpkg.WithTenantTx(t, t.Context(), db, own.tenant, func(ctx context.Context, tx testpkg.Tx) error {
					_, err := tx.ExecContext(ctx, tc.retarget, other.tenant, tc.parent(i))
					return err
				})
				requirePresenceSQLState(t, err, "42501")
			}
			// An absent school context is deny-by-default, not a global read.
			tx, err := db.BeginTx(t.Context(), nil)
			require.NoError(t, err)
			defer func() { _ = tx.Rollback() }()
			_, err = tx.ExecContext(t.Context(), `SET LOCAL ROLE phoenix_tenant; SELECT set_config('app.current_tenant_id', '', true)`)
			require.NoError(t, err)
			var count int
			require.NoError(t, tx.NewRaw("SELECT count(*) FROM "+tc.table).Scan(t.Context(), &count))
			require.Zero(t, count)
			_, err = tx.ExecContext(t.Context(), tc.insert, tc.args(0, fixtures[0].tenant)...)
			requirePresenceSQLState(t, err, "42501")
			require.NoError(t, tx.Rollback())
			// Keep the tenant-inserted rows as parents for the next level.
			if tc.collect == nil {
				return
			}
			for i, own := range fixtures {
				var id int64
				require.NoError(t, db.NewRaw("SELECT id FROM "+tc.table+" WHERE tenant_id = ?", own.tenant).
					Scan(t.Context(), &id))
				tc.collect(i, id)
			}
		})
	}
}

func TestStudentOwnerStorageExpandConstraints(t *testing.T) {
	t.Parallel()
	db := setupIsolatedStudentStorageBeforeCutover(t)
	a, b := createStudentOwnerExpandFixture(t, db), createStudentOwnerExpandFixture(t, db)
	profile, membership := a.insertChain(t, db)

	// Cross-tenant parents are rejected by the composite FKs.
	for _, tc := range []struct {
		query string
		id    int64
	}{
		{"UPDATE users.student_profiles SET person_id = ? WHERE tenant_id = ?", b.person},
		{"UPDATE users.student_school_memberships SET group_id = ? WHERE tenant_id = ?", b.group},
		{"UPDATE users.student_school_memberships SET tenant_id = ? WHERE tenant_id = ?", b.tenant},
		{"UPDATE users.student_care_profiles SET tenant_id = ? WHERE tenant_id = ?", b.tenant},
	} {
		_, err := db.ExecContext(t.Context(), tc.query, tc.id, a.tenant)
		requirePresenceSQLState(t, err, "23503")
	}
	// One profile per person, one care profile per membership.
	for _, query := range []string{
		`INSERT INTO users.student_profiles (tenant_id, person_id) VALUES (?, ?)`,
		`INSERT INTO users.student_care_profiles (tenant_id, membership_id) VALUES (?, ?)`,
	} {
		args := []any{a.tenant, membership}
		if strings.Contains(query, "person_id") {
			args = []any{a.tenant, a.person}
		}
		_, err := db.ExecContext(t.Context(), query, args...)
		requirePresenceSQLState(t, err, "23505")
	}
	// At most one live membership per profile; soft deletion frees the slot
	// again and keeps the retired row addressable.
	_, err := db.ExecContext(t.Context(), `INSERT INTO users.student_school_memberships
		(tenant_id, student_profile_id, school_class) VALUES (?, ?, '3a')`, a.tenant, profile)
	requirePresenceSQLState(t, err, "23505")
	_, err = db.ExecContext(t.Context(), `UPDATE users.student_school_memberships SET deleted_at = NOW() WHERE id = ?`, membership)
	require.NoError(t, err)
	_, err = db.ExecContext(t.Context(), `INSERT INTO users.student_school_memberships
		(tenant_id, student_profile_id, school_class) VALUES (?, ?, '3a')`, a.tenant, profile)
	require.NoError(t, err)
	_, err = db.ExecContext(t.Context(), `UPDATE users.student_school_memberships SET deleted_at = NULL WHERE id = ?`, membership)
	requirePresenceSQLState(t, err, "23505")

	// The lifecycle domain matches users.students.
	for _, status := range []string{"pending", "active", "inactive", "alumnus"} {
		_, err := db.ExecContext(t.Context(), `UPDATE users.student_school_memberships SET status = ? WHERE id = ?`, status, membership)
		require.NoError(t, err, status)
	}
	_, err = db.ExecContext(t.Context(), `UPDATE users.student_school_memberships SET status = 'graduated' WHERE id = ?`, membership)
	requirePresenceSQLState(t, err, "23514")

	// The care plan keeps the departure-shape validators of users.students:
	// the weekday keys, the per-column value shape and the mode vocabulary.
	for _, tc := range []struct {
		column, valid string
	}{
		{"departure_days", `{"mon": "bus"}`},
		{"allowed_departure_modes", `{"mon": ["bus", "pickup"]}`},
		{"pickup_days", `{"mon": true}`},
		{"bus_days", `{"mon": true}`},
	} {
		for _, invalid := range []string{`{"monday": "bus"}`, `{"mon": "nonsense"}`, `[]`} {
			_, err := db.ExecContext(t.Context(),
				fmt.Sprintf(`UPDATE users.student_care_profiles SET %s = ?::jsonb WHERE membership_id = ?`, tc.column),
				invalid, membership)
			requirePresenceSQLState(t, err, "23514")
		}
		_, err := db.ExecContext(t.Context(),
			fmt.Sprintf(`UPDATE users.student_care_profiles SET %s = ?::jsonb WHERE membership_id = ?`, tc.column),
			tc.valid, membership)
		require.NoError(t, err, tc.column)
	}
}

// The care plan reuses the four departure validators users.students already
// constrains itself with, so the backfill cannot be rejected by a divergent
// copy of the vocabulary. That makes the functions shared schema objects: a
// rollback of the column migration that introduced one no longer owns it
// alone, and must leave it standing for the target table still depending on
// it.
func TestStudentOwnerStorageExpandKeepsSharedDepartureValidators(t *testing.T) {
	t.Parallel()
	db := setupIsolatedStudentStorageBeforeCutover(t)
	f := createStudentOwnerExpandFixture(t, db)
	_, membership := f.insertChain(t, db)
	// The rollbacks are not undone afterwards: this test owns its database
	// clone, and re-running the forward migrations is not possible anyway
	// once a later migration has dropped the legacy columns they backfill from.
	for _, tc := range []struct {
		validator, column, invalid string
		down                       func(context.Context, *testpkg.DB) error
	}{
		{"is_valid_bus_days", "bus_days", `{"monday": true}`, studentsBusDaysDown},
		{"is_valid_pickup_days", "pickup_days", `{"monday": true}`, studentsPickupDaysDown},
		{"is_valid_departure_days", "departure_days", `{"monday": "bus"}`, studentsDepartureDaysDown},
		{
			"is_valid_allowed_departure_modes", "allowed_departure_modes", `{"monday": ["bus"]}`,
			studentsAllowedDepartureModesDown,
		},
	} {
		t.Run(tc.validator, func(t *testing.T) {
			require.NoError(t, tc.down(t.Context(), db))
			var kept bool
			require.NoError(t, db.NewRaw(`SELECT to_regprocedure(?) IS NOT NULL`,
				"users."+tc.validator+"(jsonb)").Scan(t.Context(), &kept))
			require.True(t, kept, "a dependent CHECK must keep the shared validator alive")
			_, err := db.ExecContext(t.Context(),
				fmt.Sprintf(`UPDATE users.student_care_profiles SET %s = ?::jsonb WHERE membership_id = ?`, tc.column),
				tc.invalid, membership)
			requirePresenceSQLState(t, err, "23514")
		})
	}
	// With the last dependent target gone the validators are droppable again.
	_, err := db.ExecContext(t.Context(), `DROP TABLE users.student_care_profiles`)
	require.NoError(t, err)
	for _, validator := range []string{
		"is_valid_bus_days", "is_valid_pickup_days", "is_valid_departure_days", "is_valid_allowed_departure_modes",
	} {
		_, err := db.ExecContext(t.Context(), `DROP FUNCTION users.`+validator+`(JSONB)`)
		require.NoError(t, err, validator)
	}
}

func TestStudentOwnerStorageExpandReferenceDeletion(t *testing.T) {
	t.Parallel()
	db := setupIsolatedStudentStorageBeforeCutover(t)
	f := createStudentOwnerExpandFixture(t, db)
	profile, membership := f.insertChain(t, db)
	_, err := db.ExecContext(t.Context(), `UPDATE users.student_profiles
		SET photo_path = 'photos/child.jpg', photo_consent_given_at = NOW(), photo_consent_given_by = ? WHERE id = ?`,
		f.account, profile)
	require.NoError(t, err)
	_, err = db.ExecContext(t.Context(), `UPDATE users.student_school_memberships SET group_id = ? WHERE id = ?`, f.group, membership)
	require.NoError(t, err)

	// Losing the consenting account clears only the attribution.
	_, err = db.ExecContext(t.Context(), `DELETE FROM auth.accounts WHERE id = ?`, f.account)
	require.NoError(t, err)
	var preserved bool
	require.NoError(t, db.NewRaw(`SELECT photo_consent_given_by IS NULL AND photo_path = 'photos/child.jpg'
		AND photo_consent_given_at IS NOT NULL FROM users.student_profiles WHERE id = ?`, profile).Scan(t.Context(), &preserved))
	require.True(t, preserved)
	// Losing the group leaves the membership without one, as on users.students.
	_, err = db.ExecContext(t.Context(), `DELETE FROM education.groups WHERE id = ?`, f.group)
	require.NoError(t, err)
	require.NoError(t, db.NewRaw(`SELECT group_id IS NULL FROM users.student_school_memberships WHERE id = ?`, membership).
		Scan(t.Context(), &preserved))
	require.True(t, preserved)

	// Deleting the membership cascades to Care Plan and leaves People intact.
	_, err = db.ExecContext(t.Context(), `DELETE FROM users.student_school_memberships WHERE id = ?`, membership)
	require.NoError(t, err)
	var careProfiles, remainingProfiles int
	require.NoError(t, db.NewRaw(`SELECT (SELECT count(*) FROM users.student_care_profiles),
		(SELECT count(*) FROM users.student_profiles)`).Scan(t.Context(), &careProfiles, &remainingProfiles))
	require.Zero(t, careProfiles)
	require.Equal(t, 1, remainingProfiles)

	// Deleting the person cascades through the whole chain.
	require.NoError(t, db.NewRaw(`INSERT INTO users.student_school_memberships (tenant_id, student_profile_id, school_class)
		VALUES (?, ?, '3a') RETURNING id`, f.tenant, profile).Scan(t.Context(), &membership))
	_, err = db.ExecContext(t.Context(), `INSERT INTO users.student_care_profiles (tenant_id, membership_id) VALUES (?, ?)`,
		f.tenant, membership)
	require.NoError(t, err)
	_, err = db.ExecContext(t.Context(), `DELETE FROM users.persons WHERE id = ?`, f.person)
	require.NoError(t, err)
	assertStudentOwnerStorageTargetsEmpty(t, db)
}

func studentOldStorageSnapshot(t *testing.T, db *testpkg.DB) []string {
	t.Helper()
	var result []string
	for _, query := range []string{
		"SELECT COALESCE(jsonb_agg(to_jsonb(row) ORDER BY id), '[]')::text FROM users.students row",
		`SELECT COALESCE(jsonb_agg(to_jsonb(c) ORDER BY ordinal_position), '[]')::text FROM information_schema.columns c WHERE table_schema = 'users' AND table_name = 'students'`,
		`SELECT COALESCE(jsonb_agg(pg_get_constraintdef(oid) ORDER BY conname), '[]')::text FROM pg_constraint WHERE conrelid = 'users.students'::regclass`,
		`SELECT COALESCE(jsonb_agg(indexdef ORDER BY indexname), '[]')::text FROM pg_indexes WHERE schemaname = 'users' AND tablename = 'students'`,
		`SELECT COALESCE(jsonb_agg(pg_get_triggerdef(oid) ORDER BY tgname), '[]')::text FROM pg_trigger WHERE tgrelid = 'users.students'::regclass AND NOT tgisinternal`,
		`SELECT COALESCE(jsonb_agg(jsonb_build_object('name', policyname, 'qual', qual, 'check', with_check) ORDER BY policyname), '[]')::text FROM pg_policies WHERE schemaname = 'users' AND tablename = 'students'`,
	} {
		var snapshot string
		require.NoError(t, db.NewRaw(query).Scan(t.Context(), &snapshot))
		result = append(result, snapshot)
	}
	return result
}

func assertStudentOwnerStorageTargetsEmpty(t *testing.T, db *testpkg.DB) {
	t.Helper()
	var count int
	require.NoError(t, db.NewRaw(`SELECT (SELECT count(*) FROM users.student_profiles)
		+ (SELECT count(*) FROM users.student_school_memberships)
		+ (SELECT count(*) FROM users.student_care_profiles)`).Scan(t.Context(), &count))
	require.Zero(t, count, "Expand must never copy or dual-write old rows")
}

func TestStudentOwnerStorageExpandUpDownPreservesOldAuthority(t *testing.T) {
	t.Parallel()
	db := setupIsolatedStudentStorageBeforeCutover(t)
	f := createStudentOwnerExpandFixture(t, db)
	student := testpkg.CreateTestStudentForTenant(t, db, f.tenant, "Old", "Authority", "1a")
	assertStudentOwnerStorageTargetsEmpty(t, db)
	before := studentOldStorageSnapshot(t, db)
	require.NoError(t, runPresenceMigration(t.Context(), db, studentOwnerStorageExpandName, false))
	require.Equal(t, before, studentOldStorageSnapshot(t, db))
	require.NoError(t, runPresenceMigration(t.Context(), db, studentOwnerStorageExpandName, true))
	require.Equal(t, before, studentOldStorageSnapshot(t, db))
	assertStudentOwnerStorageTargetsEmpty(t, db)
	// The previous application's SQL shape still reads and writes the old
	// table across all three future owners' columns. No target row is required
	// or produced.
	require.NoError(t, testpkg.WithTenantTx(t, t.Context(), db, f.tenant, func(ctx context.Context, tx testpkg.Tx) error {
		if _, err := tx.ExecContext(ctx, `INSERT INTO users.students
			(tenant_id, person_id, school_class, group_id, status, address_city, extra_info, agb_accepted_at)
			VALUES (?, ?, '4b', ?, 'pending', 'Bonn', 'Allergie', NOW())`, f.tenant, f.person, f.group); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `UPDATE users.students SET supervisor_notes = 'Notiz', health_info = 'Asthma',
			pickup_status = 'pickup', departure_days = '{"mon": "bus"}', sick = TRUE, sick_since = NOW(),
			enrolled_until = CURRENT_DATE WHERE id = ?`, student.ID)
		return err
	}))
	var written bool
	require.NoError(t, db.NewRaw(`SELECT supervisor_notes = 'Notiz' AND health_info = 'Asthma' AND sick
		AND departure_days = '{"mon": "bus"}'::jsonb FROM users.students WHERE id = ?`, student.ID).Scan(t.Context(), &written))
	require.True(t, written, "old-table reads and writes keep working")
	assertStudentOwnerStorageTargetsEmpty(t, db)
	beforeDown := studentOldStorageSnapshot(t, db)
	require.NoError(t, runPresenceMigration(t.Context(), db, studentOwnerStorageExpandName, false))
	require.Equal(t, beforeDown, studentOldStorageSnapshot(t, db))
	var remaining int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'users' AND (c.relname LIKE '%student_profiles%'
			OR c.relname LIKE '%student_school_memberships%' OR c.relname LIKE '%student_care_profiles%')`).Scan(t.Context(), &remaining))
	require.Zero(t, remaining, "tables, indexes and owned sequences must all be gone")
	var policies int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM pg_policies WHERE tablename IN
		('student_profiles', 'student_school_memberships', 'student_care_profiles')`).Scan(t.Context(), &policies))
	require.Zero(t, policies)
	require.NoError(t, runPresenceMigration(t.Context(), db, studentOwnerStorageExpandName, true))
	assertStudentOwnerStorageTargetsEmpty(t, db)
}

func TestStudentOwnerStorageExpandRollbackRefusesPopulatedTargets(t *testing.T) {
	t.Parallel()
	db := setupIsolatedStudentStorageBeforeCutover(t)
	f := createStudentOwnerExpandFixture(t, db)
	f.insertChain(t, db)
	// Drain from the leaves inward so each table is the only populated one.
	for _, table := range studentOwnerStorageTargetsLeafFirst() {
		require.ErrorContains(t, runPresenceMigration(t.Context(), db, studentOwnerStorageExpandName, false), "requires empty target tables")
		var count int
		require.NoError(t, db.NewRaw("SELECT count(*) FROM "+table).Scan(t.Context(), &count))
		require.Equal(t, 1, count)
		_, err := db.ExecContext(t.Context(), "DELETE FROM "+table)
		require.NoError(t, err)
	}
	require.NoError(t, runPresenceMigration(t.Context(), db, studentOwnerStorageExpandName, false))
	require.NoError(t, runPresenceMigration(t.Context(), db, studentOwnerStorageExpandName, true))
	assertStudentOwnerStorageTargetsEmpty(t, db)
}

func studentOwnerStorageTargetsLeafFirst() []string {
	return []string{"users.student_care_profiles", "users.student_school_memberships", "users.student_profiles"}
}

func TestStudentOwnerStorageExpandCatalogAndDefaults(t *testing.T) {
	t.Parallel()
	db := setupIsolatedStudentStorageBeforeCutover(t)
	for _, table := range studentOwnerStorageTargets {
		schema, name, _ := strings.Cut(table, ".")
		var forced bool
		require.NoError(t, db.NewRaw(`SELECT relrowsecurity AND relforcerowsecurity FROM pg_class WHERE oid = ?::regclass`, table).Scan(t.Context(), &forced))
		require.True(t, forced, table)
		var policies []string
		require.NoError(t, db.NewRaw(`SELECT policyname FROM pg_policies WHERE schemaname = ? AND tablename = ?`, schema, name).Scan(t.Context(), &policies))
		require.Equal(t, []string{"tenant_isolation_" + schema + "_" + name}, policies)
		var invalid int
		require.NoError(t, db.NewRaw(`SELECT count(*) FROM pg_index WHERE indrelid = ?::regclass AND NOT indisvalid`, table).Scan(t.Context(), &invalid))
		require.Zero(t, invalid)
		var customTriggers int
		require.NoError(t, db.NewRaw(`SELECT count(*) FROM pg_trigger WHERE tgrelid = ?::regclass AND NOT tgisinternal`, table).Scan(t.Context(), &customTriggers))
		require.Zero(t, customTriggers, "Expand adds no routing or compatibility triggers")
		for _, role := range []string{"phoenix_tenant", "phoenix_admin"} {
			var granted bool
			require.NoError(t, db.NewRaw(`SELECT has_table_privilege(?, ?::regclass, 'SELECT, INSERT, UPDATE, DELETE')`, role, table).Scan(t.Context(), &granted))
			require.True(t, granted, "%s on %s", role, table)
		}
	}
	for index, columns := range map[string]string{
		"users.uq_student_profiles_person":                           "tenant_id, person_id",
		"users.idx_student_profiles_photo_consent":                   "tenant_id",
		"users.uq_student_school_memberships_active_profile":         "tenant_id, student_profile_id",
		"users.idx_student_school_memberships_group":                 "tenant_id, group_id",
		"users.idx_student_school_memberships_class":                 "tenant_id, school_class",
		"users.idx_student_school_memberships_class_normalized":      "tenant_id, lower(btrim(school_class))",
		"users.idx_student_school_memberships_status":                "tenant_id, status",
		"users.idx_student_school_memberships_enrolled_from_pending": "tenant_id, enrolled_from",
		"users.idx_student_school_memberships_enrolled_until_active": "tenant_id, enrolled_until",
		"users.idx_student_school_memberships_enrolled_until_ended":  "tenant_id, enrolled_until",
		"users.uq_student_care_profiles_tenant_id":                   "tenant_id, membership_id",
		"users.idx_student_care_profiles_pickup_status":              "tenant_id, pickup_status",
	} {
		var actual string
		require.NoError(t, db.NewRaw(`SELECT string_agg(pg_get_indexdef(indexrelid, n, true), ', ' ORDER BY n)
			FROM pg_index CROSS JOIN LATERAL generate_series(1, indnkeyatts) n
			WHERE indexrelid = ?::regclass`, index).Scan(t.Context(), &actual))
		require.Equal(t, columns, actual, index)
	}
	f := createStudentOwnerExpandFixture(t, db)
	var defaults bool
	require.NoError(t, db.NewRaw(`INSERT INTO users.student_profiles (tenant_id, person_id) VALUES (?, ?)
		RETURNING address_street IS NULL AND address_city IS NULL AND address_postal_code IS NULL
		AND extra_info IS NULL AND photo_path IS NULL AND photo_consent_given_at IS NULL
		AND photo_consent_given_by IS NULL AND agb_accepted_at IS NULL AND data_processing_accepted_at IS NULL
		AND email_contact_accepted_at IS NULL AND created_at IS NOT NULL AND updated_at IS NOT NULL`,
		f.tenant, f.person).Scan(t.Context(), &defaults))
	require.True(t, defaults)
	require.NoError(t, db.NewRaw(`INSERT INTO users.student_school_memberships (tenant_id, student_profile_id, school_class)
		SELECT tenant_id, id, '2a' FROM users.student_profiles
		RETURNING status = 'active' AND group_id IS NULL AND enrolled_from IS NULL AND enrolled_until IS NULL
		AND deleted_at IS NULL AND created_at IS NOT NULL AND updated_at IS NOT NULL`).Scan(t.Context(), &defaults))
	require.True(t, defaults)
	require.NoError(t, db.NewRaw(`INSERT INTO users.student_care_profiles (tenant_id, membership_id)
		SELECT tenant_id, id FROM users.student_school_memberships
		RETURNING supervisor_notes IS NULL AND health_info IS NULL AND pickup_status IS NULL
		AND departure_companion_note IS NULL AND departure_days = '{}'::jsonb
		AND allowed_departure_modes = '{}'::jsonb AND pickup_days = '{}'::jsonb AND bus_days = '{}'::jsonb
		AND created_at IS NOT NULL AND updated_at IS NOT NULL`).Scan(t.Context(), &defaults))
	require.True(t, defaults)
}

func TestStudentOwnerStorageExpandFailureIsAtomic(t *testing.T) {
	t.Parallel()
	db := setupIsolatedStudentStorageBeforeCutover(t)
	require.NoError(t, runPresenceMigration(t.Context(), db, studentOwnerStorageExpandName, false))
	// Fail after the first two tables exist, before grants and RLS provisioning.
	_, err := db.ExecContext(t.Context(), `CREATE TABLE users.student_care_profiles (probe BOOLEAN)`)
	require.NoError(t, err)
	require.Error(t, runPresenceMigration(t.Context(), db, studentOwnerStorageExpandName, true))
	for _, table := range studentOwnerStorageTargets[:2] {
		var absent bool
		require.NoError(t, db.NewRaw(`SELECT to_regclass(?) IS NULL`, table).Scan(t.Context(), &absent))
		require.True(t, absent, "a failed Expand must leave no partially provisioned %s", table)
	}
	_, err = db.ExecContext(t.Context(), `DROP TABLE users.student_care_profiles`)
	require.NoError(t, err)
	require.NoError(t, runPresenceMigration(t.Context(), db, studentOwnerStorageExpandName, true))
}
