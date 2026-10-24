package compose_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type personExportFixture struct {
	db                           *bun.DB
	school, otherSchool          int64
	child, sibling, foreignChild int64
	guardian, staff, otherStaff  int64
	uploads                      string
}

func newPersonExportFixture(t *testing.T) personExportFixture {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	a := testpkg.Tenant(t)
	b, _ := testpkg.CreateTestTenant(t, db)
	x := testpkg.CreateTestStudent(t, db, "SyntheticSelected", "Fixture", "X")
	y := testpkg.CreateTestStudent(t, db, "SyntheticSibling", "Fixture", "X")
	z := testpkg.CreateTestStudentForTenant(t, db, b, "SyntheticForeign", "Fixture", "X")
	g := testpkg.CreateTestGuardianProfileForTenant(t, db, a, "SyntheticGuardian", "Fixture", fmt.Sprintf("guardian-%d@example.invalid", a))
	guardianAccount := testpkg.CreateTestAccount(t, db, fmt.Sprintf("guardian-account-%d@example.invalid", a))
	_, err := db.NewRaw("UPDATE users.guardian_profiles SET account_id = ?, has_account = true WHERE id = ?", guardianAccount.ID, g.ID).Exec(t.Context())
	require.NoError(t, err)
	for _, child := range []int64{x.ID, y.ID} {
		testpkg.CreateTestStudentGuardianLinkForTenant(t, db, a, child, g.ID, "primary_guardian")
	}
	s, account := testpkg.CreateTestStaffWithAccount(t, db, "SyntheticStaffA", "Fixture")
	sb := testpkg.CreateTestStaffForTenant(t, db, b, "SyntheticStaffB", "Fixture")
	_, err = db.NewRaw("UPDATE users.persons SET account_id = ? WHERE id = ?", account.ID, sb.PersonID).Exec(t.Context())
	require.NoError(t, err)
	for _, child := range []int64{x.ID, y.ID} {
		_, err := db.NewRaw(`WITH thread AS (
			INSERT INTO users.parent_message_threads (tenant_id, student_id, guardian_account_id)
			VALUES (?0, ?1, ?2) RETURNING id)
			INSERT INTO users.parent_messages (tenant_id, thread_id, student_id, sender_account_id, sender_kind, sender_name, body)
			SELECT ?0, id, ?1, ?2, 'guardian', 'Synthetic', 'synthetic-message-' || ?1 FROM thread`, a, child, guardianAccount.ID).Exec(t.Context())
		require.NoError(t, err)
		_, err = db.NewRaw(`INSERT INTO users.student_notes (tenant_id, student_id, author_account_id, kind, body)
			VALUES (?, ?, ?, 'permanent', ?)`, a, child, account.ID, "synthetic-note-"+strconv.FormatInt(child, 10)).Exec(t.Context())
		require.NoError(t, err)
	}
	// Historical audit records deliberately have no subject FK.
	_, err = db.NewRaw(`INSERT INTO audit.guardian_changes (tenant_id, student_id, guardian_profile_id, change_type, field_name)
		VALUES (?, ?, ?, 'updated', 'phone')`, a, x.ID, g.ID).Exec(t.Context())
	require.NoError(t, err)
	_, err = db.NewRaw(`INSERT INTO audit.staff_master_data_changes (tenant_id, staff_id, changed_by, section, field_name, old_value, new_value, note)
		VALUES (?, ?, ?, 'person', 'first_name', 'synthetic-old', 'synthetic-new', '')`, a, s.ID, s.ID).Exec(t.Context())
	require.NoError(t, err)
	_, err = db.NewRaw(`INSERT INTO audit.data_access_log (tenant_id, actor_account_id, actor_role, resource_type, range_start, range_end, metadata)
		VALUES (?, ?, 'admin', 'staff_financial_view', NOW(), NOW(), jsonb_build_object('staff_id', ?::bigint))`, a, guardianAccount.ID, s.ID).Exec(t.Context())
	require.NoError(t, err)
	group := testpkg.CreateTestEducationGroup(t, db, "SyntheticSubstitution")
	today := calendar.TodayDate()
	substitution := testpkg.CreateTestGroupSubstitution(t, db, group.ID, nil, s.ID, today, today)
	_, err = db.NewRaw(`INSERT INTO audit.substitution_changes
		(tenant_id,substitution_id,target_type,action,group_id,target_staff_id,actor_account_id,start_date,end_date)
		VALUES (?,?,'group_handover','assigned',?,?,?,?,?)`, a, substitution.ID, group.ID, s.ID, guardianAccount.ID, today, today).Exec(t.Context())
	require.NoError(t, err)
	root := t.TempDir()
	for _, student := range []struct{ id, school int64 }{{x.ID, a}, {y.ID, a}, {z.ID, b}} {
		key := fmt.Sprintf("student-photos/%d_%d.bin", student.school, student.id)
		require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(root, key)), 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(root, key), []byte("synthetic-original-"+strconv.FormatInt(student.id, 10)), 0o600))
		_, err := db.NewRaw("UPDATE users.student_profiles SET photo_path = ? WHERE id = ?", "/uploads/"+key, student.id).Exec(t.Context())
		require.NoError(t, err)
	}
	return personExportFixture{db: db, school: a, otherSchool: b, child: x.ID, sibling: y.ID, foreignChild: z.ID, guardian: g.ID, staff: s.ID, otherStaff: sb.ID, uploads: root}
}

