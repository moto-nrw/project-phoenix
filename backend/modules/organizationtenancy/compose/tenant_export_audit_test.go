package compose_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The Träger package carries the PAIN.008 files only Parent Billing opens
// (#6314, ADR 0187) in their own, clearly marked directory, each with its
// origin. Every run is in the operator audit before it can be handed over
// (#4720: every access to personal data is audited).

const syntheticIBAN = "DE89370400440532013000"

// sepaFilesFake hands files to the export one at a time. err fails the
// read before any file, or, with errAfterFiles, after every file was written.
type sepaFilesFake struct {
	called        []int64
	files         []organizationtenancy.ExportSEPADebitFile
	err           error
	errAfterFiles bool
}

func (f *sepaFilesFake) SEPADebitFiles(_ context.Context, organizationID int64, write func(organizationtenancy.ExportSEPADebitFile) error) error {
	f.called = append(f.called, organizationID)
	if f.err != nil && !f.errAfterFiles {
		return f.err
	}
	for _, file := range f.files {
		if err := write(file); err != nil {
			return err
		}
	}
	return f.err
}

// exportAuditFake records the attempt and the outcome of each run. err fails
// every record, completedErr only the record of a completed run; onBegin
// sees the output before the attempt is recorded.
type exportAuditFake struct {
	attempts     []organizationtenancy.TenantExportAttempt
	records      []organizationtenancy.TenantExportRecord
	err          error
	completedErr error
	onBegin      func()
}

func (f *exportAuditFake) BeginTenantExport(_ context.Context, attempt organizationtenancy.TenantExportAttempt) error {
	if f.onBegin != nil {
		f.onBegin()
	}
	if f.err != nil {
		return f.err
	}
	f.attempts = append(f.attempts, attempt)
	return nil
}

func (f *exportAuditFake) RecordTenantExport(_ context.Context, record organizationtenancy.TenantExportRecord) error {
	if f.err != nil {
		return f.err
	}
	if record.Outcome == organizationtenancy.TenantExportCompleted && f.completedErr != nil {
		return f.completedErr
	}
	f.records = append(f.records, record)
	return nil
}

func (f *exportAuditFake) completed() []organizationtenancy.TenantExportRecord {
	var completed []organizationtenancy.TenantExportRecord
	for _, record := range f.records {
		if record.Outcome == organizationtenancy.TenantExportCompleted {
			completed = append(completed, record)
		}
	}
	return completed
}

func syntheticSEPAFiles() []organizationtenancy.ExportSEPADebitFile {
	return []organizationtenancy.ExportSEPADebitFile{{Name: "MOTO-20290603-ABCDEFGHIJKLMNOPQRST.xml",
		Data: []byte("<Document><IBAN>" + syntheticIBAN + "</IBAN></Document>\n"), Origin: "Original", Reason: "Synthetic original."}}
}

// newTraegerExporter binds a Träger package with the fakes of the owners.
func newTraegerExporter(t *testing.T, db *bun.DB, files *sepaFilesFake, audit *exportAuditFake) organizationtenancy.TenantExporter {
	t.Helper()
	exporter, err := compose.NewTraegerTenantExporter(db, organizationtenancy.TenantExportDependencies{
		Fingerprints: organizationtenancy.ExportFingerprints{Bytes: testpkg.ContentFingerprint, Reader: testpkg.ReaderContentFingerprint},
		Sealed:       openedSealedFields{}, Projected: projectedFields{}, SEPADebitFiles: files, Audit: audit})
	require.NoError(t, err)
	return exporter
}

