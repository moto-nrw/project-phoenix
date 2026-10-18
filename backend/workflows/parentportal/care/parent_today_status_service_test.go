package care_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"

	"github.com/moto-nrw/project-phoenix/workflows/parentportal"

	"github.com/moto-nrw/project-phoenix/api"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// buildTodayStatusService baut den Parent-Service mit genau den Abhaengigkeiten,
// die der Tagesstatus braucht. ArrivalSchedules bleibt bewusst nil: ohne
// Betreuungsplan ist CareDayResolved false, und der Test kann sich auf die
// Anwesenheit konzentrieren, die hier geprueft wird.
func buildTodayStatusService(t *testing.T) (*parentportal.ChildFlows, *bun.DB) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	repos := api.NewRepositoryTestFactory(testpkg.NewDatabase(t, db))
	reposPeople := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	return parentportal.NewChildFlows(parentportal.ChildConfig{
		GuardianPolicy: api.NewParentGuardianPolicy(),

		Runtime:        api.ParentPortalRuntime{},
		RequestSharing: unconfiguredRequestSharer{},
		ChildRepo:      repos.ParentChildReads(),
		Attendance:     parentAttendance(t, db),
		StatusDayRepo:  repos.StudentStatusDay,
		StudentRepo:    api.NewParentStudentReads(reposPeople.Student),
		Logger:         slog.Default(),
		Now: func() time.Time {
			return time.Date(2026, 8, 24, 13, 0, 0, 0, time.UTC)
		},

		DeparturePolicy: api.ParentDeparturePolicy(),

		CarePlanPolicy: api.NewParentCarePlanPolicy(),
	}), db
}

// buildTodayStatusServiceWithSchedule adds the real arrival-schedule service, so
// the care-day branches run against actual rows instead of a stubbed answer.
func buildTodayStatusServiceWithSchedule(t *testing.T) (*parentportal.ChildFlows, *bun.DB) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	repos := api.NewRepositoryTestFactory(testpkg.NewDatabase(t, db))
	reposPeople := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	arrivalRules, arrivalDeps, err := api.ArrivalScheduleBinding(api.MustNewPeopleDirectory(testpkg.MustDatabase(db)), nil, nil, slog.Default())
	require.NoError(t, err)
	arrivals, err := careplanCompose.NewArrivalSchedules(db, repos.CarePlan(), nil, arrivalRules, arrivalDeps)
	require.NoError(t, err)
	pickupStudents, err := api.PickupPlanStudents(api.MustNewPeopleDirectory(testpkg.MustDatabase(db)))
	require.NoError(t, err)
	pickups, err := careplanCompose.NewPickupSchedules(db, repos.CarePlan(), newPickupBaselineService(repos.CarePlan(), approvedOfferingProjection(t)), // Auto-excusal (#2360) is not what these cases assert.
		nil, pickupStudents, slog.Default())
	require.NoError(t, err)
	return parentportal.NewChildFlows(parentportal.ChildConfig{
		GuardianPolicy: api.NewParentGuardianPolicy(),

		Runtime:          api.ParentPortalRuntime{},
		RequestSharing:   unconfiguredRequestSharer{},
		ChildRepo:        repos.ParentChildReads(),
		Attendance:       parentAttendance(t, db),
		StatusDayRepo:    repos.StudentStatusDay,
		StudentRepo:      api.NewParentStudentReads(reposPeople.Student),
		ArrivalSchedules: api.NewParentArrivalSchedules(arrivals),
		PickupSchedules:  api.NewParentPickupSchedules(pickups),
		Logger:           slog.Default(),
		Now: func() time.Time {
			return time.Date(2026, 8, 24, 13, 0, 0, 0, time.UTC)
		},

		DeparturePolicy: api.ParentDeparturePolicy(),

		CarePlanPolicy: api.NewParentCarePlanPolicy(),
	}), db
}

