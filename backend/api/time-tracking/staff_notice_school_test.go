package timetracking

// Tagesinformationen mit Zielgruppe (#2208): die Schul-Route liest als
// Lehrkraft, die OGS-Route als Betreuung, und die Leitung sieht, wer bestätigt
// hat. Geprüft wird durch die verdrahteten Router, nicht am Handler vorbei —
// die Leserart hängt an der Route, und genau das soll hier festgehalten sein.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Die HTTP-Schicht spiegelt die Leserarten des Modells als eigene Konstanten,
// weil sie models/users nicht importieren darf. Driften sie auseinander, liest
// ein Portal plötzlich nichts mehr — deshalb dieser Test.
func TestStaffNoticeReaderKindsMatchModel(t *testing.T) {
	t.Parallel()
	assert.Equal(t, userModels.StaffNoticeAudienceStaff, noticeReaderStaff)
	assert.Equal(t, userModels.StaffNoticeAudienceLehrkraft, noticeReaderLehrkraft)
}

// staffNoticeNamesByAccount stubs the People-Directory port with the persons
// this test created; the real adapter lives in services and is exercised by
// the seeder. The names are what the acknowledgement list shows.
type staffNoticeNamesByAccount map[int64]string

func (n staffNoticeNamesByAccount) ListPersonNamesByAccount(_ context.Context, ids []int64) (map[int64]string, error) {
	out := make(map[int64]string, len(ids))
	for _, id := range ids {
		if name, ok := n[id]; ok {
			out[id] = name
		}
	}
	return out, nil
}

type staffNoticeRouteFixture struct {
	db       *bun.DB
	tenantID int64
	repo     userModels.StaffNoticeRepository
	names    staffNoticeNamesByAccount
	tenant   chi.Router
	school   chi.Router
	admin    jwt.AppClaims
	staff    jwt.AppClaims
	teacher  jwt.AppClaims
}

func setupStaffNoticeRoutes(t *testing.T) *staffNoticeRouteFixture {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	names := staffNoticeNamesByAccount{}

	svc := scheduleSvc.NewStaffNoticeService(scheduleSvc.StaffNoticeServiceConfig{
		Repo:  repos.StaffNotice,
		Names: names,
		// Die Hinweise unten gelten ab 2026-08-01 unbefristet, jeder heutige
		// Tag liegt also im Zeitraum.
		CurrentDate: timezone.TodayDate,
	})
	rs := NewStaffNoticeResource(svc, db)

	suffix := time.Now().UnixNano()
	_, adminAccount := testpkg.CreateTestStaffWithAccountForTenant(t, db, tenantID, "Leitung", fmt.Sprintf("Admin-%d", suffix))
	_, staffAccount := testpkg.CreateTestStaffWithAccountForTenant(t, db, tenantID, "Betreuung", fmt.Sprintf("Kraft-%d", suffix))
	_, teacherAccount := testpkg.CreateTestStaffWithAccountForTenant(t, db, tenantID, "Lehr", fmt.Sprintf("Kraft-%d", suffix))
	testpkg.AssignLehrkraftSystemRole(t, db, teacherAccount.ID, tenantID)
	names[staffAccount.ID] = "Betreuung Kraft"
	names[teacherAccount.ID] = "Lehr Kraft"

	claims := func(accountID int64, email string, roles []string, scope string) jwt.AppClaims {
		return jwt.AppClaims{ID: int(accountID), Sub: email, Roles: roles, TenantID: tenantID, Scope: scope}
	}

	return &staffNoticeRouteFixture{
		db:       db,
		tenantID: tenantID,
		repo:     repos.StaffNotice,
		names:    names,
		tenant:   rs.Router(),
		school:   rs.SchoolRouter(),
		admin:    claims(adminAccount.ID, adminAccount.Email, []string{"admin"}, ""),
		staff:    claims(staffAccount.ID, staffAccount.Email, []string{"user"}, ""),
		teacher:  claims(teacherAccount.ID, teacherAccount.Email, []string{"lehrkraft"}, tenant.ScopeSchool),
	}
}

func (f *staffNoticeRouteFixture) createNotice(t *testing.T, title, audience string, requiresAck bool) *userModels.StaffNotice {
	t.Helper()
	notice := testpkg.NewTestStaffNotice(t, title, timezone.NewDate(2026, 8, 1), int64(f.admin.ID), testpkg.StaffNoticeOpts{
		Audience:                audience,
		RequiresAcknowledgement: requiresAck,
	})
	require.NoError(t, f.repo.Create(testpkg.Ctx(t), notice))
	return notice
}

