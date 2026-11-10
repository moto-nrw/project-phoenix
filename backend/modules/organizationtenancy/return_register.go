package organizationtenancy

import (
	"context"
	"embed"
)

// ExportFormatVersion is the format of the return package: its manifest
// names it, and the register describes it (#3838).
const ExportFormatVersion = 1

// ReturnRegisterVersion is the version of the register document itself.
const ReturnRegisterVersion = 1

// ReturnRegisterSource embeds the register of the return package (Art. 26
// Data Act, ADR 0201): what the package of this build holds, without any
// customer data. It is generated from the catalog and the migrated schema by
// the same code that writes model.json; a guard test regenerates it, so it
// cannot drift from the package.
//
//go:embed return_register.json
var ReturnRegisterSource embed.FS

// ReturnRegisterFile is the name of the embedded register.
const ReturnRegisterFile = "return_register.json"

// ReturnRegisterBuilder builds the register from the catalog and the schema
// of a migrated database, the way an export describes its tables.
type ReturnRegisterBuilder interface {
	BuildReturnRegister(context.Context) (ReturnRegister, error)
}

// ReturnRegister describes the structure and format of the return package:
// the tables with their fields, types, nullability and relations, the
// tables and files that do not leave and why, and the standards it uses.
type ReturnRegister struct {
	RegisterVersion      int                      `json:"registerVersion"`
	FormatVersion        int                      `json:"formatVersion"`
	CatalogVersion       int                      `json:"catalogVersion"`
	CatalogSHA256        string                   `json:"catalogSHA256"`
	About                string                   `json:"about"`
	Standards            []ReturnRegisterStandard `json:"standards"`
	CommonSpecifications string                   `json:"commonSpecifications"`
	TimeEstimate         string                   `json:"timeEstimate"`
	Retrieval            string                   `json:"retrieval"`
	Packages             []ReturnRegisterPackage  `json:"packages"`
	Layout               []ReturnRegisterPath     `json:"layout"`
	Values               []ReturnRegisterValue    `json:"values"`
	Tables               []ReturnRegisterTable    `json:"tables"`
	ExcludedTables       []ReturnRegisterExcluded `json:"excludedTables"`
	Files                []ReturnRegisterFileKind `json:"files"`
}

// ReturnRegisterStandard names one standard or specification the package uses.
type ReturnRegisterStandard struct {
	Name      string `json:"name"`
	Reference string `json:"reference"`
	Use       string `json:"use"`
}

// ReturnRegisterPackage names a kind of package: "school" or "organization".
type ReturnRegisterPackage struct {
	Kind        string `json:"kind"`
	Description string `json:"description"`
}

// ReturnRegisterPath describes one file or directory of a package.
type ReturnRegisterPath struct {
	Path        string   `json:"path"`
	Packages    []string `json:"packages"`
	Description string   `json:"description"`
}

// ReturnRegisterValue describes how a database type appears in the JSON lines.
type ReturnRegisterValue struct {
	Types       []string `json:"types"`
	Description string   `json:"description"`
}

// ReturnRegisterTable is one table as model.json describes it, with the
// packages that carry it and the columns that do not leave.
type ReturnRegisterTable struct {
	ExportModelTable
	Packages        []string          `json:"packages"`
	ExcludedColumns map[string]string `json:"excludedColumns,omitempty"`
}

// ReturnRegisterExcluded is a table the package does not carry, with its reason.
type ReturnRegisterExcluded struct {
	Name           string `json:"name"`
	Classification string `json:"classification"`
	Reason         string `json:"reason"`
}

// ReturnRegisterFileKind is one kind of stored file and whether it leaves.
type ReturnRegisterFileKind struct {
	Kind   string `json:"kind"`
	Export string `json:"export"`
	Reason string `json:"reason"`
}