func seedPickupScheduleForToday(t *testing.T, db *bun.DB, tenantID, studentID int64, pickup time.Time) bool {
	t.Helper()
	weekday := int(calendar.NewDate(2026, 8, 24).Weekday())
	if weekday == 0 || weekday == 6 {
		return false
	}
	author := testpkg.CreateTestStaffForTenant(t, db, tenantID, "Abholung", "Autor")
	row := &careplan.PickupSchedule{
		StudentID: studentID, Weekday: weekday, PickupTime: pickup, CreatedBy: author.ID,
	}
	row.SetTenantID(tenantID)
	ctx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), tenantID)
	require.NoError(t, testpkg.WithTenantTx(t, ctx, db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		return api.CreateParentPickupScheduleFixture(txCtx, api.NewRepositoryTestFactory(testpkg.NewDatabase(t, db)).CarePlan(), row)
	}))
	t.Cleanup(func() {
		_ = testpkg.WithTenantTx(t, ctx, db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
			return api.NewCareScheduleSuiteRepositories(testpkg.NewDatabase(t, db)).StudentPickupSchedule.DeleteByStudentID(txCtx, studentID)
		})
	})
	return true
}

// seedArrivalScheduleForToday books the child into today's weekly plan and
// reports whether it could. The weekly plan only knows Monday to Friday, so on
// a weekend it books nothing and the caller asserts the no-care-day branch
// instead. That keeps the test meaningful on every day of the week rather than
// skipping itself into a coverage hole every Saturday and Sunday.
func seedArrivalScheduleForToday(t *testing.T, db *bun.DB, tenantID, studentID int64, arrival time.Time) bool {
	t.Helper()
	weekday := int(calendar.NewDate(2026, 8, 24).Weekday())
	if weekday == 0 || weekday == 6 {
		return false
	}

	// Real staff row rather than a guessed id: created_by carries a foreign key.
	author := testpkg.CreateTestStaffForTenant(t, db, tenantID, "Plan", "Autor")

	row := &testpkg.StudentArrivalSchedule{
		StudentID:       studentID,
		Weekday:         weekday,
		ExpectedArrival: arrival,
		CreatedBy:       author.ID,
	}
	row.TenantID = tenantID

	ctx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), tenantID)
	err := testpkg.WithTenantTx(t, ctx, db, tenantID, func(txCtx context.Context, tx bun.Tx) error {
		_, insertErr := tx.NewInsert().
			Model(row).
			ModelTableExpr(`schedule.student_arrival_schedules`).
			Exec(txCtx)
		return insertErr
	})
	require.NoError(t, err, "Wochenplan konnte nicht angelegt werden")

	t.Cleanup(func() {
		cleanupCtx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), tenantID)
		_ = testpkg.WithTenantTx(t, cleanupCtx, db, tenantID, func(txCtx context.Context, tx bun.Tx) error {
			_, delErr := tx.NewDelete().
				Model((*testpkg.StudentArrivalSchedule)(nil)).
				ModelTableExpr(`schedule.student_arrival_schedules`).
				Where("student_id = ?", studentID).
				Exec(txCtx)
			return delErr
		})
	})
	return true
}

// seedClosedAttendanceOn schreibt eine abgeschlossene Anwesenheit fuer einen
// vergangenen Tag. Das ist der Beleg, dass die Schule ueberhaupt Anwesenheit
// pflegt: ohne eine einzige Zeile der SCHULE in den letzten 14 Tagen antwortet
// der Tagesstatus bewusst "unbekannt", statt "nicht angekommen" zu behaupten.
// Auf welches Kind die Zeile laeuft, ist fuer dieses Signal egal.
func seedClosedAttendanceOn(t *testing.T, db *bun.DB, tenantID, studentID int64, date calendar.Date) {
	t.Helper()
	device := testpkg.CreateTestDeviceForTenant(t, db, tenantID, "attendance-history-fixture")
	checkIn := date.BerlinMidnight().Add(8 * time.Hour)
	checkOut := date.BerlinMidnight().Add(15 * time.Hour)
	testpkg.CreateTestAttendanceForTenant(t, db, tenantID, studentID, 0, device.ID, date, checkIn, &checkOut)
}

