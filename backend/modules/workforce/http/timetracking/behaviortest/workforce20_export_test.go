package behaviortest

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/moto-ogs/project-phoenix/modules/workforce"
	"github.com/moto-ogs/project-phoenix/modules/workforce/http/absencetypes"
	timetracking "github.com/moto-ogs/project-phoenix/modules/workforce/http/timetracking"
	"github.com/moto-ogs/project-phoenix/modules/workforce/workforcetest"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/uptrace/bun"
)

// External DB tests consume the owner-local test support, as the existing
// root composition seams do. No external test imports an HTTP implementation.
func init() {
	workforcetest.OwnGrowthRouter = newWorkforce20OwnRouter
	workforcetest.OwnGrowthToken = ownToken
	workforcetest.RebookGrowthFixture = newWorkforce20RebookFixture
}

func newWorkforce20OwnRouter(t *testing.T) (*bun.DB, http.Handler) {
	t.Helper()
	testpkg.SetupTestDB(t)
	db, caps := workforcetest.ComposeAdmin(t)
	capability := newWorkforceCapability(t, db)
	resource := timetracking.NewResource(timetracking.Dependencies{WorkSessions: caps.WorkSessions, StaffAbsences: caps.StaffAbsences, AbsenceWrites: caps.AbsenceWrites, Staff: caps.Staff, WorkTimeMonth: caps.WorkTimeMonth, Allowances: capability, AllowanceResponses: func(accounts []workforce.AbsenceTypeAllowanceAccount) any {
		return absencetypes.ToOwnAllowanceResponses(accounts)
	}, Identity: testIdentity, WithinTenant: testWithinTenant(db)})
	router := chi.NewRouter()
	router.Use(testpkg.TenantRuntimeMiddleware(t, db))
	router.Mount("/time-tracking", resource.Router(timetracking.ScopeTestTenantRoutes(testpkg.TenantSessionGatesWithCaches(scopeTestSettingsCache))))
	return db, router
}

func newWorkforce20RebookFixture(t *testing.T, n int) workforcetest.RebookFixture {
	t.Helper()
	testpkg.SetupTestDB(t)
	f := setupRebookFixture(t, 30)
	f.tc.router = testpkg.MountedQueryBudgetRouter("/api", f.tc.router)
	for len(f.absences) < n {
		day := 24 + len(f.absences) - 3
		f.absences = append(f.absences, insertAbsenceRow(t, f.tc, map[string]any{"staff_id": f.subjectID, "absence_type": "comp_time", "date_start": fmt.Sprintf("2026-08-%02d", day), "date_end": fmt.Sprintf("2026-08-%02d", day), "status": "reported", "created_by": f.subjectID, "requested_at": time.Now()}))
	}
	return workforcetest.RebookFixture{DB: f.tc.db, Router: f.tc.router, SubjectID: f.subjectID, TypeID: f.typeID, Absences: f.absences, Rebook: f.rebook, Body: f.body, AugustBalance: f.augustBalance, StoredTypes: f.storedTypes}
}
