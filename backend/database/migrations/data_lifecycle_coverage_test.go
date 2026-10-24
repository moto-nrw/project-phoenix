package migrations

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// Read the production wire artifact, not a second classification register or
// a forbidden migration -> domain import. The owner's public FS embeds it.
type lifecycleCatalog struct {
	Version int              `json:"version"`
	Tables  []lifecycleTable `json:"tables"`
	Files   []lifecycleFile  `json:"files"`
}

type lifecycleTable struct {
	Name, Scope, ScopeColumn, Reason, Export, ExportReason    string
	Parent                                                    *lifecycleParent
	ExcludeSecretColumns, AllowedSensitiveColumns, PersonKeys map[string]string
	ExportOrder                                               []string
}

type lifecycleParent struct{ Table, Column, ParentColumn string }
type lifecycleFile struct{ Kind, Layout, Export, Reason string }

type lifecycleColumn struct{ TableName, ColumnName, ParentTable, ParentColumn string }

func readLifecycleCatalog(t *testing.T) lifecycleCatalog {
	t.Helper()
	data, err := os.ReadFile("../../modules/organizationtenancy/data_lifecycle_catalog.json")
	require.NoError(t, err)
	var catalog lifecycleCatalog
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	require.NoError(t, decoder.Decode(&catalog))
	require.Equal(t, 1, catalog.Version)
	require.NotEmpty(t, catalog.Tables)
	require.NotEmpty(t, catalog.Files)
	return catalog
}

// Names are a conservative tripwire, not a proof that JSON/free text contains
// no secrets. Opaque settings, delivery and credential containers are reviewed
// and explicitly excluded in the catalog, even without a suspicious name.
var lifecycleSecretName = regexp.MustCompile(`(^|_)(hash|token|secret|password|api_key|code|credential|pin|key)(_|$)|^token|^secret|^password|^api_key`)

func TestEveryTableHasDataLifecycleClassification(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	var columns []lifecycleColumn
	require.NoError(t, db.NewRaw(`
		SELECT n.nspname || '.' || c.relname AS table_name, COALESCE(a.attname, '') AS column_name,
		       COALESCE(pn.nspname || '.' || pc.relname, '') AS parent_table,
		       COALESCE(pa.attname, '') AS parent_column
		FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		LEFT JOIN pg_attribute a ON a.attrelid = c.oid AND a.attnum > 0 AND NOT a.attisdropped
		LEFT JOIN pg_constraint f ON f.conrelid = c.oid AND f.contype = 'f' AND a.attnum = ANY(f.conkey)
		LEFT JOIN pg_class pc ON pc.oid = f.confrelid
		LEFT JOIN pg_namespace pn ON pn.oid = pc.relnamespace
		LEFT JOIN pg_attribute pa ON pa.attrelid = pc.oid AND pa.attnum = f.confkey[array_position(f.conkey, a.attnum)]
		WHERE c.relkind IN ('r', 'p') AND NOT c.relispartition
		  AND n.nspname NOT IN ('pg_catalog', 'information_schema', 'pg_toast')
		ORDER BY 1, a.attnum`).Scan(context.Background(), &columns))
	require.NotEmpty(t, columns, "schema discovery must not pass vacuously")
	require.Empty(t, lifecycleClassificationProblems(readLifecycleCatalog(t).Tables, columns))
}

