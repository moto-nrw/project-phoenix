package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The erasure of 1.15.840 is the erasure of 1.15.823 plus the export files
// of the demo school, before its monthly invoices go (#4693, #6015).
func TestExportFileErasureExtendsTheErasureOf823(t *testing.T) {
	t.Parallel()
	require.Equal(t, 1, strings.Count(demoVisitorErasureCareNoticesSQL, exportFileErasureAnchor))
	assert.Equal(t, strings.Replace(demoVisitorErasureCareNoticesSQL, exportFileErasureAnchor,
		exportFileErasureStatement+exportFileErasureAnchor, 1), demoVisitorErasureExportFilesSQL)
	files := strings.Index(demoVisitorErasureExportFilesSQL, "DELETE FROM parent_billing.export_files")
	invoices := strings.Index(demoVisitorErasureExportFilesSQL, "DELETE FROM parent_billing.care_invoices")
	notices := strings.Index(demoVisitorErasureExportFilesSQL, "DELETE FROM parent_billing.care_notices")
	require.Positive(t, files)
	assert.Less(t, files, invoices, "the files go while the invoices they held still exist")
	assert.Less(t, files, notices, "and while the Jahresbescheide still exist")
}
