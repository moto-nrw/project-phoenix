package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Soll und Ist with a care contract (#4683, ADR 0156): where a sent contract
// of the Träger is in force, its booked services are the Soll; a draft and a
// contract of another school are not.

type contractCareTimeDayPayload struct {
	careTimeDayPayload
	LateMinutes   int    `json:"late_minutes"`
	PlannedSource string `json:"planned_source"`
}

func contractCareTimes(t *testing.T, tc *statisticsHTTPTestContext, accountID, childID int64) []contractCareTimeDayPayload {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/students/"+strconv.FormatInt(childID, 10)+"/care-times?"+careTimeWindow, nil)
	rec := authExec(t, tc, req, claimsFor(t, accountID), []string{permissions.UsersRead})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var payload struct {
		Data struct {
			Days []contractCareTimeDayPayload `json:"days"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	return payload.Data.Days
}

func TestStudentCareTimes_TakesTheSollFromTheSentContract(t *testing.T) {
	t.Parallel()
	tc := setupStatisticsRoute(t)
	school := testpkg.Tenant(t)
	staff, account := testpkg.CreateTestStaffWithAccountForTenant(t, tc.db, school, "Vertrag", "Leitung")
	// The care plan books Monday 08:00 to 14:00 (careTimeChild); the contract
	// books Monday until 13:00 and no Tuesday.
	childID := careTimeChild(t, tc, staff.ID)
	organization := testpkg.CreateTestOrganization(t, tc.db, &testpkg.Organization{
		Name: "Träger Soll", Slug: fmt.Sprintf("traeger-soll-%d", testpkg.UniqueTestTenantID(t)), Active: true}).ID
	testpkg.OwnTraegerCareContracts(t, tc.db, organization)
	testpkg.CreateTestCareContract(t, tc.db, organization, school, childID, "versendet", calendar.NewDate(2026, 6, 1),
		testpkg.CareContractService{Weekday: "mon", Module: "Frühbetreuung", EndsAt: "08:00"},
		testpkg.CareContractService{Weekday: "mon", Module: "OGS", EndsAt: "13:00"})
	// Another school's contract for the same ID promises Monday until 18:00;
	// the school never reads it.
	testpkg.CreateTestCareContract(t, tc.db, organization, school+1_000_000, childID, "versendet", calendar.NewDate(2026, 6, 1),
		testpkg.CareContractService{Weekday: "mon", Module: "OGS", EndsAt: "18:00"},
		testpkg.CareContractService{Weekday: "tue", Module: "OGS", EndsAt: "18:00"})

	days := contractCareTimes(t, tc, account.ID, childID)
	require.Len(t, days, 3)
	monday := days[0]
	assert.Equal(t, "2026-06-08", monday.Date)
	assert.Equal(t, "contract", monday.PlannedSource)
	require.NotNil(t, monday.PlannedEnd)
	assert.Equal(t, "13:00", *monday.PlannedEnd, "the latest module end is the pickup")
	assert.Equal(t, "08:00", *monday.PlannedStart, "the contract names no arrival; the care plan's stays")
	assert.Equal(t, "overrun", monday.Kind, "the child stayed until 14:00")
	assert.Equal(t, 60, monday.LateMinutes)
	assert.True(t, monday.Overrun)

	tuesday := days[1]
	assert.Equal(t, "unbooked", tuesday.Kind, "the contract books no Tuesday")
	assert.Equal(t, "contract", tuesday.PlannedSource)
}

func TestStudentCareTimes_IgnoresADraftContract(t *testing.T) {
	t.Parallel()
	tc := setupStatisticsRoute(t)
	school := testpkg.Tenant(t)
	staff, account := testpkg.CreateTestStaffWithAccountForTenant(t, tc.db, school, "Entwurf", "Leitung")
	childID := careTimeChild(t, tc, staff.ID)
	organization := testpkg.CreateTestOrganization(t, tc.db, &testpkg.Organization{
		Name: "Träger Entwurf", Slug: fmt.Sprintf("traeger-entwurf-%d", testpkg.UniqueTestTenantID(t)), Active: true}).ID
	testpkg.OwnTraegerCareContracts(t, tc.db, organization)
	testpkg.CreateTestCareContract(t, tc.db, organization, school, childID, "entwurf", calendar.NewDate(2026, 6, 1),
		testpkg.CareContractService{Weekday: "mon", Module: "OGS", EndsAt: "10:00"})

	days := contractCareTimes(t, tc, account.ID, childID)
	require.Len(t, days, 3)
	assert.Equal(t, "care_plan", days[0].PlannedSource)
	assert.Equal(t, "within_plan", days[0].Kind, "a draft promises nothing; the care plan's 14:00 holds")
	require.NotNil(t, days[0].PlannedEnd)
	assert.Equal(t, "14:00", *days[0].PlannedEnd)
}

// The contract applies only from its effective date: before it, the care
// plan's Soll stays.
func TestStudentCareTimes_KeepsTheCarePlanBeforeTheContractStarts(t *testing.T) {
	t.Parallel()
	tc := setupStatisticsRoute(t)
	school := testpkg.Tenant(t)
	staff, account := testpkg.CreateTestStaffWithAccountForTenant(t, tc.db, school, "Beginn", "Leitung")
	childID := careTimeChild(t, tc, staff.ID)
	organization := testpkg.CreateTestOrganization(t, tc.db, &testpkg.Organization{
		Name: "Träger Beginn", Slug: fmt.Sprintf("traeger-beginn-%d", testpkg.UniqueTestTenantID(t)), Active: true}).ID
	testpkg.OwnTraegerCareContracts(t, tc.db, organization)
	testpkg.CreateTestCareContract(t, tc.db, organization, school, childID, "versendet", calendar.NewDate(2026, 6, 15),
		testpkg.CareContractService{Weekday: "mon", Module: "OGS", EndsAt: "16:00"})

	days := contractCareTimes(t, tc, account.ID, childID)
	require.Len(t, days, 3)
	assert.Equal(t, "care_plan", days[0].PlannedSource, "8 June lies before the contract")
	assert.Equal(t, "2026-06-15", days[2].Date)
	assert.Equal(t, "contract", days[2].PlannedSource)
	require.NotNil(t, days[2].PlannedEnd)
	assert.Equal(t, "16:00", *days[2].PlannedEnd)
}

// A contract module without an end time leaves the pickup unknown: the day
// is "end_unknown", never the green "within_plan" (#4683).
func TestStudentCareTimes_ContractWithoutEndTimeIsNotWithinTheBooking(t *testing.T) {
	t.Parallel()
	tc := setupStatisticsRoute(t)
	school := testpkg.Tenant(t)
	staff, account := testpkg.CreateTestStaffWithAccountForTenant(t, tc.db, school, "Ohne", "Endzeit")
	childID := careTimeChild(t, tc, staff.ID)
	organization := testpkg.CreateTestOrganization(t, tc.db, &testpkg.Organization{
		Name: "Träger Endzeit", Slug: fmt.Sprintf("traeger-endzeit-%d", testpkg.UniqueTestTenantID(t)), Active: true}).ID
	testpkg.OwnTraegerCareContracts(t, tc.db, organization)
	testpkg.CreateTestCareContract(t, tc.db, organization, school, childID, "versendet", calendar.NewDate(2026, 6, 1),
		testpkg.CareContractService{Weekday: "mon", Module: "OGS", EndsAt: ""})

	days := contractCareTimes(t, tc, account.ID, childID)
	require.NotEmpty(t, days)
	monday := days[0]
	assert.Equal(t, "2026-06-08", monday.Date)
	assert.Equal(t, "contract", monday.PlannedSource)
	assert.Nil(t, monday.PlannedEnd, "the contract names no pickup")
	assert.Equal(t, "end_unknown", monday.Kind, "an unknown pickup is never shown as within the booking")
	assert.False(t, monday.Overrun)
}

// Reading one child's care times leaves a row with the child in the
// school's data-access log (ADR 0088 section 7, #4683).
func TestStudentCareTimes_RecordsTheReadOfTheChild(t *testing.T) {
	t.Parallel()
	tc := setupStatisticsRoute(t)
	school := testpkg.Tenant(t)
	staff, account := testpkg.CreateTestStaffWithAccountForTenant(t, tc.db, school, "Protokoll", "Leitung")
	childID := careTimeChild(t, tc, staff.ID)

	contractCareTimes(t, tc, account.ID, childID)

	var logged int
	require.NoError(t, tc.db.NewRaw(`SELECT count(*) FROM audit.data_access_log
		WHERE tenant_id = ? AND actor_account_id = ? AND student_id = ? AND resource_type = 'attendance_statistics'
			AND metadata->>'action' = 'view_student'`, school, account.ID, childID).Scan(context.Background(), &logged))
	assert.Equal(t, 1, logged, "the read of the child's care times is logged with the child")
}