func lifecycleClassificationProblems(entries []lifecycleTable, columns []lifecycleColumn) []string {
	var problems []string
	tables := make(map[string]map[string]bool)
	for _, column := range columns {
		if tables[column.TableName] == nil {
			tables[column.TableName] = make(map[string]bool)
		}
		if column.ColumnName != "" {
			tables[column.TableName][column.ColumnName] = true
		}
	}
	catalog := make(map[string]lifecycleTable, len(entries))
	for _, entry := range entries {
		if _, exists := catalog[entry.Name]; exists {
			problems = append(problems, entry.Name+" classified twice")
		}
		catalog[entry.Name] = entry
	}
	for table := range tables {
		if _, exists := catalog[table]; !exists {
			problems = append(problems, table+" has no lifecycle classification")
		}
	}
	for _, entry := range entries {
		cols, exists := tables[entry.Name]
		if !exists {
			problems = append(problems, entry.Name+" is stale")
			continue
		}
		if cols["tenant_id"] && entry.Scope != "tenant" {
			problems = append(problems, entry.Name+" has tenant_id but is not tenant-scoped")
		}
		problems = append(problems, lifecycleScopeProblems(entry, catalog, columns, cols)...)
		problems = append(problems, lifecycleExportProblems(entry, cols)...)
		for col, target := range entry.PersonKeys {
			if col == "tenant_id" || !strings.HasSuffix(target, ".id") {
				problems = append(problems, entry.Name+"."+col+" is not a person ID reference")
			}
			found := false
			for _, column := range columns {
				if column.TableName == entry.Name && column.ColumnName == col && column.ParentTable+"."+column.ParentColumn == target {
					found = true
				}
			}
			if !found {
				problems = append(problems, entry.Name+"."+col+" has no declared person FK to "+target)
			}
		}
	}
	sort.Strings(problems)
	return problems
}

func lifecycleScopeProblems(entry lifecycleTable, catalog map[string]lifecycleTable, columns []lifecycleColumn, cols map[string]bool) []string {
	var problems []string
	switch entry.Scope {
	case "tenant":
		if !cols["tenant_id"] {
			problems = append(problems, entry.Name+" has no tenant_id")
		}
	case "tenantChild":
		if !lifecycleReachesTenant(entry.Name, catalog, columns, make(map[string]bool)) {
			problems = append(problems, entry.Name+" has no declared FK path to a tenant table")
		}
	case "school", "organization", "tenantReference":
		if entry.ScopeColumn == "" || !cols[entry.ScopeColumn] {
			problems = append(problems, entry.Name+" needs an existing scopeColumn")
		}
	case "platform":
		for _, col := range columns {
			parent := catalog[col.ParentTable]
			if col.TableName == entry.Name && (parent.Scope == "tenant" || parent.Scope == "tenantChild") {
				problems = append(problems, entry.Name+" has a tenant parent and needs tenantChild scope")
				break
			}
		}
	case "account", "demo":
	default:
		problems = append(problems, entry.Name+" has unknown scope "+entry.Scope)
	}
	if entry.Scope != "tenant" && entry.Scope != "tenantChild" && strings.TrimSpace(entry.Reason) == "" {
		problems = append(problems, entry.Name+" needs a scope reason")
	}
	if entry.Scope != "tenantChild" && entry.Parent != nil {
		problems = append(problems, entry.Name+" has a parent outside tenantChild scope")
	}
	return problems
}

func lifecycleReachesTenant(name string, catalog map[string]lifecycleTable, columns []lifecycleColumn, seen map[string]bool) bool {
	entry, exists := catalog[name]
	if !exists || seen[name] {
		return false
	}
	if entry.Scope == "tenant" {
		for _, col := range columns {
			if col.TableName == name && col.ColumnName == "tenant_id" {
				return true
			}
		}
		return false
	}
	if entry.Scope != "tenantChild" || entry.Parent == nil {
		return false
	}
	seen[name] = true
	for _, col := range columns {
		if col.TableName == name && col.ColumnName == entry.Parent.Column && col.ParentTable == entry.Parent.Table && col.ParentColumn == entry.Parent.ParentColumn {
			return lifecycleReachesTenant(entry.Parent.Table, catalog, columns, seen)
		}
	}
	return false
}

func lifecycleExportProblems(entry lifecycleTable, cols map[string]bool) []string {
	var problems []string
	switch entry.Export {
	case "include":
	case "excludeSecret", "excludeInternal":
		if strings.TrimSpace(entry.ExportReason) == "" {
			problems = append(problems, entry.Name+" needs an export exclusion reason")
		}
	default:
		problems = append(problems, entry.Name+" has unknown export policy "+entry.Export)
	}
	for _, rules := range []map[string]string{entry.ExcludeSecretColumns, entry.AllowedSensitiveColumns} {
		for col, reason := range rules {
			if !cols[col] || strings.TrimSpace(reason) == "" {
				problems = append(problems, entry.Name+"."+col+" needs an existing column and reason")
			}
		}
	}
	for col := range entry.AllowedSensitiveColumns {
		if !lifecycleSecretName.MatchString(col) || entry.ExcludeSecretColumns[col] != "" {
			problems = append(problems, entry.Name+"."+col+" has a stale or conflicting sensitive-name release")
		}
	}
	if entry.Export == "include" {
		for col := range cols {
			if lifecycleSecretName.MatchString(col) && entry.ExcludeSecretColumns[col] == "" && entry.AllowedSensitiveColumns[col] == "" {
				problems = append(problems, fmt.Sprintf("%s.%s needs export exclusion or a reasoned release", entry.Name, col))
			}
		}
	}
	return problems
}

