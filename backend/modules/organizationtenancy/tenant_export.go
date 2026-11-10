package organizationtenancy

import (
	"context"
	"encoding/json"
	"errors"
	"io"
)

// ExportFingerprints supplies the platform's SHA-256 content identities.
type ExportFingerprints struct {
	Bytes  func([]byte) string
	Reader func(io.Reader) (string, int64, error)
}

// TenantExporter is the internal, read-only return procedure of ADR 0092.
// It is deliberately not exposed through an HTTP capability.
type TenantExporter interface {
	Export(context.Context, TenantExportOptions) error
}

// TenantExportSealedFields opens a field-sealed value of an exported row
// (#3838). The owner of the table binds each value to its row, so it gets the
// row as exported and the raw sealed bytes; it answers the plaintext or an
// error that names no value. Only the administrative export CLI binds it.
type TenantExportSealedFields interface {
	OpenExportField(table, column string, row map[string]json.RawMessage, sealed []byte) (string, error)
}

// TenantExportSealedFieldsByTable routes each table to the owner that sealed
// its values: bank data to Parent Billing, health data to its owners (#3838).
// A table without an owner here does not open, so the export refuses it.
type TenantExportSealedFieldsByTable map[string]TenantExportSealedFields

func (owners TenantExportSealedFieldsByTable) OpenExportField(table, column string, row map[string]json.RawMessage, sealed []byte) (string, error) {
	owner := owners[table]
	if owner == nil {
		return "", errors.New("tenant export: no owner opens the sealed values of " + table)
	}
	return owner.OpenExportField(table, column, row, sealed)
}

// TenantExportProjectedFields projects a reviewed value out of an untyped
// container column for the return export (#3838): the owner keeps what is
// customer data and drops every secret. It gets the row as exported and the
// raw JSON of the column; it answers the JSON to write, null when nothing of
// the value may leave. Only the administrative export CLI binds it.
type TenantExportProjectedFields interface {
	ProjectExportField(table, column string, row map[string]json.RawMessage, raw json.RawMessage) (json.RawMessage, error)
}

// TenantExportProjectedFieldsByTable routes each table to the owner that
// knows which parts of its container are secret.
type TenantExportProjectedFieldsByTable map[string]TenantExportProjectedFields

func (owners TenantExportProjectedFieldsByTable) ProjectExportField(table, column string, row map[string]json.RawMessage, raw json.RawMessage) (json.RawMessage, error) {
	owner := owners[table]
	if owner == nil {
		return nil, errors.New("tenant export: no owner projects the values of " + table)
	}
	return owner.ProjectExportField(table, column, row, raw)
}

// TenantExportDependencies binds the Träger package as the CLI runs it
// (ADR 0187): the owners that open sealed and project container values
// (ADR 0192, ADR 0193), the stored PAIN.008 files only Parent Billing opens,
// and the operator audit of each run. A school package needs neither of
// the last two; a Träger package refuses without them.
type TenantExportDependencies struct {
	Fingerprints   ExportFingerprints
	Sealed         TenantExportSealedFields
	Projected      TenantExportProjectedFields
	SEPADebitFiles TenantExportSEPADebitFiles
	Audit          TenantExportAudit
}

// TenantExportSEPADebitFiles hands the PAIN.008 file of each submitted
// SEPA message of a Träger to write (#6314, ADR 0187): the stored original,
// else a file reconstructed from the stored debits, each with its origin.
// It reads the next message only after write returned, so the export holds
// one file in memory; an error of write stops it. The rows of the mandates
// and debits already carry the opened account (ADR 0192); the files are
// what those rows cannot carry. The root binds it to Parent Billing, which
// owns the bank key.
type TenantExportSEPADebitFiles interface {
	SEPADebitFiles(ctx context.Context, organizationID int64, write func(ExportSEPADebitFile) error) error
}

// ExportSEPADebitFile is one PAIN.008 file: its name (message ID with .xml),
// its bytes and whether it is the stored original or reconstructed, and why.
type ExportSEPADebitFile struct {
	Name   string
	Data   []byte
	Origin string
	Reason string
}

// ExportSEPADebitFileOrigin names the origin of one PAIN.008 file in the
// manifest: its path, Original or rekonstruiert, and why.
type ExportSEPADebitFileOrigin struct {
	Path   string `json:"path"`
	Origin string `json:"origin"`
	Reason string `json:"reason"`
}

// ExportSEPADebitFiles marks the directory of the PAIN.008 files of a Träger
// package in the manifest and the audit: names and origins, never contents.
type ExportSEPADebitFiles struct {
	Directory string                      `json:"directory"`
	Notice    string                      `json:"notice"`
	Files     []ExportSEPADebitFileOrigin `json:"files"`
}

// ExportDocumentFiles marks the stored documents of a Träger package in the
// manifest and the audit (ADR 0187): every binary column the catalog exports
// from a table of the Träger, written as one file per row under Directory and
// listed in Index. Names and counts, never contents.
type ExportDocumentFiles struct {
	Directory string                 `json:"directory"`
	Index     string                 `json:"index"`
	Notice    string                 `json:"notice"`
	Columns   []ExportDocumentColumn `json:"columns"`
}

