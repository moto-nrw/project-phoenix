// Router-level tests for /api/class-list-entries (#2382): permission gates,
// the CRUD + assign flow, and the class-day merge of list-only entries.
package classlistentries_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/classday"
	"github.com/moto-nrw/project-phoenix/api/classlistentries"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestClassListEntriesAPI(t *testing.T) {
	db, factory := testutil.SetupAPITest(t)

	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("cle-api-%d@test.local", time.Now().UnixNano()))
	t.Cleanup(func() { testpkg.CleanupAuthFixtures(t, db, account.ID) })

	resource := classlistentries.NewResource(factory.ClassListEntries, db, nil)
	router := resource.Router()

	claims := jwt.AppClaims{ID: int(account.ID), Sub: account.Email, Roles: []string{"admin"}, TenantID: 1}
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

	var created struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	entryID := created.Data.ID
	require.NotZero(t, entryID)
	t.Cleanup(func() { testpkg.CleanupClassListEntryFixtures(t, db, entryID) })

	// Duplicate create → 400 with the German message.
	req = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec = testutil.ExecuteWithAuthPermissions(t, router, req, claims, []string{"users:create"})
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "existiert in dieser Klasse bereits")

	// List (users:read) shows the entry.
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	rec = testutil.ExecuteWithAuthPermissions(t, router, req, claims, []string{"users:read"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "Aalders")

	// Move to another class (users:update).
	moveBody := fmt.Sprintf(`{"first_name":"Zoe","last_name":"Aalders","school_class":"%s-b"}`, className)
	req = httptest.NewRequest(http.MethodPut, fmt.Sprintf("/%d", entryID), strings.NewReader(moveBody))
	req.Header.Set("Content-Type", "application/json")
	rec = testutil.ExecuteWithAuthPermissions(t, router, req, claims, []string{"users:update"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), className+"-b")

	// Assign to a real student (users:delete) deletes the entry.
	student := testpkg.CreateTestStudent(t, db, "Zoe", "Aalders", className+"-b")
	t.Cleanup(func() { testpkg.CleanupActivityFixtures(t, db, student.ID) })

	assignBody := fmt.Sprintf(`{"student_id":%d}`, student.ID)
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/%d/assign", entryID), strings.NewReader(assignBody))
	req.Header.Set("Content-Type", "application/json")
	rec = testutil.ExecuteWithAuthPermissions(t, router, req, claims, []string{"users:delete"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// Gone after the assign.
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	rec = testutil.ExecuteWithAuthPermissions(t, router, req, claims, []string{"users:read"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), fmt.Sprintf(`"id":%d`, entryID))
}

func TestClassListEntriesAppearInClassDay(t *testing.T) {
	db, factory := testutil.SetupAPITest(t)

	staff, account := testpkg.CreateTestStaffWithAccount(t, db, "CleDay", fmt.Sprintf("API-%d", time.Now().UnixNano()))
	className := fmt.Sprintf("cled%d", time.Now().UnixNano()%100000)
	assignment := testpkg.CreateTestClassTeacher(t, db, staff.ID, className)
	student := testpkg.CreateTestStudent(t, db, "Klara", "Klassentag", className)
	entry := testpkg.CreateTestClassListEntry(t, db, "Nico", "NurListe", className)
	t.Cleanup(func() {
		tenantCtx := testpkg.TenantContext(1)
		_, _ = db.NewDelete().TableExpr("education.class_teachers").Where("id = ?", assignment.ID).Exec(tenantCtx)
		testpkg.CleanupActivityFixtures(t, db, student.ID)
		testpkg.CleanupStaffFixtures(t, db, staff.ID)
		testpkg.CleanupAuthFixtures(t, db, account.ID)
	})

	classDayRouter := classday.NewResource(factory.EnrollmentReport, factory.UserContext, db, nil).Router()
	claims := jwt.AppClaims{ID: int(account.ID), Sub: account.Email, Roles: []string{"lehrkraft"}, TenantID: 1}

	req := httptest.NewRequest(http.MethodGet, "/?date=2026-08-05", nil)
	rec := testutil.ExecuteWithAuthPermissions(t, classDayRouter, req, claims, []string{"class_day:read"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// Both the regular student and the list-only entry are on the sheet; the
	// entry is flagged, never staying, and carries no departure instruction.
	body := rec.Body.String()
	assert.Contains(t, body, "Klassentag")
	assert.Contains(t, body, "NurListe")
	assert.Contains(t, body, fmt.Sprintf(`"list_entry_id":%d`, entry.ID))
	assert.Contains(t, body, `"list_entry":true`)
}

// Whitespace-only fields are an invalid request, not a server error: Bind
// trims and rejects them before the service runs (#2399 review).
func TestClassListEntriesWhitespaceOnlyIs400(t *testing.T) {
	db, factory := testutil.SetupAPITest(t)

	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("cle-ws-%d@test.local", time.Now().UnixNano()))
	t.Cleanup(func() { testpkg.CleanupAuthFixtures(t, db, account.ID) })

	resource := classlistentries.NewResource(factory.ClassListEntries, db, nil)
	router := resource.Router()
	claims := jwt.AppClaims{ID: int(account.ID), Sub: account.Email, Roles: []string{"admin"}, TenantID: 1}

	body := `{"first_name":"  ","last_name":"Aalders","school_class":"1a"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := testutil.ExecuteWithAuthPermissions(t, router, req, claims, []string{"users:create"})
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "erforderlich")
}
