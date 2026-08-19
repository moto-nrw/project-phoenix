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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/parent"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	repositories "github.com/moto-nrw/project-phoenix/database/repositories"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	absenceSvc "github.com/moto-nrw/project-phoenix/services/absence"
	configService "github.com/moto-nrw/project-phoenix/services/config"
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// alwaysOnSettings enables the parent-portal features for the handler E2E
// tests; only ResolveBoolForTenant is exercised. The excused-approval gate
// (#1845) is deliberately kept OFF here so the existing excused tests keep
// exercising the direct status-day write they were written for (issue #1735);
// the approval path has its own test that flips it on explicitly.
type alwaysOnSettings struct{ configService.SettingsService }

func (alwaysOnSettings) ResolveBoolForTenant(_ context.Context, _ int64, key string) (bool, error) {
	if key == configModels.KeyParentExcusedRequiresApproval {
		return false, nil
	}
	return true, nil
}

// excusedApprovalOnSettings is alwaysOnSettings with the excused-approval gate
// turned ON, for the handler-level approval-path test.
type excusedApprovalOnSettings struct{ configService.SettingsService }

func (excusedApprovalOnSettings) ResolveBoolForTenant(_ context.Context, _ int64, _ string) (bool, error) {
	return true, nil
}

// testJWTSecret must match the constant testpkg.GetTestTokenAuth signs
// with, so the Router's MustNewTokenAuth (which reads viper) validates the
// tokens these tests mint.
const testJWTSecret = "test-jwt-secret-32-chars-minimum"

func newWriteRouter(t *testing.T, db *bun.DB) http.Handler {
	return newWriteRouterWithSettings(t, db, alwaysOnSettings{})
}

func newWriteRouterWithSettings(t *testing.T, db *bun.DB, settings configService.SettingsService) http.Handler {
	t.Helper()
	repos := repositories.NewFactory(db)
	excused := absenceSvc.NewExcusedAbsenceRequestServiceWithPartialAbsences(
		repos.ExcusedAbsenceRequest,
		repos.StudentStatusDay,
		repos.StudentPickupException,
		repos.Student,
		repos.Person,
		nil, nil, nil,
		slog.Default(),
		db,
	)
	svc := parentService.NewService(parentService.ServiceConfig{
		ChildRepo:           repos.ParentChild,
		StatusDayRepo:       repos.StudentStatusDay,
		StudentRepo:         repos.Student,
		PickupExceptionRepo: repos.StudentPickupException,
		Settings:            settings,
		ExcusedRequests:     excused,
		DB:                  db,
		Logger:              slog.Default(),
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
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	router := newWriteRouter(t, db)
	token := parentToken(t, chain.AccountID)
	sid := strconv.FormatInt(chain.StudentID, 10)

	// Submit.
	rr := doRequest(t, router, http.MethodPost, "/me/children/"+sid+"/sick-note", token,
		map[string]any{"dates": []string{nowISO(), futureISO(3)}, "reason": "Fieber"})
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	var env envelope
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &env))
	// A direct write responds with the bare status-day array (pre-#1845 shape),
	// not the {status_days, pending_request} envelope — see submitSickNote.
	var days []map[string]any
	require.NoError(t, json.Unmarshal(env.Data, &days))
	assert.Len(t, days, 2)

	// List.
	rr = doRequest(t, router, http.MethodGet, "/me/children/"+sid+"/sick-note", token, nil)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
}

func TestWriteEndpoints_RejectMissingToken(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	router := newWriteRouter(t, db)

	rr := doRequest(t, router, http.MethodGet, "/me/children/1/sick-note", "", nil)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestSickNoteEndpoint_ForbidsNonOwnedChild(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
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
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	router := newWriteRouter(t, db)
	token := parentToken(t, chain.AccountID)
	sid := strconv.FormatInt(chain.StudentID, 10)

	rr := doRequest(t, router, http.MethodPost, "/me/children/"+sid+"/sick-note", token,
		map[string]any{"dates": []string{}})
	assert.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
}

func TestSickNoteEndpoint_RejectsBadStudentID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
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
	repos := repositories.NewFactory(db)
	svc := parentService.NewService(parentService.ServiceConfig{
		ChildRepo:     repos.ParentChild,
		StatusDayRepo: repos.StudentStatusDay,
		StudentRepo:   repos.Student,
		Settings:      disabledSettings{},
		DB:            db,
		Logger:        slog.Default(),
	})
	return parent.NewResource(nil, svc, nil, nil, nil, db).Router()
}

func TestWriteEndpoints_FeatureDisabledForbidden(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	router := newDisabledWriteRouter(t, db)
	token := parentToken(t, chain.AccountID)
	sid := strconv.FormatInt(chain.StudentID, 10)

	rr := doRequest(t, router, http.MethodPost, "/me/children/"+sid+"/sick-note", token,
		map[string]any{"dates": []string{nowISO()}})
	assert.Equal(t, http.StatusForbidden, rr.Code, "sick notes disabled → 403")
}

func TestSickNoteEndpoint_RejectsBadDateRange(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
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
	chain := testpkg.CreateTestParentGuardianChain(t, db)
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
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	router := newWriteRouter(t, db)
	token := parentToken(t, chain.AccountID)
	sid := strconv.FormatInt(chain.StudentID, 10)

	rr := doRequest(t, router, http.MethodPost, "/me/children/"+sid+"/sick-note", token,
		map[string]any{"dates": []string{nowISO()}, "reason": "Zahnarzttermin", "status": "excused"})
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())

	var env envelope
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &env))
	// Direct write (approval gate off) → bare status-day array, not the envelope.
	var days []map[string]any
	require.NoError(t, json.Unmarshal(env.Data, &days))
	require.Len(t, days, 1)
	assert.Equal(t, "excused", days[0]["status"], "an excused submission must store an excused status day")
	assert.Equal(t, "Zahnarzttermin", days[0]["note"])
}

