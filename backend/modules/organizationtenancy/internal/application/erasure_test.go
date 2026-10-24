package application

import (
	"encoding/json"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/stretchr/testify/require"
)

func TestErasureFileCatalogRequiresReviewedDeletionPath(t *testing.T) {
	t.Parallel()
	raw, err := organizationtenancy.DataLifecycleCatalogSource.ReadFile("data_lifecycle_catalog.json")
	require.NoError(t, err)
	var catalog organizationtenancy.DataLifecycleCatalog
	require.NoError(t, json.Unmarshal(raw, &catalog))
	require.NoError(t, validateErasureFiles(catalog.Files))
	catalog.Files = append(catalog.Files, organizationtenancy.LifecycleFileKind{Kind: "unreviewed", Layout: "tenantDir"})
	require.Error(t, validateErasureFiles(catalog.Files))
	catalog.Files = catalog.Files[:len(catalog.Files)-1]
	catalog.Files[0].Layout = "account"
	require.Error(t, validateErasureFiles(catalog.Files))
}