// openAttendanceToday schreibt eine offene Anwesenheitszeile fuer heute. Sie
// laeuft im Tenant-Kontext, damit RLS greift wie im Betrieb.
func openAttendanceToday(t *testing.T, db *bun.DB, tenantID, studentID int64, checkIn time.Time) {
	t.Helper()
	device := testpkg.CreateTestDeviceForTenant(t, db, tenantID, "today-status-fixture")
	testpkg.CreateTestAttendanceForTenant(t, db, tenantID, studentID, 0, device.ID, calendar.NewDate(2026, 8, 24), checkIn, nil)
}

// TestGetChildTodayStatusRejectsForeignChild ist die Mandantengrenze: ein
// Elternkonto darf ausschliesslich verknuepfte Kinder lesen.
func TestGetChildTodayStatusRejectsForeignChild(t *testing.T) {
	t.Parallel()

	svc, db := buildTodayStatusService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	_, err := svc.GetChildTodayStatus(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID+999999)

	require.Error(t, err, "ein nicht verknuepftes Kind muss abgewiesen werden")
}

// TestGetChildTodayStatusPresent prueft den Gutfall Ende zu Ende gegen die
// Datenbank: eine offene Anwesenheit ergibt die Ja-Aussage plus die Uhrzeit.
func TestGetChildTodayStatusPresent(t *testing.T) {
	t.Parallel()

	svc, db := buildTodayStatusService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	checkIn := calendar.Now().Add(-90 * time.Minute)
	openAttendanceToday(t, db, chain.TenantID, chain.StudentID, checkIn)

	status, err := svc.GetChildTodayStatus(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID)

	require.NoError(t, err)
	require.NotNil(t, status.AtOgs, "eine offene Anwesenheit belegt die Ja-Aussage")
	assert.True(t, *status.AtOgs)
	assert.Equal(t, parentportal.DayStatePresent, status.State)
	assert.Equal(t, calendar.NormalizeWallClock(checkIn).Format("15:04"), status.Since)
	assert.Empty(t, status.Until)
}

// TestGetChildTodayStatusWithoutAttendanceIsUnknown deckt den Rueckfall ab:
// ohne jede Anwesenheitszeile und ohne lesbaren Betreuungsplan trifft die
// Antwort keine Ja/Nein-Aussage, statt "nicht angekommen" zu behaupten und
// Eltern grundlos zu beunruhigen.
func TestGetChildTodayStatusWithoutAttendanceIsUnknown(t *testing.T) {
	t.Parallel()

	svc, db := buildTodayStatusService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	status, err := svc.GetChildTodayStatus(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID)

	require.NoError(t, err)
	assert.Nil(t, status.AtOgs, "ohne Beleg darf keine Ja/Nein-Aussage entstehen")
	assert.Equal(t, parentportal.DayStateUnknown, status.State)
}

// TestGetChildTodayStatusCareDayWithoutAttendance ist der Fall, der Eltern am
// meisten interessiert: heute ist ein Betreuungstag, das Kind ist aber noch
// nicht da. Die Antwort darf jetzt "nein" sagen, weil ein Betreuungstag belegt
// ist, und nennt die erwartete Ankunftszeit.
func TestGetChildTodayStatusCareDayWithoutAttendance(t *testing.T) {
	t.Parallel()

	svc, db := buildTodayStatusServiceWithSchedule(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	arrival := calendar.NormalizeWallClock(time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC))
	seeded := seedArrivalScheduleForToday(t, db, chain.TenantID, chain.StudentID, arrival)
	// Belegt, dass die Schule Anwesenheit pflegt, ohne heute eine anzulegen.
	seedClosedAttendanceOn(t, db, chain.TenantID, chain.StudentID, calendar.NewDate(2026, 8, 24).AddDays(-3))

	status, err := svc.GetChildTodayStatus(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID)

	require.NoError(t, err)
	require.NotNil(t, status.AtOgs, "mit Betreuungstag und gepflegter Anwesenheit gibt es eine Aussage")
	assert.False(t, *status.AtOgs, "ohne Anwesenheit heute ist das Kind nicht da")
	if !seeded {
		assert.Empty(t, status.ExpectedFrom, "das Wochenende ist nie ein Betreuungstag")
		return
	}
	assert.Equal(t, "08:00", status.ExpectedFrom, "die erwartete Ankunft kommt aus dem Wochenplan")
}

