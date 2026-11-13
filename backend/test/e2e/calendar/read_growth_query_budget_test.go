package calendar_e2e_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"testing"

	"github.com/moto-ogs/project-phoenix/auth/authorize/permissions"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

// These captures include the resource, session/permission middleware and all
// transaction statements. The external live-session verifier is not mounted.
func TestCalendarReadPositiveGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	for _, n := range []int{3, 8} {
		t.Run(fmt.Sprintf("N%d", n), func(t *testing.T) {
			testpkg.OwnTenant(t)
			testpkg.SetupIsolatedTestDB(t)
			db, router := setupParentCalendarRoute(t)
			_, organizer := testpkg.CreateTestCalendarStaff(t, db, "Read", "Organizer")
			family := testpkg.CreateTestParentGuardianChain(t, db)
			targets := []map[string]any{{"type": "guardian_profile", "id": family.GuardianProfileID}}
			for i := 1; i < n; i++ {
				student := testpkg.CreateTestStudent(t, db, "Child", fmt.Sprintf("Read%d", i), "1a")
				testpkg.CreateTestStudentGuardianLink(t, db, student.ID, family.GuardianProfileID, "primary_guardian")
				other := testpkg.CreateTestParentGuardianChain(t, db)
				targets = append(targets, map[string]any{"type": "guardian_profile", "id": other.GuardianProfileID})
			}
			staffToken := calendarToken(t, organizer.ID, permissions.CalendarOwn, permissions.CalendarManage)
			parentToken := parentCalendarToken(t, family.AccountID)
			var appointmentID int64
			for i := range n {
				rec := doJSON(t, router, http.MethodPost, "/calendar/appointments", staffToken, map[string]any{
					"title": fmt.Sprintf("Read appointment %d", i), "start_date": "2026-05-04", "end_date": "2026-05-04",
					"start_time": "14:00", "end_time": "15:00", "delivery_mode": "rsvp_required", "overview_visibility": "all", "targets": targets,
				})
				require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
				var created createAppointmentE2EResponse
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
				appointmentID = created.Data.Appointment.ID
			}
			base := "/calendar/appointments/" + strconv.FormatInt(appointmentID, 10)
			counter := testpkg.CaptureQueries(t, db)
			read := func(path, token string, prefix ...string) []byte {
				t.Helper()
				counter.Reset()
				var target http.Handler = router
				if len(prefix) > 0 && prefix[0] != "" {
					target = testpkg.MountedQueryBudgetRouter(prefix[0], router)
				}
				rec := doJSON(t, target, http.MethodGet, path, token, nil)
				require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
				return rec.Body.Bytes()
			}
			var detail struct {
				Data struct {
					Appointment struct {
						ID int64 `json:"id"`
					} `json:"appointment"`
					Recipients []struct {
						ID int64 `json:"id"`
					} `json:"recipients"`
				} `json:"data"`
			}
			require.NoError(t, json.Unmarshal(read(base, staffToken, "/api"), &detail))
			require.Equal(t, appointmentID, detail.Data.Appointment.ID)
			require.Len(t, detail.Data.Recipients, n)
			assertCalendarReadBudget(t, "appointment_detail", n, counter.Measurement(counter.Queries()))
			for _, surface := range []struct{ name, path, token, mount string }{
				{"staff_overview", base + "/overview", staffToken, "/api"},
				{"parent_overview", "/parent/me/calendar/appointments/" + strconv.FormatInt(appointmentID, 10) + "/overview", parentToken, ""},
			} {
				var overview struct {
					Data struct {
						AppointmentID string `json:"appointment_id"`
						Counts        struct {
							Pending int `json:"pending"`
						} `json:"counts"`
						Attendees []struct {
							Name        string `json:"name"`
							Status      string `json:"status"`
							RecipientID string `json:"recipient_id"`
						} `json:"attendees"`
					} `json:"data"`
				}
				require.NoError(t, json.Unmarshal(read(surface.path, surface.token, surface.mount), &overview))
				require.Equal(t, strconv.FormatInt(appointmentID, 10), overview.Data.AppointmentID)
				require.Equal(t, n, overview.Data.Counts.Pending)
				// Staff see every invited parent by name; a parent sees only
				// their own entry, the others stay in the counts (#6846).
				wantListed := n
				if surface.name == "parent_overview" {
					wantListed = 1
				}
				require.Len(t, overview.Data.Attendees, wantListed)
				for _, attendee := range overview.Data.Attendees {
					require.Equal(t, "Sabine Schneider", attendee.Name)
					require.Equal(t, "pending", attendee.Status)
					require.NotEmpty(t, attendee.RecipientID)
				}
				assertCalendarReadBudget(t, surface.name, n, counter.Measurement(counter.Queries()))
			}
			var listed calendarListE2EResponse
			require.NoError(t, json.Unmarshal(read("/parent/me/calendar?from=2026-05-04&to=2026-05-04", parentToken), &listed))
			require.Len(t, listed.Data.Events, n)
			for _, event := range listed.Data.Events {
				require.Equal(t, "2026-05-04", event.StartDate)
				require.True(t, event.CanRespond)
				require.Equal(t, "pending", event.ResponseStatus)
			}
			assertCalendarReadBudget(t, "parent_list", n, counter.Measurement(counter.Queries()))
		})
	}
}

func assertCalendarReadBudget(t *testing.T, route string, n int, queries querybudgets.Measurement) {
	t.Helper()
	t.Logf("full calendar %s N%d statements=%d", route, n, len(queries.Statements))
	switch route {
	case "appointment_detail":
		querybudgets.AssertQueryBudget(t, "api.calendar.appointment_detail", queries)
	case "staff_overview":
		querybudgets.AssertQueryBudget(t, "api.calendar.staff_overview", queries)
	case "parent_overview":
		querybudgets.AssertQueryBudget(t, "api.calendar.parent_overview", queries)
	case "parent_list":
		querybudgets.AssertQueryBudget(t, "api.calendar.parent_list", queries)
	default:
		t.Fatalf("unknown calendar read %q", route)
	}
}