// ExportDocumentColumn counts the files of one document column.
type ExportDocumentColumn struct {
	Table  string `json:"table"`
	Column string `json:"column"`
	Files  int    `json:"files"`
	Bytes  int64  `json:"bytes"`
}

// TenantExportAudit records each run of a Träger package in the operator
// audit (#4720: every access to personal data is audited).
// BeginTenantExport runs before anything is read or written and refuses an
// unknown or inactive operator; RecordTenantExport records the outcome. A
// run whose completion is not recorded never completes.
type TenantExportAudit interface {
	BeginTenantExport(ctx context.Context, attempt TenantExportAttempt) error
	RecordTenantExport(ctx context.Context, record TenantExportRecord) error
}

// TenantExportAttempt names a run before its extraction: who, which Träger,
// when it started. No personal data.
type TenantExportAttempt struct {
	OperatorID     int64  `json:"operator_id"`
	OrganizationID int64  `json:"organization_id"`
	StartedAt      string `json:"started_at"`
}

// The outcomes of a Träger package run in the operator audit.
const (
	TenantExportCompleted = "completed"
	TenantExportFailed    = "failed"
)

// TenantExportRecord names the outcome of a run without personal data. A
// completed run carries the manifest checksum of the COMPLETE file and the
// counts; a failed run its error class (the step that failed) and whether
// its partial files were removed.
type TenantExportRecord struct {
	OperatorID          int64                 `json:"operator_id"`
	OrganizationID      int64                 `json:"organization_id"`
	Outcome             string                `json:"outcome"`
	StartedAt           string                `json:"started_at"`
	ErrorClass          string                `json:"error_class,omitempty"`
	PartialFilesRemoved bool                  `json:"partial_files_removed,omitempty"`
	ManifestSHA256      string                `json:"manifest_sha256,omitempty"`
	ExportedAt          string                `json:"exported_at,omitempty"`
	Tables              int                   `json:"tables,omitempty"`
	Artifacts           int                   `json:"artifacts,omitempty"`
	SEPADebitFiles      *ExportSEPADebitFiles `json:"sepa_debit_files,omitempty"`
	DocumentFiles       *ExportDocumentFiles  `json:"document_files,omitempty"`
}

type TenantExportOptions struct {
	// SchoolID or OrganizationID, exactly one. A school package carries the
	// school's data and, as before, the data of its Träger. A Träger package
	// (#3838) carries only the Träger's own tables, its schools' master rows
	// and the accounts of its Träger office: everything a school holds goes
	// in the package of that school.
	SchoolID       int64
	OrganizationID int64
	// OperatorID is the moto operator who runs a Träger package; the
	// operator audit names it (ADR 0187). Required with OrganizationID.
	OperatorID int64
	// Person optionally derives an INTERNAL review workspace: kind:<profile ID>,
	// guardian:<profile ID>, staff:<school membership ID>. Never hand it over
	// without the completeness and third-party review in the runbook.
	Person string
	// Output must not exist. Failed attempts have no COMPLETE marker and must
	// never be handed over. Retry into a new directory, not a previous attempt.
	Output string
	// Uploads is a complete, stable local copy of the uploads root. Freeze
	// uploads/deletions while capturing the DB snapshot and copying the files.
	Uploads string
}

// ExportManifest contains no row contents or original object names. Original
// names and their mapping live in objects.jsonl, inside the protected export.
type ExportManifest struct {
	ReviewRequired bool `json:"reviewRequired,omitempty"`
	// Package is "organization" for a Träger package; a school package has
	// none, as in format version 1 before it.
	Package        string        `json:"package,omitempty"`
	FormatVersion  int           `json:"formatVersion"`
	CatalogVersion int           `json:"catalogVersion"`
	CatalogSHA256  string        `json:"catalogSHA256"`
	ExportedAt     string        `json:"exportedAt"`
	Tables         []ExportTable `json:"tables"`
	// SEPADebitFiles marks the PAIN.008 files of a Träger package (ADR 0187).
	SEPADebitFiles *ExportSEPADebitFiles `json:"sepaDebitFiles,omitempty"`
	// DocumentFiles marks the stored documents of a Träger package as files.
	DocumentFiles *ExportDocumentFiles `json:"documentFiles,omitempty"`
	Artifacts     []ExportArtifact     `json:"artifacts"`
}

type ExportTable struct {
	Name            string            `json:"name"`
	Classification  string            `json:"classification"`
	Reason          string            `json:"reason,omitempty"`
	Rows            int64             `json:"rows"`
	ExcludedColumns map[string]string `json:"excludedColumns,omitempty"`
}

type ExportArtifact struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

type ExportColumn struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	UDT      string `json:"udt"`
	Nullable bool   `json:"nullable"`
}

type ExportRelationship struct {
	Column       string `json:"column"`
	TargetTable  string `json:"targetTable"`
	TargetColumn string `json:"targetColumn"`
}

type ExportModelTable struct {
	Name          string               `json:"name"`
	Columns       []ExportColumn       `json:"columns"`
	Relationships []ExportRelationship `json:"relationships"`
}
