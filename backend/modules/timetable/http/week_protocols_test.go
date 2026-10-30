package timetablehttp_test

// Wochenprotokolle (#4769) through the wired router: who sees a protocol,
// what the notes accept, that the PDF exists only for a closed protocol, and
// that every opening is recorded.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil/routetest"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	timetablehttp "github.com/moto-nrw/project-phoenix/modules/timetable/http"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type weekProtocolCaller struct {
	staff  bool
	groups []int64
}

func (c weekProtocolCaller) HasCurrentStaff(context.Context) (bool, error) { return c.staff, nil }
func (c weekProtocolCaller) MyGroupIDs(context.Context) ([]int64, error)   { return c.groups, nil }

type weekProtocolAccessLog struct {
	mu      sync.Mutex
	entries []timetablehttp.WeekProtocolAccess
}

func (l *weekProtocolAccessLog) RecordWeekProtocolAccess(_ context.Context, access timetablehttp.WeekProtocolAccess) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, access)
	return nil
}

type weekProtocolRenderer struct {
	docs []timetable.WeekProtocolReportDocument
}

func (r *weekProtocolRenderer) RenderReport(doc timetable.WeekProtocolReportDocument, filenameBase string) (timetable.WeekProtocolReportFile, error) {
	r.docs = append(r.docs, doc)
	return timetable.WeekProtocolReportFile{Data: []byte("%PDF-1.4"), ContentType: "application/pdf", Filename: filenameBase + ".pdf"}, nil
}

// weekProtocolDayLog answers one group with two missing children.
type weekProtocolDayLog struct{ groupID int64 }

func (d weekProtocolDayLog) GroupDayAttendance(context.Context, calendar.Date, []int64) ([]timetable.WeekProtocolGroupDay, error) {
	return []timetable.WeekProtocolGroupDay{{GroupID: d.groupID, Present: 8, Sick: 1, Absent: 1, Missing: []timetable.WeekProtocolMissingChild{
		{StudentID: 1, Name: "Mia Kurz", Reason: timetable.WeekProtocolAbsenceSick, Label: "Krank"},
		{StudentID: 2, Name: "Ole Lang", Reason: timetable.WeekProtocolAbsenceAbsent, Label: "Fehlt ohne Abmeldung"},
	}}}, nil
}

type noNonWorkingDays struct{}

func (noNonWorkingDays) TenantHolidayDates(context.Context, string, string) (map[string]bool, error) {
	return map[string]bool{}, nil
}

func (noNonWorkingDays) ClosingDayDates(context.Context, string, string) (map[string]bool, error) {
	return map[string]bool{}, nil
}

type weekProtocolRoutes struct {
	tenantID   int64
	protocolID int64
	accessLog  *weekProtocolAccessLog
	renderer   *weekProtocolRenderer
	router     func(weekProtocolCaller) chi.Router
}

const weekProtocolTestGroup = int64(9101)

// setupWeekProtocolRoutes records Friday 2027-09-10 for one group, so its
// week is a draft.
func setupWeekProtocolRoutes(t *testing.T) *weekProtocolRoutes {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	repos, err := owners.NewTimetableTestRepositoriesWithMembers(testpkg.NewDatabase(t, db), owners.MustNewMembershipTestRepositories(testpkg.NewDatabase(t, db)))
	require.NoError(t, err)
	owner, ctx := repos.Timetable, testpkg.Ctx(t)
	friday := calendar.NewDate(2027, 9, 10)
	_, err = owner.RunWeekProtocolEvening(ctx, friday.BerlinMidnight().Add(21*time.Hour), timetable.WeekProtocolSources{
		Notices: compose.NewStaffNotices(compose.StaffNoticeDependencies{DB: db}), Calendar: noNonWorkingDays{},
		Attendance: weekProtocolDayLog{groupID: weekProtocolTestGroup},
		Groups: func(context.Context) ([]timetable.WeekProtocolGroup, error) {
			return []timetable.WeekProtocolGroup{{ID: weekProtocolTestGroup, Name: "Bären"}}, nil
		},
		SchoolMode: func(context.Context) (bool, error) { return false, nil },
		Renderer:   &weekProtocolRenderer{},
	})
	require.NoError(t, err)
	listed, err := owner.ListWeekProtocols(ctx, timetable.WeekProtocolReach{Leadership: true}, timetable.WeekProtocolFilter{})
	require.NoError(t, err)
	require.Len(t, listed, 1)
	routes := &weekProtocolRoutes{tenantID: testpkg.Tenant(t), protocolID: listed[0].ID, accessLog: &weekProtocolAccessLog{}, renderer: &weekProtocolRenderer{}}
	routes.router = func(caller weekProtocolCaller) chi.Router {
		return timetablehttp.NewWeekProtocolResource(timetablehttp.WeekProtocolResourceConfig{
			Service: owner, Caller: caller, AccessLog: routes.accessLog, Renderer: routes.renderer,
			Names: func(_ context.Context, ids []int64) (map[int64]string, error) {
				return map[int64]string{ids[0]: "Lea Leitung"}, nil
			},
		}).Router(api.ProtectedTenantRoutes)
	}
	return routes
}