func TestTenantExportOrganizationCarriesThePAIN008FilesAndIsAudited(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	school, _ := testpkg.CreateTestTenant(t, db)
	var organization int64
	require.NoError(t, db.NewRaw(`SELECT organization_id FROM platform.schools WHERE id = ?`, school).Scan(t.Context(), &organization))
	files := &sepaFilesFake{files: syntheticSEPAFiles()}
	audit := &exportAuditFake{}
	exporter := newTraegerExporter(t, db, files, audit)
	options := organizationtenancy.TenantExportOptions{OrganizationID: organization, OperatorID: 42, Output: filepath.Join(t.TempDir(), "export"), Uploads: t.TempDir()}
	require.NoError(t, exporter.Export(t.Context(), options))

	require.Equal(t, []int64{organization}, files.called, "the files of exactly this Träger")
	manifest := readExportManifest(t, options.Output)
	assertExportArtifacts(t, options.Output, manifest)
	require.Equal(t, "organization", manifest.Package, "the Träger package of ADR 0193")
	require.NotNil(t, manifest.SEPADebitFiles, "the manifest marks the PAIN.008 files")
	require.Equal(t, "SEPA-LASTSCHRIFTDATEIEN", manifest.SEPADebitFiles.Directory)
	require.Equal(t, []organizationtenancy.ExportSEPADebitFileOrigin{{Path: "SEPA-LASTSCHRIFTDATEIEN/MOTO-20290603-ABCDEFGHIJKLMNOPQRST.xml",
		Origin: "Original", Reason: "Synthetic original."}}, manifest.SEPADebitFiles.Files)
	for _, file := range syntheticSEPAFiles() {
		data, err := os.ReadFile(filepath.Join(options.Output, "SEPA-LASTSCHRIFTDATEIEN", file.Name))
		require.NoError(t, err, file.Name)
		require.Equal(t, file.Data, data, file.Name)
		require.Contains(t, artifactPaths(manifest), "SEPA-LASTSCHRIFTDATEIEN/"+file.Name, "the manifest checksums every PAIN.008 file")
	}
	notice, err := os.ReadFile(filepath.Join(options.Output, "SEPA-LASTSCHRIFTDATEIEN", "LIESMICH.txt"))
	require.NoError(t, err)
	require.Contains(t, string(notice), "vollständigen Bankdaten")
	require.Contains(t, string(notice), "MOTO-20290603-ABCDEFGHIJKLMNOPQRST.xml: Original")
	manifestData, err := os.ReadFile(filepath.Join(options.Output, "manifest.json"))
	require.NoError(t, err)
	require.NotContains(t, string(manifestData), syntheticIBAN, "the manifest names no bank data")

	require.Len(t, audit.attempts, 1, "every run is in the operator audit")
	require.Equal(t, organizationtenancy.TenantExportAttempt{OperatorID: 42, OrganizationID: organization, StartedAt: audit.attempts[0].StartedAt}, audit.attempts[0])
	require.NotEmpty(t, audit.attempts[0].StartedAt)
	require.Len(t, audit.records, 1, "and its outcome")
	record := audit.records[0]
	require.Equal(t, organizationtenancy.TenantExportCompleted, record.Outcome)
	require.Equal(t, audit.attempts[0].StartedAt, record.StartedAt)
	require.Empty(t, record.ErrorClass)
	marker, err := os.ReadFile(filepath.Join(options.Output, "COMPLETE"))
	require.NoError(t, err)
	require.Equal(t, int64(42), record.OperatorID)
	require.Equal(t, organization, record.OrganizationID)
	require.Equal(t, strings.TrimSpace(string(marker)), record.ManifestSHA256, "the audit names the handed-over export")
	require.Equal(t, manifest.ExportedAt, record.ExportedAt)
	require.Equal(t, len(manifest.Tables), record.Tables)
	require.Equal(t, len(manifest.Artifacts), record.Artifacts)
	require.Equal(t, manifest.SEPADebitFiles, record.SEPADebitFiles)
	encoded, err := json.Marshal(record)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), syntheticIBAN, "the audit holds counts, no personal data")
	require.NotContains(t, string(encoded), options.Output, "nor the local path")
}

