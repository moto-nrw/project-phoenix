package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	schoolCalendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/compose"
	settingsModule "github.com/moto-nrw/project-phoenix/modules/settings"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	portalCompose "github.com/moto-nrw/project-phoenix/workflows/parentportal/compose"
	parentHTTP "github.com/moto-nrw/project-phoenix/workflows/parentportal/http"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The parent child read contract of ADR 0161 step 4 (#5862), observed over
// HTTP on a real database with two schools and two families. Behaviour
// assertions (status, code, body, refusals, the statement after a failure,
// the discovery reads) hold on the code before the handoff as well; the
// frame-shape test names the one transaction explicitly.

type childReadRig struct {
	db     *bun.DB
	router chi.Router
	// childCheckDelay slows the child lookup of the administrative part.
	childCheckDelay *atomic.Int64
}

func newChildReadRig(t *testing.T) childReadRig {
	t.Helper()
	return newChildReadRigOn(t, testpkg.SetupIsolatedTestDB(t))
}

// newChildReadRigOn builds the rig on db, a pool on the test's isolated clone.
// with adjusts the portal's dependencies before they are composed.
func newChildReadRigOn(t *testing.T, db *bun.DB, with ...func(*portalCompose.Dependencies)) childReadRig {
	t.Helper()
	now := func() time.Time { return childReadToday.BerlinMidnight().Add(10 * time.Hour) }
	database := testpkg.NewDatabase(t, db)
	people := NewPeopleRepositorySuiteFactory(database)
	members := owners.MustNewMembershipTestRepositories(database)
	repos, err := NewParentRouteTestRepositories(database, members)
	require.NoError(t, err)
	settings, err := owners.NewSettingsTestModule(database, testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	profiles, err := NewParentPortalCareProfiles(database)
	require.NoError(t, err)
	sharing, err := NewStudentPeopleRows(database, members)
	require.NoError(t, err)
	feedback, err := NewFeedbackTestModule(database, testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	module, _, _, _, _, _, err := NewStudentTestModule(database, testpkg.TenantRuntime(t, db), feedback.Feedback, now)
	require.NoError(t, err)
	directory := owners.MustNewPeopleDirectory(database)
	enrollment := owners.BuildEnrollmentRows(owners.NewEnrollmentBookings(), repos.CareExceptions, directory)
	catalog := enrollmentCompose.NewCareOfferingRecords(repos.CareExceptions)
	mealSettings, err := owners.BuildMealPlanSettings(settings.Settings)
	require.NoError(t, err)
	schoolCalendar, err := owners.BuildSchoolCalendarRecords(database, func(schoolCalendarCompose.Observation) {}, settings.Settings)
	require.NoError(t, err)
	meals, err := owners.BuildServeMealPlan(database, mealSettings, owners.NewStudentRootReads(people.Student), directory, schoolCalendar)
	require.NoError(t, err)
	childCheckDelay := &atomic.Int64{}
	deps := WithParentEnrollmentBudgetReads(portalCompose.Dependencies{
		GuardianChanges: owners.ParentGuardianChanges(), DeparturePolicy: owners.ParentDeparturePolicy(), GuardianPolicy: owners.NewParentGuardianPolicy(), Fingerprints: owners.SecurityFingerprints{}, Declarations: owners.ParentDeclarationPolicy(), Locales: owners.ParentPortalLocales(), Runtime: owners.ParentPortalRuntime{}, Logger: slog.Default(), Now: now,
		ChildRepo: owners.ParentChildReadsWithDelay(repos.ParentChildReads(), childCheckDelay), StudentRepo: owners.NewParentStudentReads(people.Student), PersonRepo: owners.NewParentPersonReads(people.Person), GuardianProfileRepo: owners.NewParentGuardianProfileReads(people.GuardianProfile), GuardianPhoneRepo: owners.NewParentGuardianPhoneReads(people.GuardianPhoneNumber), StudentGuardianRepo: owners.NewParentStudentGuardianReads(people.StudentGuardian),
		Memberships: owners.NewPeopleIdentityAccess(database, nil),
		Settings:    settings.Settings, CareRequests: module.CareRequests, OfferingChanges: owners.NewParentOfferingChangeRequests(module.OfferingChanges), CareOfferingRepo: owners.NewParentCareOfferingReads(catalog), MealPlan: meals, CareProfiles: profiles, People: owners.MustNewPeopleDirectory(database), CarePlan: owners.NewParentCarePlanCommands(repos.CareExceptions, nil), CareExceptions: owners.NewParentCareExceptions(repos.CareExceptions), CarePlanPolicy: owners.NewParentCarePlanPolicy(), DataRequests: owners.NewParentDataRequestCommands(repos.CareExceptions), ChangeRequestRepo: owners.NewParentDataRequestReads(repos.CareExceptions), ExcusedRequests: owners.NewParentExcusedAbsenceRequests(repos.ExcusedRequests), StudentConsents: owners.NewParentStudentConsents(NewStudentConsents(database)),
		FamilyProtectionEvents: owners.BindParentFamilyProtectionReads(sharing.FamilyProtection), ParentRequestShares: owners.BindParentRequestShareRecords(sharing.ParentRequestShare), ParentRequestEvents: owners.NewParentRequestEvents(sharing.ParentRequestEvent),
	}, enrollment)
	for _, adjust := range with {
		adjust(&deps)
	}
	portal := portalCompose.New(deps)
	enableChildReadSettings(t, settings.Settings, testpkg.Ctx(t))
	router := chi.NewRouter()
	router.Use(testpkg.TenantRuntimeMiddleware(t, db))
	router.Use(RequestSettingsCacheMiddleware, RequestIdentityCacheMiddleware)
	router.Mount("/parent", parentHTTP.NewResource(parentHTTP.ResourceConfig{Parent: portal}).Router(ParentSessionChain))
	return childReadRig{db: db, router: router, childCheckDelay: childCheckDelay}
}

type childReadSettings interface {
	SetValue(ctx context.Context, key string, value any, changedBy *int64, permissions []string) error
}

func enableChildReadSettings(t *testing.T, settings childReadSettings, ctx context.Context) {
	t.Helper()
	require.NoError(t, settings.SetValue(ctx, settingsModule.KeyMealPlanEnabled, true, nil, nil))
	require.NoError(t, settings.SetValue(ctx, settingsModule.KeyMealRegistrationEnabled, true, nil, nil))
	require.NoError(t, settings.SetValue(ctx, "enrollment.offering_changes_enabled", true, nil, nil))
}

// childReadFamily is one family of one school: the account, its child and
// one pending master-data request the request-scoped routes address.
type childReadFamily struct {
	testpkg.ParentChain
	requestID int64
	token     string
}

func (r childReadRig) family(t *testing.T) childReadFamily {
	t.Helper()
	chain := testpkg.CreateTestParentGuardianChain(t, r.db)
	// The offering catalog reads the child's approved enrollment.
	phase := testpkg.CreateTestEnrollmentPhaseAround(t, r.db, childReadToday)
	enrollment := testpkg.CreateTestEnrollmentRequestAwaitingGuardian(t, r.db, phase.ID, chain.Email, fmt.Sprintf("handoff-%d", testpkg.UniqueSuffix()))
	_, err := r.db.NewRaw(`INSERT INTO enrollment.request_children (tenant_id,request_id,first_name,last_name,date_of_birth,status,created_student_id,target_grade_level,target_school_class) VALUES (?,?,'Felix','Handoff','2018-04-15','approved',?,2,'2b')`, chain.TenantID, enrollment.ID, chain.StudentID).Exec(context.Background())
	require.NoError(t, err)
	var requestID int64
	require.NoError(t, r.db.NewRaw(`INSERT INTO users.student_data_change_requests (tenant_id,student_id,submitted_by,target,field_key,new_value,status) VALUES (?,?,?,'person','first_name','"Pending"'::jsonb,'pending') RETURNING id`, chain.TenantID, chain.StudentID, chain.AccountID).Scan(context.Background(), &requestID))
	return childReadFamily{ParentChain: chain, requestID: requestID, token: parentReadToken(t, chain.AccountID)}
}

func parentReadToken(t *testing.T, accountID int64) string {
	t.Helper()
	return testutil.MintTestJWT(t, sessiontoken.AppClaims{ID: int(accountID), Sub: fmt.Sprint(accountID), Roles: []string{"parent"}, Scope: "parent"})
}

// childReadToday is the rig's fixed school day.
var childReadToday = calendar.NewDate(2026, 8, 31)

// childReadRoutes are the 13 read routes of ADR 0161 step 4 for one child and
// one request id.
func childReadRoutes(studentID, requestID int64) map[string]string {
	mealFrom := childReadToday.StartOfISOWeek()
	base := fmt.Sprintf("/parent/me/children/%d", studentID)
	return map[string]string{
		"consents":             base + "/consents",
		"meal_participation":   base + "/meal-participation?from=" + mealFrom.String() + "&to=" + mealFrom.AddDays(4).String(),
		"excused_requests":     base + "/excused-requests",
		"events":               fmt.Sprintf("%s/requests/master_data/%d/events", base, requestID),
		"sharing":              fmt.Sprintf("%s/request-sharing/master_data/%d", base, requestID),
		"sharing_options":      base + "/request-sharing-options",
		"care_exception":       base + "/care-exception?from=2026-08-03&to=2026-08-10",
		"pickup_requests":      base + "/pickup-change-requests",
		"care_offerings":       base + "/care-offerings",
		"offering_catalog":     base + "/care-offerings/catalog",
		"master_data":          base + "/master-data",
		"master_data_requests": base + "/master-data/requests",
		"related_accounts":     base + "/related-accounts",
	}
}

// sickNoteBody is a parent write that checks the relationship first.
var sickNoteBody = map[string]any{"dates": []string{"2026-09-02"}}

// refusedAsRequest are the routes whose missing child is the request's 404.
var refusedAsRequest = map[string]bool{"events": true, "sharing": true, "sharing_options": true}

func (r childReadRig) get(t *testing.T, path, token string) (*httptest.ResponseRecorder, []string) {
	t.Helper()
	counter := testpkg.CaptureQueries(t, r.db)
	req := testutil.NewAuthenticatedRequest(t, http.MethodGet, path, nil, testutil.WithJWTBearer(token))
	rec := testutil.ExecuteRequest(r.router, req)
	queries := counter.Queries()
	counter.Stop()
	return rec, queries
}

// serve answers path without counting statements, for a pool testpkg did
// not open.
func (r childReadRig) serve(t *testing.T, path, token string) *httptest.ResponseRecorder {
	t.Helper()
	return testutil.ExecuteRequest(r.router, testutil.NewAuthenticatedRequest(t, http.MethodGet, path, nil, testutil.WithJWTBearer(token)))
}

// refusal is the wire of an error answer without its instance, which names
// the path and so the student id.
func refusal(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), rec.Body.String())
	delete(body, "instance")
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	return fmt.Sprintf("%d %s %s", rec.Code, rec.Header().Get("Content-Type"), raw)
}

func childReadCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct{ Code string }
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), rec.Body.String())
	return body.Code
}