func (r *weekProtocolRoutes) call(t *testing.T, caller weekProtocolCaller, method, path, body string, granted []string) *httptest.ResponseRecorder {
	t.Helper()
	claims := routetest.DefaultTestClaims()
	claims.ID, claims.TenantID, claims.Roles, claims.IsAdmin = 4242, r.tenantID, []string{"user"}, false
	var reader *strings.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	var req *http.Request
	if reader != nil {
		req = httptest.NewRequest(method, path, reader)
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	return routetest.ExecuteWithAuthPermissions(t, r.router(caller), req, claims, granted)
}

func weekProtocolBody(t *testing.T, rr *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body), rr.Body.String())
	return body.Data
}

var (
	// The staff role "user" holds groups:update; the view-only guest only
	// groups:read (ADR 0091).
	weekProtocolRead       = []string{permissions.GroupsUpdate}
	weekProtocolGuest      = []string{permissions.GroupsRead}
	weekProtocolLeadership = []string{permissions.GroupsManage}
)

func TestWeekProtocolRoutesReach(t *testing.T) {
	t.Parallel()
	routes := setupWeekProtocolRoutes(t)
	path := "/" + strconv.FormatInt(routes.protocolID, 10)

	rr := routes.call(t, weekProtocolCaller{staff: true, groups: []int64{weekProtocolTestGroup}}, http.MethodGet, path, "", nil)
	assert.Equal(t, http.StatusForbidden, rr.Code, "a group permission is required")

	stranger := weekProtocolCaller{staff: true, groups: []int64{1}}
	rr = routes.call(t, stranger, http.MethodGet, path, "", weekProtocolRead)
	assert.Equal(t, http.StatusNotFound, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), "timetable.week_protocol_not_found")
	rr = routes.call(t, stranger, http.MethodGet, "/", "", weekProtocolRead)
	require.Equal(t, http.StatusOK, rr.Code)
	assert.JSONEq(t, `{"data":[]}`, rr.Body.String(), "staff of another group list nothing")

	member := weekProtocolCaller{staff: true, groups: []int64{weekProtocolTestGroup}}
	rr = routes.call(t, member, http.MethodGet, path, "", weekProtocolRead)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	data := weekProtocolBody(t, rr)
	assert.Equal(t, "draft", data["status"])
	assert.Equal(t, "2027-09-06", data["week_start"])
	assert.Equal(t, "2027-09-20", data["auto_close_on"])
	days := data["days"].([]any)
	assert.Len(t, days, 5, "Monday to Friday, no weekend without a record")
	friday := days[4].(map[string]any)
	assert.Equal(t, true, friday["captured"])
	assert.Len(t, friday["missing"], 2)
	assert.Equal(t, false, days[0].(map[string]any)["captured"], "Monday was not recorded")

	// A guest has a staff record and may even supervise the group, but
	// reads no protocol and writes nothing.
	rr = routes.call(t, member, http.MethodGet, "/", "", weekProtocolGuest)
	require.Equal(t, http.StatusOK, rr.Code)
	assert.JSONEq(t, `{"data":[]}`, rr.Body.String(), "a guest lists nothing")
	rr = routes.call(t, member, http.MethodGet, path, "", weekProtocolGuest)
	assert.Equal(t, http.StatusNotFound, rr.Code, "a guest opens nothing")
	rr = routes.call(t, member, http.MethodPut, path+"/notes", `{"week_note":"x"}`, weekProtocolGuest)
	assert.Equal(t, http.StatusForbidden, rr.Code, "a guest writes no notes")
	rr = routes.call(t, member, http.MethodPost, path+"/close", "", weekProtocolGuest)
	assert.Equal(t, http.StatusForbidden, rr.Code, "a guest closes nothing")

	rr = routes.call(t, weekProtocolCaller{}, http.MethodGet, path, "", weekProtocolLeadership)
	assert.Equal(t, http.StatusOK, rr.Code, "the leadership sees every protocol without a group of its own")

	require.Len(t, routes.accessLog.entries, 2, "each opening is recorded")
	assert.Equal(t, int64(4242), routes.accessLog.entries[0].ActorAccountID)
	assert.Equal(t, weekProtocolTestGroup, *routes.accessLog.entries[0].GroupID)
}