func TestTenantExportOrganizationWithoutPAIN008FilesOrAuditNeverCompletes(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	school, _ := testpkg.CreateTestTenant(t, db)
	var organization int64
	require.NoError(t, db.NewRaw(`SELECT organization_id FROM platform.schools WHERE id = ?`, school).Scan(t.Context(), &organization))
	fingerprints := organizationtenancy.ExportFingerprints{Bytes: testpkg.ContentFingerprint, Reader: testpkg.ReaderContentFingerprint}

	plain, err := compose.NewTenantExporter(db, fingerprints, openedSealedFields{}, projectedFields{}, &exportAuditFake{})
	require.NoError(t, err)
	out := filepath.Join(t.TempDir(), "export")
	require.Error(t, plain.Export(t.Context(), organizationtenancy.TenantExportOptions{OrganizationID: organization, OperatorID: 42, Output: out, Uploads: t.TempDir()}),
		"a Träger package needs its PAIN.008 files and its audit")
	require.NoFileExists(t, filepath.Join(out, "COMPLETE"))

	escaping := append(syntheticSEPAFiles(), organizationtenancy.ExportSEPADebitFile{Name: "../manifest.json", Data: []byte("x"), Origin: "Original"})
	for name, failing := range map[string]struct {
		files    *sepaFilesFake
		audit    *exportAuditFake
		operator int64
	}{
		"without operator":                {files: &sepaFilesFake{files: syntheticSEPAFiles()}, audit: &exportAuditFake{}, operator: 0},
		"PAIN.008 files unavailable":      {files: &sepaFilesFake{err: errors.New("bank key missing")}, audit: &exportAuditFake{}, operator: 42},
		"PAIN.008 file outside directory": {files: &sepaFilesFake{files: escaping}, audit: &exportAuditFake{}, operator: 42},
		"audit unavailable":               {files: &sepaFilesFake{files: syntheticSEPAFiles()}, audit: &exportAuditFake{err: errors.New("ledger down")}, operator: 42},
	} {
		exporter := newTraegerExporter(t, db, failing.files, failing.audit)
		out := filepath.Join(t.TempDir(), "export")
		require.Error(t, exporter.Export(t.Context(), organizationtenancy.TenantExportOptions{
			OrganizationID: organization, OperatorID: failing.operator, Output: out, Uploads: t.TempDir()}), name)
		require.NoFileExists(t, filepath.Join(out, "COMPLETE"), name)
		require.Empty(t, failing.audit.completed(), "a failed run is never recorded as handed over: %s", name)
	}
}

// The attempt is in the operator audit before anything is read or written,
// and every outcome after it: a failed run names its error class, and its
// partial files are removed, so nothing readable stays without a record.
func TestTenantExportOrganizationRecordsTheAttemptFirstAndEveryOutcome(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	school, _ := testpkg.CreateTestTenant(t, db)
	var organization int64
	require.NoError(t, db.NewRaw(`SELECT organization_id FROM platform.schools WHERE id = ?`, school).Scan(t.Context(), &organization))
	export := func(files *sepaFilesFake, audit *exportAuditFake) (string, error) {
		exporter := newTraegerExporter(t, db, files, audit)
		out := filepath.Join(t.TempDir(), "export")
		audit.onBegin = func() { require.NoDirExists(t, out, "the attempt is recorded before any file is written") }
		return out, exporter.Export(t.Context(), organizationtenancy.TenantExportOptions{OrganizationID: organization, OperatorID: 42, Output: out, Uploads: t.TempDir()})
	}

	refused := &sepaFilesFake{files: syntheticSEPAFiles()}
	out, err := export(refused, &exportAuditFake{err: errors.New("operator inactive")})
	require.Error(t, err, "an attempt the audit refuses, for an unknown or inactive operator, never starts")
	require.Empty(t, refused.called, "nothing is read")
	require.NoDirExists(t, out, "nothing is written")

	for name, failing := range map[string]struct {
		files *sepaFilesFake
		audit *exportAuditFake
		class string
	}{
		"PAIN.008 files unavailable": {files: &sepaFilesFake{err: errors.New("bank key missing")}, audit: &exportAuditFake{}, class: "sepa_debit_files"},
		"PAIN.008 read fails after a written file": {files: &sepaFilesFake{files: syntheticSEPAFiles(), err: errors.New("bank key missing"),
			errAfterFiles: true}, audit: &exportAuditFake{}, class: "sepa_debit_files"},
		"completion unrecorded": {files: &sepaFilesFake{files: syntheticSEPAFiles()}, audit: &exportAuditFake{completedErr: errors.New("ledger down")}, class: "audit"},
	} {
		out, err := export(failing.files, failing.audit)
		require.Error(t, err, name)
		require.NoDirExists(t, out, "%s: the partial files are removed", name)
		require.Len(t, failing.audit.attempts, 1, name)
		require.Len(t, failing.audit.records, 1, "%s: the failure is recorded", name)
		record := failing.audit.records[0]
		require.Equal(t, organizationtenancy.TenantExportFailed, record.Outcome, name)
		require.Equal(t, failing.class, record.ErrorClass, name)
		require.True(t, record.PartialFilesRemoved, name)
		require.Equal(t, failing.audit.attempts[0].StartedAt, record.StartedAt, name)
		require.Empty(t, record.ManifestSHA256, "%s: a failed run names no handed-over export", name)
	}
}

