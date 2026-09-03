package classday_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestClassListEntriesAppearInClassDay pins the class-day merge of
// class-list-only entries (#2382): a list-only entry appears on the sheet,
// flagged and never staying. It lives with the class-day adapter since the
// entries moved to the School Membership owner (#2668).
func TestClassListEntriesAppearInClassDay(t *testing.T) {
	t.Parallel()
	db, resource := setupClassDayRoute(t)

	staff, account := testpkg.CreateTestStaffWithAccount(t, db, "CleDay", fmt.Sprintf("API-%d", time.Now().UnixNano()))
	className := fmt.Sprintf("cled%d", time.Now().UnixNano()%100000)
	assignment := testpkg.CreateTestClassTeacher(t, db, staff.ID, className)
	_ = testpkg.CreateTestStudent(t, db, "Klara", "Klassentag", className)
	entry := testpkg.CreateTestClassListEntry(t, db, "Nico", "NurListe", className)
	t.Cleanup(func() {
		tenantCtx := testpkg.Ctx(t)
		_, _ = db.NewDelete().TableExpr("education.class_teachers").Where("id = ?", assignment.ID).Exec(tenantCtx)
	})

	// Since the cutover (#2207 PR 3) the class-day sheet is reachable only
	// through the school portal, so this drives SchoolRouter with school-scope
	// claims.
	router := resource.SchoolRouter()
	claims := jwt.AppClaims{ID: int(account.ID), Sub: account.Email, Roles: []string{"lehrkraft"}, TenantID: testpkg.Tenant(t), Scope: tenant.ScopeSchool}

	req := httptest.NewRequest(http.MethodGet, "/?date=2026-08-05", nil)
	rec := testutil.ExecuteWithAuthPermissions(t, router, req, claims, []string{"class_day:read"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// Both the regular student and the list-only entry are on the sheet; the
	// entry is flagged, never staying, and carries no departure instruction.
	body := rec.Body.String()
	assert.Contains(t, body, "Klassentag")
	assert.Contains(t, body, "NurListe")
	assert.Contains(t, body, fmt.Sprintf(`"list_entry_id":"%d"`, entry.ID))
	assert.Contains(t, body, `"list_entry":true`)
}
