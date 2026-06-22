package parent_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/parent"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	repositories "github.com/moto-nrw/project-phoenix/database/repositories"
	configService "github.com/moto-nrw/project-phoenix/services/config"
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// alwaysOnSettings enables both parent-portal features for the handler
// E2E tests; only ResolveBoolForTenant is exercised.
type alwaysOnSettings struct{ configService.SettingsService }

func (alwaysOnSettings) ResolveBoolForTenant(_ context.Context, _ int64, _ string) (bool, error) {
	return true, nil
}

// testJWTSecret must match the constant testpkg.GetTestTokenAuth signs
// with, so the Router's MustNewTokenAuth (which reads viper) validates the
// tokens these tests mint.
const testJWTSecret = "test-jwt-secret-32-chars-minimum"

// init pins the JWT config before the testpkg token-auth singleton is
// lazily created, so minted tokens are signed with the matching secret
// and carry a real lifetime.
func init() {
	viper.Set("auth_jwt_secret", testJWTSecret)
	viper.Set("auth_jwt_expiry", time.Hour)
}

func newWriteRouter(t *testing.T, db *bun.DB) http.Handler {
	t.Helper()
	// Align the Router's MustNewTokenAuth with the secret testpkg signs
	// with, and give minted tokens a real (non-zero) lifetime so the real
	// Verifier doesn't treat them as already-expired.
	viper.Set("auth_jwt_secret", testJWTSecret)
	viper.Set("auth_jwt_expiry", time.Hour)
	repos := repositories.NewFactory(db)
	svc := parentService.NewService(parentService.ServiceConfig{
		ChildRepo:     repos.ParentChild,
		StatusDayRepo: repos.StudentStatusDay,
		StudentRepo:   repos.Student,
		NoteRepo:      repos.StudentParentNote,
		Settings:      alwaysOnSettings{},
		DB:            db,
		Logger:        slog.Default(),
	})
	rs := parent.NewResource(nil, svc, nil, nil, nil, db)
	return rs.Router()
}

func parentToken(t *testing.T, accountID int64) string {
	t.Helper()
	tokenAuth := testpkg.GetTestTokenAuth(t)
	token, err := tokenAuth.CreateJWT(jwt.AppClaims{
		ID:    int(accountID),
		Sub:   strconv.FormatInt(accountID, 10),
		Scope: tenant.ScopeParent,
		Roles: []string{"guardian"},
	})
	require.NoError(t, err)
	return token
}

func doRequest(t *testing.T, router http.Handler, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

// envelope mirrors common.Respond's { status, data, message } shape.
type envelope struct {
	Status string          `json:"status"`
	Data   json.RawMessage `json:"data"`
}

func TestSickNoteEndpoint_SubmitAndList(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	router := newWriteRouter(t, db)
	token := parentToken(t, chain.AccountID)
	sid := strconv.FormatInt(chain.StudentID, 10)

	// Submit.
	rr := doRequest(t, router, http.MethodPost, "/me/children/"+sid+"/sick-note", token,
		map[string]any{"dates": []string{nowISO(), futureISO(3)}, "reason": "Fieber"})
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	var env envelope
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &env))
	var days []map[string]any
	require.NoError(t, json.Unmarshal(env.Data, &days))
	assert.Len(t, days, 2)

	// List.
	rr = doRequest(t, router, http.MethodGet, "/me/children/"+sid+"/sick-note", token, nil)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
}

func TestNotesEndpoint_AddAndList(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	router := newWriteRouter(t, db)
	token := parentToken(t, chain.AccountID)
	sid := strconv.FormatInt(chain.StudentID, 10)

	rr := doRequest(t, router, http.MethodPost, "/me/children/"+sid+"/notes", token,
		map[string]any{"body": "Bitte um Rückruf"})
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())

	rr = doRequest(t, router, http.MethodGet, "/me/children/"+sid+"/notes", token, nil)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var env envelope
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &env))
	var notes []map[string]any
	require.NoError(t, json.Unmarshal(env.Data, &notes))
	require.Len(t, notes, 1)
	assert.Equal(t, "Bitte um Rückruf", notes[0]["body"])
}

func TestWriteEndpoints_RejectMissingToken(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	router := newWriteRouter(t, db)

	rr := doRequest(t, router, http.MethodGet, "/me/children/1/notes", "", nil)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestSickNoteEndpoint_ForbidsNonOwnedChild(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	router := newWriteRouter(t, db)
	token := parentToken(t, chain.AccountID)

	// A student id the account is not a guardian of.
	other := testpkg.CreateTestStudent(t, db, "Fremd", "Kind", "4a")
	defer func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM users.students WHERE id = ?`, other.ID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM users.persons WHERE id = ?`, other.PersonID)
	}()
	sid := strconv.FormatInt(other.ID, 10)

	rr := doRequest(t, router, http.MethodPost, "/me/children/"+sid+"/sick-note", token,
		map[string]any{"dates": []string{nowISO()}})
	assert.Equal(t, http.StatusForbidden, rr.Code, rr.Body.String())
}

func TestSickNoteEndpoint_RejectsEmptyDates(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	router := newWriteRouter(t, db)
	token := parentToken(t, chain.AccountID)
	sid := strconv.FormatInt(chain.StudentID, 10)

	rr := doRequest(t, router, http.MethodPost, "/me/children/"+sid+"/sick-note", token,
		map[string]any{"dates": []string{}})
	assert.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
}

func TestNotesEndpoint_RejectsEmptyBody(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	router := newWriteRouter(t, db)
	token := parentToken(t, chain.AccountID)
	sid := strconv.FormatInt(chain.StudentID, 10)

	rr := doRequest(t, router, http.MethodPost, "/me/children/"+sid+"/notes", token,
		map[string]any{"body": "   "})
	assert.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
}