// TestSickNoteEndpoint_ExcusedApprovalPending covers the #1845 approval gate at
// the HTTP boundary: with the setting on, an excused submission creates a pending
// request and writes NO status day (the child stays expected). The submit
// response is the bare status-day array (empty) — the same shape every other
// path returns — so an already-open #1735-era tab, which has the "excused" option
// but expects an array and calls .map() on the response, never crashes on the
// gated path (#1845 review). The freshly created request is discovered via the
// excused-requests list endpoint the parent UI refetches after a submit.
func TestSickNoteEndpoint_ExcusedApprovalPending(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	router := newWriteRouterWithSettings(t, db, excusedApprovalOnSettings{})
	token := parentToken(t, chain.AccountID)
	sid := strconv.FormatInt(chain.StudentID, 10)

	rr := doRequest(t, router, http.MethodPost, "/me/children/"+sid+"/sick-note", token,
		map[string]any{"dates": []string{futureISO(3)}, "reason": "Familienfeier", "status": "excused"})
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())

	var env envelope
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &env))
	// The gated path returns a bare (empty) status-day ARRAY, never the
	// {status_days, pending_request} object — the crash an old tab's .map() would hit.
	var days []map[string]any
	require.NoError(t, json.Unmarshal(env.Data, &days),
		"the gated excused path must return a bare status-day array, not an envelope object")
	assert.Empty(t, days, "no status day is written while the request is pending")
	var asObject map[string]any
	assert.Error(t, json.Unmarshal(env.Data, &asObject),
		"data must be a JSON array, never the {status_days} object, for older clients")

	// The child's excused-requests endpoint lists the pending request the submit
	// created (this is how a new client discovers it).
	rr = doRequest(t, router, http.MethodGet, "/me/children/"+sid+"/excused-requests", token, nil)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &env))
	var reqs []map[string]any
	require.NoError(t, json.Unmarshal(env.Data, &reqs))
	require.Len(t, reqs, 1)
	assert.Equal(t, "pending", reqs[0]["status"])
	assert.Equal(t, true, reqs[0]["is_self"], "the calling guardian sees their own request as is_self")
}

// TestSickNoteEndpoint_ExcusedRequiresNote covers AC2 at the HTTP boundary: an
// excused submission with a blank note is rejected.
func TestSickNoteEndpoint_ExcusedRequiresNote(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	router := newWriteRouter(t, db)
	token := parentToken(t, chain.AccountID)
	sid := strconv.FormatInt(chain.StudentID, 10)

	rr := doRequest(t, router, http.MethodPost, "/me/children/"+sid+"/sick-note", token,
		map[string]any{"dates": []string{futureISO(3)}, "reason": "", "status": "excused"})
	assert.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
}

// TestSickNoteEndpoint_DefaultsToSickWhenStatusOmitted pins the status default:
// a client that omits "status" still files a Krankmeldung.
func TestSickNoteEndpoint_DefaultsToSickWhenStatusOmitted(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	router := newWriteRouter(t, db)
	token := parentToken(t, chain.AccountID)
	sid := strconv.FormatInt(chain.StudentID, 10)

	rr := doRequest(t, router, http.MethodPost, "/me/children/"+sid+"/sick-note", token,
		map[string]any{"dates": []string{nowISO()}, "reason": "Fieber"})
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())

	var env envelope
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &env))
	// A client that omits status hits the direct-write path → bare array.
	var days []map[string]any
	require.NoError(t, json.Unmarshal(env.Data, &days))
	require.Len(t, days, 1)
	assert.Equal(t, "sick", days[0]["status"], "an omitted status must default to a Krankmeldung")
}

// TestSickNoteEndpoint_DirectWriteReturnsArray pins the #1845 backward-compat
// contract: a direct write responds with a JSON ARRAY at data, not the
// {status_days, ...} object. A parent tab loaded before this deploy calls .map()
// on the response, so an object here would crash it.
func TestSickNoteEndpoint_DirectWriteReturnsArray(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	router := newWriteRouter(t, db)
	token := parentToken(t, chain.AccountID)
	sid := strconv.FormatInt(chain.StudentID, 10)

	rr := doRequest(t, router, http.MethodPost, "/me/children/"+sid+"/sick-note", token,
		map[string]any{"dates": []string{nowISO()}, "reason": "Fieber"})
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())

	var env envelope
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &env))
	// Decodes into a slice; would error if data were an object.
	var days []map[string]any
	require.NoError(t, json.Unmarshal(env.Data, &days),
		"a direct write must return a bare status-day array, not an envelope object")
	require.Len(t, days, 1)
	// And explicitly NOT the envelope object shape.
	var asObject map[string]any
	assert.Error(t, json.Unmarshal(env.Data, &asObject),
		"data must be a JSON array, never the {status_days} object, for older clients")
}

// TestSickNoteEndpoint_RejectsInvalidStatus covers the ErrInvalidStatus arm of
// renderParentWriteError: a status that is neither sick nor excused is a 400 at
// the API boundary, never silently coerced.
func TestSickNoteEndpoint_RejectsInvalidStatus(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
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
