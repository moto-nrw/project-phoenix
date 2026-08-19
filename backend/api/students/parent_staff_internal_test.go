package students

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"testing"

	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/strutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// --- sick reason normalization (pure) ---

func TestNormalizeSickReason(t *testing.T) {
	assert.Nil(t, strutil.TrimPtrToNil(nil), "nil stays nil")

	blank := "   "
	assert.Nil(t, strutil.TrimPtrToNil(&blank), "blank trims to nil")

	raw := "  Fieber, beim Arzt  "
	got := strutil.TrimPtrToNil(&raw)
	require.NotNil(t, got)
	assert.Equal(t, "Fieber, beim Arzt", *got, "non-blank is trimmed")
}

// --- staff sick reason round-trip via status-days ---

func newStaffNotesResource(db *bun.DB) *Resource {
	rf := repositories.NewFactory(db)
	return NewResource(ResourceConfig{
		PersonService:           usersSvc.NewPersonService(usersSvc.PersonServiceDependencies{StudentRepo: rf.Student}),
		StudentService:          usersSvc.NewStudentService(rf.Student, rf.PrivacyConsent, rf.StudentCompanion, nil),
		StudentStatusDayService: activeSvc.NewStudentStatusDayService(rf.StudentStatusDay),
		Logger:                  slog.Default(),
		DB:                      db,
	})
}

func staffNotesRouter(rs *Resource) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Get("/{id}/status-days", rs.getStudentStatusDays)
	r.Post("/{id}/status-days", rs.createStudentStatusDays)
	return r
}

func TestStaffStatusDay_ReasonStoredAndReturned(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	router := staffNotesRouter(newStaffNotesResource(db))

	student := testpkg.CreateTestStudent(t, db, "Reason", "Kind", "1a")
	defer func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM active.student_status_days WHERE student_id = ?`, student.ID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM users.students WHERE id = ?`, student.ID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM users.persons WHERE id = ?`, student.PersonID)
	}()

	createReq := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/status-days", student.ID), map[string]any{
		"status": "sick",
		"dates":  []string{"2026-05-25"},
		"reason": "Fieber",
	})
	createRR := executeStatusDayHandler(router, createReq, testutil.AdminTestClaims(42), []string{"admin:*"})
	require.Equal(t, http.StatusCreated, createRR.Code, createRR.Body.String())

	getReq := testutil.NewRequest("GET", fmt.Sprintf("/%d/status-days?from=2026-05-25&to=2026-05-26", student.ID), nil)
	getRR := executeStatusDayHandler(router, getReq, testutil.AdminTestClaims(42), []string{"admin:*"})
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
