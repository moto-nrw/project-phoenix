package compose_test

import (
	"encoding/json"
	"testing"

	"github.com/moto-ogs/project-phoenix/modules/organizationtenancy"
	"github.com/stretchr/testify/require"
)

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
