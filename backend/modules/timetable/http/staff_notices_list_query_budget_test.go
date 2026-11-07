package timetablehttp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil/routetest"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	timetablehttp "github.com/moto-nrw/project-phoenix/modules/timetable/http"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
)

type noticeBudgetRoute struct {
	path, scenario, scope, permission string
	management                        bool
}

var noticeBudgetRoutes = []noticeBudgetRoute{
	{path: "/api/staff-notices/", scenario: "api.staff_notices.list", permission: "admin:*", management: true},
	{path: "/api/staff-notices/today", scenario: "api.staff_notices.today", permission: "users:read"},
	{path: "/school/staff-notices/today", scenario: "school.staff_notices.today", scope: "school", permission: "staff_notices:read"},
}

type noticeBudgetFixture struct {
	db                    *bun.DB
	ctx                   context.Context
	router                chi.Router
	service               timetable.StaffNotices
	author, reader, other int64
}

// The private pool is warmed before entering the fake-clock bubble. The
// route-sized production security chain includes JWT verification, portal and
// permission checks, request caches and the real tenant transaction. Outer
// server session-family liveness is deliberately outside this resource seam.
func newNoticeBudgetFixture(t *testing.T) noticeBudgetFixture {
	t.Helper()
	db := testpkg.SetupClosableTestDB(t)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	author := testpkg.CreateTestAccount(t, db, "notice-budget-author@example.test")
	reader := testpkg.CreateTestAccount(t, db, "notice-budget-reader@example.test")
	other := testpkg.CreateTestAccount(t, db, "notice-budget-other@example.test")
	service := newStaffNoticeService(t, db)
	resource := timetablehttp.NewStaffNoticeResource(service, noticeAccountID)
	router := chi.NewRouter()
	// ExecuteRequestForTest selects the correct clone; this middleware selects
	// its private pool, keeping database/sql channels inside this bubble.
	router.Use(testpkg.TenantRuntimeMiddleware(t, db))
	router.Mount("/api/staff-notices", resource.Router(api.ProtectedTenantRoutes))
	router.Mount("/school/staff-notices", resource.SchoolRouter(api.ProtectedSchoolRoutes))
	return noticeBudgetFixture{db: db, ctx: testpkg.WithTenantRuntime(t, testpkg.Ctx(t), db), router: router,
		service: service, author: author.ID, reader: reader.ID, other: other.ID}
}

func (f noticeBudgetFixture) request(t *testing.T, route noticeBudgetRoute, permission, scope string, authenticated bool) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, route.path, nil)
	if authenticated {
		claims := routetest.DefaultTestClaims()
		claims.ID = int(f.reader)
		claims.TenantID = testpkg.Tenant(t)
		claims.Roles = []string{"user"}
		claims.IsAdmin = false
		claims.Scope = scope
		claims.Permissions = []string{permission}
		routetest.WithJWTBearer(routetest.MintTestJWT(t, claims))(req)
	}
	return req
}

func (f noticeBudgetFixture) create(t *testing.T, ctx context.Context, index int, input timetable.StaffNoticeInput) timetable.StaffNotice {
	t.Helper()
	notice, err := f.service.CreateStaffNotice(ctx, f.author, input)
	require.NoError(t, err)
	// Distinct persisted instants make both order contracts independent of
	// PostgreSQL's clock and of unspecified ordering between equal timestamps.
	created := time.Date(1999, 12, 1, 12, index, 0, 0, time.UTC)
	_, err = f.db.NewUpdate().Table("users.staff_notices").Set("created_at = ?", created).
		Where("tenant_id = ?", testpkg.TenantIDFromContext(ctx)).Where("id = ?", notice.ID).Exec(ctx)
	require.NoError(t, err)
	return notice
}

func noticeBudgetInput(index int) timetable.StaffNoticeInput {
	priority := "info"
	if index%2 == 0 {
		priority = "important"
	}
	return timetable.StaffNoticeInput{Title: fmt.Sprintf("Notice %02d", index), Body: fmt.Sprintf("Body %02d", index),
		Priority: priority, Audience: "all", ValidFrom: "2000-01-01", ValidUntil: "2000-01-01",
		Weekdays: []int16{6}, WeekPattern: 1, RequiresAcknowledgement: true, Active: true}
}

