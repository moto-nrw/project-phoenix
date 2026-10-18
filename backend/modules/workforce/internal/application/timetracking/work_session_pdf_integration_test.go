package timetracking_test

import (
	"bytes"
	"testing"

	"github.com/moto-nrw/project-phoenix/api"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestWorkSessionPDFUsesComposedRenderer(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	staff := testpkg.CreateTestStaff(t, db, "PDF", "Export")
	timeTracking, _, err := api.NewWorkSessionTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	file, err := timeTracking.WorkSessions.ExportSessions(testpkg.Ctx(t), staff.ID, "2024-01-01", "2024-01-07", "pdf")
	require.NoError(t, err)
	require.NotNil(t, file)
	require.Equal(t, "zeiterfassung_2024-01-01_2024-01-07.pdf", file.Filename)
	require.Equal(t, "application/pdf", file.ContentType)
	require.True(t, bytes.HasPrefix(file.Data, []byte("%PDF")))
	workbook, err := timeTracking.WorkSessions.ExportSessions(testpkg.Ctx(t), staff.ID, "2024-01-01", "2024-01-07", "xlsx")
	require.NoError(t, err)
	require.NotNil(t, workbook)
	require.Equal(t, "zeiterfassung_2024-01-01_2024-01-07.xlsx", workbook.Filename)
	require.Contains(t, workbook.ContentType, "spreadsheetml")
	require.True(t, bytes.HasPrefix(workbook.Data, []byte("PK")))
}
