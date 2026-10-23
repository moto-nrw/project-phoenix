package organizationtenancy_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/stretchr/testify/require"
)

// The migration guards read the wire artifact without a domain import. Check
// separately that the shipped runtime artifact and the public DTO agree.
func TestDataLifecycleCatalogRuntimeContract(t *testing.T) {
	t.Parallel()
	data, err := organizationtenancy.DataLifecycleCatalogSource.ReadFile("data_lifecycle_catalog.json")
	require.NoError(t, err)
	var catalog organizationtenancy.DataLifecycleCatalog
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	require.NoError(t, decoder.Decode(&catalog))
	require.Equal(t, 1, catalog.Version)
	require.NotEmpty(t, catalog.Tables)
	require.NotEmpty(t, catalog.Files)
	for _, table := range catalog.Tables {
		if table.Name == "auth.accounts" {
			require.Equal(t, "account", table.Scope)
			require.Equal(t, "include", table.Export)
			require.NotEmpty(t, table.ExcludeSecretColumns["password_hash"])
			require.NotEmpty(t, table.ExcludeSecretColumns["calendar_feed_token"])
			return
		}
	}
	t.Fatal("auth.accounts classification missing from runtime artifact")
}
