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
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// --- normalizeSickReason (pure) ---

func TestNormalizeSickReason(t *testing.T) {
	assert.Nil(t, normalizeSickReason(nil), "nil stays nil")

	blank := "   "
	assert.Nil(t, normalizeSickReason(&blank), "blank trims to nil")

	raw := "  Fieber, beim Arzt  "
	got := normalizeSickReason(&raw)
	require.NotNil(t, got)
	assert.Equal(t, "Fieber, beim Arzt", *got, "non-blank is trimmed")
}

// --- staff sick reason round-trip via status-days ---

func newStaffNotesResource(db *bun.DB) *Resource {
	rf := repositories.NewFactory(db)
	return NewResource(ResourceConfig{
		PersonService:           usersSvc.NewPersonService(usersSvc.PersonServiceDependencies{StudentRepo: rf.Student}),
		StudentService:          usersSvc.NewStudentService(rf.Student, rf.PrivacyConsent, rf.StudentParentNote),
		StudentStatusDayService: activeSvc.NewStudentStatusDayService(rf.StudentStatusDay),
		Logger:                  slog.Default(),
		DB:                      db,
	})
}

func staffNotesRouter(rs *Resource) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Get("/{id}/parent-notes", rs.getStudentParentNotes)
	r.Get("/{id}/status-days", rs.getStudentStatusDays)
	r.Post("/{id}/status-days", rs.createStudentStatusDays)
	return r
}

func TestStaffStatusDay_ReasonStoredAndReturned(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
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

// --- getStudentParentNotes (staff read) ---

func TestGetStudentParentNotes_ReturnsNewestNotes(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	router := staffNotesRouter(newStaffNotesResource(db))

	student := testpkg.CreateTestStudent(t, db, "Notes", "Kind", "1a")
	account := testpkg.CreateTestAccount(t, db, "staffnotes")
	defer func() {
		bg := context.Background()
		_, _ = db.ExecContext(bg, `DELETE FROM users.student_parent_notes WHERE student_id = ?`, student.ID)
		_, _ = db.ExecContext(bg, `DELETE FROM users.students WHERE id = ?`, student.ID)
		_, _ = db.ExecContext(bg, `DELETE FROM users.persons WHERE id = ?`, student.PersonID)
		_, _ = db.ExecContext(bg, `DELETE FROM auth.accounts WHERE id = ?`, account.ID)
	}()

	noteRepo := repositories.NewFactory(db).StudentParentNote
	ctx := tenant.WithTenantID(context.Background(), 1)
	for _, body := range []string{"alt", "neu"} {
		n := &users.StudentParentNote{StudentID: student.ID, GuardianAccountID: account.ID, Body: body}
		n.SetTenantID(1)
		require.NoError(t, noteRepo.Create(ctx, n))
	}

	req := testutil.NewRequest("GET", fmt.Sprintf("/%d/parent-notes", student.ID), nil)
	rr := executeStatusDayHandler(router, req, testutil.AdminTestClaims(42), []string{"admin:*"})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	var env struct {
		Data []struct {
			Body string `json:"body"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &env))
	require.Len(t, env.Data, 2)
	assert.Equal(t, "neu", env.Data[0].Body, "newest first")
}

func TestGetStudentParentNotes_NilRepoReturnsEmpty(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	// Resource intentionally built WITHOUT the student service (the
	// parent-note lookup dependency); the person lookup still works.
	rf := repositories.NewFactory(db)
	rs := NewResource(ResourceConfig{
		PersonService: usersSvc.NewPersonService(usersSvc.PersonServiceDependencies{StudentRepo: rf.Student}),
		Logger:        slog.Default(),
		DB:            db,
	})
	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	router.Get("/{id}/parent-notes", rs.getStudentParentNotes)

	student := testpkg.CreateTestStudent(t, db, "NilRepo", "Kind", "1a")
	defer func() {
		bg := context.Background()
		_, _ = db.ExecContext(bg, `DELETE FROM users.students WHERE id = ?`, student.ID)
		_, _ = db.ExecContext(bg, `DELETE FROM users.persons WHERE id = ?`, student.PersonID)
	}()

	req := testutil.NewRequest("GET", fmt.Sprintf("/%d/parent-notes", student.ID), nil)
	rr := executeStatusDayHandler(router, req, testutil.AdminTestClaims(42), []string{"admin:*"})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
}

func TestGetStudentParentNotes_EmptyWhenNone(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	router := staffNotesRouter(newStaffNotesResource(db))

	student := testpkg.CreateTestStudent(t, db, "NoNotes", "Kind", "1a")
	defer func() {
		bg := context.Background()
		_, _ = db.ExecContext(bg, `DELETE FROM users.students WHERE id = ?`, student.ID)
		_, _ = db.ExecContext(bg, `DELETE FROM users.persons WHERE id = ?`, student.PersonID)
	}()

	req := testutil.NewRequest("GET", fmt.Sprintf("/%d/parent-notes", student.ID), nil)
	rr := executeStatusDayHandler(router, req, testutil.AdminTestClaims(42), []string{"admin:*"})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	var env struct {
		Data []json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &env))
	assert.Empty(t, env.Data)
}