func (f *staffNoticeRouteFixture) call(t *testing.T, router chi.Router, method, path string, claims jwt.AppClaims, permissions []string) *httptest.ResponseRecorder {
	t.Helper()
	var body *strings.Reader
	if method == http.MethodPost {
		body = strings.NewReader(`{}`)
	} else {
		body = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, body)
	req.Header.Set("Content-Type", "application/json")
	return testutil.ExecuteWithAuthPermissions(t, router, req, claims, permissions)
}

func noticeTitles(t *testing.T, body []byte) []string {
	t.Helper()
	var envelope struct {
		Data []struct {
			Title    string `json:"title"`
			Audience string `json:"audience"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &envelope), string(body))
	titles := make([]string, 0, len(envelope.Data))
	for _, row := range envelope.Data {
		titles = append(titles, row.Title)
	}
	return titles
}

// TestStaffNoticeAudienceFollowsThePortal ist das Akzeptanzkriterium von
// #2208: ein Hinweis "nur Lehrkräfte" erscheint in moto schule und nicht bei
// der Betreuung, ein Hinweis "nur Betreuung" umgekehrt, "alle" bei beiden.
func TestStaffNoticeAudienceFollowsThePortal(t *testing.T) {
	t.Parallel()
	f := setupStaffNoticeRoutes(t)

	forAll := f.createNotice(t, "Räumungsübung", userModels.StaffNoticeAudienceAll, false)
	forStaff := f.createNotice(t, "Turnhalle belegt", userModels.StaffNoticeAudienceStaff, false)
	forTeachers := f.createNotice(t, "Übergabe später", userModels.StaffNoticeAudienceLehrkraft, true)

	rec := f.call(t, f.tenant, http.MethodGet, "/today", f.staff, []string{"users:read"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	staffView := noticeTitles(t, rec.Body.Bytes())
	assert.Contains(t, staffView, forAll.Title)
	assert.Contains(t, staffView, forStaff.Title)
	assert.NotContains(t, staffView, forTeachers.Title, "die Betreuung sieht keinen Hinweis nur für Lehrkräfte")

	rec = f.call(t, f.school, http.MethodGet, "/today", f.teacher, []string{"staff_notices:read"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	teacherView := noticeTitles(t, rec.Body.Bytes())
	assert.Contains(t, teacherView, forAll.Title)
	assert.Contains(t, teacherView, forTeachers.Title)
	assert.NotContains(t, teacherView, forStaff.Title, "eine Lehrkraft sieht keinen Hinweis nur für die Betreuung")

	// Die Leserart hängt an der Route: ein Lehrkräfte-Hinweis ist über die
	// OGS-Route auch dann nicht bestätigbar, wenn man seine Id kennt.
	rec = f.call(t, f.tenant, http.MethodPost, fmt.Sprintf("/%d/acknowledge", forTeachers.ID), f.staff, []string{"users:read"})
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())

	rec = f.call(t, f.school, http.MethodPost, fmt.Sprintf("/%d/acknowledge", forTeachers.ID), f.teacher, []string{"staff_notices:read"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

// TestStaffNoticeSchoolRouteNeedsItsOwnPermission: die Schul-Route hängt an
// staff_notices:read, nicht an users:read — und nimmt nur Schul-Token.
func TestStaffNoticeSchoolRouteNeedsItsOwnPermission(t *testing.T) {
	t.Parallel()
	f := setupStaffNoticeRoutes(t)
	f.createNotice(t, "Für alle", userModels.StaffNoticeAudienceAll, false)

	rec := f.call(t, f.school, http.MethodGet, "/today", f.teacher, []string{"users:read"})
	assert.Equal(t, http.StatusForbidden, rec.Code, "users:read öffnet die Schul-Route nicht")

	rec = f.call(t, f.school, http.MethodGet, "/today", f.teacher, []string{})
	assert.Equal(t, http.StatusForbidden, rec.Code)

	// Ein Tenant-Token (Scope leer) wird an der Schul-Route abgewiesen, ein
	// Schul-Token an der OGS-Route — die Portale bleiben getrennt.
	rec = f.call(t, f.school, http.MethodGet, "/today", f.staff, []string{"staff_notices:read"})
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	rec = f.call(t, f.tenant, http.MethodGet, "/today", f.teacher, []string{"users:read"})
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	// Schreiben gibt es in moto schule nicht: die Verwaltungsrouten sind dort
	// gar nicht eingehängt.
	rec = f.call(t, f.school, http.MethodGet, "/", f.teacher, []string{"staff_notices:read", "admin:*"})
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// TestStaffNoticeAcknowledgementsListNames: die Leitung sieht Namen, nicht nur
// eine Zahl — und nur die Leitung. Zugleich der Query-Budget-Test des
// Listen-Endpunkts (#2940): drei Kenntnisnahmen kosten so viele Abfragen wie
// eine.
func TestStaffNoticeAcknowledgementsListNames(t *testing.T) {
	t.Parallel()
	f := setupStaffNoticeRoutes(t)
	notice := f.createNotice(t, "Bitte bestätigen", userModels.StaffNoticeAudienceAll, true)

	// Betreuung und Lehrkraft bestätigen je über ihr Portal.
	rec := f.call(t, f.tenant, http.MethodPost, fmt.Sprintf("/%d/acknowledge", notice.ID), f.staff, []string{"users:read"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = f.call(t, f.school, http.MethodPost, fmt.Sprintf("/%d/acknowledge", notice.ID), f.teacher, []string{"staff_notices:read"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// Ein drittes Konto ohne Person im Verzeichnis bleibt mit Platzhalter drin.
	ghost := testpkg.CreateTestAccount(t, f.db, fmt.Sprintf("ghost-%d@test.local", time.Now().UnixNano()))
	require.NoError(t, f.repo.Acknowledge(testpkg.Ctx(t), notice.ID, ghost.ID))

	path := fmt.Sprintf("/%d/acknowledgements", notice.ID)

	rec = f.call(t, f.tenant, http.MethodGet, path, f.staff, []string{"users:read"})
	assert.Equal(t, http.StatusForbidden, rec.Code, "die Bestätigungsliste ist Leitungssache")

	counter := testpkg.CaptureQueriesForContext(t, f.db)
	req := httptest.NewRequest(http.MethodGet, path, strings.NewReader("")).WithContext(counter.Context(context.Background()))
	rec = testutil.ExecuteWithAuthPermissions(t, f.tenant, req, f.admin, []string{"admin:*"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	testpkg.AssertQueryBudget(t, "api.staff_notices.acknowledgements", counter.Queries())

	var envelope struct {
		Data []struct {
			AccountID      string `json:"account_id"`
			Name           string `json:"name"`
			AcknowledgedAt string `json:"acknowledged_at"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope), rec.Body.String())
	require.Len(t, envelope.Data, 3)

	names := map[string]string{}
	for _, row := range envelope.Data {
		names[row.AccountID] = row.Name
		assert.NotEmpty(t, row.AcknowledgedAt)
	}
	assert.Equal(t, "Betreuung Kraft", names[fmt.Sprint(f.staff.ID)])
	assert.Equal(t, "Lehr Kraft", names[fmt.Sprint(f.teacher.ID)])
	assert.Equal(t, scheduleSvc.StaffNoticeUnknownAcknowledgerName, names[fmt.Sprint(ghost.ID)])

	// Unbekannter Hinweis: 404, keine leere Liste.
	rec = f.call(t, f.tenant, http.MethodGet, "/999999/acknowledgements", f.admin, []string{"admin:*"})
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// TestStaffNoticeAudienceRoundTrip: die Zielgruppe wird gespeichert, geliefert
// und geprüft; leer fällt auf "alle" zurück.
func TestStaffNoticeAudienceRoundTrip(t *testing.T) {
	t.Parallel()
	f := setupStaffNoticeRoutes(t)

	post := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		return testutil.ExecuteWithAuthPermissions(t, f.tenant, req, f.admin, []string{"admin:*"})
	}

	rec := post(`{"title":"Nur Lehrkräfte","valid_from":"2026-08-01","audience":"lehrkraft","weekdays":[]}`)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"audience":"lehrkraft"`)

	rec = post(`{"title":"Ohne Zielgruppe","valid_from":"2026-08-01","weekdays":[]}`)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"audience":"all"`)

	rec = post(`{"title":"Falsch","valid_from":"2026-08-01","audience":"eltern","weekdays":[]}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
}
