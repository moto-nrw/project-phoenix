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
	// PersonKeys lists direct FK evidence only. It is not a complete person
	// disclosure selector: historical snapshots, JSON and indirect links remain.
	PersonKeys map[string]string `json:"personKeys,omitempty"`
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