func TestWeekProtocolRoutesNotesCloseAndPDF(t *testing.T) {
	t.Parallel()
	routes := setupWeekProtocolRoutes(t)
	member := weekProtocolCaller{staff: true, groups: []int64{weekProtocolTestGroup}}
	path := "/" + strconv.FormatInt(routes.protocolID, 10)

	rr := routes.call(t, member, http.MethodGet, path+"/pdf", "", weekProtocolRead)
	assert.Equal(t, http.StatusConflict, rr.Code, "no PDF before the protocol is closed")
	assert.Contains(t, rr.Body.String(), "timetable.week_protocol_not_closed")

	rr = routes.call(t, member, http.MethodPut, path+"/notes", `{"week_note":"x","days":[{"date":"10.09.2027","note":"y"}]}`, weekProtocolRead)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "timetable.week_protocol_notes_invalid")
	rr = routes.call(t, member, http.MethodPut, path+"/notes", `{"week_note":"x","days":[{"date":"2027-09-13","note":"y"}]}`, weekProtocolRead)
	assert.Equal(t, http.StatusBadRequest, rr.Code, "a day of the following week is refused")

	rr = routes.call(t, member, http.MethodPut, path+"/notes", `{"week_note":"Ruhige Woche","days":[{"date":"2027-09-10","note":"Ausflug in den Park"}]}`, weekProtocolRead)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Equal(t, "Ruhige Woche", weekProtocolBody(t, rr)["week_note"])

	rr = routes.call(t, member, http.MethodPost, path+"/close", "", weekProtocolRead)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	closed := weekProtocolBody(t, rr)
	assert.Equal(t, "closed", closed["status"])
	assert.Equal(t, "Lea Leitung", closed["closed_by_name"])

	rr = routes.call(t, member, http.MethodPut, path+"/notes", `{"week_note":"später"}`, weekProtocolRead)
	assert.Equal(t, http.StatusConflict, rr.Code, "a closed protocol takes no notes")
	assert.Contains(t, rr.Body.String(), "timetable.week_protocol_not_draft")

	require.Len(t, routes.renderer.docs, 1, "closing renders the PDF and stores it")
	for range 2 {
		rr = routes.call(t, member, http.MethodGet, path+"/pdf", "", weekProtocolRead)
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		assert.Equal(t, "application/pdf", rr.Header().Get("Content-Type"))
		assert.Contains(t, rr.Header().Get("Content-Disposition"), "wochenprotokoll-baeren-2027-09-06.pdf")
		assert.Equal(t, "%PDF-1.4", rr.Body.String(), "the stored file")
	}
	require.Len(t, routes.renderer.docs, 1, "a download serves the stored file and renders nothing")
	doc := routes.renderer.docs[0]
	assert.Equal(t, "Wochenprotokoll", doc.Title)
	assert.Equal(t, "Bären · 06.09.2027 bis 12.09.2027", doc.Subtitle)
	require.Len(t, doc.Sections, 6, "Monday to Friday and the week")
	friday := doc.Sections[4]
	assert.Equal(t, "Freitag, 10.09.2027", friday.Title)
	assert.Equal(t, "Fehlende Kinder", friday.Cards[0].Blocks[0].Title)
	assert.Equal(t, fmt.Sprint([]string{"Mia Kurz", "Ole Lang"}), fmt.Sprint([]string{friday.Cards[0].Blocks[0].Fields[0].Label, friday.Cards[0].Blocks[0].Fields[1].Label}))
	assert.Equal(t, "Unentschuldigt", friday.Cards[0].Blocks[0].Fields[1].Value, "the child's line uses the counter's word, not the stored label")
	assert.Equal(t, "Ausflug in den Park", friday.Cards[len(friday.Cards)-1].Fields[0].Value)
	assert.Equal(t, "Ruhige Woche", doc.Sections[5].Cards[0].Fields[0].Value)
}

func TestWeekProtocolLeadershipCanWriteWithoutGroupMembership(t *testing.T) {
	t.Parallel()
	routes := setupWeekProtocolRoutes(t)
	path := "/" + strconv.FormatInt(routes.protocolID, 10)

	rr := routes.call(t, weekProtocolCaller{}, http.MethodPut, path+"/notes", `{"week_note":"Leitungsnotiz"}`, weekProtocolLeadership)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Equal(t, "Leitungsnotiz", weekProtocolBody(t, rr)["week_note"])

	rr = routes.call(t, weekProtocolCaller{}, http.MethodPost, path+"/close", "", weekProtocolLeadership)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	rr = routes.call(t, weekProtocolCaller{}, http.MethodGet, path+"/pdf", "", weekProtocolLeadership)
	assert.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
}