func personExport(t *testing.T, fixture personExportFixture, subject string) string {
	t.Helper()
	exporter, err := compose.NewTenantExporter(fixture.db, organizationtenancy.ExportFingerprints{Bytes: testpkg.ContentFingerprint, Reader: testpkg.ReaderContentFingerprint})
	require.NoError(t, err)
	out := filepath.Join(t.TempDir(), "person")
	require.NoError(t, exporter.Export(t.Context(), organizationtenancy.TenantExportOptions{SchoolID: fixture.school, Person: subject, Output: out, Uploads: fixture.uploads}))
	manifest := readExportManifest(t, out)
	require.True(t, manifest.ReviewRequired)
	assertExportArtifacts(t, out, manifest)
	return out
}

func personTable(t *testing.T, out, table string) []map[string]json.RawMessage {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(out, table+".jsonl"))
	require.NoError(t, err)
	var rows []map[string]json.RawMessage
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var row map[string]json.RawMessage
		require.NoError(t, json.Unmarshal([]byte(line), &row))
		rows = append(rows, row)
	}
	return rows
}

func TestPersonExportChildDoesNotExpandSharedGuardianToSibling(t *testing.T) {
	t.Parallel()
	f := newPersonExportFixture(t)
	out := personExport(t, f, "kind:"+strconv.FormatInt(f.child, 10))
	children := personTable(t, out, "users.student_profiles")
	require.Len(t, children, 1)
	require.JSONEq(t, strconv.FormatInt(f.child, 10), string(children[0]["id"]))
	require.Len(t, personTable(t, out, "users.student_notes"), 1)
	require.Len(t, personTable(t, out, "users.parent_messages"), 1)
	require.Len(t, personTable(t, out, "audit.guardian_changes"), 1)
	require.Len(t, personTable(t, out, "users.student_guardian_relationships"), 1)
	require.Empty(t, personTable(t, out, "users.staff_school_memberships"), "a note author is context, never a second subject")
	objects := personTable(t, out, "objects")
	require.Len(t, objects, 1)
	var key, path string
	require.NoError(t, json.Unmarshal(objects[0]["key"], &key))
	require.NoError(t, json.Unmarshal(objects[0]["path"], &path))
	require.Equal(t, fmt.Sprintf("student-photos/%d_%d.bin", f.school, f.child), key)
	data, err := os.ReadFile(filepath.Join(out, path))
	require.NoError(t, err)
	require.Equal(t, "synthetic-original-"+strconv.FormatInt(f.child, 10), string(data))
	assertPersonReview(t, out, true)
}

