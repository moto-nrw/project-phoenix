package studentpresencehttp

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose/presenceservice"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func newStaffNotesResource(db *bun.DB) *Resource {
	rows := ComposeStudentRows(db)
	return NewResource(ResourceConfig{

		PeopleDirectory: rows.PeopleDirectory,

		StudentStatusDayService: presenceservice.NewStatusDays(rows.StudentStatusDay, nil, nil, rows.LockExceptionDay),
		Logger:                  slog.Default(),
	})
}

func staffNotesRouter(rs *Resource) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Get("/{id}/status-days", ComposeStatusDayReader(rs))
	r.Post("/{id}/status-days", rs.createStudentStatusDays)
	return r
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
