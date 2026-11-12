package tenantexport

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/organizationtenancy"
)

// A row whose first document column is NULL still writes its later
// document columns (ADR 0187): a missing letter does not drop the contract.
func TestWriteTableDocumentsWritesTheLaterColumnsOfARowWithANullDocument(t *testing.T) {
	t.Parallel()
	out := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(out, documentDirectory), 0o700))
	rows := `{"id": 7, "letter": null, "contract": "\\x255044462d"}` + "\n" + `{"id": 8, "letter": "\\x255044462d", "contract": null}` + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(out, "billing.documents.jsonl"), []byte(rows), 0o600))
	run := &exportRun{
		options:      organizationtenancy.TenantExportOptions{Output: out},
		schema:       &exportSchema{primaryKeys: map[string][]string{"billing.documents": {"id"}}},
		fingerprints: organizationtenancy.ExportFingerprints{Bytes: func([]byte) string { return "checksum" }},
	}
	summary := &organizationtenancy.ExportDocumentFiles{}
	index, err := os.Create(filepath.Join(out, "index.jsonl"))
	require.NoError(t, err)
	defer func() { _ = index.Close() }()

	require.NoError(t, run.writeTableDocuments(context.Background(), "billing.documents", []string{"letter", "contract"},
		json.NewEncoder(index), summary))

	require.FileExists(t, filepath.Join(out, documentDirectory, "billing.documents", "7-contract.pdf"),
		"the contract of row 7 although its letter is NULL")
	require.FileExists(t, filepath.Join(out, documentDirectory, "billing.documents", "8-letter.pdf"))
	require.Equal(t, []organizationtenancy.ExportDocumentColumn{
		{Table: "billing.documents", Column: "letter", Files: 1, Bytes: 5},
		{Table: "billing.documents", Column: "contract", Files: 1, Bytes: 5},
	}, summary.Columns)
}
