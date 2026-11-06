package timetracking

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/http/absencetypes"
	"github.com/moto-nrw/project-phoenix/modules/workforce/workforcetest"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/uptrace/bun"
)

// Test-only exports keep the DB behavior tests external while reusing the
// existing route composition, signed caller, and rebooking fixture.
var Workforce20OwnToken = ownToken
var Workforce20DecodeRebook = decodeRebook

func NewWorkforce20OwnRouter(t *testing.T) (*bun.DB, http.Handler) {
	t.Helper()
	testpkg.SetupTestDB(t)
	db, caps := workforcetest.ComposeAdmin(t)
	capability := newWorkforceCapability(t, db)
	resource := NewResource(Dependencies{WorkSessions: caps.WorkSessions, StaffAbsences: caps.StaffAbsences, AbsenceWrites: caps.AbsenceWrites, Staff: caps.Staff, WorkTimeMonth: caps.WorkTimeMonth, Allowances: capability, AllowanceResponses: func(accounts []workforce.AbsenceTypeAllowanceAccount) any {
		return absencetypes.ToOwnAllowanceResponses(accounts)
	}, Identity: testIdentity, WithinTenant: testWithinTenant(db)})
	router := chi.NewRouter()
	router.Use(testpkg.TenantRuntimeMiddleware(t, db))
	router.Mount("/time-tracking", resource.Router(scopeTestTenantRoutes(testpkg.TenantSessionGatesWithCaches(scopeTestSettingsCache))))
	return db, router
}

type Workforce20RebookFixture struct {
	DB                *bun.DB
	Router            http.Handler
	SubjectID, TypeID int64
	Absences          []int64
	Rebook            func(*testing.T, map[string]any) *httptest.ResponseRecorder
	Body              func(string, bool) map[string]any
	AugustBalance     func(*testing.T) int
	StoredTypes       func(*testing.T) []string
}

func NewWorkforce20RebookFixture(t *testing.T, n int) Workforce20RebookFixture {
	t.Helper()
	testpkg.SetupTestDB(t)
	f := setupRebookFixture(t, 30)
	for len(f.absences) < n {
		day := 24 + len(f.absences) - 3
		f.absences = append(f.absences, insertAbsenceRow(t, f.tc, map[string]any{"staff_id": f.subjectID, "absence_type": "comp_time", "date_start": fmt.Sprintf("2026-08-%02d", day), "date_end": fmt.Sprintf("2026-08-%02d", day), "status": "reported", "created_by": f.subjectID, "requested_at": time.Now()}))
	}
	return Workforce20RebookFixture{DB: f.tc.db, Router: f.tc.router, SubjectID: f.subjectID, TypeID: f.typeID, Absences: f.absences, Rebook: f.rebook, Body: f.body, AugustBalance: f.augustBalance, StoredTypes: f.storedTypes}
}
