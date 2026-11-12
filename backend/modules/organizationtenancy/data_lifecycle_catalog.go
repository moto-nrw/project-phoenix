package organizationtenancy

import "embed"

// DataLifecycleCatalogSource embeds the single production catalog (ADR 0092).
// Read data_lifecycle_catalog.json and decode it into DataLifecycleCatalog.
// embed.FS returns a fresh byte slice on ReadFile; readers do not share mutable
// maps or slices. Migration guards read the same serialized source directly.
// The tenant-export adapter consumes this artifact; erasure remains separate.
//
//go:embed data_lifecycle_catalog.json
var DataLifecycleCatalogSource embed.FS

// DataLifecycleCatalog is the common inventory for tenant erasure and return
// (ADR 0092). Classification alone is not an implemented lifecycle procedure.
// Scope and export values are documented in docs/reference/datenkatalog.md.
type DataLifecycleCatalog struct {
	Version int                 `json:"version"`
	Tables  []LifecycleTable    `json:"tables"`
	Files   []LifecycleFileKind `json:"files"`
}

// LifecycleTable describes selection and export restrictions, not SQL to run.
// An include entry never authorizes SELECT *, unfiltered account reads or
// recursive export of JSON containers. Executors must apply authorization
// and field projections, including the exclusions recorded here.
type LifecycleTable struct {
	Name                    string            `json:"name"`
	Scope                   string            `json:"scope"`
	ScopeColumn             string            `json:"scopeColumn,omitempty"`
	Parent                  *LifecycleParent  `json:"parent,omitempty"`
	Reason                  string            `json:"reason,omitempty"`
	Export                  string            `json:"export"`
	ExportReason            string            `json:"exportReason,omitempty"`
	ExcludeSecretColumns    map[string]string `json:"excludeSecretColumns,omitempty"`
	AllowedSensitiveColumns map[string]string `json:"allowedSensitiveColumns,omitempty"`
	// OpenSealedColumns maps a field-sealed column to the plaintext column the
	// return export writes instead (#3838). The sealed column stays in
	// ExcludeSecretColumns: its ciphertext never leaves; the owner of the table
	// opens the value through TenantExportSealedFields.
	OpenSealedColumns map[string]string `json:"openSealedColumns,omitempty"`
	// SealedBindings adds a value of another table in the same snapshot to
	// the row the owner opens (#6412): a consent row names its mandate only
	// by ID, while its text is bound to the mandate reference. The key is the
	// name the owner reads; the value never reaches the artifact or model.
	SealedBindings map[string]LifecycleSealedBinding `json:"sealedBindings,omitempty"`
	// ProjectedColumns maps an untyped container column to the column the
	// return export writes its reviewed projection to (#3838). The container
	// stays in ExcludeSecretColumns; the owner of the table drops every secret
	// through TenantExportProjectedFields.
	ProjectedColumns map[string]string `json:"projectedColumns,omitempty"`
	// PersonKeys lists direct FK evidence only. It is not a complete person
	// disclosure selector: historical snapshots, JSON and indirect links remain.
	PersonKeys map[string]string `json:"personKeys,omitempty"`
}

// LifecycleSealedBinding reads Column of the single Table row whose columns
// equal the exported row's: Match maps a column of the exported table to a
// column of Table. It is a lookup for opening, not an export column.
type LifecycleSealedBinding struct {
	Table  string            `json:"table"`
	Column string            `json:"column"`
	Match  map[string]string `json:"match"`
}

// LifecycleParent is an actual foreign-key edge from Column to ParentColumn.
// A tenantChild follows these edges until a table with tenant_id is reached.
type LifecycleParent struct {
	Table        string `json:"table"`
	Column       string `json:"column"`
	ParentColumn string `json:"parentColumn"`
}

// LifecycleFileKind describes one upload namespace below the uploads root.
// tenantDir: kind/tenant/name; tenantPrefix: kind/tenant_name;
// account: shared account references (currently avatars/global/name).
type LifecycleFileKind struct {
	Kind   string `json:"kind"`
	Layout string `json:"layout"`
	Export string `json:"export"`
	Reason string `json:"reason"`
}
