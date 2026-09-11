package timetracking

// Tagesinformationen mit Zielgruppe (#2208): die Schul-Route liest als
// Lehrkraft, die OGS-Route als Betreuung, und die Leitung sieht, wer bestätigt
// hat. Geprüft wird durch die verdrahteten Router, nicht am Handler vorbei —
// die Leserart hängt an der Route, und genau das soll hier festgehalten sein.
// Hinweise entstehen über die Verwaltungsroute der Leitung, wie im Betrieb.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Zielgruppen und der Platzhaltername sind Wire-Werte des Adapters; der Test
// hält sie als Literale fest, so wie das Frontend sie liest.
const (
	audienceAll       = "all"
	audienceStaff     = "staff"
	audienceLehrkraft = "lehrkraft"
	unknownPersonName = "Unbekannte Person"
)

// noticePrincipal ist ein Konto samt Rolle und Portal-Scope. Die Claims werden
// je Anfrage aus den testutil-Standardclaims abgeleitet; der Adapter-Test
// importiert den Token-Adapter nicht selbst.
type noticePrincipal struct {
	accountID int64
	email     string
	roles     []string
	scope     string
	// name ist der Anzeigename im Personenverzeichnis, leer ohne Person.
	name string
}

type staffNoticeRouteFixture struct {
	db       *bun.DB
	tenantID int64
	tenant   chi.Router
	school   chi.Router
	admin    noticePrincipal
	staff    noticePrincipal
	teacher  noticePrincipal
}

func setupStaffNoticeRoutes(t *testing.T) *staffNoticeRouteFixture {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)

	svc, err := services.NewStaffNoticeTestService(db)
	require.NoError(t, err)
	rs := NewStaffNoticeResource(svc, testIdentity, db)

	suffix := time.Now().UnixNano()
	staffLast := fmt.Sprintf("Kraft-%d", suffix)
	_, adminAccount := testpkg.CreateTestStaffWithAccountForTenant(t, db, tenantID, "Leitung", fmt.Sprintf("Admin-%d", suffix))
	_, staffAccount := testpkg.CreateTestStaffWithAccountForTenant(t, db, tenantID, "Betreuung", staffLast)
	_, teacherAccount := testpkg.CreateTestStaffWithAccountForTenant(t, db, tenantID, "Lehr", staffLast)
	testpkg.AssignLehrkraftSystemRole(t, db, teacherAccount.ID, tenantID)

	return &staffNoticeRouteFixture{
		db:       db,
		tenantID: tenantID,
		tenant:   rs.Router(),
		school:   rs.SchoolRouter(),
		admin:    noticePrincipal{accountID: adminAccount.ID, email: adminAccount.Email, roles: []string{"admin"}},
		staff:    noticePrincipal{accountID: staffAccount.ID, email: staffAccount.Email, roles: []string{"user"}, name: "Betreuung " + staffLast},
		teacher:  noticePrincipal{accountID: teacherAccount.ID, email: teacherAccount.Email, roles: []string{"lehrkraft"}, scope: tenant.ScopeSchool, name: "Lehr " + staffLast},
	}
}

// execute signiert die Anfrage für das Konto mit genau diesen Rechten und
// schickt sie durch den Router samt Produktions-Middleware.
func (f *staffNoticeRouteFixture) execute(t *testing.T, router chi.Router, req *http.Request, who noticePrincipal, permissions []string) *httptest.ResponseRecorder {
	t.Helper()
	claims := testutil.DefaultTestClaims()
	claims.ID = int(who.accountID)
	claims.Sub = who.email
	claims.Username = who.email
	claims.FirstName = ""
	claims.LastName = ""
	claims.Roles = who.roles
	claims.IsAdmin = false
	claims.TenantID = f.tenantID
	claims.Scope = who.scope
	return testutil.ExecuteWithAuthPermissions(t, router, req, claims, permissions)
}

func (f *staffNoticeRouteFixture) call(t *testing.T, router chi.Router, method, path string, who noticePrincipal, permissions []string) *httptest.ResponseRecorder {
	t.Helper()
	var body *strings.Reader
	if method == http.MethodPost {
		body = strings.NewReader(`{}`)
	} else {
		body = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, body)
	req.Header.Set("Content-Type", "application/json")
	return f.execute(t, router, req, who, permissions)
}

