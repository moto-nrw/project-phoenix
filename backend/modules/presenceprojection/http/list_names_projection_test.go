package presenceprojectionhttp_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The Betreuungsplan only shows who a child is (#4996). view=names returns the
// same children in the same order as the full list, with nothing but the ID
// and the name.
func TestListStudents_NamesProjection(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)
	var studentIDs []int64
	for i := range 3 {
		student := testpkg.CreateTestStudent(t, tc.db, "NamesOnly", fmt.Sprintf("Kind%d", i), "NO1")
		testpkg.FillWideStudentFields(t, tc.db, student.ID, student.PersonID)
		studentIDs = append(studentIDs, student.ID)
	}
	query := "search=NamesOnly&page_size=100"

	fullRR := authExec(t, tc, testutil.NewRequest("GET", "/?"+query, nil), testutil.AdminTestClaims(1), listStudentsPerms)
	require.Equal(t, http.StatusOK, fullRR.Code, "body: %s", fullRR.Body.String())
	namesRR := authExec(t, tc, testutil.NewRequest("GET", "/?"+query+"&view=names", nil), testutil.AdminTestClaims(1), listStudentsPerms)
	require.Equal(t, http.StatusOK, namesRR.Code, "body: %s", namesRR.Body.String())

	fullIDs := listStudentIDs(t, fullRR.Body.Bytes())
	require.Len(t, fullIDs, len(studentIDs))
	assert.Equal(t, fullIDs, listStudentIDs(t, namesRR.Body.Bytes()),
		"view=names must not change which children are returned or their order")

	var envelope struct {
		Data []map[string]json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(namesRR.Body.Bytes(), &envelope))
	require.Len(t, envelope.Data, len(studentIDs))
	for _, row := range envelope.Data {
		keys := make([]string, 0, len(row))
		for key := range row {
			keys = append(keys, key)
		}
		assert.ElementsMatch(t, []string{"id", "first_name", "last_name"}, keys)
	}
	assert.Contains(t, namesRR.Body.String(), `"first_name":"NamesOnly"`)
	assert.Less(t, namesRR.Body.Len(), fullRR.Body.Len()/4, "the names view must stay a small fraction of the full list")
}