func TestLifecycleClassificationRejectsGaps(t *testing.T) {
	t.Parallel()
	columns := []lifecycleColumn{
		{TableName: "sample.parent", ColumnName: "tenant_id"},
		{TableName: "sample.parent", ColumnName: "id"},
		{TableName: "sample.parent", ColumnName: "password_hash"},
		{TableName: "sample.child", ColumnName: "parent_id", ParentTable: "sample.parent", ParentColumn: "id"},
	}
	valid := func() []lifecycleTable {
		return []lifecycleTable{
			{Name: "sample.parent", Scope: "tenant", Export: "include", ExcludeSecretColumns: map[string]string{"password_hash": "Authentication secret."}},
			{Name: "sample.child", Scope: "tenantChild", Export: "include", Parent: &lifecycleParent{Table: "sample.parent", Column: "parent_id", ParentColumn: "id"}},
		}
	}
	require.Empty(t, lifecycleClassificationProblems(valid(), columns))
	tests := []struct {
		name    string
		mutate  func([]lifecycleTable) []lifecycleTable
		message string
	}{
		{"new table", func(v []lifecycleTable) []lifecycleTable { return v[:1] }, "has no lifecycle classification"},
		{"stale table", func(v []lifecycleTable) []lifecycleTable { v[0].Name = "sample.absent"; return v }, "is stale"},
		{"duplicate", func(v []lifecycleTable) []lifecycleTable { return append(v, v[0]) }, "classified twice"},
		{"missing tenant column", func(v []lifecycleTable) []lifecycleTable { v[1].Scope = "tenant"; v[1].Parent = nil; return v }, "has no tenant_id"},
		{"misclassified tenant", func(v []lifecycleTable) []lifecycleTable { v[0].Scope = "platform"; v[0].Reason = "fixture"; return v }, "has tenant_id but is not tenant-scoped"},
		{"hidden tenant child", func(v []lifecycleTable) []lifecycleTable {
			v[1].Scope = "platform"
			v[1].Parent = nil
			v[1].Reason = "fixture"
			return v
		}, "needs tenantChild scope"},
		{"broken FK", func(v []lifecycleTable) []lifecycleTable { v[1].Parent.Column = "absent"; return v }, "has no declared FK path"},
		{"cyclic parent", func(v []lifecycleTable) []lifecycleTable { v[1].Parent.Table = "sample.child"; return v }, "has no declared FK path"},
		{"unexcluded secret", func(v []lifecycleTable) []lifecycleTable { v[0].ExcludeSecretColumns = nil; return v }, "needs export exclusion"},
		{"stale column", func(v []lifecycleTable) []lifecycleTable { v[0].ExcludeSecretColumns["absent"] = "fixture"; return v }, "needs an existing column"},
		{"unreasoned exclusion", func(v []lifecycleTable) []lifecycleTable { v[0].Export = "excludeSecret"; return v }, "needs an export exclusion reason"},
		{"unreasoned release", func(v []lifecycleTable) []lifecycleTable {
			v[0].AllowedSensitiveColumns = map[string]string{"password_hash": " "}
			return v
		}, "needs an existing column and reason"},
		{"tenant is not a person", func(v []lifecycleTable) []lifecycleTable {
			v[0].PersonKeys = map[string]string{"tenant_id": "sample.parent.tenant_id"}
			return v
		}, "not a person ID reference"},
		{"unknown export", func(v []lifecycleTable) []lifecycleTable { v[0].Export = "unknown"; return v }, "unknown export policy"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Contains(t, strings.Join(lifecycleClassificationProblems(tt.mutate(valid()), columns), "\n"), tt.message)
		})
	}
}
