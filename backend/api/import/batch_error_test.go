package importapi_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	importAPI "github.com/moto-nrw/project-phoenix/api/import"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBatchFailureResponseRetainsCommittedProgressAndRowErrors(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	_, module := testutil.SetupImportModule(t, db)
	resource := importAPI.NewResource(module.Import, module.StaffImport, module.ClassListImport, module.Users, db)
	_, account := testpkg.CreateTestTeacherWithAccount(t, db, "Batch", "Admin")
	_, err := db.ExecContext(context.Background(), `ALTER TABLE users.class_list_entries ADD CONSTRAINT refuse_batch CHECK (first_name <> 'Refused')`)
	require.NoError(t, err)
	var csv strings.Builder
	csv.WriteString("Vorname,Nachname,Klasse\n")
	for i := range 205 {
		name := fmt.Sprintf("Kind%d", i)
		if i == 150 {
			name = "Refused"
		}
		fmt.Fprintf(&csv, "%s,Batch,1a\n", name)
	}
	req := testutil.NewMultipartRequest(t, http.MethodPost, "/class-list-entries/import", "file", "failure.csv", csv.String(), adminBearer(t, account.ID))
	rr := testutil.ExecuteRequestForTest(t, resource.Router(), req)
	require.Equal(t, http.StatusInternalServerError, rr.Code, "%s", rr.Body.String())
	var response struct {
		Status  string
		Code    string
		Details struct {
			Result struct {
				CreatedCount, TotalRows int
				Errors                  []struct {
					RowNumber int
					Errors    []struct{ Code string }
				}
			}
		}
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &response))
	assert.Equal(t, "error", response.Status)
	assert.Equal(t, "import_batch_failed", response.Code)
	assert.Equal(t, 100, response.Details.Result.CreatedCount)
	assert.Equal(t, 205, response.Details.Result.TotalRows)
	require.Len(t, response.Details.Result.Errors, 1)
	assert.Equal(t, 152, response.Details.Result.Errors[0].RowNumber)
	assert.Equal(t, "creation_failed", response.Details.Result.Errors[0].Errors[0].Code)
	var count int
	require.NoError(t, db.NewSelect().TableExpr("users.class_list_entries").ColumnExpr("count(*)").Where("tenant_id = ?", testpkg.Tenant(t)).Scan(testpkg.Ctx(t), &count))
	assert.Equal(t, 100, count)
}
