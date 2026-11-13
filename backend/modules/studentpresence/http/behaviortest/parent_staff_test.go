package studentpresencehttp_test

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/moto-ogs/project-phoenix/api/testutil"
	"github.com/moto-ogs/project-phoenix/modules/studentpresence/compose/presenceservice"
	studentsAPI "github.com/moto-ogs/project-phoenix/modules/studentpresence/http"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func newStaffNotesResource(db *bun.DB) *studentsAPI.Resource {
	rows := composeStudentRows(db)
	return studentsAPI.NewResource(studentsAPI.ResourceConfig{

		PeopleDirectory: rows.PeopleDirectory,

		StudentStatusDayService: presenceservice.NewStatusDays(rows.StudentStatusDay, nil, nil, rows.LockExceptionDay),
		SettingsService:         &settingsDouble{},
		Logger:                  slog.Default(),
	})
}

func staffNotesRouter(rs *studentsAPI.Resource) chi.Router {
	return statusDayTestRouter(rs)
}

func TestStaffStatusDay_ReasonStoredAndReturned(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	router := staffNotesRouter(newStaffNotesResource(db))

	student := testpkg.CreateTestStudent(t, db, "Reason", "Kind", "1a")

	createReq := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/status-days", student.ID), map[string]any{
		"status": "sick",
		"dates":  []string{"2026-05-25"},
		"reason": "Fieber",
	})
	createRR := executeStatusDayHandler(t, router, createReq, testutil.AdminTestClaims(42), []string{"admin:*"})
	require.Equal(t, http.StatusCreated, createRR.Code, createRR.Body.String())

	getReq := testutil.NewRequest("GET", fmt.Sprintf("/%d/status-days?from=2026-05-25&to=2026-05-26", student.ID), nil)
	getRR := executeStatusDayHandler(t, router, getReq, testutil.AdminTestClaims(42), []string{"admin:*"})
	require.Equal(t, http.StatusOK, getRR.Code, getRR.Body.String())

	var env struct {
		Data []struct {
			Status string  `json:"status"`
			Note   *string `json:"note"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(getRR.Body.Bytes(), &env))
	require.NotEmpty(t, env.Data)
	require.NotNil(t, env.Data[0].Note, "staff-supplied reason must round-trip in the status-day response")
	assert.Equal(t, "Fieber", *env.Data[0].Note)
}
