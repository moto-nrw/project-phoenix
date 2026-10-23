package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/delivery/objects"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	tenancyCompose "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type failingErasureFiles struct {
	*objects.Local
	fail       bool
	onRemove   func()
	removeOnce sync.Once
}

func (f *failingErasureFiles) TenantObjects(ctx context.Context, kind, layout string, id int64, remove bool) (int64, error) {
	if remove && f.onRemove != nil {
		f.removeOnce.Do(f.onRemove)
	}
	if remove && kind == "student-documents" && f.fail {
		return 0, fmt.Errorf("synthetic failure")
	}
	return f.Local.TenantObjects(ctx, kind, layout, id, remove)
}

func erasureTableCounts(t *testing.T, db *bun.DB, id int64) map[string]int64 {
	t.Helper()
	var raw string
	require.NoError(t, db.NewRaw(`SELECT platform.school_erasure_plan()::text`).Scan(t.Context(), &raw))
	var plan []struct{ Name, Predicate string }
	require.NoError(t, json.Unmarshal([]byte(raw), &plan))
	counts := map[string]int64{}
	for _, entry := range plan {
		var n int64
		require.NoError(t, db.NewRaw("SELECT count(*) FROM "+entry.Name+" AS r WHERE "+strings.ReplaceAll(entry.Predicate, "$1", "?"), id).Scan(t.Context(), &n))
		counts[entry.Name] = n
	}
	return counts
}
func seedCLIErasure(t *testing.T, db *bun.DB, id int64) {
	t.Helper()
	child := testpkg.CreateTestStudentForTenant(t, db, id, "Synthetic", "Child", "Fixture")
	_, account := testpkg.CreateTestStaffWithAccountForTenant(t, db, id, "Synthetic", "Staff")
	testpkg.MapAccountToTenant(t, db, account.ID, id)
	guardian := testpkg.CreateTestGuardianProfileForTenant(t, db, id, "Synthetic", "Guardian", "fixture")
	link := testpkg.CreateTestStudentGuardianLinkForTenant(t, db, id, child.ID, guardian.ID, "custom")
	_, err := db.ExecContext(t.Context(), `INSERT INTO meta.parent_student_consent_permission_grants(student_guardian_id) VALUES (?)`, link.ID)
	require.NoError(t, err)
	_, err = db.ExecContext(t.Context(), `INSERT INTO audit.auth_events(tenant_id,account_id,event_type,ip_address) VALUES (?,?,'login','127.0.0.1')`, id, account.ID)
	require.NoError(t, err)
	_, err = db.ExecContext(t.Context(), `INSERT INTO platform.email_outbox(tenant_id,kind,recipient,payload) VALUES (?,'fixture','{}','{}')`, id)
	require.NoError(t, err)
	testpkg.CreateTestDataDeletionForTenant(t, db, id, child.ID)
}
func erasureFileList(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	require.NoError(t, filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			files = append(files, rel)
		}
		return nil
	}))
	return files
}
func TestErasureCLISeparatesSchoolsAndResumesBothPhases(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	a := testpkg.Tenant(t)
	b, _ := testpkg.CreateTestTenant(t, db)
	seedCLIErasure(t, db, a)
	seedCLIErasure(t, db, b)
	operator := testpkg.CreateTestOperator(t, db)
	root := t.TempDir()
	raw, err := organizationtenancy.DataLifecycleCatalogSource.ReadFile("data_lifecycle_catalog.json")
	require.NoError(t, err)
	var catalog organizationtenancy.DataLifecycleCatalog
	require.NoError(t, json.Unmarshal(raw, &catalog))
	for _, id := range []int64{a, b} {
		for _, kind := range catalog.Files {
			name := fmt.Sprintf("%s/%d/orphan.bin", kind.Kind, id)
			if kind.Layout == "tenantPrefix" {
				name = fmt.Sprintf("%s/%d_orphan.bin", kind.Kind, id)
			}
			require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0700))
			require.NoError(t, os.WriteFile(filepath.Join(root, name), []byte("synthetic"), 0600))
		}
	}
	require.NoError(t, os.MkdirAll(filepath.Join(root, "avatars/global"), 0700))
	require.NoError(t, os.WriteFile(filepath.Join(root, "avatars/global/shared.bin"), []byte("synthetic"), 0600))
	files := &failingErasureFiles{Local: objects.NewLocal(root)}
	service, err := tenancyCompose.NewErasure(db, files, newTestErasureLedger(t))
	require.NoError(t, err)
	request := organizationtenancy.ErasureRequest{SchoolID: a, OperatorID: operator.ID, OrderReference: "ORDER-3837", Confirmation: fmt.Sprintf("school:%d", a), PrerequisitesChecked: true, WritersStopped: true, LockTimeout: time.Second, StatementTimeout: time.Minute, DryRun: true}
	beforeA := erasureTableCounts(t, db, a)
	beforeB := erasureTableCounts(t, db, b)
	beforeFiles := erasureFileList(t, root)
	preview, err := service.Run(t.Context(), request)
	require.NoError(t, err)
	require.Len(t, preview, 1)
	var previewCounts struct {
		Rows map[string]int64 `json:"row_counts"`
	}
	require.NoError(t, json.Unmarshal(preview[0], &previewCounts))
	require.Equal(t, beforeA, previewCounts.Rows)
	require.Equal(t, beforeA, erasureTableCounts(t, db, a))
	require.Equal(t, beforeB, erasureTableCounts(t, db, b))
	require.Equal(t, beforeFiles, erasureFileList(t, root))
	var orders int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM platform.school_erasures WHERE school_id=?`, a).Scan(t.Context(), &orders))
	require.Zero(t, orders)
	request.DryRun = false
	// A trigger failure rolls back every DB deletion while preserving inventory.
	_, err = db.ExecContext(t.Context(), fmt.Sprintf(`CREATE FUNCTION platform.fail_cli_erasure() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'synthetic error detail'; END $$;
 CREATE TRIGGER fail_cli_erasure BEFORE DELETE ON users.persons FOR EACH ROW WHEN (OLD.tenant_id = %d) EXECUTE FUNCTION platform.fail_cli_erasure()`, a))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := db.ExecContext(context.Background(), `DROP TRIGGER IF EXISTS fail_cli_erasure ON users.persons; DROP FUNCTION IF EXISTS platform.fail_cli_erasure()`)
		require.NoError(t, err)
	})
	result, err := service.Run(t.Context(), request)
	require.Error(t, err)
	require.Len(t, result, 1)
	require.Contains(t, string(result[0]), `"status": "failed"`)
	require.NotContains(t, string(result[0]), "synthetic error detail")
	require.Equal(t, beforeA, erasureTableCounts(t, db, a))
	require.Equal(t, beforeB, erasureTableCounts(t, db, b))
	require.Equal(t, beforeFiles, erasureFileList(t, root))
	_, err = db.ExecContext(t.Context(), `DROP TRIGGER fail_cli_erasure ON users.persons; DROP FUNCTION platform.fail_cli_erasure()`)
	require.NoError(t, err)
	files.fail = true
	result, err = service.Run(t.Context(), request)
	require.Error(t, err)
	require.Len(t, result, 1)
	var proof organizationtenancy.ErasureProof
	require.NoError(t, json.Unmarshal(result[0], &proof))
	require.Equal(t, "failed", proof.Status)
	require.NotNil(t, proof.DBDoneAt)
	require.NotNil(t, proof.InventoryAt)
	for _, n := range erasureTableCounts(t, db, a) {
		require.Zero(t, n)
	}
	require.Equal(t, beforeB, erasureTableCounts(t, db, b))
	files.fail = false
	result, err = service.Run(t.Context(), request)
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.NoError(t, json.Unmarshal(result[0], &proof))
	require.Equal(t, "completed", proof.Status)
	afterFiles := erasureFileList(t, root)
	var expected []string
	for _, path := range beforeFiles {
		if strings.Contains(path, fmt.Sprintf("/%d/", b)) || strings.Contains(path, fmt.Sprintf("/%d_", b)) || path == "avatars/global/shared.bin" {
			expected = append(expected, path)
		}
	}
	require.Equal(t, expected, afterFiles)
	require.Equal(t, beforeB, erasureTableCounts(t, db, b))
	// A DB/upload snapshot is not atomic. Restored files must be removable even
	// when the restored DB already contains the completed receipt, not a school.
	restoredFile := filepath.Join(root, fmt.Sprintf("files/%d/restored.bin", a))
	require.NoError(t, os.WriteFile(restoredFile, []byte("synthetic"), 0600))
	repeated, err := service.Run(t.Context(), request)
	require.NoError(t, err)
	require.JSONEq(t, string(result[0]), string(repeated[0]))
	require.NoFileExists(t, restoredFile)
	require.Equal(t, expected, erasureFileList(t, root))
	receipts, err := service.Run(t.Context(), organizationtenancy.ErasureRequest{ReceiptID: proof.ID})
	require.NoError(t, err)
	require.JSONEq(t, string(result[0]), string(receipts[0]))
}

func TestErasureCLITimeoutAndRoleBoundaries(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	id := testpkg.Tenant(t)
	operator := testpkg.CreateTestOperator(t, db)
	service, err := tenancyCompose.NewErasure(db, objects.NewLocal(t.TempDir()), newTestErasureLedger(t))
	require.NoError(t, err)
	request := organizationtenancy.ErasureRequest{SchoolID: id, OperatorID: operator.ID, OrderReference: "ORDER-TIMEOUT", Confirmation: fmt.Sprintf("school:%d", id), PrerequisitesChecked: true, WritersStopped: true, LockTimeout: 20 * time.Millisecond, StatementTimeout: time.Minute}
	invalid := request
	invalid.Confirmation = "school:wrong"
	_, err = service.Run(t.Context(), invalid)
	require.Error(t, err)
	var n int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM platform.school_erasures WHERE school_id=?`, id).Scan(t.Context(), &n))
	require.Zero(t, n)
	for _, function := range []string{"platform.school_erasure_preview(bigint,bigint)", "platform.school_erasure_schools(bigint,bigint,text)", "platform.school_erasure_file_phase(bigint,text,jsonb)"} {
		for _, role := range []string{"phoenix_auth", "phoenix_tenant", "phoenix_admin", "phoenix_demo"} {
			var allowed bool
			require.NoError(t, db.NewRaw(`SELECT has_function_privilege(?,?,'EXECUTE')`, role, function).Scan(t.Context(), &allowed))
			require.False(t, allowed)
		}
	}
	tx, err := db.BeginTx(t.Context(), nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(t.Context(), `LOCK TABLE users.persons IN SHARE MODE`)
	require.NoError(t, err)
	results, err := service.Run(t.Context(), request)
	require.Error(t, err)
	require.Len(t, results, 1)
	var proof struct {
		Status    string     `json:"status"`
		Code      string     `json:"last_error_code"`
		Completed *time.Time `json:"completed_at"`
	}
	require.NoError(t, json.Unmarshal(results[0], &proof))
	require.Equal(t, "failed", proof.Status)
	require.Equal(t, "55P03", proof.Code)
	require.Nil(t, proof.Completed)
	require.NoError(t, tx.Rollback())
	results, err = service.Run(t.Context(), request)
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.NoError(t, json.Unmarshal(results[0], &proof))
	require.Equal(t, "completed", proof.Status)
}