func TestGetChildTodayStatusCareDayWithoutArrivalTime(t *testing.T) {
	t.Parallel()

	svc, db := buildTodayStatusServiceWithSchedule(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	seeded := seedArrivalScheduleForToday(t, db, chain.TenantID, chain.StudentID, time.Time{})
	seedClosedAttendanceOn(t, db, chain.TenantID, chain.StudentID, calendar.NewDate(2026, 8, 24).AddDays(-3))

	status, err := svc.GetChildTodayStatus(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	if !seeded {
		assert.Equal(t, parentportal.DayStateNoCare, status.State)
		return
	}
	assert.Equal(t, parentportal.DayStateUnknown, status.State)
	assert.Empty(t, status.ExpectedFrom, "a missing class time must not appear as 00:00")
}

// TestGetChildTodayStatusPresentOnCareDay: sobald das Kind da ist, zaehlt die
// Anwesenheit und nicht mehr der Plan. Die erwartete Ankunft verschwindet dann
// aus der Antwort, sie wuerde neben "ist da seit ..." nur verwirren.
func TestGetChildTodayStatusPresentOnCareDay(t *testing.T) {
	t.Parallel()

	svc, db := buildTodayStatusServiceWithSchedule(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	arrival := calendar.NormalizeWallClock(time.Date(2026, 1, 1, 7, 30, 0, 0, time.UTC))
	seedArrivalScheduleForToday(t, db, chain.TenantID, chain.StudentID, arrival)
	checkIn := calendar.Now().Add(-30 * time.Minute)
	openAttendanceToday(t, db, chain.TenantID, chain.StudentID, checkIn)

	status, err := svc.GetChildTodayStatus(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID)

	require.NoError(t, err)
	require.NotNil(t, status.AtOgs, "eine offene Anwesenheit belegt die Ja-Aussage")
	assert.True(t, *status.AtOgs)
	assert.Equal(t, parentportal.DayStatePresent, status.State)
	assert.Equal(t, calendar.NormalizeWallClock(checkIn).Format("15:04"), status.Since)
	assert.Empty(t, status.ExpectedFrom, "wer da ist, wird nicht mehr erwartet")
}

// TestGetChildTodayStatusWithoutPlanIsNoCareDay: ein Kind ohne Eintrag fuer
// heute hat keinen Betreuungstag, und ohne Betreuungstag gibt es auch keine
// erwartete Ankunftszeit zu melden.
func TestGetChildTodayStatusWithoutPlanIsNoCareDay(t *testing.T) {
	t.Parallel()

	svc, db := buildTodayStatusServiceWithSchedule(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	status, err := svc.GetChildTodayStatus(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID)

	require.NoError(t, err)
	assert.Empty(t, status.ExpectedFrom, "ohne Wochenplan gibt es keine Ankunftszeit")
}

func TestGetChildTodayStatusPickupOnlyDoesNotClaimNoCare(t *testing.T) {
	t.Parallel()

	svc, db := buildTodayStatusServiceWithSchedule(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	if !seedPickupScheduleForToday(t, db, chain.TenantID, chain.StudentID, calendar.NormalizeWallClock(time.Date(2026, 1, 1, 15, 30, 0, 0, time.UTC))) {
		t.Skip("Wochenplaene gelten nur montags bis freitags")
	}
	seedClosedAttendanceOn(t, db, chain.TenantID, chain.StudentID, calendar.NewDate(2026, 8, 24).AddDays(-3))

	status, err := svc.GetChildTodayStatus(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID)

	require.NoError(t, err)
	assert.Equal(t, parentportal.DayStateUnknown, status.State)
	assert.Nil(t, status.AtOgs, "ohne Ankunftszeit darf der Status nicht keine Betreuung behaupten")
}

// TestGetChildTodayStatusTracksAttendanceSchoolWide: das Signal "die Schule
// pflegt Anwesenheit" haengt an der Schule, nicht am einzelnen Kind. Ein neu
// aufgenommenes Kind hat selbst keine Historie; solange irgendein anderes Kind
// der Schule im Fenster erfasst wurde, darf die Antwort trotzdem "noch nicht
// da" lauten statt zu schweigen. (Derselbe Fall trifft nach den Ferien jedes
// Kind der Schule gleichzeitig.)
func TestGetChildTodayStatusTracksAttendanceSchoolWide(t *testing.T) {
	t.Parallel()

	svc, db := buildTodayStatusServiceWithSchedule(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	if !seedArrivalScheduleForToday(t, db, chain.TenantID, chain.StudentID, calendar.NormalizeWallClock(time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC))) {
		t.Skip("Wochenplaene gelten nur montags bis freitags")
	}

	// Die Historie gehoert einem MITSCHUELER, das angefragte Kind hat keine.
	classmate := testpkg.CreateTestStudentForTenant(t, db, chain.TenantID, "Mit", "Schueler", "3a")
	seedClosedAttendanceOn(t, db, chain.TenantID, classmate.ID, calendar.NewDate(2026, 8, 24).AddDays(-3))

	status, err := svc.GetChildTodayStatus(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID)

	require.NoError(t, err)
	require.NotNil(t, status.AtOgs, "die Anwesenheitskultur belegt die Schule, nicht das einzelne Kind")
	assert.False(t, *status.AtOgs)
	assert.Equal(t, "08:00", status.ExpectedFrom)
}

func TestGetChildTodayStatusAbsentArrivalExceptionOverridesWeeklyPlan(t *testing.T) {
	t.Parallel()

	svc, db := buildTodayStatusServiceWithSchedule(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	if !seedArrivalScheduleForToday(t, db, chain.TenantID, chain.StudentID, calendar.NormalizeWallClock(time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC))) {
		t.Skip("Wochenplaene gelten nur montags bis freitags")
	}
	staff := testpkg.CreateTestStaffForTenant(t, db, chain.TenantID, "Abwesenheit", "Autor")
	exception := &careplan.ArrivalException{
		StudentID: chain.StudentID, ExceptionDate: careplan.Date(calendar.NewDate(2026, 8, 24)), ExpectedArrival: nil, CreatedBy: staff.ID,
	}
	exception.SetTenantID(chain.TenantID)
	ctx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), chain.TenantID)
	require.NoError(t, testpkg.WithTenantTx(t, ctx, db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		return api.CreateParentArrivalExceptionFixture(txCtx, api.NewRepositoryTestFactory(testpkg.NewDatabase(t, db)).CarePlan(), exception)
	}))

	status, err := svc.GetChildTodayStatus(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID)

	require.NoError(t, err)
	assert.Equal(t, parentportal.DayStateNoCare, status.State)
	assert.Empty(t, status.ExpectedFrom)
}

func parentAttendance(t *testing.T, db *bun.DB) parentportal.AttendanceReader {
	t.Helper()
	module, err := api.NewCheckinTestPresence(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	return api.BindParentAttendance(module)
}

func approvedOfferingProjection(t *testing.T) *api.ApprovedOfferingTestProjection {
	t.Helper()
	projection, err := api.NewOwnerApprovedOfferingTestProjection(testpkg.NewDatabase(t, testpkg.SetupTestDB(t)))
	require.NoError(t, err)
	return projection
}
