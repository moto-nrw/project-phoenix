package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The tests below exercise the composed /class-list-entries router (#2668):
// the School Membership class-list adapter bound to the shared renderer and
// the JWT identity, over the owner's audited write flows. They keep the
// contract the former api/classlistentries router tests pinned (#2382).

func setupClassListEntriesRoute(t *testing.T) (*testpkg.DB, chi.Router) {
	t.Helper()
	db, module := testutil.SetupClassListModule(t)
	return db, newClassListEntriesResource(module.Membership, db, slog.Default()).Router()
}

func classListEntryClaims(t *testing.T, db *testpkg.DB, prefix string) jwt.AppClaims {
	t.Helper()
	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("%s-%d@test.local", prefix, time.Now().UnixNano()))
	return jwt.AppClaims{ID: int(account.ID), Sub: account.Email, Roles: []string{"admin"}, TenantID: testpkg.Tenant(t)}
}

func TestClassListEntriesCompositionRunsTheCRUDAndAssignFlow(t *testing.T) {
	t.Parallel()
	db, router := setupClassListEntriesRoute(t)
	claims := classListEntryClaims(t, db, "cle-api")
	className := fmt.Sprintf("cle%d", time.Now().UnixNano()%100000)

	// Missing permission → 403 before any data access.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := testutil.ExecuteWithAuthPermissions(t, router, req, claims, []string{"class_day:read"})
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())

	// Create.
	body := fmt.Sprintf(`{"first_name":"Zoe","last_name":"Aalders","school_class":"%s"}`, className)
	req = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec = testutil.ExecuteWithAuthPermissions(t, router, req, claims, []string{"users:create"})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	// IDs travel as JSON strings (lossless beyond 2^53) — `,string` decodes
	// the quoted wire value.
	var created struct {
		Data struct {
			ID int64 `json:"id,string"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	entryID := created.Data.ID
	require.NotZero(t, entryID)

	// Duplicate create → 400 with the German message.
	req = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec = testutil.ExecuteWithAuthPermissions(t, router, req, claims, []string{"users:create"})
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "existiert in dieser Klasse bereits")

	// List (users:read) shows the entry with an empty match hint.
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	rec = testutil.ExecuteWithAuthPermissions(t, router, req, claims, []string{"users:read"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "Aalders")
	assert.Contains(t, rec.Body.String(), `"matching_student_ids":[]`)

	// Move to another class (users:update).
	moveBody := fmt.Sprintf(`{"first_name":"Zoe","last_name":"Aalders","school_class":"%s-b"}`, className)
	req = httptest.NewRequest(http.MethodPut, fmt.Sprintf("/%d", entryID), strings.NewReader(moveBody))
	req.Header.Set("Content-Type", "application/json")
	rec = testutil.ExecuteWithAuthPermissions(t, router, req, claims, []string{"users:update"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), className+"-b")

	// A regular student of the same name and class shows up as the hint.
	student := testpkg.CreateTestStudent(t, db, "Zoe", "Aalders", className+"-b")
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	rec = testutil.ExecuteWithAuthPermissions(t, router, req, claims, []string{"users:read"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), fmt.Sprintf(`"matching_student_ids":["%d"]`, student.ID))

	// Assign to the real student (users:delete) deletes the entry.
	assignBody := fmt.Sprintf(`{"student_id":"%d"}`, student.ID)
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/%d/assign", entryID), strings.NewReader(assignBody))
	req.Header.Set("Content-Type", "application/json")
	rec = testutil.ExecuteWithAuthPermissions(t, router, req, claims, []string{"users:delete"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// Gone after the assign; a repeated delete is 404.
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	rec = testutil.ExecuteWithAuthPermissions(t, router, req, claims, []string{"users:read"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), fmt.Sprintf(`"id":"%d"`, entryID))

	req = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/%d", entryID), nil)
	rec = testutil.ExecuteWithAuthPermissions(t, router, req, claims, []string{"users:delete"})
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "Klassenlisteneintrag nicht gefunden")

	// Entries carry children's names, so every write leaves a trace (#2382).
	// The trail is written by the owner through the shared Audit command, in
	// the same transaction as the change it describes.
	assert.Equal(t, []classListEntryChangeRow{
		{Action: "created", NewValue: "Zoe Aalders (" + className + ")", ChangedBy: int64(claims.ID)},
		{Action: "updated", OldValue: "Zoe Aalders (" + className + ")", NewValue: "Zoe Aalders (" + className + "-b)", ChangedBy: int64(claims.ID)},
		{Action: "assigned", OldValue: "Zoe Aalders (" + className + "-b)", MatchedStudentID: &student.ID, ChangedBy: int64(claims.ID)},
	}, classListEntryTrail(t, db, entryID))
}

// The listing resolves the "Zuordnen" hint per entry (#2382): the entry set is
// a hand-maintained handful per school and the lookup is an exact-name query,
// so one pair of statements per entry is the accepted shape. The budget pins
// that pair — a third statement per entry, or a new constant read inside the
// loop, fails here.
func TestClassListEntriesCompositionKeepsItsQueryBudget(t *testing.T) {
	t.Parallel()
	db, router := setupClassListEntriesRoute(t)
	claims := classListEntryClaims(t, db, "cle-budget")
	className := fmt.Sprintf("cleb%d", time.Now().UnixNano()%100000)

	// Every entry gets a namesake student, so both legs of the hint run: the
	// person lookup and the student read behind it.
	add := func(from, to int) {
		for i := from; i < to; i++ {
			name := fmt.Sprintf("Budget%d", i)
			testpkg.CreateTestClassListEntry(t, db, name, "Kind", className)
			testpkg.CreateTestStudent(t, db, name, "Kind", className)
		}
	}
	add(0, 3)

	counter := testpkg.CaptureQueriesForContext(t, db)
	list := func() []string {
		counter.Reset()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(counter.Context(req.Context()))
		rec := testutil.ExecuteWithAuthPermissions(t, router, req, claims, []string{"users:read"})
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return counter.Operation("SELECT")
	}

	small := list()
	testpkg.AssertQueryBudget(t, "api.class_list_entries.list", small)

	add(3, 6)
	large := list()
	assert.Equal(t, len(small)+6, len(large),
		"three more entries cost three more hint lookups, two statements each, and nothing else")
}

// classListEntryChangeRow is the part of the audit trail this contract pins:
// what changed, into what, and who did it.
type classListEntryChangeRow struct {
	Action           string `bun:"action"`
	OldValue         string `bun:"old_value"`
	NewValue         string `bun:"new_value"`
	MatchedStudentID *int64 `bun:"matched_student_id"`
	ChangedBy        int64  `bun:"changed_by"`
}

func classListEntryTrail(t *testing.T, db *testpkg.DB, entryID int64) []classListEntryChangeRow {
	t.Helper()
	rows := []classListEntryChangeRow{}
	require.NoError(t, db.NewSelect().
		ColumnExpr(`"change".action, "change".old_value, "change".new_value, "change".matched_student_id, "change".changed_by`).
		TableExpr(`audit.class_list_entry_changes AS "change"`).
		Where(`"change".entry_id = ?`, entryID).
		OrderExpr(`"change".id ASC`).
		Scan(testpkg.Ctx(t), &rows))
	return rows
}

// Whitespace-only fields are an invalid request, not a server error: Bind
// trims and rejects them before the flow runs (#2399 review).
func TestClassListEntriesCompositionWhitespaceOnlyIs400(t *testing.T) {
	t.Parallel()
	db, router := setupClassListEntriesRoute(t)
	claims := classListEntryClaims(t, db, "cle-ws")

	body := `{"first_name":"  ","last_name":"Aalders","school_class":"1a"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := testutil.ExecuteWithAuthPermissions(t, router, req, claims, []string{"users:create"})
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "erforderlich")
}

// The listing keeps the class-then-name order of the legacy handler: grades
// sort numerically ("2a" before "10a"), names with German collation.
func TestClassListEntriesCompositionKeepsTheDisplayOrder(t *testing.T) {
	t.Parallel()
	db, router := setupClassListEntriesRoute(t)
	claims := classListEntryClaims(t, db, "cle-order")
	stamp := time.Now().UnixNano() % 100000

	testpkg.CreateTestClassListEntry(t, db, "Zoe", "Zander", fmt.Sprintf("2a%d", stamp))
	testpkg.CreateTestClassListEntry(t, db, "Anna", "Ärger", fmt.Sprintf("2a%d", stamp))
	testpkg.CreateTestClassListEntry(t, db, "Ben", "Berg", fmt.Sprintf("10a%d", stamp))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := testutil.ExecuteWithAuthPermissions(t, router, req, claims, []string{"users:read"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	body := rec.Body.String()
	assert.Less(t, strings.Index(body, "Ärger"), strings.Index(body, "Zander"), "Ä sorts with A in German collation")
	assert.Less(t, strings.Index(body, "Zander"), strings.Index(body, "Berg"), "grade 2 sorts before grade 10")
}