func TestSickNoteEndpoint_RejectsBadStudentID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	router := newWriteRouter(t, db)
	token := parentToken(t, 12345)

	rr := doRequest(t, router, http.MethodGet, "/me/children/not-a-number/sick-note", token, nil)
	assert.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
}

// disabledSettings turns both parent-portal features off, exercising the
// feature-gate (403) arms of renderParentWriteError.
type disabledSettings struct{ configService.SettingsService }

func (disabledSettings) ResolveBoolForTenant(_ context.Context, _ int64, _ string) (bool, error) {
	return false, nil
}

func newDisabledWriteRouter(t *testing.T, db *bun.DB) http.Handler {
	t.Helper()
	viper.Set("auth_jwt_secret", testJWTSecret)
	viper.Set("auth_jwt_expiry", time.Hour)
	repos := repositories.NewFactory(db)
	svc := parentService.NewService(parentService.ServiceConfig{
		ChildRepo:     repos.ParentChild,
		StatusDayRepo: repos.StudentStatusDay,
		StudentRepo:   repos.Student,
		NoteRepo:      repos.StudentParentNote,
		Settings:      disabledSettings{},
		DB:            db,
		Logger:        slog.Default(),
	})
	return parent.NewResource(nil, svc, nil, nil, nil, db).Router()
}

func TestWriteEndpoints_FeatureDisabledForbidden(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	router := newDisabledWriteRouter(t, db)
	token := parentToken(t, chain.AccountID)
	sid := strconv.FormatInt(chain.StudentID, 10)

	rr := doRequest(t, router, http.MethodPost, "/me/children/"+sid+"/sick-note", token,
		map[string]any{"dates": []string{nowISO()}})
	assert.Equal(t, http.StatusForbidden, rr.Code, "sick notes disabled → 403")

	rr = doRequest(t, router, http.MethodPost, "/me/children/"+sid+"/notes", token,
		map[string]any{"body": "Hallo"})
	assert.Equal(t, http.StatusForbidden, rr.Code, "notes disabled → 403")
}

func TestSickNoteEndpoint_RejectsBadDateRange(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	router := newWriteRouter(t, db)
	token := parentToken(t, chain.AccountID)
	sid := strconv.FormatInt(chain.StudentID, 10)

	for _, q := range []string{"?from=broken", "?to=broken", "?from=2026-05-29&to=2026-05-25"} {
		rr := doRequest(t, router, http.MethodGet, "/me/children/"+sid+"/sick-note"+q, token, nil)
		assert.Equal(t, http.StatusBadRequest, rr.Code, "bad range %q → 400", q)
	}
}

func TestSickNoteEndpoint_RejectsInvalidBody(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	router := newWriteRouter(t, db)
	token := parentToken(t, chain.AccountID)
	sid := strconv.FormatInt(chain.StudentID, 10)

	// "dates" as a string fails to decode into []string → 400.
	rr := doRequest(t, router, http.MethodPost, "/me/children/"+sid+"/sick-note", token,
		map[string]any{"dates": "not-an-array"})
	assert.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
}

// TestSickNoteEndpoint_SubmitExcused covers the issue #1735 path: a parent who
// picks "Termin/Abwesenheit" sends status="excused", and the API stores and
// returns an excused status day (not a Krankmeldung).
func TestSickNoteEndpoint_SubmitExcused(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	router := newWriteRouter(t, db)
	token := parentToken(t, chain.AccountID)
	sid := strconv.FormatInt(chain.StudentID, 10)

	rr := doRequest(t, router, http.MethodPost, "/me/children/"+sid+"/sick-note", token,
		map[string]any{"dates": []string{nowISO()}, "reason": "Zahnarzttermin", "status": "excused"})
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())

	var env envelope
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &env))
	var days []map[string]any
	require.NoError(t, json.Unmarshal(env.Data, &days))
	require.Len(t, days, 1)
	assert.Equal(t, "excused", days[0]["status"], "an excused submission must store an excused status day")
	assert.Equal(t, "Zahnarzttermin", days[0]["note"])
}

// TestSickNoteEndpoint_DefaultsToSickWhenStatusOmitted pins the backward-compat
// default: an older client that omits "status" still files a Krankmeldung.
func TestSickNoteEndpoint_DefaultsToSickWhenStatusOmitted(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	router := newWriteRouter(t, db)
	token := parentToken(t, chain.AccountID)
	sid := strconv.FormatInt(chain.StudentID, 10)

	rr := doRequest(t, router, http.MethodPost, "/me/children/"+sid+"/sick-note", token,
		map[string]any{"dates": []string{nowISO()}})
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())

	var env envelope
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &env))
	var days []map[string]any
	require.NoError(t, json.Unmarshal(env.Data, &days))
	require.Len(t, days, 1)
	assert.Equal(t, "sick", days[0]["status"], "an omitted status must default to a Krankmeldung")
}

// TestSickNoteEndpoint_RejectsInvalidStatus covers the ErrInvalidStatus arm of
// renderParentWriteError: a status that is neither sick nor excused is a 400 at
// the API boundary, never silently coerced.
func TestSickNoteEndpoint_RejectsInvalidStatus(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	router := newWriteRouter(t, db)
	token := parentToken(t, chain.AccountID)
	sid := strconv.FormatInt(chain.StudentID, 10)

	rr := doRequest(t, router, http.MethodPost, "/me/children/"+sid+"/sick-note", token,
		map[string]any{"dates": []string{nowISO()}, "status": "class_trip"})
	assert.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
}

func nowISO() string         { return isoDay(0) }
func futureISO(d int) string { return isoDay(d) }
func isoDay(addDays int) string {
	return time.Now().AddDate(0, 0, addDays).Format("2006-01-02")
}