// createNotice legt einen Hinweis über die Verwaltungsroute an, gültig ab
// 2026-08-01 und unbefristet: jeder heutige Tag liegt im Zeitraum.
func (f *staffNoticeRouteFixture) createNotice(t *testing.T, title, audience string, requiresAck bool) int64 {
	t.Helper()
	body := fmt.Sprintf(`{"title":%q,"valid_from":"2026-08-01","audience":%q,"requires_acknowledgement":%t,"weekdays":[]}`, title, audience, requiresAck)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := f.execute(t, f.tenant, req, f.admin, []string{"admin:*"})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	var envelope struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope), rec.Body.String())
	id, err := strconv.ParseInt(envelope.Data.ID, 10, 64)
	require.NoError(t, err, rec.Body.String())
	return id
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

	const forAll, forStaff, forTeachers = "Räumungsübung", "Turnhalle belegt", "Übergabe später"
	f.createNotice(t, forAll, audienceAll, false)
	f.createNotice(t, forStaff, audienceStaff, false)
	forTeachersID := f.createNotice(t, forTeachers, audienceLehrkraft, true)

	rec := f.call(t, f.tenant, http.MethodGet, "/today", f.staff, []string{"users:read"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	staffView := noticeTitles(t, rec.Body.Bytes())
	assert.Contains(t, staffView, forAll)
	assert.Contains(t, staffView, forStaff)
	assert.NotContains(t, staffView, forTeachers, "die Betreuung sieht keinen Hinweis nur für Lehrkräfte")

	rec = f.call(t, f.school, http.MethodGet, "/today", f.teacher, []string{"staff_notices:read"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	teacherView := noticeTitles(t, rec.Body.Bytes())
	assert.Contains(t, teacherView, forAll)
	assert.Contains(t, teacherView, forTeachers)
	assert.NotContains(t, teacherView, forStaff, "eine Lehrkraft sieht keinen Hinweis nur für die Betreuung")

	// Die Leserart hängt an der Route: ein Lehrkräfte-Hinweis ist über die
	// OGS-Route auch dann nicht bestätigbar, wenn man seine Id kennt.
	rec = f.call(t, f.tenant, http.MethodPost, fmt.Sprintf("/%d/acknowledge", forTeachersID), f.staff, []string{"users:read"})
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())

	rec = f.call(t, f.school, http.MethodPost, fmt.Sprintf("/%d/acknowledge", forTeachersID), f.teacher, []string{"staff_notices:read"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

// TestStaffNoticeSchoolRouteNeedsItsOwnPermission: die Schul-Route hängt an
// staff_notices:read, nicht an users:read — und nimmt nur Schul-Token.
func TestStaffNoticeSchoolRouteNeedsItsOwnPermission(t *testing.T) {
	t.Parallel()
	f := setupStaffNoticeRoutes(t)
	f.createNotice(t, "Für alle", audienceAll, false)

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
	noticeID := f.createNotice(t, "Bitte bestätigen", audienceAll, true)
	acknowledgePath := fmt.Sprintf("/%d/acknowledge", noticeID)

	// Betreuung und Lehrkraft bestätigen je über ihr Portal.
	rec := f.call(t, f.tenant, http.MethodPost, acknowledgePath, f.staff, []string{"users:read"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = f.call(t, f.school, http.MethodPost, acknowledgePath, f.teacher, []string{"staff_notices:read"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// Ein drittes Konto ohne Person im Verzeichnis bestätigt ebenfalls und
	// bleibt mit Platzhalter in der Liste.
	ghostAccount := testpkg.CreateTestAccount(t, f.db, fmt.Sprintf("ghost-%d@test.local", time.Now().UnixNano()))
	ghost := noticePrincipal{accountID: ghostAccount.ID, email: ghostAccount.Email, roles: []string{"user"}}
	rec = f.call(t, f.tenant, http.MethodPost, acknowledgePath, ghost, []string{"users:read"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	path := fmt.Sprintf("/%d/acknowledgements", noticeID)

	rec = f.call(t, f.tenant, http.MethodGet, path, f.staff, []string{"users:read"})
	assert.Equal(t, http.StatusForbidden, rec.Code, "die Bestätigungsliste ist Leitungssache")

	counter := testpkg.CaptureQueriesForContext(t, f.db)
	req := httptest.NewRequest(http.MethodGet, path, strings.NewReader("")).WithContext(counter.Context(context.Background()))
	rec = f.execute(t, f.tenant, req, f.admin, []string{"admin:*"})
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
	assert.Equal(t, f.staff.name, names[fmt.Sprint(f.staff.accountID)])
	assert.Equal(t, f.teacher.name, names[fmt.Sprint(f.teacher.accountID)])
	assert.Equal(t, unknownPersonName, names[fmt.Sprint(ghost.accountID)])

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
		return f.execute(t, f.tenant, req, f.admin, []string{"admin:*"})
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
