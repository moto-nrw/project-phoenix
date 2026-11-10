package compose_test

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

var updateReturnRegister = flag.Bool("update-return-register", false, "rewrite modules/organizationtenancy/return_register.json from the migrated schema")

// TestReturnRegisterMatchesMigratedSchema keeps the register of the return
// package (Art. 26 Data Act, ADR 0200) equal to what the code that writes
// model.json reads from the migrated schema. A migration or catalog change
// that alters the package fails here until the register is regenerated:
//
//	go test -tags=testhooks ./modules/organizationtenancy/compose -run TestReturnRegisterMatchesMigratedSchema -update-return-register
func TestReturnRegisterMatchesMigratedSchema(t *testing.T) {
	t.Parallel()
	builder, err := compose.NewReturnRegisterBuilder(testpkg.SetupTestDB(t), organizationtenancy.ExportFingerprints{Bytes: testpkg.ContentFingerprint})
	require.NoError(t, err)
	register, err := builder.BuildReturnRegister(t.Context())
	require.NoError(t, err)
	got, err := json.MarshalIndent(register, "", "  ")
	require.NoError(t, err)
	got = append(got, '\n')
	if *updateReturnRegister {
		require.NoError(t, os.WriteFile(filepath.Join("..", organizationtenancy.ReturnRegisterFile), got, 0o644))
	}
	embedded, err := organizationtenancy.ReturnRegisterSource.ReadFile(organizationtenancy.ReturnRegisterFile)
	require.NoError(t, err)
	if !*updateReturnRegister {
		require.Equal(t, string(embedded), string(got), "the return register drifted from the schema; regenerate it with -update-return-register")
	}

	source, err := organizationtenancy.DataLifecycleCatalogSource.ReadFile("data_lifecycle_catalog.json")
	require.NoError(t, err)
	var catalog organizationtenancy.DataLifecycleCatalog
	require.NoError(t, json.Unmarshal(source, &catalog))
	require.Equal(t, testpkg.ContentFingerprint(source), register.CatalogSHA256)
	require.Len(t, register.Files, len(catalog.Files))
	require.Equal(t, len(catalog.Tables), len(register.Tables)+len(register.ExcludedTables), "every catalog table is described or named as excluded")
	for _, table := range register.Tables {
		require.NotEmpty(t, table.Columns, table.Name)
		require.Contains(t, table.Packages, "school", "a school package carries every included table: %s", table.Name)
		for _, column := range table.Columns {
			require.NotContains(t, table.ExcludedColumns, column.Name, "%s.%s is excluded but described as exported", table.Name, column.Name)
		}
	}
	for _, table := range register.ExcludedTables {
		require.NotEmpty(t, table.Reason, "%s needs the reason it does not leave", table.Name)
	}
}

// registerModels answers the tables of the embedded register as model.json
// describes them.
func registerModels(t *testing.T) map[string]organizationtenancy.ReturnRegisterTable {
	t.Helper()
	data, err := organizationtenancy.ReturnRegisterSource.ReadFile(organizationtenancy.ReturnRegisterFile)
	require.NoError(t, err)
	var register organizationtenancy.ReturnRegister
	require.NoError(t, json.Unmarshal(data, &register))
	tables := make(map[string]organizationtenancy.ReturnRegisterTable, len(register.Tables))
	for _, table := range register.Tables {
		tables[table.Name] = table
	}
	return tables
}
