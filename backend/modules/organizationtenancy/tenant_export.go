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

type TenantExportOptions struct {
	SchoolID int64
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
	ReviewRequired bool             `json:"reviewRequired,omitempty"`
	FormatVersion  int              `json:"formatVersion"`
	CatalogVersion int              `json:"catalogVersion"`
	CatalogSHA256  string           `json:"catalogSHA256"`
	ExportedAt     string           `json:"exportedAt"`
	Tables         []ExportTable    `json:"tables"`
	Artifacts      []ExportArtifact `json:"artifacts"`
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
