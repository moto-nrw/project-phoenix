// Router-level tests for the access log of the Träger-Portal (#4655, ADR
// 0084 section 7, ADR 0111): only entries of the Träger's own schools, the
// filters, the CSV export, the right check, and that every answer is itself
// written to the access log of each school it shows.
package api

import (
	"context"
	"encoding/csv"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// accessLogDay is the fixed Berlin day the fixtures record their entries on.
var accessLogDay = calendar.NewDate(2026, time.October, 20)

func accessLogAt(hour int) time.Time {
	return accessLogDay.BerlinMidnight().Add(time.Duration(hour) * time.Hour)
}

type accessLogEntry struct {
	ID         string    `json:"id"`
	OccurredAt time.Time `json:"occurred_at"`
	SchoolID   int64     `json:"school_id"`
	SchoolName string    `json:"school_name"`
	AccountID  *int64    `json:"account_id"`
	Person     string    `json:"person"`
	Kind       string    `json:"kind"`
	Data       string    `json:"data"`
	DataLabel  string    `json:"data_label"`
}

type accessLogPage struct {
	Entries []accessLogEntry `json:"entries"`
	HasMore bool             `json:"has_more"`
	AsOf    time.Time        `json:"as_of"`
}

type accessLogPerson struct {
	AccountID int64  `json:"account_id"`
	Name      string `json:"name"`
}

func accessLogRequest(t *testing.T, router http.Handler, token, path string, query url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path+"?"+query.Encode(), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func accessLogWindow(extra map[string]string) url.Values {
	query := url.Values{"from": {accessLogDay.String()}, "to": {accessLogDay.String()}}
	for key, value := range extra {
		query.Set(key, value)
	}
	return query
}

func listAccessLog(t *testing.T, router http.Handler, token string, extra map[string]string) accessLogPage {
	t.Helper()
	rec := accessLogRequest(t, router, token, "/traeger/access-log/", accessLogWindow(extra))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[accessLogPage](t, rec.Body.Bytes())
}

// accessLogPersonAt gives the account a person at the school.
func accessLogPersonAt(t *testing.T, db *bun.DB, school, account int64, first, last string) {
	t.Helper()
	_, err := db.ExecContext(context.Background(), `INSERT INTO users.persons (tenant_id, first_name, last_name, account_id) VALUES (?, ?, ?, ?)`,
		school, first, last, account)
	require.NoError(t, err)
}

func recordAccessLogRead(t *testing.T, db *bun.DB, school, account int64, resourceType string, at time.Time) {
	t.Helper()
	_, err := db.ExecContext(context.Background(), `INSERT INTO audit.data_access_log
		(tenant_id, actor_account_id, actor_role, resource_type, range_start, range_end, accessed_at)
		VALUES (?, ?, 'staff', ?, ?, ?, ?)`, school, account, resourceType, at, at, at)
	require.NoError(t, err)
}

func recordAccessLogPersonnelNumberChange(t *testing.T, db *bun.DB, school, account int64, at time.Time) {
	t.Helper()
	staff := testpkg.CreateTestStaffForTenant(t, db, school, "Paula", "Personal")
	_, err := db.ExecContext(context.Background(), `INSERT INTO audit.personnel_number_changes
		(tenant_id, staff_id, changed_by, old_value, new_value, note, occurred_at) VALUES (?, ?, ?, '1', '2', '', ?)`,
		school, staff.ID, account, at)
	require.NoError(t, err)
}

// traegerAccessLogFixture is a Träger with two schools, a foreign Träger
// with one, and a member of the Träger-Büro with the right to read the
// access log.
type traegerAccessLogFixture struct {
	db                           *bun.DB
	router                       http.Handler
	own, foreign                 int64
	first, second, foreignSchool int64
	staff, otherStaff, member    int64
	token                        string
}

func newTraegerAccessLogFixture(t *testing.T, role string) traegerAccessLogFixture {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	unit := testpkg.TenantRuntime(t, db)
	auth, err := NewAuthTestModule(testpkg.NewDatabase(t, db), unit)
	require.NoError(t, err)
	router := traegerPortalRouter(t, db, auth.Auth, unit)
	password := testpkg.NewTestPassword()
	f := traegerAccessLogFixture{db: db, router: router}
	f.own = traegerOrganization(t, db, "Träger Protokoll", true)
	f.foreign = traegerOrganization(t, db, "Träger Fremd", true)
	f.first = testpkg.CreateTestSchoolInOrganization(t, db, f.own, "OGS Am Bach")
	f.second = testpkg.CreateTestSchoolInOrganization(t, db, f.own, "OGS Waldweg")
	f.foreignSchool = testpkg.CreateTestSchoolInOrganization(t, db, f.foreign, "OGS Fremd")
	f.staff = testpkg.CreateTestAccount(t, db, "fachkraft").ID
	f.otherStaff = testpkg.CreateTestAccount(t, db, "leitung").ID
	accessLogPersonAt(t, db, f.first, f.staff, "Frieda", "Fachkraft")
	accessLogPersonAt(t, db, f.second, f.otherStaff, "Lena", "Leitung")
	account := traegerAccount(t, db, password)
	f.member = account.ID
	testpkg.CreateTestTraegerMember(t, db, f.own, account.ID, role)
	code, answer := traegerLogin(t, router, account.Email, password, 0)
	require.Equal(t, http.StatusOK, code)
	f.token = answer.AccessToken
	return f
}

// traegerAccessReads counts the reads of the Träger-Portal the school's
// access log holds for the account.
func traegerAccessReads(t *testing.T, db *bun.DB, school, account int64, resourceType string) int {
	t.Helper()
	var count int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM audit.data_access_log
		WHERE tenant_id = ? AND actor_account_id = ? AND resource_type = ? AND actor_role = 'traeger'
			AND metadata->>'channel' = 'traeger_portal'`, school, account, resourceType).Scan(context.Background(), &count))
	return count
}

func TestTraegerAccessLogShowsOnlyTheOwnSchoolsAndIsItselfLogged(t *testing.T) {
	t.Parallel()
	f := newTraegerAccessLogFixture(t, identityaccess.TraegerRoleAdministration)
	recordAccessLogRead(t, f.db, f.first, f.staff, "attendance_history", accessLogAt(9))
	recordAccessLogPersonnelNumberChange(t, f.db, f.second, f.otherStaff, accessLogAt(10))
	recordAccessLogRead(t, f.db, f.foreignSchool, f.staff, "attendance_history", accessLogAt(11))
	recordAccessLogRead(t, f.db, f.first, f.staff, "attendance_history", accessLogDay.AddDays(-1).BerlinMidnight().Add(12*time.Hour))

	page := listAccessLog(t, f.router, f.token, nil)
	require.Len(t, page.Entries, 2, "the day's entries of the Träger's schools only: %+v", page.Entries)
	assert.False(t, page.HasMore)
	change, read := page.Entries[0], page.Entries[1]
	assert.Equal(t, f.second, change.SchoolID, "newest first")
	assert.Equal(t, "OGS Waldweg", change.SchoolName)
	assert.Equal(t, "Lena Leitung", change.Person, "the person of the account at that school")
	assert.Equal(t, "change", change.Kind)
	assert.Equal(t, "Personalnummer einer Fachkraft", change.DataLabel)
	assert.Equal(t, f.first, read.SchoolID)
	assert.Equal(t, "Frieda Fachkraft", read.Person)
	assert.Equal(t, "read", read.Kind)
	assert.Equal(t, "attendance_history", read.Data)
	assert.Equal(t, "Anwesenheit eines Kindes", read.DataLabel)

	assert.Equal(t, 1, traegerAccessReads(t, f.db, f.first, f.member, "traeger_access_log"), "the answer is in the access log of every school it shows")
	assert.Equal(t, 1, traegerAccessReads(t, f.db, f.second, f.member, "traeger_access_log"))
	assert.Equal(t, 0, traegerAccessReads(t, f.db, f.foreignSchool, f.member, "traeger_access_log"), "a school the answer does not show is not written")
}

func TestTraegerAccessLogFiltersByPersonSchoolPeriodAndKind(t *testing.T) {
	t.Parallel()
	f := newTraegerAccessLogFixture(t, identityaccess.TraegerRoleAdministration)
	recordAccessLogRead(t, f.db, f.first, f.staff, "class_day_view", accessLogAt(9))
	recordAccessLogRead(t, f.db, f.second, f.otherStaff, "staff_financial_view", accessLogAt(10))
	recordAccessLogPersonnelNumberChange(t, f.db, f.second, f.otherStaff, accessLogAt(11))

	page := listAccessLog(t, f.router, f.token, map[string]string{"school_id": fmt.Sprint(f.first)})
	require.Len(t, page.Entries, 1)
	assert.Equal(t, f.first, page.Entries[0].SchoolID)

	page = listAccessLog(t, f.router, f.token, map[string]string{"account_id": fmt.Sprint(f.otherStaff)})
	require.Len(t, page.Entries, 2)
	for _, entry := range page.Entries {
		require.NotNil(t, entry.AccountID)
		assert.Equal(t, f.otherStaff, *entry.AccountID)
	}

	page = listAccessLog(t, f.router, f.token, map[string]string{"kind": "change"})
	require.Len(t, page.Entries, 1)
	assert.Equal(t, "staff_personnel_number", page.Entries[0].Data)

	next := accessLogDay.AddDays(1).String()
	page = listAccessLog(t, f.router, f.token, map[string]string{"from": next, "to": next})
	assert.Empty(t, page.Entries, "a later day holds none of the entries")

	page = listAccessLog(t, f.router, f.token, map[string]string{"school_id": fmt.Sprint(f.foreignSchool)})
	assert.Empty(t, page.Entries, "a school of another Träger reads nothing")

	rec := accessLogRequest(t, f.router, f.token, "/traeger/access-log/people", accessLogWindow(nil))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	people := decodeSchoolPortalData[[]accessLogPerson](t, rec.Body.Bytes())
	assert.Equal(t, []accessLogPerson{{AccountID: f.staff, Name: "Frieda Fachkraft"}, {AccountID: f.otherStaff, Name: "Lena Leitung"}}, people)
}

func TestTraegerAccessLogRejectsAnInvalidWindow(t *testing.T) {
	t.Parallel()
	f := newTraegerAccessLogFixture(t, identityaccess.TraegerRoleAdministration)
	for name, query := range map[string]url.Values{
		"missing window":  {},
		"reversed window": {"from": {accessLogDay.String()}, "to": {accessLogDay.AddDays(-1).String()}},
		"over a year":     {"from": {accessLogDay.AddDays(-366).String()}, "to": {accessLogDay.String()}},
		"unknown kind":    accessLogWindow(map[string]string{"kind": "deleted"}),
	} {
		t.Run(name, func(t *testing.T) {
			rec := accessLogRequest(t, f.router, f.token, "/traeger/access-log/", query)
			assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
		})
	}
}

func TestTraegerAccessLogRequiresTheAccessLogRight(t *testing.T) {
	t.Parallel()
	for _, role := range []string{identityaccess.TraegerRoleReadOnly, identityaccess.TraegerRoleBilling, identityaccess.TraegerRolePersonnel} {
		t.Run(role, func(t *testing.T) {
			f := newTraegerAccessLogFixture(t, role)
			recordAccessLogRead(t, f.db, f.first, f.staff, "attendance_history", accessLogAt(9))
			for _, path := range []string{"/traeger/access-log/", "/traeger/access-log/people", "/traeger/access-log/export"} {
				rec := accessLogRequest(t, f.router, f.token, path, accessLogWindow(nil))
				assert.Equal(t, http.StatusForbidden, rec.Code, "%s %s", role, path)
			}
			assert.Equal(t, 0, traegerAccessReads(t, f.db, f.first, f.member, "traeger_access_log"), "a refused read leaves no trail")
		})
	}
}

func TestTraegerAccessLogExportsCSVAndLogsTheExport(t *testing.T) {
	t.Parallel()
	f := newTraegerAccessLogFixture(t, identityaccess.TraegerRoleAdministration)
	recordAccessLogRead(t, f.db, f.first, f.staff, "student_health_list_export", accessLogAt(9))
	recordAccessLogRead(t, f.db, f.foreignSchool, f.staff, "attendance_history", accessLogAt(10))

	rec := accessLogRequest(t, f.router, f.token, "/traeger/access-log/export", accessLogWindow(nil))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "text/csv; charset=utf-8", rec.Header().Get("Content-Type"))
	assert.Contains(t, rec.Header().Get("Content-Disposition"), "zugriffsprotokoll-2026-10-20-bis-2026-10-20.csv")
	body := rec.Body.String()
	require.True(t, strings.HasPrefix(body, "\xEF\xBB\xBF"), "spreadsheet programs need the BOM")
	reader := csv.NewReader(strings.NewReader(strings.TrimPrefix(body, "\xEF\xBB\xBF")))
	reader.Comma = ';'
	rows, err := reader.ReadAll()
	require.NoError(t, err)
	require.Equal(t, [][]string{
		{"Datum", "Uhrzeit", "Schule", "Person", "Art", "Daten"},
		{"20.10.2026", "09:00:00", "OGS Am Bach", "Frieda Fachkraft", "Gelesen", "Gesundheitsliste (Export)"},
	}, rows, "only the Träger's school, in Berlin time")
	assert.Equal(t, 1, traegerAccessReads(t, f.db, f.first, f.member, "traeger_access_log_export"))
	assert.Equal(t, 0, traegerAccessReads(t, f.db, f.foreignSchool, f.member, "traeger_access_log_export"))
}

func TestTraegerAccessLogQueryBudgetIsFlat(t *testing.T) {
	t.Parallel()
	f := newTraegerAccessLogFixture(t, identityaccess.TraegerRoleAdministration)
	recordAccessLogRead(t, f.db, f.first, f.staff, "attendance_history", accessLogAt(8))
	recordAccessLogPersonnelNumberChange(t, f.db, f.second, f.otherStaff, accessLogAt(9))
	list := func() []string {
		counter := testpkg.CaptureQueriesForContext(t, f.db)
		req := httptest.NewRequest(http.MethodGet, "/traeger/access-log/?"+accessLogWindow(nil).Encode(), nil)
		req.Header.Set("Authorization", "Bearer "+f.token)
		rec := httptest.NewRecorder()
		f.router.ServeHTTP(rec, req.WithContext(counter.Context(req.Context())))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return counter.Selects("")
	}
	first := list()
	for hour := 10; hour < 14; hour++ {
		recordAccessLogRead(t, f.db, f.first, f.staff, "class_day_view", accessLogAt(hour))
		recordAccessLogPersonnelNumberChange(t, f.db, f.second, f.otherStaff, accessLogAt(hour))
	}
	assert.Len(t, list(), len(first), "the access log reads the same statements for any number of entries")
	testpkg.AssertQueryBudget(t, "api.traeger.access_log", first)
}

// The pages of one list end at its as_of instant, so entries recorded after
// the first page, such as the reads the access log itself records, never
// shift an entry onto the next page twice (quorum review of #4932).
func TestTraegerAccessLogPagesEndAtTheirAsOfInstant(t *testing.T) {
	t.Parallel()
	f := newTraegerAccessLogFixture(t, identityaccess.TraegerRoleAdministration)
	start := accessLogAt(6)
	for i := 0; i < 51; i++ {
		recordAccessLogRead(t, f.db, f.first, f.staff, "attendance_history", start.Add(time.Duration(i)*time.Minute))
	}
	asOf := accessLogAt(12)
	recordAccessLogRead(t, f.db, f.first, f.staff, "class_day_view", accessLogAt(13))
	window := map[string]string{"as_of": asOf.Format(time.RFC3339Nano)}

	first := listAccessLog(t, f.router, f.token, window)
	require.Len(t, first.Entries, 50)
	assert.True(t, first.HasMore)
	assert.True(t, first.AsOf.Equal(asOf), "the list answers the instant it reads up to")
	for _, entry := range first.Entries {
		assert.Equal(t, "attendance_history", entry.Data, "nothing after as_of is listed")
	}
	window["page"] = "2"
	second := listAccessLog(t, f.router, f.token, window)
	require.Len(t, second.Entries, 1)
	assert.False(t, second.HasMore)
	seen := map[string]bool{}
	for _, entry := range append(first.Entries, second.Entries...) {
		assert.False(t, seen[entry.ID], "entry %s is listed twice", entry.ID)
		seen[entry.ID] = true
	}
	assert.Len(t, seen, 51)
}
