package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/moto-ogs/project-phoenix/api/testutil"
	enrollmentCompose "github.com/moto-ogs/project-phoenix/modules/enrollment/compose"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/sessiontoken"
	schoolCalendarCompose "github.com/moto-ogs/project-phoenix/modules/schoolcalendar/compose"
	settingsModule "github.com/moto-ogs/project-phoenix/modules/settings"
	"github.com/moto-ogs/project-phoenix/roots/owners"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
	portalCompose "github.com/moto-ogs/project-phoenix/workflows/parentportal/compose"
	parentHTTP "github.com/moto-ogs/project-phoenix/workflows/parentportal/http"
	"github.com/stretchr/testify/require"
)

func TestParentListsPositiveGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	now := func() time.Time { return time.Date(2026, 8, 31, 10, 0, 0, 0, time.UTC) }
	today := calendar.NewDate(2026, 8, 31)
	db := testpkg.SetupIsolatedTestDB(t)
	database := testpkg.NewDatabase(t, db)
	people := NewPeopleRepositorySuiteFactory(database)
	members := owners.MustNewMembershipTestRepositories(database)
	repos, err := NewParentRouteTestRepositories(database, members)
	require.NoError(t, err)
	settings, err := owners.NewSettingsTestModule(database, testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	profiles, err := NewParentPortalCareProfiles(database)
	require.NoError(t, err)
	sharing, err := NewStudentPeopleRows(database, members)
	require.NoError(t, err)
	feedback, err := NewFeedbackTestModule(database, testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	module, _, _, _, _, _, err := NewStudentTestModule(database, testpkg.TenantRuntime(t, db), feedback.Feedback, now)
	require.NoError(t, err)
	directory := owners.MustNewPeopleDirectory(database)
	enrollment := owners.BuildEnrollmentRows(owners.NewEnrollmentBookings(), repos.CareExceptions, directory)
	catalog := enrollmentCompose.NewCareOfferingRecords(repos.CareExceptions)
	mealSettings, err := owners.BuildMealPlanSettings(settings.Settings)
	require.NoError(t, err)
	schoolCalendar, err := owners.BuildSchoolCalendarRecords(database, func(schoolCalendarCompose.Observation) {}, settings.Settings)
	require.NoError(t, err)
	meals, err := owners.BuildServeMealPlan(database, mealSettings, owners.NewStudentRootReads(people.Student), directory, schoolCalendar)
	require.NoError(t, err)
	require.NoError(t, settings.Settings.SetValue(testpkg.Ctx(t), settingsModule.KeyMealPlanEnabled, true, nil, nil))
	require.NoError(t, settings.Settings.SetValue(testpkg.Ctx(t), settingsModule.KeyMealRegistrationEnabled, true, nil, nil))
	require.NoError(t, settings.Settings.SetValue(testpkg.Ctx(t), "enrollment.offering_changes_enabled", true, nil, nil))
	portal := portalCompose.New(WithParentEnrollmentBudgetReads(portalCompose.Dependencies{
		GuardianChanges: owners.ParentGuardianChanges(), DeparturePolicy: owners.ParentDeparturePolicy(), GuardianPolicy: owners.NewParentGuardianPolicy(), Fingerprints: owners.SecurityFingerprints{}, Declarations: owners.ParentDeclarationPolicy(), Locales: owners.ParentPortalLocales(), Runtime: owners.ParentPortalRuntime{}, Logger: slog.Default(), Now: now,
		ChildRepo: repos.ParentChildReads(), StudentRepo: owners.NewParentStudentReads(people.Student), PersonRepo: owners.NewParentPersonReads(people.Person), GuardianProfileRepo: owners.NewParentGuardianProfileReads(people.GuardianProfile), GuardianPhoneRepo: owners.NewParentGuardianPhoneReads(people.GuardianPhoneNumber), StudentGuardianRepo: owners.NewParentStudentGuardianReads(people.StudentGuardian),
		Memberships: owners.NewParentSchoolMembershipLock(database),
		Settings:    settings.Settings, CareRequests: module.CareRequests, OfferingChanges: owners.NewParentOfferingChangeRequests(module.OfferingChanges), CareOfferingRepo: owners.NewParentCareOfferingReads(catalog), MealPlan: meals, CareProfiles: profiles, People: owners.MustNewPeopleDirectory(database), CarePlan: owners.NewParentCarePlanCommands(repos.CareExceptions, nil), CareExceptions: owners.NewParentCareExceptions(repos.CareExceptions), CarePlanPolicy: owners.NewParentCarePlanPolicy(), DataRequests: owners.NewParentDataRequestCommands(repos.CareExceptions), ChangeRequestRepo: owners.NewParentDataRequestReads(repos.CareExceptions), ExcusedRequests: owners.NewParentExcusedAbsenceRequests(repos.ExcusedRequests), StudentConsents: owners.NewParentStudentConsents(NewStudentConsents(database)),
		FamilyProtectionEvents: owners.BindParentFamilyProtectionReads(sharing.FamilyProtection), ParentRequestShares: owners.BindParentRequestShareRecords(sharing.ParentRequestShare), ParentRequestEvents: owners.NewParentRequestEvents(sharing.ParentRequestEvent),
	}, enrollment))
	router := chi.NewRouter()
	router.Use(testpkg.TenantRuntimeMiddleware(t, db))
	router.Use(RequestSettingsCacheMiddleware, RequestIdentityCacheMiddleware)
	router.Mount("/parent", parentHTTP.NewResource(parentHTTP.ResourceConfig{Parent: portal}).Router(ParentSessionChain))
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	token := testutil.MintTestJWT(t, sessiontoken.AppClaims{ID: int(chain.AccountID), Sub: fmt.Sprint(chain.AccountID), Roles: []string{"parent"}, Scope: "parent"})
	ctx := testpkg.Ctx(t)
	var pendingIDs []string
	for _, field := range []string{"first_name", "last_name", "school_class"} {
		target := "person"
		if field == "school_class" {
			target = "student"
		}
		var id int64
		require.NoError(t, db.NewRaw(`INSERT INTO users.student_data_change_requests (tenant_id,student_id,submitted_by,target,field_key,new_value,status) VALUES (?,?,?,?,?,?::jsonb,'pending') RETURNING id`, testpkg.Tenant(t), chain.StudentID, chain.AccountID, target, field, `"Pending"`).Scan(ctx, &id))
		pendingIDs = append(pendingIDs, fmt.Sprint(id))
	}
	staff, _ := testpkg.CreateTestStaffWithAccount(t, db, "Care", "Growth")
	phase := testpkg.CreateTestEnrollmentPhaseAround(t, db, today)
	request := testpkg.CreateTestEnrollmentRequestAwaitingGuardian(t, db, phase.ID, chain.Email, fmt.Sprintf("budget-%d", testpkg.UniqueSuffix()))
	var requestChildID int64
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.request_children (tenant_id,request_id,first_name,last_name,date_of_birth,status,created_student_id,target_grade_level,target_school_class) VALUES (?,?, 'Felix','Growth','2018-04-15','approved',?,2,'2b') RETURNING id`, testpkg.Tenant(t), request.ID, chain.StudentID).Scan(ctx, &requestChildID))
	var offeringIDs []int64
	var offeringNames []string
	var pickupIDs []string
	mealFrom := today.StartOfISOWeek()
	_, err = db.NewRaw(`INSERT INTO schedule.meal_participation_schedules (tenant_id,student_id,effective_from,monday,tuesday,wednesday,thursday,friday,changed_by_account_id) VALUES (?,?,?,true,true,true,true,true,?)`, testpkg.Tenant(t), chain.StudentID, mealFrom, chain.AccountID).Exec(ctx)
	require.NoError(t, err)
	student, err := testpkg.TenantQuery1(t, ctx, db, people.Student.FindByID, any(chain.StudentID))
	require.NoError(t, err)
	stamp := time.Date(2026, 8, 31, 9, 30, 0, 0, time.UTC)
	student.AGBAcceptedAt = &stamp
	student.DataProcessingAcceptedAt = &stamp
	student.PhotoConsentGivenAt = &stamp
	require.NoError(t, testpkg.TenantCommand1(t, ctx, db, people.Student.Update, student))
	var absenceIDs []string
	var co []testpkg.ParentChain
	childIDs := []int64{chain.StudentID}
	requestIDs := append([]string{}, pendingIDs...)
	counter := testpkg.CaptureQueriesForContext(t, db)
	counter.Stop()
	for _, n := range []int{3, 8} {
		mealDates := []string{}
		mealThrough := mealFrom
		for len(mealDates) < n {
			if mealThrough.Weekday() != time.Saturday && mealThrough.Weekday() != time.Sunday {
				mealDates = append(mealDates, mealThrough.String())
			}
			if len(mealDates) < n {
				mealThrough = mealThrough.AddDays(1)
			}
		}
		for len(co) < n {
			i := len(co)
			offering := testpkg.CreateTestCareOffering(t, db, phase.ID, fmt.Sprintf("Offering%02d", i))
			offeringIDs = append(offeringIDs, offering.ID)
			offeringNames = append(offeringNames, offering.Name)
			_, err := db.NewRaw(`INSERT INTO enrollment.care_offering_bookings (tenant_id,request_child_id,care_offering_id,manual_selected_days) VALUES (?,?,?,'["mon","tue","wed","thu","fri"]'::jsonb)`, testpkg.Tenant(t), requestChildID, offering.ID).Exec(ctx)
			require.NoError(t, err)
			var pickupID int64
			require.NoError(t, db.NewRaw(`INSERT INTO schedule.care_schedule_change_requests (tenant_id,student_id,submitted_by,request_kind,payload,status) VALUES (?,?,?,'pickup_change',?::jsonb,'pending') RETURNING id`, testpkg.Tenant(t), chain.StudentID, chain.AccountID, fmt.Sprintf(`{"date":"%s","pickup_time":"15:30","reason":"Ask%02d"}`, today.AddDays(i+1), i)).Scan(ctx, &pickupID))
			pickupIDs = append(pickupIDs, fmt.Sprint(pickupID))
			day := calendar.NewDate(2026, 8, 3).AddDays(i)
			testpkg.CreateTestPickupException(t, db, chain.StudentID, day, staff.ID, "15:45", fmt.Sprintf("Pickup%02d", i))
			testpkg.CreateTestArrivalException(t, db, chain.StudentID, day, staff.ID, "08:15", fmt.Sprintf("Arrival%02d", i))
			var absenceID int64
			require.NoError(t, db.NewRaw(`INSERT INTO active.excused_absence_requests (tenant_id,student_id,submitted_by,dates,note,status) VALUES (?,?,?,?::jsonb,?,'pending') RETURNING id`, testpkg.Tenant(t), chain.StudentID, chain.AccountID, fmt.Sprintf(`["%s"]`, day), fmt.Sprintf("Absence%02d", i)).Scan(ctx, &absenceID))
			absenceIDs = append(absenceIDs, fmt.Sprint(absenceID))
			co = append(co, testpkg.CreateTestCoGuardianForStudent(t, db, chain.StudentID, fmt.Sprintf("Co%02d", i), "Growth"))
			_, err = db.NewRaw(`INSERT INTO users.guardian_phone_numbers (tenant_id,guardian_profile_id,phone_number,phone_type,is_primary) VALUES (?,?,?,'mobile',true)`, testpkg.Tenant(t), co[i].GuardianProfileID, fmt.Sprintf("+492111000%03d", i)).Exec(ctx)
			require.NoError(t, err)
			_, err = db.NewRaw(`INSERT INTO users.guardian_phone_numbers (tenant_id,guardian_profile_id,phone_number,phone_type,is_primary) VALUES (?,?,?,'mobile',?)`, testpkg.Tenant(t), chain.GuardianProfileID, fmt.Sprintf("+492112000%03d", i), i == 0).Exec(ctx)
			require.NoError(t, err)
			if len(childIDs) < n {
				student := testpkg.CreateTestStudent(t, db, fmt.Sprintf("Sibling%02d", i), "Growth", "2b")
				testpkg.CreateTestStudentGuardianLink(t, db, student.ID, chain.GuardianProfileID, "primary_guardian")
				childIDs = append(childIDs, student.ID)
			}
			var id int64
			require.NoError(t, db.NewRaw(`INSERT INTO users.student_data_change_requests (tenant_id,student_id,submitted_by,target,field_key,new_value,status) VALUES (?,?,?,'person','first_name',?::jsonb,'approved') RETURNING id`, testpkg.Tenant(t), chain.StudentID, chain.AccountID, fmt.Sprintf(`"History%02d"`, i)).Scan(ctx, &id))
			requestIDs = append(requestIDs, fmt.Sprint(id))
			_, err = db.NewRaw(`INSERT INTO users.parent_request_events (tenant_id,student_id,request_type,request_id,event_type,actor_account_id,version) VALUES (?,?,'master_data',?,'guardian_edited',?,?)`, testpkg.Tenant(t), chain.StudentID, pendingIDs[0], chain.AccountID, fmt.Sprintf("version-%02d", i)).Exec(ctx)
			require.NoError(t, err)
		}
		for _, name := range []string{"children", "guardians", "related_accounts", "master_data", "master_data_requests", "sharing_options", "sharing", "events", "consents", "care_exception", "excused_requests", "pickup_requests", "care_offerings", "offering_catalog", "meal_participation"} {
			base := fmt.Sprintf("/parent/me/children/%d", chain.StudentID)
			path := map[string]string{"children": "/parent/me/children", "guardians": base + "/guardians", "related_accounts": base + "/related-accounts", "master_data": base + "/master-data", "master_data_requests": base + "/master-data/requests", "sharing_options": base + "/request-sharing-options", "sharing": base + "/request-sharing/master_data/" + pendingIDs[0], "events": base + "/requests/master_data/" + pendingIDs[0] + "/events", "consents": base + "/consents"}[name]
			switch name {
			case "pickup_requests":
				path = base + "/pickup-change-requests"
			case "care_offerings":
				path = base + "/care-offerings"
			case "offering_catalog":
				path = base + "/care-offerings/catalog"
			case "meal_participation":
				path = base + "/meal-participation?from=" + mealFrom.String() + "&to=" + mealThrough.String()
			case "care_exception":
				path = base + "/care-exception?from=2026-08-03&to=2026-08-10"
			case "excused_requests":
				path = base + "/excused-requests"
			}
			req := testutil.NewAuthenticatedRequest(t, http.MethodGet, path, nil, testutil.WithJWTBearer(token))
			req = req.WithContext(counter.Context(ctx))
			counter.Reset()
			counter.Start()
			rr := testutil.ExecuteRequest(router, req)
			counter.Stop()
			require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
			var envelope struct{ Data json.RawMessage }
			require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &envelope))
			switch name {
			case "children":
				var rows []parentHTTP.ChildResponse
				require.NoError(t, json.Unmarshal(envelope.Data, &rows))
				require.Len(t, rows, n)
				ids := []string{}
				for _, row := range rows {
					ids = append(ids, row.StudentID)
					require.NotEmpty(t, row.FirstName)
					require.NotEmpty(t, row.LastName)
				}
				for _, id := range childIDs {
					require.Contains(t, ids, fmt.Sprint(id))
				}
			case "guardians", "related_accounts":
				var rows []struct {
					ID         string `json:"guardian_profile_id"`
					First      string `json:"first_name"`
					Last       string `json:"last_name"`
					Self       bool   `json:"is_self"`
					Status     string
					HasAccount bool   `json:"has_account"`
					Email      string `json:"email"`
					Phones     []struct {
						Number  string `json:"phone_number"`
						Type    string `json:"phone_type"`
						Primary bool   `json:"is_primary"`
					} `json:"phones"`
				}
				require.NoError(t, json.Unmarshal(envelope.Data, &rows))
				require.Len(t, rows, n+1)
				byID := map[string]int{}
				for i, row := range rows {
					byID[row.ID] = i
				}
				require.Len(t, byID, n+1)
				self, ok := byID[fmt.Sprint(chain.GuardianProfileID)]
				require.True(t, ok)
				require.True(t, rows[self].Self)
				require.Equal(t, chain.Email, rows[self].Email)
				if name == "guardians" {
					require.Len(t, rows[self].Phones, n)
					numbers := []string{}
					for _, phone := range rows[self].Phones {
						numbers = append(numbers, phone.Number)
						require.Equal(t, "mobile", phone.Type)
					}
					for i := range co {
						require.Contains(t, numbers, fmt.Sprintf("+492112000%03d", i))
					}
				}
				for i, c := range co {
					idx, ok := byID[fmt.Sprint(c.GuardianProfileID)]
					require.True(t, ok)
					row := rows[idx]
					require.Equal(t, fmt.Sprintf("Co%02d", i), row.First)
					require.Equal(t, "Growth", row.Last)
					require.False(t, row.Self)
					if name == "related_accounts" {
						require.Equal(t, "active", row.Status)
						require.Equal(t, c.Email, row.Email)
					} else {
						require.True(t, row.HasAccount)
						require.Empty(t, row.Email, "other account holders keep their contact identifiers private")
						require.Empty(t, row.Phones, "stored co-guardian phones stay redacted")
					}
				}
			case "master_data", "master_data_requests":
				var rows []parentHTTP.MasterDataChangeResponse
				if name == "master_data" {
					var row parentHTTP.MasterDataResponse
					require.NoError(t, json.Unmarshal(envelope.Data, &row))
					require.Equal(t, fmt.Sprint(chain.StudentID), row.StudentID)
					require.Equal(t, fmt.Sprint(chain.GuardianProfileID), row.GuardianProfileID)
					require.Equal(t, chain.Email, *row.Email)
					rows = row.PendingChanges
					require.Len(t, rows, 3)
				} else {
					require.NoError(t, json.Unmarshal(envelope.Data, &rows))
					require.Len(t, rows, n+2)
					for _, row := range rows {
						require.NotEqual(t, pendingIDs[2], row.ID, "school-controlled class requests are not parent-visible")
					}
				}
				seen := map[string]bool{}
				for _, row := range rows {
					require.False(t, seen[row.ID])
					seen[row.ID] = true
					require.Contains(t, requestIDs, row.ID)
					require.True(t, row.IsSelf)
					require.NotEmpty(t, row.CreatedAt)
					if row.Status == "pending" {
						require.Contains(t, pendingIDs, row.ID)
						require.JSONEq(t, `"Pending"`, string(row.NewValue))
					} else {
						require.Equal(t, "approved", row.Status)
						require.Contains(t, string(row.NewValue), "History")
					}
				}
			case "sharing_options", "sharing":
				var row struct {
					Protected  bool `json:"family_protected"`
					Recipients []struct {
						ID       string `json:"guardian_profile_id"`
						First    string `json:"first_name"`
						Last     string `json:"last_name"`
						Selected bool
					}
				}
				require.NoError(t, json.Unmarshal(envelope.Data, &row))
				require.False(t, row.Protected)
				require.Len(t, row.Recipients, n)
				seen := map[string]bool{}
				for _, r := range row.Recipients {
					require.False(t, seen[r.ID])
					seen[r.ID] = true
					idx := -1
					for i, c := range co {
						if fmt.Sprint(c.GuardianProfileID) == r.ID {
							idx = i
						}
					}
					require.GreaterOrEqual(t, idx, 0)
					require.Equal(t, fmt.Sprintf("Co%02d", idx), r.First)
					require.Equal(t, "Growth", r.Last)
					require.False(t, r.Selected)
				}
			case "events":
				var rows []struct {
					Kind    string `json:"event_type"`
					Created string `json:"created_at"`
					Version string
				}
				require.NoError(t, json.Unmarshal(envelope.Data, &rows))
				require.Len(t, rows, n)
				for i, row := range rows {
					require.Equal(t, "guardian_edited", row.Kind)
					require.Equal(t, fmt.Sprintf("version-%02d", i), row.Version)
					require.NotEmpty(t, row.Created)
				}
			case "pickup_requests":
				var rows []parentHTTP.PickupChangeRequestResponse
				require.NoError(t, json.Unmarshal(envelope.Data, &rows))
				require.Len(t, rows, n)
				seen := map[string]bool{}
				for _, row := range rows {
					require.False(t, seen[row.ID])
					seen[row.ID] = true
					idx := -1
					for i, id := range pickupIDs {
						if row.ID == id {
							idx = i
						}
					}
					require.GreaterOrEqual(t, idx, 0)
					require.Equal(t, today.AddDays(idx+1).String(), row.Date)
					require.NotNil(t, row.PickupTime)
					require.Equal(t, "15:30", *row.PickupTime)
					require.Equal(t, "pending", row.Status)
					require.Equal(t, fmt.Sprintf("Ask%02d", idx), row.Reason)
				}
			case "care_offerings":
				var row parentHTTP.CareOfferingsResponse
				require.NoError(t, json.Unmarshal(envelope.Data, &row))
				require.Equal(t, phase.Name, row.PeriodName)
				require.Len(t, row.Offerings, n)
				seen := map[string]bool{}
				for _, offering := range row.Offerings {
					require.False(t, seen[offering.ID])
					seen[offering.ID] = true
					idx := -1
					for i, id := range offeringIDs {
						if offering.ID == fmt.Sprint(id) {
							idx = i
						}
					}
					require.GreaterOrEqual(t, idx, 0)
					require.Equal(t, offeringNames[idx], offering.Name)
					require.Equal(t, []int{1, 2, 3, 4, 5}, offering.Weekdays)
				}
			case "offering_catalog":
				var row parentHTTP.OfferingCatalogResponse
				require.NoError(t, json.Unmarshal(envelope.Data, &row))
				require.Len(t, row.Items, n)
				names := []string{}
				ids := []string{}
				for _, offering := range row.Items {
					names = append(names, offering.Name)
					ids = append(ids, offering.ID)
					require.True(t, offering.Selected)
					require.True(t, offering.IsActive)
					require.Equal(t, []string{"mon", "tue", "wed", "thu", "fri"}, offering.AvailableDays)
					require.Equal(t, []string{"mon", "tue", "wed", "thu", "fri"}, offering.SelectedDays)
				}
				require.ElementsMatch(t, offeringNames, names)
				wantIDs := []string{}
				for _, id := range offeringIDs {
					wantIDs = append(wantIDs, fmt.Sprint(id))
				}
				require.ElementsMatch(t, wantIDs, ids)
			case "meal_participation":
				var row parentHTTP.MealParticipationResponse
				require.NoError(t, json.Unmarshal(envelope.Data, &row))
				require.Len(t, row.Days, n)
				for i, day := range row.Days {
					require.Equal(t, mealDates[i], day.Date)
					require.True(t, day.Regular)
					require.True(t, day.Participating)
					require.Equal(t, "regular", day.Source)
				}
			case "care_exception":
				var rows []parentHTTP.CareExceptionResponse
				require.NoError(t, json.Unmarshal(envelope.Data, &rows))
				require.Len(t, rows, n)
				for i, row := range rows {
					require.Equal(t, calendar.NewDate(2026, 8, 3).AddDays(i).String(), row.Date)
					require.NotNil(t, row.PickupTime)
					require.Equal(t, "15:45", *row.PickupTime)
					require.NotNil(t, row.ArrivalTime)
					require.Equal(t, "08:15", *row.ArrivalTime)
					require.False(t, row.PickupAbsent)
					require.False(t, row.ArrivalAbsent)
				}
			case "excused_requests":
				var rows []parentHTTP.ParentExcusedRequestResponse
				require.NoError(t, json.Unmarshal(envelope.Data, &rows))
				require.Len(t, rows, n)
				seen := map[string]bool{}
				for _, row := range rows {
					require.False(t, seen[row.ID])
					seen[row.ID] = true
					idx := -1
					for i, id := range absenceIDs {
						if row.ID == id {
							idx = i
						}
					}
					require.GreaterOrEqual(t, idx, 0)
					require.Equal(t, fmt.Sprint(chain.StudentID), row.StudentID)
					require.Equal(t, fmt.Sprintf("Absence%02d", idx), row.Note)
					require.Equal(t, "pending", row.Status)
					require.Equal(t, []string{calendar.NewDate(2026, 8, 3).AddDays(idx).String()}, row.Dates)
					require.True(t, row.IsSelf)
				}
			case "consents":
				var rows []struct {
					Key   string
					State string
				}
				require.NoError(t, json.Unmarshal(envelope.Data, &rows))
				require.Len(t, rows, 4)
				keys := []string{}
				for _, row := range rows {
					keys = append(keys, row.Key)
					if row.Key == "email_contact" {
						require.Equal(t, "not_recorded", row.State)
					} else {
						require.Equal(t, "granted", row.State)
					}
				}
				require.ElementsMatch(t, []string{"agb", "data_processing", "email_contact", "photo"}, keys)
			}
			t.Logf("MEASURE parent.%s N%d=%d", name, n, counter.Total())
			switch name {
			case "children":
				querybudgets.AssertQueryBudget(t, "parent.children.children.http_positive_growth", counter.Measurement(counter.Queries()))
			case "guardians":
				querybudgets.AssertQueryBudget(t, "parent.children.guardians.http_positive_growth", counter.Measurement(counter.Queries()))
			case "related_accounts":
				if n == 3 {
					querybudgets.AssertQueryBudget(t, "parent.children.related_accounts.http_positive_growth.n3", counter.Measurement(counter.Queries()))
				}
				if n == 8 {
					querybudgets.AssertQueryBudget(t, "parent.children.related_accounts.http_positive_growth.n8", counter.Measurement(counter.Queries()))
				}
			case "master_data":
				querybudgets.AssertQueryBudget(t, "parent.children.master_data.http_positive_growth", counter.Measurement(counter.Queries()))
			case "master_data_requests":
				querybudgets.AssertQueryBudget(t, "parent.children.master_data_requests.http_positive_growth", counter.Measurement(counter.Queries()))
			case "sharing_options":
				querybudgets.AssertQueryBudget(t, "parent.children.sharing_options.http_positive_growth", counter.Measurement(counter.Queries()))
			case "sharing":
				querybudgets.AssertQueryBudget(t, "parent.children.sharing.http_positive_growth", counter.Measurement(counter.Queries()))
			case "events":
				querybudgets.AssertQueryBudget(t, "parent.children.events.http_positive_growth", counter.Measurement(counter.Queries()))
			case "consents":
				querybudgets.AssertQueryBudget(t, "parent.children.consents.http_positive_growth", counter.Measurement(counter.Queries()))
			case "care_exception":
				querybudgets.AssertQueryBudget(t, "parent.children.care_exception.http_positive_growth", counter.Measurement(counter.Queries()))
			case "excused_requests":
				querybudgets.AssertQueryBudget(t, "parent.children.excused_requests.http_positive_growth", counter.Measurement(counter.Queries()))
			case "pickup_requests":
				querybudgets.AssertQueryBudget(t, "parent.children.pickup_requests.http_positive_growth", counter.Measurement(counter.Queries()))
			case "care_offerings":
				querybudgets.AssertQueryBudget(t, "parent.children.care_offerings.http_positive_growth", counter.Measurement(counter.Queries()))
			case "offering_catalog":
				querybudgets.AssertQueryBudget(t, "parent.children.offering_catalog.http_positive_growth", counter.Measurement(counter.Queries()))
			case "meal_participation":
				querybudgets.AssertQueryBudget(t, "parent.children.meal_participation.http_positive_growth", counter.Measurement(counter.Queries()))
			}
		}
	}
}