func TestPersonExportGuardianAndSharedStaffStayInSchool(t *testing.T) {
	t.Parallel()
	f := newPersonExportFixture(t)
	guardian := personExport(t, f, "guardian:"+strconv.FormatInt(f.guardian, 10))
	require.Len(t, personTable(t, guardian, "users.student_guardian_relationships"), 2)
	require.Len(t, personTable(t, guardian, "users.parent_messages"), 2)
	// Children are context: their other notes/attendance are not automatically
	// the guardian's own data. Each context row needs third-party review.
	require.Empty(t, personTable(t, guardian, "users.student_notes"))
	staff := personExport(t, f, "staff:"+strconv.FormatInt(f.staff, 10))
	rows := personTable(t, staff, "users.staff_school_memberships")
	require.Len(t, rows, 1)
	require.JSONEq(t, strconv.FormatInt(f.staff, 10), string(rows[0]["id"]))
	require.Len(t, personTable(t, staff, "audit.staff_master_data_changes"), 1)
	require.Len(t, personTable(t, staff, "audit.substitution_changes"), 1)
	require.Len(t, personTable(t, staff, "audit.data_access_log"), 1)
	for _, out := range []string{guardian, staff} {
		for _, table := range readExportManifest(t, out).Tables {
			if table.Classification != "include" {
				continue
			}
			for _, row := range personTable(t, out, table.Name) {
				if tenant, ok := row["tenant_id"]; ok {
					require.Equal(t, strconv.FormatInt(f.school, 10), string(tenant), table.Name)
				}
				for column := range table.ExcludedColumns {
					require.NotContains(t, row, column, table.Name)
				}
			}
		}
		assertPersonReview(t, out, false)
	}
}

func assertPersonReview(t *testing.T, out string, hasUnassigned bool) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(out, "review.json"))
	require.NoError(t, err)
	var review struct {
		Status     string   `json:"status"`
		Unassigned int64    `json:"unassignedObjects"`
		Checks     []string `json:"requiredChecks"`
		Items      []struct {
			Table string   `json:"table"`
			Row   int64    `json:"row"`
			Rules []string `json:"rules"`
		} `json:"items"`
	}
	require.NoError(t, json.Unmarshal(data, &review))
	require.Equal(t, "internal-review-only-not-approved-for-handover", review.Status)
	require.Contains(t, review.Checks, "catalog-excluded-columns-and-tables-lawful-context-without-credentials")
	if hasUnassigned {
		require.Positive(t, review.Unassigned)
	}
	require.NotEmpty(t, review.Items)
	marked := map[string]map[int64]bool{}
	for _, item := range review.Items {
		require.Contains(t, item.Rules, "third-party-review")
		if marked[item.Table] == nil {
			marked[item.Table] = map[int64]bool{}
		}
		marked[item.Table][item.Row] = true
	}
	for _, table := range readExportManifest(t, out).Tables {
		if table.Classification == "include" {
			for row := range table.Rows {
				require.True(t, marked[table.Name][row+1], table.Name)
			}
		}
	}
}

func TestPersonExportRejectsForeignSubjectAndMissingOriginal(t *testing.T) {
	t.Parallel()
	f := newPersonExportFixture(t)
	exporter, err := compose.NewTenantExporter(f.db, organizationtenancy.ExportFingerprints{Bytes: testpkg.ContentFingerprint, Reader: testpkg.ReaderContentFingerprint})
	require.NoError(t, err)
	for _, subject := range []string{"kind:" + strconv.FormatInt(f.foreignChild, 10), "staff:" + strconv.FormatInt(f.otherStaff, 10), "student:1", "kind:0", "kind:01"} {
		out := filepath.Join(t.TempDir(), "failed")
		err := exporter.Export(t.Context(), organizationtenancy.TenantExportOptions{SchoolID: f.school, Person: subject, Output: out, Uploads: f.uploads})
		require.Error(t, err)
		require.NoFileExists(t, filepath.Join(out, "COMPLETE"))
	}
	require.NoError(t, os.Remove(filepath.Join(f.uploads, fmt.Sprintf("student-photos/%d_%d.bin", f.school, f.child))))
	out := filepath.Join(t.TempDir(), "missing")
	err = exporter.Export(t.Context(), organizationtenancy.TenantExportOptions{SchoolID: f.school, Person: "kind:" + strconv.FormatInt(f.child, 10), Output: out, Uploads: f.uploads})
	require.Error(t, err)
	require.NoFileExists(t, filepath.Join(out, "COMPLETE"))
	require.NotContains(t, err.Error(), f.uploads)
}