func TestErasureCLIOrganizationKeepsRootAndResumesSchoolReceipts(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	// A single-connection pool also proves role/session cleanup between phases.
	db.SetMaxOpenConns(1)
	a := testpkg.Tenant(t)
	b, _ := testpkg.CreateTestTenant(t, db)
	var organizationID int64
	require.NoError(t, db.NewRaw(`SELECT organization_id FROM platform.schools WHERE id=?`, a).Scan(t.Context(), &organizationID))
	_, err := db.ExecContext(t.Context(), `UPDATE platform.schools SET organization_id=? WHERE id=?`, organizationID, b)
	require.NoError(t, err)
	operator := testpkg.CreateTestOperator(t, db)
	service, err := tenancyCompose.NewErasure(db, objects.NewLocal(t.TempDir()), newTestErasureLedger(t))
	require.NoError(t, err)
	request := organizationtenancy.ErasureRequest{OrganizationID: organizationID, OperatorID: operator.ID, OrderReference: "ORDER-ORGANIZATION", Confirmation: fmt.Sprintf("organization:%d", organizationID), PrerequisitesChecked: true, WritersStopped: true, LockTimeout: time.Second, StatementTimeout: time.Minute, DryRun: true}
	results, err := service.Run(t.Context(), request)
	require.NoError(t, err)
	require.Len(t, results, 2)
	request.DryRun = false
	results, err = service.Run(t.Context(), request)
	require.Error(t, err)
	require.Len(t, results, 2)
	for _, raw := range results {
		var proof organizationtenancy.ErasureProof
		require.NoError(t, json.Unmarshal(raw, &proof))
		require.Equal(t, "completed", proof.Status)
	}
	var roots int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM platform.organizations WHERE id=?`, organizationID).Scan(t.Context(), &roots))
	require.Equal(t, 1, roots)
	repeated, err := service.Run(t.Context(), request)
	require.Error(t, err)
	require.Len(t, repeated, 2)
	for i := range results {
		require.JSONEq(t, string(results[i]), string(repeated[i]))
	}
}

func newTestErasureLedger(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ledger")
	require.NoError(t, tenancyCompose.InitializeErasureLedger(path))
	return path
}

func TestErasureCLIExternalLedgerFailureNeverReportsCompletion(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	a := testpkg.Tenant(t)
	b, _ := testpkg.CreateTestTenant(t, db)
	seedCLIErasure(t, db, b)
	beforeB := erasureTableCounts(t, db, b)
	operator := testpkg.CreateTestOperator(t, db)
	path := newTestErasureLedger(t)
	files := &failingErasureFiles{Local: objects.NewLocal(t.TempDir()), onRemove: func() {
		require.NoError(t, os.Chmod(path, 0400))
	}}
	service, err := tenancyCompose.NewErasure(db, files, path)
	require.NoError(t, err)
	request := organizationtenancy.ErasureRequest{SchoolID: a, OperatorID: operator.ID, OrderReference: "ORDER-LEDGER", Confirmation: fmt.Sprintf("school:%d", a), PrerequisitesChecked: true, WritersStopped: true, LockTimeout: time.Second, StatementTimeout: time.Minute}
	results, err := service.Run(t.Context(), request)
	require.Error(t, err)
	require.Empty(t, results, "must not emit completed after external write failure")
	require.Equal(t, beforeB, erasureTableCounts(t, db, b))
	require.NoError(t, os.Chmod(path, 0600))
	results, err = service.Run(t.Context(), request)
	require.NoError(t, err)
	require.Len(t, results, 1)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(data), `"order_reference":"ORDER-LEDGER"`)
	repeated, err := service.Run(t.Context(), request)
	require.NoError(t, err)
	require.JSONEq(t, string(results[0]), string(repeated[0]))
	after, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, data, after)
	require.Equal(t, beforeB, erasureTableCounts(t, db, b))
}