func artifactPaths(manifest organizationtenancy.ExportManifest) []string {
	paths := make([]string, len(manifest.Artifacts))
	for i, artifact := range manifest.Artifacts {
		paths[i] = artifact.Path
	}
	return paths
}

// A school package is in the operator audit like the Träger package
// (#6326): without an operator nothing is read, every run records its
// attempt first and its outcome, without personal data, and a run whose
// completion is not recorded never completes.
func TestTenantExportSchoolIsAuditedLikeTheTraegerPackage(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	school := testpkg.Tenant(t)
	const name = "Synthetisch-Auditkind"
	testpkg.CreateTestStudentForTenant(t, db, school, name, "Rueckgabe", "1a")
	fingerprints := organizationtenancy.ExportFingerprints{Bytes: testpkg.ContentFingerprint, Reader: testpkg.ReaderContentFingerprint}
	_, err := compose.NewTenantExporter(db, fingerprints, openedSealedFields{}, projectedFields{}, nil)
	require.Error(t, err, "a school package needs its operator audit")

	export := func(audit *exportAuditFake, operator int64) (string, error) {
		exporter, err := compose.NewTenantExporter(db, fingerprints, openedSealedFields{}, projectedFields{}, audit)
		require.NoError(t, err)
		out := filepath.Join(t.TempDir(), "export")
		audit.onBegin = func() { require.NoDirExists(t, out, "the attempt is recorded before any file is written") }
		return out, exporter.Export(t.Context(), organizationtenancy.TenantExportOptions{SchoolID: school, OperatorID: operator, Output: out, Uploads: t.TempDir()})
	}

	unattributed := &exportAuditFake{}
	out, err := export(unattributed, 0)
	require.Error(t, err, "a school package without operator is refused")
	require.NoDirExists(t, out, "nothing is written")
	require.Empty(t, unattributed.attempts)

	refused := &exportAuditFake{err: errors.New("operator inactive")}
	out, err = export(refused, 42)
	require.Error(t, err, "an attempt the audit refuses never starts")
	require.NoDirExists(t, out)

	audit := &exportAuditFake{}
	out, err = export(audit, 42)
	require.NoError(t, err)
	require.Equal(t, []organizationtenancy.TenantExportAttempt{{OperatorID: 42, SchoolID: school, StartedAt: audit.attempts[0].StartedAt}}, audit.attempts)
	require.Len(t, audit.records, 1)
	record := audit.records[0]
	manifest := readExportManifest(t, out)
	marker, err := os.ReadFile(filepath.Join(out, "COMPLETE"))
	require.NoError(t, err)
	require.Equal(t, organizationtenancy.TenantExportRecord{OperatorID: 42, SchoolID: school, Outcome: organizationtenancy.TenantExportCompleted,
		StartedAt: audit.attempts[0].StartedAt, ManifestSHA256: strings.TrimSpace(string(marker)), ExportedAt: manifest.ExportedAt,
		Tables: len(manifest.Tables), Artifacts: len(manifest.Artifacts)}, record, "the school instead of the Träger, the same counts")
	encoded, err := json.Marshal(record)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), name, "the audit holds no personal data")
	require.NotContains(t, string(encoded), out, "nor the local path")

	unrecorded := &exportAuditFake{completedErr: errors.New("ledger down")}
	out, err = export(unrecorded, 42)
	require.Error(t, err)
	require.NoFileExists(t, filepath.Join(out, "COMPLETE"), "no COMPLETE without its audit entry")
	require.Len(t, unrecorded.records, 1, "the failure is recorded")
	require.Equal(t, organizationtenancy.TenantExportFailed, unrecorded.records[0].Outcome)
	require.Equal(t, "audit", unrecorded.records[0].ErrorClass)
	require.Equal(t, school, unrecorded.records[0].SchoolID)
}
