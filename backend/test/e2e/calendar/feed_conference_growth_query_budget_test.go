package calendar_e2e_test

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	rootAPI "github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	calendarHTTP "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/http"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

func TestPublicCalendarFeedPositiveGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db, module := setupCalendarModule(t)
	router := chi.NewRouter()
	router.Use(testpkg.TenantRuntimeMiddleware(t, db))
	router.Mount("/calendar", calendarHTTP.NewResource(module.Calendar, slog.Default()).Router(rootAPI.TenantSessionChain))
	router.Get("/public/calendar/{token}", calendarHTTP.PublicFeed(module.Calendar))
	_, account := testpkg.CreateTestCalendarStaff(t, db, "Feed", "Growth")
	token := calendarToken(t, account.ID, permissions.CalendarOwn, permissions.CalendarManage)
	feedRec := doJSON(t, router, http.MethodGet, "/calendar/feed", token, nil)
	require.Equal(t, http.StatusOK, feedRec.Code, feedRec.Body.String())
	var feed feedE2EResponse
	require.NoError(t, json.Unmarshal(feedRec.Body.Bytes(), &feed))
	feedToken := feed.Data.URL[strings.LastIndexByte(feed.Data.URL, '/')+1:]
	require.NotEmpty(t, feedToken)
	count := 0
	for _, n := range []int{3, 8} {
		for count < n {
			rec := doJSON(t, router, http.MethodPost, "/calendar/appointments", token, map[string]any{
				"title": fmt.Sprintf("Feed growth %d", count), "start_date": "2026-01-08", "end_date": "2026-01-08", "start_time": "14:00", "end_time": "15:00", "delivery_mode": "informational",
			})
			require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
			count++
		}
		counter := testpkg.CaptureQueries(t, db)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/public/calendar/"+feedToken, nil))
		queries := counter.Measurement(counter.Queries())
		counter.Stop()
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		require.Equal(t, "text/calendar; charset=utf-8", rec.Header().Get("Content-Type"))
		require.Equal(t, n, strings.Count(rec.Body.String(), "BEGIN:VEVENT"))
		for i := range n {
			require.Contains(t, rec.Body.String(), fmt.Sprintf("SUMMARY:Feed growth %d", i))
		}
		t.Logf("public calendar staff-token N%d statements=%d", n, len(queries.Statements))
		querybudgets.AssertQueryBudget(t, "api.calendar.public_feed.staff", queries)
	}
}

type conferenceDetailWire struct {
	ID          string `json:"id"`
	BookedCount int    `json:"booked_count"`
	Slots       []struct {
		Booked      bool    `json:"booked"`
		StudentID   *string `json:"student_id,omitempty"`
		StudentName *string `json:"student_name,omitempty"`
	} `json:"slots"`
}

func TestSchoolCalendarConferenceDetailPositiveGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db, module := setupCalendarModule(t)
	router := chi.NewRouter()
	router.Use(testpkg.TenantRuntimeMiddleware(t, db))
	router.Mount("/school/calendar", calendarHTTP.NewResource(module.Calendar, slog.Default()).SchoolRouter(rootAPI.ProtectedSchoolRoutes))
	staff, account := testpkg.CreateTestStaffWithAccount(t, db, "Conference", "Growth")
	testpkg.AssignLehrkraftSystemRole(t, db, account.ID, testpkg.Tenant(t))
	testpkg.CreateTestClassTeacher(t, db, staff.ID, "1a")
	claims := testutil.TenantUserTestClaims(int(account.ID), testpkg.Tenant(t), permissions.CalendarSchool)
	claims.Scope = "school"
	token := testutil.MintTestJWT(t, claims)
	created := doJSON(t, router, http.MethodPost, "/school/calendar/parent-conferences", token, map[string]any{
		"school_class": "1a", "date": "2026-03-04", "start_time": "14:00", "end_time": "17:00", "slot_minutes": 15, "breaks": []any{},
	})
	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())
	var initial struct {
		Data conferenceDetailWire `json:"data"`
	}
	require.NoError(t, json.Unmarshal(created.Body.Bytes(), &initial))
	conferenceID, err := strconv.ParseInt(initial.Data.ID, 10, 64)
	require.NoError(t, err)
	booked := 0
	wantIDs := []string{}
	for _, n := range []int{3, 8} {
		for booked < n {
			family := testpkg.CreateTestParentGuardianChain(t, db)
			// No shared booking-row fixture exists. Insert only fixture data; the
			// request under measurement is the real read resource, not a command.
			_, err := db.NewRaw(`INSERT INTO calendar.parent_conference_bookings
    (tenant_id, conference_id, student_id, guardian_profile_id, slot_start_minute)
    VALUES (?, ?, ?, ?, ?)`, testpkg.Tenant(t), conferenceID, family.StudentID, family.GuardianProfileID, 14*60+booked*15).Exec(testpkg.Ctx(t))
			require.NoError(t, err)
			wantIDs = append(wantIDs, strconv.FormatInt(family.StudentID, 10))
			booked++
		}
		counter := testpkg.CaptureQueries(t, db)
		rec := doJSON(t, router, http.MethodGet, "/school/calendar/parent-conferences/"+initial.Data.ID, token, nil)
		queries := counter.Measurement(counter.Queries())
		counter.Stop()
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var out struct {
			Data conferenceDetailWire `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
		require.Equal(t, initial.Data.ID, out.Data.ID)
		require.Equal(t, n, out.Data.BookedCount)
		gotIDs := []string{}
		for _, slot := range out.Data.Slots {
			if slot.Booked {
				require.NotNil(t, slot.StudentID)
				require.NotNil(t, slot.StudentName)
				require.Equal(t, "Felix Schneider", *slot.StudentName)
				gotIDs = append(gotIDs, *slot.StudentID)
			}
		}
		require.ElementsMatch(t, wantIDs, gotIDs)
		t.Logf("school calendar conference bookings N%d statements=%d", n, len(queries.Statements))
		querybudgets.AssertQueryBudget(t, "api.school.calendar.parent_conference_detail", queries)
	}
}