func isChildAdminRole(q string) bool { return q == "SET LOCAL ROLE phoenix_admin" }

func isChildTenantSwitch(q string) bool {
	return strings.Contains(q, "set_config('app.current_tenant_id'")
}

func childStatementIndex(queries []string, match func(string) bool) int {
	for i, q := range queries {
		if match(q) {
			return i
		}
	}
	return -1
}

func childStatementCount(queries []string, match func(string) bool) int {
	n := 0
	for _, q := range queries {
		if match(q) {
			n++
		}
	}
	return n
}

// discovery is the administrative part: the statements after the admin role
// up to the first COMMIT, ROLLBACK or switch to a school.
func discovery(queries []string) []string {
	start := childStatementIndex(queries, isChildAdminRole)
	if start < 0 {
		return nil
	}
	out := []string{}
	for _, q := range queries[start+1:] {
		if q == "COMMIT" || q == "ROLLBACK" || isChildTenantSwitch(q) {
			break
		}
		out = append(out, q)
	}
	return out
}

// TestParentChildReadsRefuseLikeBefore: every way a parent can miss the
// relationship-level permission gets the answer of an unknown child, per
// route, and reaches no school. Two schools, two families.
func TestParentChildReadsRefuseLikeBefore(t *testing.T) {
	t.Parallel()
	rig := newChildReadRig(t)
	ctx := context.Background()
	familyA := rig.family(t)
	stranger := rig.family(t) // same school, other family
	var familyB childReadFamily
	t.Run("school B", func(t *testing.T) {
		testpkg.EnsureTestTenant(t, rig.db, testpkg.OwnTenant(t))
		familyB = rig.family(t)
	})
	require.NotEqual(t, familyA.TenantID, familyB.TenantID)

	withoutAccess := testpkg.CreateTestCoGuardianForStudent(t, rig.db, familyA.StudentID, "Ohne", "Zugang")
	_, err := rig.db.NewRaw(`UPDATE auth.guardian_student_access SET permissions = permissions - 'parent_portal.access' WHERE account_id = ?`, withoutAccess.AccountID).Exec(ctx)
	require.NoError(t, err)
	membershipEnded := testpkg.CreateTestCoGuardianForStudent(t, rig.db, familyA.StudentID, "Ehemals", "Mitglied")
	_, err = rig.db.NewRaw(`UPDATE auth.account_tenants SET status = 'inactive' WHERE account_id = ? AND tenant_id = ?`, membershipEnded.AccountID, familyA.TenantID).Exec(ctx)
	require.NoError(t, err)
	revoked := testpkg.CreateTestCoGuardianForStudent(t, rig.db, familyA.StudentID, "Ohne", "Rechte")
	_, err = rig.db.NewRaw(`UPDATE auth.guardian_student_access SET permissions = '{}'::jsonb WHERE account_id = ?`, revoked.AccountID).Exec(ctx)
	require.NoError(t, err)

	// The answer for an id that names no child at all, per route: a reserved
	// student id without a row.
	var unknownStudent int64
	require.NoError(t, rig.db.NewRaw("SELECT nextval(pg_get_serial_sequence('users.student_profiles', 'id'))").Scan(ctx, &unknownStudent))
	unrelatedChild := testpkg.CreateTestStudent(t, rig.db, "Ohne", "Familie", "1a").ID
	for name, path := range childReadRoutes(unknownStudent, familyA.requestID) {
		unknown, queries := rig.get(t, path, familyA.token)
		want := http.StatusForbidden
		wantCode := "care.child_not_linked"
		if refusedAsRequest[name] {
			want, wantCode = http.StatusNotFound, "care.request_not_found"
		}
		require.Equal(t, want, unknown.Code, "%s: %s", name, unknown.Body.String())
		require.Equal(t, wantCode, childReadCode(t, unknown), name)
		require.Equal(t, -1, childStatementIndex(queries, isChildTenantSwitch), "%s: an unknown child reaches no school: %q", name, queries)
		unknownWire := refusal(t, unknown)

		for who, attempt := range map[string]struct {
			studentID int64
			token     string
		}{
			"child without guardians":     {unrelatedChild, familyA.token},
			"child of another school":     {familyB.StudentID, familyA.token},
			"parent of another school":    {familyA.StudentID, familyB.token},
			"other family, same school":   {familyA.StudentID, stranger.token},
			"relationship without access": {familyA.StudentID, parentReadToken(t, withoutAccess.AccountID)},
			"membership ended":            {familyA.StudentID, parentReadToken(t, membershipEnded.AccountID)},
			"all permissions revoked":     {familyA.StudentID, parentReadToken(t, revoked.AccountID)},
		} {
			rec, queries := rig.get(t, childReadRoutes(attempt.studentID, familyA.requestID)[name], attempt.token)
			require.Equal(t, unknownWire, refusal(t, rec), "%s / %s", name, who)
			require.Equal(t, -1, childStatementIndex(queries, isChildTenantSwitch), "%s / %s reaches no school: %q", name, who, queries)
		}
	}

	// The catalog asks for parent_portal.request.submit: a relationship with
	// access but without it is the permission refusal, still before any
	// school statement; the other reads stay open to it.
	withoutSubmit := testpkg.CreateTestCoGuardianForStudent(t, rig.db, familyA.StudentID, "Ohne", "Antrag")
	_, err = rig.db.NewRaw(`UPDATE auth.guardian_student_access SET permissions = permissions - 'parent_portal.request.submit' WHERE account_id = ?`, withoutSubmit.AccountID).Exec(ctx)
	require.NoError(t, err)
	routes := childReadRoutes(familyA.StudentID, familyA.requestID)
	rec, queries := rig.get(t, routes["offering_catalog"], parentReadToken(t, withoutSubmit.AccountID))
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	require.Equal(t, "care.guardian_permission_denied", childReadCode(t, rec))
	require.Equal(t, -1, childStatementIndex(queries, isChildTenantSwitch), "%q", queries)
	rec, _ = rig.get(t, routes["consents"], parentReadToken(t, withoutSubmit.AccountID))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

// TestParentChildReadsDiscoverLikeAWrite: the administrative part of every
// read route reads exactly what the relationship check of a parent write
// reads, nothing more; the school part runs for the child's school only and
// answers with that school's rows, for an account with children in two
// schools.
func TestParentChildReadsDiscoverLikeAWrite(t *testing.T) {
	t.Parallel()
	rig := newChildReadRig(t)
	ctx := context.Background()
	family := rig.family(t)
	coGuardian := testpkg.CreateTestCoGuardianForStudent(t, rig.db, family.StudentID, "Mitte", "SchuleA")
	var childB, coGuardianB int64
	var schoolB int64
	t.Run("school B", func(t *testing.T) {
		schoolB = testpkg.OwnTenant(t)
		testpkg.EnsureTestTenant(t, rig.db, schoolB)
		student := testpkg.CreateTestStudent(t, rig.db, "Zweite", "SchuleB", "2b")
		profile := testpkg.CreateTestGuardianProfileForTenant(t, rig.db, schoolB, "Sabine", "SchuleB", "familie-b")
		_, err := rig.db.NewRaw(`UPDATE users.guardian_profiles SET account_id = ?, has_account = true WHERE id = ?`, family.AccountID, profile.ID).Exec(ctx)
		require.NoError(t, err)
		testpkg.CreateTestStudentGuardianLinkForTenant(t, rig.db, schoolB, student.ID, profile.ID, "primary_guardian")
		_, err = rig.db.NewRaw(`INSERT INTO auth.account_tenants (account_id, tenant_id, status, activated_at) VALUES (?, ?, 'active', now()) ON CONFLICT (account_id, tenant_id) DO UPDATE SET status = 'active'`, family.AccountID, schoolB).Exec(ctx)
		require.NoError(t, err)
		coGuardianB = testpkg.CreateTestCoGuardianForStudent(t, rig.db, student.ID, "Mitte", "SchuleB").GuardianProfileID
		childB = student.ID
	})
	require.NotEqual(t, family.TenantID, schoolB)

	// A parent write resolves the child in its own administrative unit.
	write := httptest.NewRecorder()
	counter := testpkg.CaptureQueries(t, rig.db)
	req := testutil.NewAuthenticatedRequest(t, http.MethodPost, fmt.Sprintf("/parent/me/children/%d/sick-note", family.StudentID), sickNoteBody, testutil.WithJWTBearer(family.token))
	rig.router.ServeHTTP(write, req)
	writeQueries := counter.Queries()
	counter.Stop()
	writeDiscovery := discovery(writeQueries)
	require.NotEmpty(t, writeDiscovery, "%q", writeQueries)

	for name, path := range childReadRoutes(family.StudentID, family.requestID) {
		rec, queries := rig.get(t, path, family.token)
		require.Equal(t, http.StatusOK, rec.Code, "%s: %s", name, rec.Body.String())
		require.Equal(t, writeDiscovery, discovery(queries), "%s reads what the relationship check reads", name)
		require.Equal(t, 1, childStatementCount(queries, isChildTenantSwitch), "%s: one school part: %q", name, queries)
		require.Contains(t, queries[childStatementIndex(queries, isChildTenantSwitch)], fmt.Sprintf("'app.current_tenant_id', '%d'", family.TenantID), name)
		require.Less(t, childStatementIndex(queries, isChildAdminRole), childStatementIndex(queries, isChildTenantSwitch), name)
	}

	// The related accounts of each child come from that child's school only.
	for _, tc := range []struct {
		studentID, school, own, foreign int64
	}{{family.StudentID, family.TenantID, coGuardian.GuardianProfileID, coGuardianB}, {childB, schoolB, coGuardianB, coGuardian.GuardianProfileID}} {
		rec, queries := rig.get(t, childReadRoutes(tc.studentID, family.requestID)["related_accounts"], family.token)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		require.Contains(t, queries[childStatementIndex(queries, isChildTenantSwitch)], fmt.Sprintf("'app.current_tenant_id', '%d'", tc.school))
		require.Contains(t, rec.Body.String(), fmt.Sprintf(`"guardian_profile_id":"%d"`, tc.own))
		require.NotContains(t, rec.Body.String(), fmt.Sprintf(`"guardian_profile_id":"%d"`, tc.foreign))
	}
}

// TestParentChildReadsDatabaseFaults injects a real database failure into the
// administrative and into the school part and aborts between them. Every
// point answers like before, and no statement but ROLLBACK follows a failed
// one.
func TestParentChildReadsDatabaseFaults(t *testing.T) {
	t.Parallel()
	rig := newChildReadRig(t)
	ctx := context.Background()
	family := rig.family(t)
	routes := childReadRoutes(family.StudentID, family.requestID)

	for _, fault := range []struct {
		name, route, table, role string
		failing                  func(string) bool
	}{
		{"administrative part", "consents", "auth.guardian_student_access", "phoenix_admin", func(q string) bool { return strings.Contains(q, "guardian_student_access") }},
		{"school part, consents", "consents", "users.student_care_profiles", "phoenix_tenant", func(q string) bool {
			return strings.HasPrefix(q, `SELECT "student".id`)
		}},
		{"school part, related accounts", "related_accounts", "users.guardian_profiles", "phoenix_tenant", func(q string) bool {
			return strings.Contains(q, "guardian_profiles")
		}},
	} {
		t.Run(fault.name, func(t *testing.T) {
			_, err := rig.db.NewRaw("REVOKE SELECT ON " + fault.table + " FROM " + fault.role).Exec(ctx)
			require.NoError(t, err)
			rec, queries := rig.get(t, routes[fault.route], family.token)
			_, err = rig.db.NewRaw("GRANT SELECT ON " + fault.table + " TO " + fault.role).Exec(ctx)
			require.NoError(t, err)

			require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
			require.Equal(t, "general.server", childReadCode(t, rec))
			last := -1
			for i, q := range queries {
				if fault.failing(q) {
					last = i
				}
			}
			require.GreaterOrEqual(t, last, 0, "%q", queries)
			require.Equal(t, []string{"ROLLBACK"}, queries[last+1:], "%q", queries)

			ok, _ := rig.get(t, routes[fault.route], family.token)
			require.Equal(t, http.StatusOK, ok.Code, "grant restored")
		})
	}

	// A check between the child and its school ends the request with its own
	// answer and reaches no school.
	pastWeek := childReadToday.StartOfISOWeek().AddDays(-7)
	rec, queries := rig.get(t, fmt.Sprintf("/parent/me/children/%d/meal-participation?from=%s&to=%s", family.StudentID, pastWeek, pastWeek.AddDays(4)), family.token)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	require.Equal(t, "meals.participation_out_of_range", childReadCode(t, rec))
	require.Equal(t, -1, childStatementIndex(queries, isChildTenantSwitch), "%q", queries)

	_, err := rig.db.NewRaw(`UPDATE users.student_school_memberships SET enrolled_until = ? WHERE student_profile_id = ?`, childReadToday.AddDays(-1), family.StudentID).Exec(ctx)
	require.NoError(t, err)
	rec, queries = rig.get(t, routes["offering_catalog"], family.token)
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	require.Equal(t, "care.child_care_ended", childReadCode(t, rec))
	require.Equal(t, -1, childStatementIndex(queries, isChildTenantSwitch), "%q", queries)
	rec, _ = rig.get(t, routes["consents"], family.token)
	require.Equal(t, http.StatusOK, rec.Code, "reads stay open after the care ended: %s", rec.Body.String())
}

// TestParentChildReadsAreOneTransaction pins the handoff: discovery and the
// school read share one transaction, and the pooled connection keeps neither
// role nor school afterwards, after a commit as after a rollback. Parent
// writes keep their two units.
func TestParentChildReadsAreOneTransaction(t *testing.T) {
	t.Parallel()
	rig := newChildReadRig(t)
	family := rig.family(t)

	for name, path := range childReadRoutes(family.StudentID, family.requestID) {
		rec, queries := rig.get(t, path, family.token)
		require.Equal(t, http.StatusOK, rec.Code, "%s: %s", name, rec.Body.String())
		require.Equal(t, "BEGIN", queries[0], name)
		require.True(t, isChildAdminRole(queries[1]), "%s: %q", name, queries)
		require.Equal(t, 1, childStatementCount(queries, func(q string) bool { return q == "BEGIN" }), "%s: one transaction: %q", name, queries)
		require.Equal(t, 1, childStatementCount(queries, isChildAdminRole), "%s: %q", name, queries)
		require.Equal(t, "COMMIT", queries[len(queries)-1], name)
		require.Equal(t, 1, childStatementCount(queries, func(q string) bool { return q == "COMMIT" }), "%s: %q", name, queries)
	}

	// A refusal between the parts rolls the one transaction back.
	pastWeek := childReadToday.StartOfISOWeek().AddDays(-7)
	refused := fmt.Sprintf("/parent/me/children/%d/meal-participation?from=%s&to=%s", family.StudentID, pastWeek, pastWeek.AddDays(4))
	_, queries := rig.get(t, refused, family.token)
	require.Equal(t, "ROLLBACK", queries[len(queries)-1], "%q", queries)
	require.Equal(t, 1, childStatementCount(queries, func(q string) bool { return q == "BEGIN" }), "%q", queries)

	// A write still commits its relationship check before its school unit.
	counter := testpkg.CaptureQueries(t, rig.db)
	req := testutil.NewAuthenticatedRequest(t, http.MethodPost, fmt.Sprintf("/parent/me/children/%d/sick-note", family.StudentID), sickNoteBody, testutil.WithJWTBearer(family.token))
	rig.router.ServeHTTP(httptest.NewRecorder(), req)
	writeQueries := counter.Queries()
	counter.Stop()
	adminRole := childStatementIndex(writeQueries, isChildAdminRole)
	require.GreaterOrEqual(t, adminRole, 0, "%q", writeQueries)
	end := adminRole + 1 + len(discovery(writeQueries))
	require.Equal(t, "COMMIT", writeQueries[end], "the write's check is its own unit: %q", writeQueries)

	rig.db.SetMaxIdleConns(1)
	rig.db.SetMaxOpenConns(1)
	var loginRole string
	require.NoError(t, rig.db.NewRaw("SELECT current_user").Scan(context.Background(), &loginRole))
	for _, path := range []string{childReadRoutes(family.StudentID, family.requestID)["consents"], refused} {
		rig.get(t, path, family.token)
		var role, school string
		require.NoError(t, rig.db.NewRaw("SELECT current_user, coalesce(current_setting('app.current_tenant_id', true), '')").Scan(context.Background(), &role, &school))
		require.Equal(t, loginRole, role)
		require.Empty(t, school)
	}
}

// TestParentChildReadsFailAFailedStartOnce: a dropped connection at the
// first BEGIN of a read route ends the request with the answer the first unit
// gave before the handoff, the child lookup's own start error. There is no
// second BEGIN: no retry, and nothing swallows the failure.
func TestParentChildReadsFailAFailedStartOnce(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	sqlDB, fault := testpkg.OpenPostgresSQLWithBeginFault(testpkg.IsolatedTestDSN(t))
	db := testpkg.NewBunDB(sqlDB)
	t.Cleanup(func() { _ = db.Close() })
	rig := newChildReadRigOn(t, db)
	family := rig.family(t)

	for name, path := range childReadRoutes(family.StudentID, family.requestID) {
		ok := rig.serve(t, path, family.token)
		require.Equal(t, http.StatusOK, ok.Code, "%s: %s", name, ok.Body.String())

		before := fault.Begins()
		fault.Arm()
		rec := rig.serve(t, path, family.token)
		require.EqualValues(t, 1, fault.Begins()-before, "%s: exactly one BEGIN, the failed one", name)
		require.Equal(t, http.StatusInternalServerError, rec.Code, "%s: %s", name, rec.Body.String())
		require.Equal(t, "general.server", childReadCode(t, rec), name)
	}
}

// TestParentExcusedRequestsStartTheirWindowAfterTheChildCheck: the two-week
// window of decided requests starts when the child check is done, as before
// the handoff. With a slow child check, a request decided just inside the
// window at the time of the request is outside it once the check is done.
// The production code reads the system clock, so the test uses it too.
func TestParentExcusedRequestsStartTheirWindowAfterTheChildCheck(t *testing.T) {
	t.Parallel()
	rig := newChildReadRig(t)
	family := rig.family(t)
	path := childReadRoutes(family.StudentID, family.requestID)["excused_requests"]
	decide := func(at time.Time) string {
		var id int64
		require.NoError(t, rig.db.NewRaw(`INSERT INTO active.excused_absence_requests (tenant_id,student_id,submitted_by,dates,note,absence_status,status,reviewed_at) VALUES (?,?,?,'["2026-08-03"]'::jsonb,'Fenster','excused','rejected',?) RETURNING id`, family.TenantID, family.StudentID, family.AccountID, at).Scan(context.Background(), &id))
		return fmt.Sprintf(`"id":"%d"`, id)
	}
	// The window's start exactly as the production code computes it: two
	// calendar weeks back in the local zone, so a daylight saving change
	// inside the window moves both alike.
	const margin, slow = 2 * time.Second, 4 * time.Second
	windowStart := func() time.Time { return time.Now().AddDate(0, 0, -14) }

	inside := decide(windowStart().Add(margin))
	rec, _ := rig.get(t, path, family.token)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), inside, "inside the window without a delay")

	rig.childCheckDelay.Store(int64(slow))
	t.Cleanup(func() { rig.childCheckDelay.Store(0) })
	leaving := decide(windowStart().Add(margin))
	rec, _ = rig.get(t, path, family.token)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.NotContains(t, rec.Body.String(), leaving, "the window starts after the slow child check")
}