func TestPersonExportIncludesDocumentsAndOnlySelectedEnrollmentChild(t *testing.T) {
	t.Parallel()
	f := newPersonExportFixture(t)
	phase := testpkg.CreateTestEnrollmentPhase(t, f.db)
	var request int64
	require.NoError(t, f.db.NewRaw(`INSERT INTO enrollment.requests
		(tenant_id,phase_id,guardian_first_name,guardian_last_name,guardian_email,status_token)
		VALUES (?,?,'Synthetic','Fixture','synthetic@example.invalid',?) RETURNING id`,
		f.school, phase.ID, fmt.Sprintf("example-test-status-%d", phase.ID)).Scan(t.Context(), &request))
	for _, student := range []int64{f.child, f.sibling} {
		var child, change int64
		require.NoError(t, f.db.NewRaw(`INSERT INTO enrollment.request_children
			(tenant_id,request_id,first_name,last_name,date_of_birth,status,created_student_id)
			VALUES (?,?,'Synthetic','Fixture','2019-01-01','approved',?) RETURNING id`, f.school, request, student).Scan(t.Context(), &child))
		require.NoError(t, f.db.NewRaw(`INSERT INTO enrollment.change_requests
			(tenant_id,request_id,request_child_id,base_snapshot,proposed_snapshot)
			VALUES (?,?,?,'{}'::jsonb,'{"synthetic":"snapshot"}'::jsonb) RETURNING id`, f.school, request, child).Scan(t.Context(), &change))
		_, err := f.db.NewRaw(`INSERT INTO enrollment.change_request_messages
			(tenant_id,change_request_id,author_type,body)
			VALUES (?,?,'system',?)`, f.school, change, fmt.Sprintf("synthetic-change-%d", student)).Exec(t.Context())
		require.NoError(t, err)
		_, err = f.db.NewRaw(`INSERT INTO users.student_documents
			(tenant_id,student_id,category,filename_display,filename_stored,size_bytes,content_type,uploaded_by)
			VALUES (?,?,'sonstiges','synthetic.bin',?,1,'application/octet-stream',?)`,
			f.school, student, fmt.Sprintf("synthetic-%d.bin", student), f.staff).Exec(t.Context())
		require.NoError(t, err)
		dir := filepath.Join(f.uploads, "student-documents", strconv.FormatInt(f.school, 10))
		require.NoError(t, os.MkdirAll(dir, 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(dir, fmt.Sprintf("synthetic-%d.bin", student)), []byte("synthetic-document-original"), 0o600))
	}
	out := personExport(t, f, "kind:"+strconv.FormatInt(f.child, 10))
	for _, table := range []string{"enrollment.requests", "enrollment.request_children", "enrollment.change_requests", "enrollment.change_request_messages", "users.student_documents"} {
		require.Len(t, personTable(t, out, table), 1, table)
	}
	child := personTable(t, out, "enrollment.request_children")[0]
	require.Equal(t, strconv.FormatInt(f.child, 10), string(child["created_student_id"]))
	messages := personTable(t, out, "enrollment.change_request_messages")
	require.JSONEq(t, strconv.Quote(fmt.Sprintf("synthetic-change-%d", f.child)), string(messages[0]["body"]))
	require.Len(t, personTable(t, out, "objects"), 2, "only the selected child's photo and document")
	assertPersonReview(t, out, true)
}
