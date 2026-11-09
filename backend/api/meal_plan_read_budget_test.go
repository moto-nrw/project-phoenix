package api

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Uses the real Serve root, including the point roster (and health decoding),
// tenant/RLS transactions, settings, calendar, permissions and export renderer.
// The family-verification transaction has its own budget; no other SQL is removed.
func checkMealPlanReadQueryBudgets(t *testing.T, api *goldenRuntime) {
	t.Helper()
	t.Run("staff", func(t *testing.T) { checkStaffMealPlanReadBudgets(t, api) })
	t.Run("parent", func(t *testing.T) { checkParentMealPlanReadBudget(t, api) })
}

func captureMealRead(t *testing.T, api *goldenRuntime, counter *testpkg.QueryCounter, token, path string) (*httptest.ResponseRecorder, testpkg.QueryMeasurement) {
	t.Helper()
	counter.Reset()
	response := checkpointRequest(api.queryBudgetHandler(counter), checkpointScenario{Method: http.MethodGet, Path: path, Authenticated: true}, token)
	queries := counter.Queries()
	// Preserve complete diagnostics before content or count assertions.
	diagnostic, _ := json.Marshal(struct {
		Path    string
		Status  int
		Queries []string
	}{path, response.Code, queries})
	t.Log(string(diagnostic))
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	for _, q := range queries {
		upper := strings.ToUpper(strings.TrimSpace(q))
		for _, prefix := range []string{"INSERT ", "UPDATE ", "DELETE ", "MERGE ", "SAVEPOINT ", "ROLLBACK TO "} {
			assert.False(t, strings.HasPrefix(upper, prefix), q)
		}
		for _, fragment := range []string{"PG_ADVISORY", "FOR UPDATE", "NEXTVAL("} {
			assert.NotContains(t, upper, fragment)
		}
	}
	return response, counter.Measurement(withoutSessionCheck(t, queries))
}

func mealReadSnapshot(t *testing.T) []string {
	t.Helper()
	tables := []string{"meal_plan_entries", "meal_orders", "meal_order_days", "meal_order_lines", "meal_order_changes", "meal_order_change_counters", "meal_participation_schedules", "meal_participation_overrides", "meal_registration_changes", "meal_sickness_status_history"}
	rows := make([]string, 0, len(tables))
	for _, table := range tables {
		var data string
		require.NoError(t, testpkg.SetupTestDB(t).NewRaw(`SELECT COALESCE(jsonb_agg(to_jsonb(r) ORDER BY to_jsonb(r)::text), '[]'::jsonb)::text FROM schedule.`+table+` AS r WHERE tenant_id = ?`, testpkg.Tenant(t)).Scan(testpkg.Ctx(t), &data))
		rows = append(rows, table+":"+data)
	}
	return rows
}

