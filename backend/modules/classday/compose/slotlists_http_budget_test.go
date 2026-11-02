package compose_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	rootAPI "github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/classday"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSlotListOptionsHTTPQueryBudget(t *testing.T) {
	t.Parallel()
	small, large := measureSlotListHTTPQueries(t, "options")
	querybudgets.AssertQueryBudget(t, "api.timetable.slot_lists.options", small)
	querybudgets.AssertQueryBudget(t, "api.timetable.slot_lists.options", large)
}

func TestSlotListPreviewHTTPQueryBudget(t *testing.T) {
	t.Parallel()
	small, large := measureSlotListHTTPQueries(t, "preview")
	querybudgets.AssertQueryBudget(t, "api.timetable.slot_lists.preview", small)
	querybudgets.AssertQueryBudget(t, "api.timetable.slot_lists.preview", large)
}

func TestSlotListExportHTTPQueryBudget(t *testing.T) {
	t.Parallel()
	small, large := measureSlotListHTTPQueries(t, "export")
	querybudgets.AssertQueryBudget(t, "api.timetable.slot_lists.export", small)
	querybudgets.AssertQueryBudget(t, "api.timetable.slot_lists.export", large)
}

// Count the real HTTP adapter, its permission gate, and the existing composed
// projection's tenant transaction. Session verification and registry-backed
// settings have their own scenarios; these fixture bindings are unchanged.
func measureSlotListHTTPQueries(t *testing.T, action string) ([]string, []string) {
	t.Helper()
	db := testpkg.SetupIsolatedTestDB(t)
	f := buildMensaFixtureOn(t, db)
	_, account := testpkg.CreateTestStaffWithAccount(t, db, "Slot", "Budget")
	claims := testpkg.Claims{ID: int(account.ID), Sub: account.Email, Roles: []string{"schulverwaltung"}, TenantID: testpkg.Tenant(t), Scope: tenant.ScopeSchool}
	router := rootAPI.NewSlotListTestRouter(f.svc)
	counter := testpkg.CaptureQueries(t, db)
	measure := func(wantSlots int) []string {
		params := map[string]any{"date": listDate.String()}
		if action != "options" {
			params["target"], params["source"] = "slots", "reconciliation"
		}
		if action == "export" {
			preview, err := f.svc.BuildList(testpkg.Ctx(t), classday.Params{
				Date: classday.Date(listDate.String()), Target: classday.TargetSlots, Source: classday.SourceReconciliation,
			})
			require.NoError(t, err)
			params["format"], params["expected_signature"] = "xlsx", preview.Signature
		}
		body, err := json.Marshal(params)
		require.NoError(t, err)
		request := httptest.NewRequest(http.MethodPost, "/lists/"+action, bytes.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		request = request.WithContext(testpkg.Ctx(t))
		counter.Reset()
		response := testutil.ExecuteWithAuthPermissions(t, router, request, claims, []string{"schedules:read", "users:read"})
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		queries := counter.Queries()
		if action == "export" {
			assert.Equal(t, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", response.Header().Get("Content-Type"))
			assert.True(t, bytes.HasPrefix(response.Body.Bytes(), []byte("PK")), "the adapter must return the rendered workbook")
		} else {
			var result struct {
				Slots []classday.Slot `json:"slots"`
				Rows  []classday.Row  `json:"rows"`
			}
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &result))
			require.Len(t, result.Slots, wantSlots)
			if action == "preview" {
				require.Len(t, result.Rows, 3)
			}
		}
		require.Zero(t, counter.WriteRows())
		for _, operation := range []string{"INSERT", "UPDATE", "DELETE", "MERGE"} {
			require.Empty(t, counter.Operation(operation))
		}
		t.Logf("action=%s slots=%d total=%d", action, wantSlots, len(queries))
		require.Len(t, counter.Selects("facilities.rooms"), 1)
		return queries
	}
	small := measure(1)
	for i := range 7 {
		room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("HTTP-Slot-Room-%d", i))
		instance := &testpkg.ActivityInstance{
			Date: testpkg.ScheduleDate(listDate), Title: room.Name, RoomID: room.ID,
			StartTime: atOn(listDate, 11, 30), EndTime: atOn(listDate, 13, 0), Status: testpkg.ScheduleInstanceStatusPlanned,
		}
		instance.TenantID = testpkg.Tenant(t)
		testpkg.InsertActivityInstanceRow(t, testpkg.Ctx(t), db, instance)
	}
	large := measure(8)
	assert.Len(t, large, len(small), "the HTTP list must not add a read for each room")
	return small, large
}
