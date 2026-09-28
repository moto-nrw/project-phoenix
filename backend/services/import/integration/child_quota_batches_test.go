package integration

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Every batch checks the Kinderkontingent under the quota lock, even when no
// preflight ran before it (#3571). That is the backstop for children created
// between the import's start check and its batches: the batch that would
// overshoot is refused whole, the batches before it stay.
func TestImportBatches_EachBatchChecksTheChildQuota(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, err := services.NewImportTestModule(db, testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	actor := newImporter(t, db)
	testpkg.SetTestChildQuota(t, db, 1, 150)
	rows := make([]importModels.StudentImportRow, 160)
	for i := range rows {
		rows[i] = importModels.StudentImportRow{FirstName: fmt.Sprintf("Kind%d", i), LastName: "Kontingent", SchoolClass: "1A", DataRetentionDays: 30}
	}

	_, err = module.Import.ImportBatches(testpkg.Ctx(t),
		importModels.ImportRequest[importModels.StudentImportRow]{Rows: rows, Mode: importModels.ImportModeCreate, UserID: actor.staffID, SkipInvalidRows: true},
		importModels.BatchAudit{EntityType: "student", Filename: "kontingent.csv", AccountID: actor.accountID})

	// Read as the HTTP adapter reads a business rejection: code and details.
	var refusal interface {
		ErrorCode() string
		ErrorDetails() any
	}
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, "students.child_quota_reached", refusal.ErrorCode())
	details, err := json.Marshal(refusal.ErrorDetails())
	require.NoError(t, err)
	assert.JSONEq(t, `{"booked_places":150,"occupied_places":150,"requested_places":1}`, string(details),
		"the second batch reaches the quota at its 51st child")
	count, err := db.NewSelect().TableExpr("users.student_school_memberships").
		Where("tenant_id = ?", testpkg.Tenant(t)).Where("deleted_at IS NULL").Count(testpkg.Ctx(t))
	require.NoError(t, err)
	assert.Equal(t, 100, count, "the first batch stays, the refused one is rolled back whole")
}
