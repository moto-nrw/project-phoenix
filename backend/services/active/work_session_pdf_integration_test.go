package active_test

import (
	"bytes"
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestWorkSessionPDFUsesComposedRenderer(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	staff := testpkg.CreateTestStaff(t, db, "PDF", "Export")
	module, err := services.NewWorkSessionTestModule(db, testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	file, err := module.WorkSession.ExportSessions(testpkg.Ctx(t), staff.ID,
		timezone.NewDate(2024, 1, 1), timezone.NewDate(2024, 1, 7), "pdf")
	require.NoError(t, err)
	require.NotNil(t, file)
	require.Equal(t, "zeiterfassung_2024-01-01_2024-01-07.pdf", file.Filename)
	require.Equal(t, "application/pdf", file.ContentType)
	require.True(t, bytes.HasPrefix(file.Data, []byte("%PDF")))
	workbook, err := module.WorkSession.ExportSessions(testpkg.Ctx(t), staff.ID,
		timezone.NewDate(2024, 1, 1), timezone.NewDate(2024, 1, 7), "xlsx")
	require.NoError(t, err)
	require.NotNil(t, workbook)
	require.Equal(t, "zeiterfassung_2024-01-01_2024-01-07.xlsx", workbook.Filename)
	require.Contains(t, workbook.ContentType, "spreadsheetml")
	require.True(t, bytes.HasPrefix(workbook.Data, []byte("PK")))
}
