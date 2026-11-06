package enrollmenthttp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Confirmed registration rows arrange the input, not replacement services.
// The public root builds the real People/Care Plan/Audit graph; assigning the
// group creates the guest identities, contacts and dated care exceptions.
type holidayGrowthFixture struct {
	db                           *bun.DB
	router                       http.Handler
	school, account, week, group int64
	token                        string
	children                     []int64
	students                     []int64
}

func newHolidayGrowthFixture(t *testing.T, db *bun.DB, n int) holidayGrowthFixture {
	t.Helper()
	testpkg.OwnTenant(t)
	school := testpkg.Tenant(t)
	_, account := testpkg.CreateTestStaffWithAccountForTenant(t, db, school, "Host", "Growth")
	group := testpkg.CreateTestEducationGroupForTenant(t, db, school, "Holiday growth").ID
	// This test's school and Träger have the same fixture ID, but the runtime
	// contexts below remain distinct: registrations are Träger rows, guests
	// and all measured requests are school-tenant commands with RLS.
	testpkg.OwnTraegerHolidayPeriods(t, db, school)
	rowID := func(sql string, args ...any) int64 {
		var id int64
		require.NoError(t, db.NewRaw(sql, args...).Scan(t.Context(), &id))
		return id
	}
	period := rowID(`INSERT INTO enrollment.holiday_periods (organization_id,name,registration_opens_on,registration_closes_on,status,published_at) VALUES (?, 'Holiday growth', '2020-01-01','2099-06-30','published',CURRENT_TIMESTAMP) RETURNING id`, school)
	week := rowID(`INSERT INTO enrollment.holiday_period_weeks (organization_id,holiday_period_id,starts_on,ends_on,host_school_id) VALUES (?,?,'2099-07-06','2099-07-10',?) RETURNING id`, school, period, school)
	model := rowID(`INSERT INTO enrollment.holiday_week_time_models (organization_id,holiday_period_week_id,name,starts_at,ends_at,price_cents) VALUES (?,?,'Morning care','08:00','13:00',0) RETURNING id`, school, week)
	f := holidayGrowthFixture{db: db, school: school, account: account.ID, week: week, group: group}
	for i := 0; i < n+2; i++ {
		status := "confirmed"
		if i == n {
			status = "received"
		}
		if i == n+1 {
			status = "cancelled"
		}
		reg := rowID(`INSERT INTO enrollment.holiday_registrations (organization_id,holiday_period_id,guardian_first_name,guardian_last_name,guardian_email,guardian_phone,status,privacy_consent_at) VALUES (?,?,'Guardian',?,?,'012345',?,CURRENT_TIMESTAMP) RETURNING id`, school, period, fmt.Sprintf("Family%02d", i), fmt.Sprintf("holiday%d@example.test", i), status)
		child := rowID(`INSERT INTO enrollment.holiday_registration_children (organization_id,holiday_period_id,holiday_registration_id,position,first_name,last_name,birth_date,guest_school_name) VALUES (?,?,?,1,?,'Growth','2018-03-01','External school') RETURNING id`, school, period, reg, fmt.Sprintf("HolidayChild%02d", i))
		if i < n {
			f.children = append(f.children, child)
		}
		rowID(`INSERT INTO enrollment.holiday_registration_bookings (organization_id,holiday_period_id,holiday_registration_child_id,holiday_period_week_id,holiday_week_time_model_id) VALUES (?,?,?,?,?) RETURNING id`, school, period, child, week, model)
		// 3 = parent + one emergency + one pickup; 8 = parent + three
		// emergencies + four pickups. Both respect the public 3/5 limits.
		emergency, pickup := 1, 1
		if n == 8 {
			emergency, pickup = 3, 4
		}
		for _, kind := range []string{"emergency", "pickup"} {
			count := emergency
			if kind == "pickup" {
				count = pickup
			}
			for j := 0; j < count; j++ {
				rowID(`INSERT INTO enrollment.holiday_registration_contacts (organization_id,holiday_registration_id,kind,position,name,phone,relationship) VALUES (?,?,?,?,?,'054021234','Family') RETURNING id`, school, reg, kind, j+1, fmt.Sprintf("%s%02d", kind, j))
			}
		}
	}
	database := testpkg.NewDatabase(t, db)
	people := owners.MustNewPeopleDirectory(database)
	organizations, err := owners.NewOrganizationTenancy(database)
	require.NoError(t, err)
	care, err := owners.NewUnobservedCarePlan(database, owners.MustNewUnobservedTimetable(database), people)
	require.NoError(t, err)
	staff, err := api.NewRFIDTestPeople(database)
	require.NoError(t, err)
	cipher, err := api.HealthDataCipher(testpkg.TestHealthDataKeys)
	require.NoError(t, err)
	audit := owners.NewTestAuditPersistence(database, nil).DataAccessLog
	today := func() calendar.Date { return calendar.NewDate(2099, 7, 6) }
	guests := owners.NewHolidayGuests(people, organizations, care, staff, audit, cipher, today)
	unit := testpkg.TenantRuntime(t, db)
	schoolID, err := tenant.NewTenantID(school)
	require.NoError(t, err)
	require.NoError(t, tenant.WithinTenant(tenant.WithUnitOfWork(t.Context(), unit), schoolID, func(ctx context.Context) error {
		transfer, err := guests.AssignHolidayGuestGroup(ctx, enrollment.HolidayHost{SchoolID: school, AccountID: account.ID, Role: "admin"}, week, group)
		if err == nil {
			require.Equal(t, enrollment.HolidayGuestTransfer{WeekID: week, GroupID: group, Guests: n}, transfer)
		}
		return err
	}))
	require.NoError(t, db.NewRaw(`SELECT student_id FROM enrollment.holiday_guests WHERE tenant_id=? AND holiday_period_week_id=? ORDER BY holiday_registration_child_id`, school, week).Scan(t.Context(), &f.students))
	require.Len(t, f.students, n)
	require.NoError(t, tenant.WithinTenant(tenant.WithUnitOfWork(t.Context(), unit), schoolID, func(ctx context.Context) error {
		for i, id := range f.students {
			identity, err := people.ReadEnrollmentStudent(ctx, id, "")
			if err != nil {
				return err
			}
			person, err := people.FindPerson(ctx, identity.PersonID)
			if err != nil {
				return err
			}
			require.Equal(t, id, identity.ID)
			require.Equal(t, school, identity.TenantID)
			require.Equal(t, fmt.Sprintf("HolidayChild%02d", i), person.FirstName)
			require.Equal(t, "Growth", person.LastName)
			require.Equal(t, "active", identity.Status)
			require.Equal(t, "Feriengast", identity.SchoolClass)
			require.Equal(t, &group, identity.GroupID)
			require.Equal(t, "2099-07-06", identity.EnrolledFrom)
			require.Equal(t, "2099-07-10", identity.EnrolledUntil)
		}
		return nil
	}))
	var plans int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM schedule.student_arrival_exceptions a JOIN schedule.student_pickup_exceptions p ON p.tenant_id=a.tenant_id AND p.student_id=a.student_id AND p.exception_date=a.exception_date WHERE a.tenant_id=? AND a.student_id IN (?) AND a.expected_arrival='08:00' AND p.pickup_time='13:00' AND a.exception_date BETWEEN '2099-07-06' AND '2099-07-10'`, school, bun.List(f.students)).Scan(t.Context(), &plans))
	require.Equal(t, n*5, plans)
	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(tenant.WithUnitOfWork(r.Context(), unit)))
		})
	})
	router.Mount("/api/holiday-guests", api.NewHolidayGuestHTTP(guests).Router(api.ProtectedTenantRoutes))
	router.Mount("/api/holiday-weeks", api.NewHolidayHostHTTP(organizations, people, audit, cipher, today).Router(api.ProtectedTenantRoutes))
	f.router = testpkg.SessionVerifier(router)
	f.token = f.mint(t, school, "config:read", "users:read")
	return f
}

func (f holidayGrowthFixture) mint(t *testing.T, school int64, perms ...string) string {
	t.Helper()
	return testpkg.MintTestJWT(t, sessiontoken.AppClaims{ID: int(f.account), Sub: "host@example.test", Username: "host", TenantID: school, Roles: []string{"staff"}, Permissions: perms})
}
func (f holidayGrowthFixture) get(token, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	f.router.ServeHTTP(rec, req)
	return rec
}
func (f holidayGrowthFixture) auditCount(t *testing.T, kind string) int {
	t.Helper()
	var count int
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM audit.data_access_log WHERE tenant_id=? AND resource_type=? AND actor_account_id=?`, f.school, kind, f.account).Scan(t.Context(), &count))
	return count
}

func TestHolidayPositiveGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	for _, n := range []int{3, 8} {
		t.Run(fmt.Sprintf("N%d", n), func(t *testing.T) {
			f := newHolidayGrowthFixture(t, db, n)
			counter := testpkg.CaptureQueries(t, f.db)
			sheetPath := fmt.Sprintf("/api/holiday-guests/%d", f.students[0])
			exportPath := fmt.Sprintf("/api/holiday-weeks/%d/participants/export?format=xlsx", f.week)
			// The companion DTO identifies exactly the children whose rows
			// are rendered by the measured export, not an alias budget.
			dtoRec := f.get(f.token, fmt.Sprintf("/api/holiday-weeks/%d/participants", f.week))
			require.Equal(t, http.StatusOK, dtoRec.Code, dtoRec.Body.String())
			var dto struct {
				Data struct {
					Week         enrollment.HostedHolidayWeek    `json:"week"`
					Participants []enrollment.HolidayParticipant `json:"participants"`
				} `json:"data"`
			}
			require.NoError(t, json.Unmarshal(dtoRec.Body.Bytes(), &dto))
			require.Equal(t, f.week, dto.Data.Week.ID)
			require.Equal(t, n, dto.Data.Week.Participants)
			require.Equal(t, n, dto.Data.Week.Guests)
			require.Len(t, dto.Data.Participants, n)
			for i, child := range dto.Data.Participants {
				require.Equal(t, f.children[i], child.ChildID)
				require.Equal(t, fmt.Sprintf("HolidayChild%02d", i), child.FirstName)
				require.Equal(t, "Growth", child.LastName)
				require.Equal(t, "External school", child.GuestSchoolName)
			}
			before := f.auditCount(t, "holiday_guest_sheet")
			counter.Reset()
			rec := f.get(f.token, sheetPath)
			queries := counter.Queries()
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			var body struct {
				Data enrollment.HolidayGuestSheet `json:"data"`
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			sheet := body.Data
			require.Equal(t, f.students[0], sheet.StudentID)
			require.Equal(t, f.week, sheet.WeekID)
			require.Equal(t, calendar.NewDate(2099, 7, 6), sheet.StartsOn)
			require.Equal(t, calendar.NewDate(2099, 7, 10), sheet.EndsOn)
			require.Equal(t, "Morning care", sheet.TimeModelName)
			require.Equal(t, "08:00", sheet.TimeModelStarts)
			require.Equal(t, "13:00", sheet.TimeModelEnds)
			require.Equal(t, "External school", sheet.SchoolName)
			require.False(t, sheet.ReadOnly)
			expectedEmergency := []enrollment.HolidayGuestContact{{Name: "Guardian Family00", Phone: "012345", Relationship: "Eltern"}}
			emergency, pickup := 1, 1
			if n == 8 {
				emergency, pickup = 3, 4
			}
			for i := 0; i < emergency; i++ {
				expectedEmergency = append(expectedEmergency, enrollment.HolidayGuestContact{Name: fmt.Sprintf("emergency%02d", i), Phone: "054021234", Relationship: "Family"})
			}
			require.Equal(t, expectedEmergency, sheet.Emergency)
			require.Len(t, sheet.Pickup, pickup)
			for i, contact := range sheet.Pickup {
				require.Equal(t, enrollment.HolidayGuestContact{Name: fmt.Sprintf("pickup%02d", i), Phone: "054021234", Relationship: "Family"}, contact)
			}
			require.Equal(t, n, len(sheet.Emergency)+len(sheet.Pickup))
			require.Equal(t, before+1, f.auditCount(t, "holiday_guest_sheet"))
			t.Logf("guest sheet N%d contacts=%d guests=%d SQL=%d", n, n, len(f.students), len(queries))
			querybudgets.AssertQueryBudget(t, "api.holiday_guests.growth.sheet", queries)
			before = f.auditCount(t, "holiday_participants_export")
			counter.Reset()
			rec = f.get(f.token, exportPath)
			queries = counter.Queries()
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			require.Contains(t, rec.Header().Get("Content-Type"), "spreadsheetml")
			require.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
			rows := holidayWorkbookRows(t, rec.Body.Bytes())
			var gotNames []string
			for _, row := range rows {
				for _, cell := range row {
					if len(cell) >= len("HolidayChild") && cell[:len("HolidayChild")] == "HolidayChild" {
						gotNames = append(gotNames, cell)
						require.Contains(t, row, "Growth")
						require.Contains(t, row, "Morning care")
						require.Contains(t, row, "Feriengast (External school)")
					}
				}
			}
			var expectedNames []string
			for i := 0; i < n; i++ {
				expectedNames = append(expectedNames, fmt.Sprintf("HolidayChild%02d", i))
			}
			require.Equal(t, expectedNames, gotNames, "exact confirmed XLSX rows, received and cancelled children excluded")
			require.Equal(t, before+1, f.auditCount(t, "holiday_participants_export"))
			t.Logf("participant XLSX N%d SQL=%d children=%v", n, len(queries), f.children)
			querybudgets.AssertQueryBudget(t, "api.holiday_weeks.growth.participants_export", queries)
			var metadata string
			require.NoError(t, f.db.NewRaw(`SELECT metadata::text FROM audit.data_access_log WHERE tenant_id=? AND resource_type='holiday_participants_export' ORDER BY id DESC LIMIT 1`, f.school).Scan(t.Context(), &metadata))
			var audit struct {
				Week         int64  `json:"week_id"`
				Participants int    `json:"participants"`
				Format       string `json:"format"`
			}
			require.NoError(t, json.Unmarshal([]byte(metadata), &audit))
			require.Equal(t, f.week, audit.Week)
			require.Equal(t, n, audit.Participants)
			require.Equal(t, "xlsx", audit.Format)
			// Denied reads neither expose data nor append successful-read audit rows.
			for _, path := range []string{sheetPath, exportPath} {
				rec = f.get(f.mint(t, f.school, "students:read"), path)
				require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
				var problem struct {
					Code string `json:"code"`
				}
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &problem))
				require.Equal(t, "general.permission", problem.Code)
			}
			require.Equal(t, 1, f.auditCount(t, "holiday_guest_sheet"))
			require.Equal(t, 1, f.auditCount(t, "holiday_participants_export"))
			t.Run("foreign school", func(t *testing.T) {
				testpkg.OwnTenant(t)
				otherSchool := testpkg.Tenant(t)
				_, other := testpkg.CreateTestStaffWithAccountForTenant(t, f.db, otherSchool, "Other", "Host")
				foreign := f
				foreign.account = other.ID
				for i, path := range []string{sheetPath, exportPath} {
					rec := f.get(foreign.mint(t, otherSchool, "config:read", "users:read"), path)
					require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
					var problem struct {
						Code string `json:"code"`
					}
					require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &problem))
					require.Equal(t, []string{"enrollment.holiday_guest_not_found", "enrollment.hosted_holiday_week_not_found"}[i], problem.Code)
				}
			})
			require.Equal(t, 1, f.auditCount(t, "holiday_guest_sheet"))
			require.Equal(t, 1, f.auditCount(t, "holiday_participants_export"))

		})
	}
}