func noticeBudgetDTO(notice timetable.StaffNotice, management bool, ack bool) map[string]any {
	weekdays := make([]any, 0, len(notice.Weekdays))
	for _, weekday := range notice.Weekdays {
		weekdays = append(weekdays, float64(weekday))
	}
	out := map[string]any{"id": strconv.FormatInt(notice.ID, 10), "title": notice.Title, "body": notice.Body,
		"priority": notice.Priority, "audience": notice.Audience, "valid_from": notice.ValidFrom,
		"weekdays": weekdays, "week_pattern": float64(notice.WeekPattern),
		"requires_acknowledgement": notice.RequiresAcknowledgement, "active": notice.Active}
	if notice.ValidUntil != "" {
		out["valid_until"] = notice.ValidUntil
	}
	if ack {
		out["acknowledged_at"] = "2000-01-01T11:30:00+01:00"
	}
	if management {
		count := float64(0)
		if ack {
			count = 2 // reader + other; the automatically acknowledged author is excluded.
		}
		out["acknowledged_count"] = count
	}
	return out
}

func assertNoticeBudgetResponse(t *testing.T, rec *httptest.ResponseRecorder, expected []map[string]any) {
	t.Helper()
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	data := make([]any, 0, len(expected))
	for _, row := range expected {
		data = append(data, row)
	}
	require.Equal(t, map[string]any{"data": data}, body, "exact wire fields, order, dates and acknowledgement visibility")
}

func assertNoticeBudgetReadOnly(t *testing.T, counter *testpkg.QueryCounter) {
	t.Helper()
	require.Zero(t, counter.WriteRows())
	for _, query := range counter.Queries() {
		upper := strings.ToUpper(query)
		for _, forbidden := range []string{"INSERT ", "UPDATE ", "DELETE ", "MERGE ", "FOR UPDATE", "FOR SHARE", "FOR NO KEY UPDATE", "FOR KEY SHARE", "PG_ADVISORY"} {
			require.NotContains(t, upper, forbidden, "list must not write or acquire explicit row/advisory locks")
		}
	}
}

func TestStaffNoticeListRouteQueryBudgets(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	for _, route := range noticeBudgetRoutes {
		t.Run(route.scenario, func(t *testing.T) {
			testpkg.OwnTenant(t)
			f := newNoticeBudgetFixture(t)
			day := calendar.NewDate(2000, 1, 1)
			period := testpkg.CreateTestWeekCyclePeriodForTenant(t, f.db, testpkg.Tenant(t), "Notice cycle", day.AddDays(-7), day.AddDays(14), 2, day)
			_, err := f.db.NewUpdate().Table("schedule.calendar_periods").Set("period_type = ?", "school_year").
				Where("id = ?", period.ID).Where("tenant_id = ?", testpkg.Tenant(t)).Exec(f.ctx)
			require.NoError(t, err)
			foreign := testpkg.NewTenantScope(t, f.db)
			_, foreignAuthor := testpkg.CreateTestStaffWithAccountForTenant(t, f.db, foreign.TenantID, "Foreign", "Notice")
			foreignFixture := f
			foreignFixture.author = foreignAuthor.ID
			foreignFixture.create(t, testpkg.WithTenantRuntime(t, foreign.Context(), f.db), 55, noticeBudgetInput(55))
			synctest.Test(t, func(t *testing.T) {
				counter := testpkg.CaptureQueries(t, f.db)
				counter.Stop()
				notices := make([]timetable.StaffNotice, 0, 8)
				counts := make(map[int]int)
				for _, size := range []int{0, 3, 8} {
					for len(notices) < size {
						i := len(notices)
						notice := f.create(t, f.ctx, i, noticeBudgetInput(i))
						if i%2 == 0 {
							repo := compose.NewStaffNoticeRepository(f.db)
							for _, accountID := range []int64{f.reader, f.other} {
								require.NoError(t, repo.Acknowledge(f.ctx, notice.ID, accountID))
							}
							_, err := f.db.NewUpdate().Table("users.staff_notice_acks").Set("acknowledged_at = ?", time.Date(2000, 1, 1, 10, 30, 0, 0, time.UTC)).
								Where("notice_id = ?", notice.ID).Where("tenant_id = ?", testpkg.Tenant(t)).Exec(f.ctx)
							require.NoError(t, err)
						}
						notices = append(notices, notice)
					}
					req := f.request(t, route, route.permission, route.scope, true)
					counter.Reset()
					counter.Start()
					rec := routetest.ExecuteRequestForTest(t, f.router, req)
					counter.Stop()
					queries := counter.Queries()
					counts[size] = len(queries)
					t.Logf("%s N=%d statements=%d SQL:\n%s", route.path, size, len(queries), strings.Join(queries, "\n"))
					expected := make([]map[string]any, 0, size)
					indices := make([]int, size)
					for i := range size {
						indices[i] = i
					}
					sort.Slice(indices, func(i, j int) bool {
						left, right := indices[i], indices[j]
						if !route.management && left%2 != right%2 {
							return left%2 == 0
						}
						return left > right
					})
					for _, i := range indices {
						expected = append(expected, noticeBudgetDTO(notices[i], route.management, i%2 == 0))
					}
					assertNoticeBudgetResponse(t, rec, expected)
					assertNoticeBudgetReadOnly(t, counter)
					assertNoticeRouteBudget(t, route, size, queries)
				}
				require.Equal(t, counts[3], counts[8], "no statement growth with notices/acknowledgements")
			})
		})
	}
}