func mealExportText(t *testing.T, response *httptest.ResponseRecorder) string {
	t.Helper()
	require.Equal(t, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", response.Header().Get("Content-Type"))
	require.Contains(t, response.Header().Get("Content-Disposition"), "attachment;")
	reader, err := zip.NewReader(bytes.NewReader(response.Body.Bytes()), int64(response.Body.Len()))
	require.NoError(t, err)
	var text strings.Builder
	for _, file := range reader.File {
		if !strings.HasSuffix(file.Name, ".xml") {
			continue
		}
		stream, err := file.Open()
		require.NoError(t, err)
		data, err := io.ReadAll(stream)
		closeErr := stream.Close()
		require.NoError(t, err)
		require.NoError(t, closeErr)
		text.Write(data)
	}
	return text.String()
}

func checkStaffMealPlanReadBudgets(t *testing.T, api *goldenRuntime) {
	t.Helper()
	testpkg.OwnTenant(t)
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	for _, key := range []string{"operations.meal_plan_enabled", "operations.meal_registration_enabled"} {
		testpkg.SetTestTenantSetting(t, db, tenantID, key, "true")
	}
	_, account := testpkg.CreateTestStaffWithAccountForTenant(t, db, tenantID, "Küche", "Budget")
	token := testpkg.MintLiveSessionJWT(t, db, testutil.AdminTestClaimsForTenant(int(account.ID), tenantID))
	counter := testpkg.CaptureQueriesForContext(t, api.db.Pool())
	get := func(path string) (*httptest.ResponseRecorder, testpkg.QueryMeasurement) {
		return captureMealRead(t, api, counter, token, path)
	}
	monday := calendar.NewDate(2030, 3, 4)
	// Frozen order fixture gives the monthly export real, named billed rows.
	// No app clock is replaced. All dates passed to DATE columns are calendar.Date.
	var orderID int64
	require.NoError(t, db.NewRaw(`INSERT INTO schedule.meal_orders (tenant_id,period_start,period_end,deadline_at,frozen_at,format_version) VALUES (?,?,?,?,?,1) RETURNING id`, tenantID, monday, monday, monday.BerlinMidnight(), monday.BerlinMidnight()).Scan(ctx, &orderID))
	_, err := db.ExecContext(ctx, `INSERT INTO schedule.meal_order_days (tenant_id,order_id,date) VALUES (?,?,?)`, tenantID, orderID, monday)
	require.NoError(t, err)
	var studentIDs []int64
	add := func(count int) {
		t.Helper()
		for range count {
			index := len(studentIDs)
			student := testpkg.CreateTestStudent(t, db, fmt.Sprintf("Kind%02d", index), "Essensbudget", "2a")
			studentIDs = append(studentIDs, student.ID)
			_, err := db.ExecContext(ctx, `INSERT INTO schedule.meal_participation_schedules (tenant_id,student_id,effective_from,monday,tuesday,wednesday,thursday,friday,changed_by_account_id) VALUES (?,?,?,true,true,true,true,true,?)`, tenantID, student.ID, monday, account.ID)
			require.NoError(t, err)
			_, err = db.ExecContext(ctx, `INSERT INTO schedule.meal_order_lines (tenant_id,order_id,date,student_id,school_class) VALUES (?,?,?,?,'2a')`, tenantID, orderID, monday, student.ID)
			require.NoError(t, err)
			_, err = db.ExecContext(ctx, `INSERT INTO schedule.meal_plan_entries (tenant_id,date,position,dish) VALUES (?,?,?,?)`, tenantID, monday, index, fmt.Sprintf("Gericht%02d", index))
			require.NoError(t, err)
		}
	}
	recorded := map[string]testpkg.QueryMeasurement{}
	run := func(children int) map[string]int {
		t.Helper()
		before := mealReadSnapshot(t)
		counts := map[string]int{}
		response, queries := get("/api/meal-plan/?week_start=" + monday.String())
		var week struct {
			Data []struct {
				Date      string   `json:"date"`
				Position  int      `json:"position"`
				Dish      string   `json:"dish"`
				Allergens []string `json:"allergens"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &week))
		require.Len(t, week.Data, children)
		for i, row := range week.Data {
			assert.Equal(t, monday.String(), row.Date)
			assert.Equal(t, i, row.Position)
			assert.Equal(t, fmt.Sprintf("Gericht%02d", i), row.Dish)
			assert.Empty(t, row.Allergens)
		}
		counts["api.mealplan.week"] = len(queries.Statements)
		recorded["api.mealplan.week"] = queries
		response, queries = get("/api/meal-plan/participants?date=" + monday.String() + "&include=all")
		var daily struct {
			Data struct {
				Date         string `json:"date"`
				Closed       bool   `json:"closed"`
				Participants []struct {
					ID    string `json:"student_id"`
					First string `json:"first_name"`
					Last  string `json:"last_name"`
					Class string `json:"school_class"`
				} `json:"participants"`
				Others []json.RawMessage `json:"others"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &daily))
		assert.Equal(t, monday.String(), daily.Data.Date)
		assert.False(t, daily.Data.Closed)
		require.Len(t, daily.Data.Participants, children)
		assert.Empty(t, daily.Data.Others)
		for i, row := range daily.Data.Participants {
			assert.Equal(t, strconv.FormatInt(studentIDs[i], 10), row.ID)
			assert.Equal(t, fmt.Sprintf("Kind%02d", i), row.First)
			assert.Equal(t, "Essensbudget", row.Last)
			assert.Equal(t, "2a", row.Class)
		}
		counts["api.mealplan.participants"] = len(queries.Statements)
		recorded["api.mealplan.participants"] = queries
		response, queries = get(fmt.Sprintf("/api/meal-plan/participation/%d?from=%s&to=%s", studentIDs[0], monday, monday.AddDays(4)))
		var participation struct {
			Data struct {
				Weekdays []int `json:"weekdays"`
				Days     []struct {
					Date          string `json:"date"`
					Participating bool   `json:"participating"`
					Regular       bool   `json:"regular_participating"`
					Source        string `json:"source"`
				} `json:"days"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &participation))
		assert.Equal(t, []int{1, 2, 3, 4, 5}, participation.Data.Weekdays)
		require.Len(t, participation.Data.Days, 5)
		for i, day := range participation.Data.Days {
			assert.Equal(t, monday.AddDays(i).String(), day.Date)
			assert.True(t, day.Participating)
			assert.True(t, day.Regular)
			assert.Equal(t, "regular", day.Source)
		}
		counts["api.mealplan.child_participation"] = len(queries.Statements)
		recorded["api.mealplan.child_participation"] = queries
		for _, export := range []struct{ scenario, path string }{
			{"api.mealplan.participants.export", "/api/meal-plan/participants/export?date=" + monday.String() + "&format=xlsx"},
			{"api.mealplan.participants.week_export", "/api/meal-plan/participants/week/export?week_start=" + monday.String() + "&format=xlsx"},
			{"api.mealplan.orders.month_export", "/api/meal-plan/orders/month/export?month=2030-03&format=xlsx"},
		} {
			response, queries = get(export.path)
			text := mealExportText(t, response)
			for i := range children {
				assert.Contains(t, text, fmt.Sprintf("Essensbudget, Kind%02d", i))
			}
			assert.Contains(t, text, "2a")
			counts[export.scenario] = len(queries.Statements)
			recorded[export.scenario] = queries
		}
		for _, days := range []int{1, 5, 8, 20} {
			last := monday
			for weekdays := 1; weekdays < days; {
				last = last.AddDays(1)
				if last.Weekday() >= 1 && last.Weekday() <= 5 {
					weekdays++
				}
			}
			response, queries = get(fmt.Sprintf("/api/meal-plan/orders?from=%s&to=%s", monday, last))
			var orders struct {
				Data struct {
					Days []struct {
						Date    string `json:"date"`
						Total   int    `json:"total"`
						Status  string `json:"status"`
						OrderID string `json:"order_id"`
						Closed  bool   `json:"closed"`
						Classes []struct {
							Class string `json:"school_class"`
							Count int    `json:"count"`
						} `json:"classes"`
						Changes []json.RawMessage `json:"changes"`
					} `json:"days"`
				} `json:"data"`
			}
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &orders))
			require.Len(t, orders.Data.Days, days)
			for i, day := range orders.Data.Days {
				assert.False(t, day.Closed)
				assert.Equal(t, children, day.Total)
				require.Len(t, day.Classes, 1)
				assert.Equal(t, "2a", day.Classes[0].Class)
				assert.Equal(t, children, day.Classes[0].Count)
				assert.Empty(t, day.Changes)
				if i == 0 {
					assert.Equal(t, strconv.FormatInt(orderID, 10), day.OrderID)
				} else {
					assert.Empty(t, day.OrderID)
					assert.Equal(t, "open", day.Status)
				}
			}
			scenario := map[int]string{1: "api.mealplan.orders.dates_1", 5: "api.mealplan.orders.dates_5", 8: "api.mealplan.orders.dates_8", 20: "api.mealplan.orders.dates_20"}[days]
			counts[scenario] = len(queries.Statements)
			recorded[scenario] = queries
		}
		assert.Equal(t, before, mealReadSnapshot(t), "reads preserve every tenant-owned meal row")
		t.Logf("MEAL_MEASURE children=%d counts=%v", children, counts)
		return counts
	}
	add(3)
	small := run(3)
	add(5)
	large := run(8)
	assert.Equal(t, small, large, "children-N is separate from date-N")
	querybudgets.AssertQueryBudget(t, "api.mealplan.week", recorded["api.mealplan.week"])
	querybudgets.AssertQueryBudget(t, "api.mealplan.participants", recorded["api.mealplan.participants"])
	querybudgets.AssertQueryBudget(t, "api.mealplan.child_participation", recorded["api.mealplan.child_participation"])
	querybudgets.AssertQueryBudget(t, "api.mealplan.participants.export", recorded["api.mealplan.participants.export"])
	querybudgets.AssertQueryBudget(t, "api.mealplan.participants.week_export", recorded["api.mealplan.participants.week_export"])
	querybudgets.AssertQueryBudget(t, "api.mealplan.orders.month_export", recorded["api.mealplan.orders.month_export"])
	querybudgets.AssertQueryBudget(t, "api.mealplan.orders.dates_1", recorded["api.mealplan.orders.dates_1"])
	querybudgets.AssertQueryBudget(t, "api.mealplan.orders.dates_5", recorded["api.mealplan.orders.dates_5"])
	querybudgets.AssertQueryBudget(t, "api.mealplan.orders.dates_8", recorded["api.mealplan.orders.dates_8"])
	querybudgets.AssertQueryBudget(t, "api.mealplan.orders.dates_20", recorded["api.mealplan.orders.dates_20"])
}

func checkParentMealPlanReadBudget(t *testing.T, api *goldenRuntime) {
	t.Helper()
	testpkg.OwnTenant(t)
	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	testpkg.SetTestTenantSetting(t, db, testpkg.Tenant(t), "operations.meal_plan_enabled", "true")
	token := testpkg.MintLiveSessionJWT(t, db, testutil.ParentTestClaims(int(chain.AccountID)))
	counter := testpkg.CaptureQueriesForContext(t, api.db.Pool())
	get := func(path string) (*httptest.ResponseRecorder, testpkg.QueryMeasurement) {
		return captureMealRead(t, api, counter, token, path)
	}
	// This real Serve graph keeps the production clock, and the parent route
	// deliberately accepts only the current/next week. Use next week so a run
	// crossing Sunday midnight still requests an allowed week.
	monday := calendar.TodayDate().StartOfISOWeek().AddDays(7)
	counts := []int{}
	var captureRouteQueries testpkg.QueryMeasurement
	for _, size := range []int{3, 8} {
		for i := len(counts) * 3; i < size; i++ {
			_, err := db.ExecContext(testpkg.Ctx(t), `INSERT INTO schedule.meal_plan_entries (tenant_id,date,position,dish,allergens) VALUES (?,?,?,?,ARRAY['gluten'])`, chain.TenantID, monday, i, fmt.Sprintf("Elterngericht%02d", i))
			require.NoError(t, err)
		}
		before := mealReadSnapshot(t)
		response, queries := get(fmt.Sprintf("/parent/me/children/%d/meal-plan?week_start=%s", chain.StudentID, monday))
		var result struct {
			Data []struct {
				Date      string   `json:"date"`
				Position  int      `json:"position"`
				Dish      string   `json:"dish"`
				Allergens []string `json:"allergens"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &result))
		require.Len(t, result.Data, size)
		for i, row := range result.Data {
			assert.Equal(t, monday.String(), row.Date)
			assert.Equal(t, i, row.Position)
			assert.Equal(t, fmt.Sprintf("Elterngericht%02d", i), row.Dish)
			assert.Equal(t, []string{"gluten"}, row.Allergens)
		}
		assert.Equal(t, before, mealReadSnapshot(t))
		counts = append(counts, len(queries.Statements))
		captureRouteQueries = queries
	}
	assert.Equal(t, counts[0], counts[1])
	t.Logf("MEAL_BUDGET parent.mealplan.week %d", counts[1])
	querybudgets.AssertQueryBudget(t, "parent.mealplan.week", captureRouteQueries)
}
