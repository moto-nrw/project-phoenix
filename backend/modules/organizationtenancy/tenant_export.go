package organizationtenancy

import (
	"context"
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