// Literal scenario references also let the source-level budget ratchet verify
// that each authored contract is reached by a real assertion.
func assertNoticeRouteBudget(t *testing.T, route noticeBudgetRoute, size int, queries []string) {
	t.Helper()
	switch {
	case route.management && size == 0:
		querybudgets.AssertQueryBudget(t, "api.staff_notices.list.empty", queries)
	case route.management:
		querybudgets.AssertQueryBudget(t, "api.staff_notices.list", queries)
	case route.scope == "school" && size == 0:
		querybudgets.AssertQueryBudget(t, "school.staff_notices.today.empty", queries)
	case route.scope == "school":
		querybudgets.AssertQueryBudget(t, "school.staff_notices.today", queries)
	case size == 0:
		querybudgets.AssertQueryBudget(t, "api.staff_notices.today.empty", queries)
	default:
		querybudgets.AssertQueryBudget(t, "api.staff_notices.today", queries)
	}
}

func TestStaffNoticeListRouteContracts(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	for _, route := range noticeBudgetRoutes {
		t.Run(route.scenario, func(t *testing.T) {
			testpkg.OwnTenant(t)
			f := newNoticeBudgetFixture(t)
			day := calendar.NewDate(2000, 1, 1)
			period := testpkg.CreateTestWeekCyclePeriodForTenant(t, f.db, testpkg.Tenant(t), "Contract cycle", day.AddDays(-7), day.AddDays(14), 2, day)
			_, err := f.db.NewUpdate().Table("schedule.calendar_periods").Set("period_type = ?", "school_year").
				Where("id = ?", period.ID).Where("tenant_id = ?", testpkg.Tenant(t)).Exec(f.ctx)
			require.NoError(t, err)
			notices := make([]timetable.StaffNotice, 0, 10)
			for i := range 10 {
				input := noticeBudgetInput(i)
				input.Priority, input.WeekPattern, input.RequiresAcknowledgement = "info", 0, false
				switch i {
				case 0:
					input.ValidUntil, input.Weekdays = "", nil
				case 1:
					input.Priority, input.RequiresAcknowledgement = "important", true
				case 2:
					input.Priority, input.Audience = "important", "staff"
				case 3:
					input.Priority, input.Audience = "important", "lehrkraft"
				case 4:
					input.Active = false
				case 5:
					input.ValidFrom, input.ValidUntil, input.Weekdays = "2000-01-02", "", nil
				case 6:
					input.ValidFrom, input.ValidUntil = "1999-12-01", "1999-12-31"
				case 7:
					input.Weekdays = []int16{5}
				case 8:
					input.WeekPattern = 2
				case 9:
					input.ValidFrom = "1999-12-01"
				}
				notices = append(notices, f.create(t, f.ctx, i, input))
			}
			synctest.Test(t, func(t *testing.T) {
				counter := testpkg.CaptureQueries(t, f.db)
				counter.Stop()
				indices := []int{2, 1, 9, 0}
				if route.scope == "school" {
					indices[0] = 3
				}
				if route.management {
					indices = []int{9, 8, 7, 6, 5, 4, 3, 2, 1, 0}
				}
				expected := make([]map[string]any, 0, len(indices))
				for _, i := range indices {
					expected = append(expected, noticeBudgetDTO(notices[i], route.management, false))
				}
				req := f.request(t, route, route.permission, route.scope, true)
				// Neither another date nor another audience may widen a portal.
				req.URL.RawQuery = "date=2000-01-02&audience=all&reader=all"
				counter.Reset()
				counter.Start()
				rec := routetest.ExecuteRequestForTest(t, f.router, req)
				counter.Stop()
				assertNoticeBudgetResponse(t, rec, expected)
				assertNoticeBudgetReadOnly(t, counter)
				for _, denied := range []struct {
					name, permission, scope string
					authenticated           bool
					status                  int
				}{
					{name: "anonymous", status: http.StatusUnauthorized},
					{name: "missing permission", scope: route.scope, authenticated: true, status: http.StatusForbidden},
					{name: "other route permission", permission: otherNoticePermission(route), scope: route.scope, authenticated: true, status: http.StatusForbidden},
					{name: "wrong portal", permission: route.permission, scope: oppositeNoticeScope(route.scope), authenticated: true, status: http.StatusUnauthorized},
				} {
					req := f.request(t, route, denied.permission, denied.scope, denied.authenticated)
					counter.Reset()
					counter.Start()
					rec := routetest.ExecuteRequestForTest(t, f.router, req)
					counter.Stop()
					require.Equal(t, denied.status, rec.Code, "%s: %s", denied.name, rec.Body.String())
					require.Zero(t, counter.Total(), "%s must reject before any SQL", denied.name)
					routetest.AssertErrorResponse(t, rec, denied.status)
				}
				req = f.request(t, route, route.permission, route.scope, true)
				cancelled, cancel := context.WithCancel(req.Context())
				cancel()
				counter.Reset()
				counter.Start()
				rec = routetest.ExecuteRequestForTest(t, f.router, req.WithContext(cancelled))
				counter.Stop()
				routetest.AssertErrorResponse(t, rec, http.StatusInternalServerError)
				require.NotContains(t, rec.Body.String(), `"data"`, "a failed request must not return a partial list")
				require.NotContains(t, counter.Queries(), "COMMIT", "cancelled request must not commit")
				assertNoticeBudgetReadOnly(t, counter)
				if !route.management {
					// At 23:30 UTC it is already January 2 in Berlin. The real
					// handler must use that calendar day, not the UTC date or
					// client-supplied date. synctest advances no wall-clock time.
					time.Sleep(23*time.Hour + 30*time.Minute)
					req := f.request(t, route, route.permission, route.scope, true)
					req.URL.RawQuery = "date=2000-01-01"
					counter.Reset()
					counter.Start()
					rec := routetest.ExecuteRequestForTest(t, f.router, req)
					counter.Stop()
					assertNoticeBudgetResponse(t, rec, []map[string]any{
						noticeBudgetDTO(notices[5], false, false), noticeBudgetDTO(notices[0], false, false),
					})
					assertNoticeBudgetReadOnly(t, counter)
				}
			})
		})
	}
}

func otherNoticePermission(route noticeBudgetRoute) string {
	if route.scope == "school" {
		return "users:read"
	}
	if route.management {
		return "users:read"
	}
	return "staff_notices:read"
}

func oppositeNoticeScope(scope string) string {
	if scope == "school" {
		return ""
	}
	return "school"
}
