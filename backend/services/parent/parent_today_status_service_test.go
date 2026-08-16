package parent_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	repositories "github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// buildTodayStatusService baut den Parent-Service mit genau den Abhaengigkeiten,
// die der Tagesstatus braucht. ArrivalSchedules bleibt bewusst nil: ohne
// Betreuungsplan ist CareDayResolved false, und der Test kann sich auf die
// Anwesenheit konzentrieren, die hier geprueft wird.
func buildTodayStatusService(t *testing.T) (parentService.Service, *bun.DB) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	repos := repositories.NewFactory(db)
	return parentService.NewService(parentService.ServiceConfig{
		ChildRepo:      repos.ParentChild,
		AttendanceRepo: repos.Attendance,
		StatusDayRepo:  repos.StudentStatusDay,
		StudentRepo:    repos.Student,
		DB:             db,
		Logger:         slog.Default(),
	}), db
}

// buildTodayStatusServiceWithSchedule adds the real arrival-schedule service, so
// the care-day branches run against actual rows instead of a stubbed answer.
func buildTodayStatusServiceWithSchedule(t *testing.T) (parentService.Service, *bun.DB) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	repos := repositories.NewFactory(db)
	return parentService.NewService(parentService.ServiceConfig{
		ChildRepo:      repos.ParentChild,
		AttendanceRepo: repos.Attendance,
		StatusDayRepo:  repos.StudentStatusDay,
		StudentRepo:    repos.Student,
		ArrivalSchedules: scheduleSvc.NewArrivalScheduleService(
			repos.StudentArrivalSchedule,
			repos.StudentArrivalException,
			repos.StudentArrivalNote,
			repos.Student,
			repos.Person,
			db,
			slog.Default(),
		),
		DB:     db,
		Logger: slog.Default(),
	}), db
}

// seedArrivalScheduleForToday books the child into today's weekly plan and
// reports whether it could. The weekly plan only knows Monday to Friday, so on
// a weekend it books nothing and the caller asserts the no-care-day branch
// instead. That keeps the test meaningful on every day of the week rather than
// skipping itself into a coverage hole every Saturday and Sunday.
func seedArrivalScheduleForToday(t *testing.T, db *bun.DB, tenantID, studentID int64, arrival time.Time) bool {
	t.Helper()
	weekday := int(timezone.TodayDate().Weekday())
	if weekday == 0 || weekday == 6 {
		return false
	}

	row := &scheduleModels.StudentArrivalSchedule{
		StudentID:       studentID,
		Weekday:         weekday,
		ExpectedArrival: arrival,
	}
	row.SetTenantID(tenantID)

	ctx := tenant.WithTenantID(context.Background(), tenantID)
	err := tenant.WithTenantTx(ctx, db, tenantID, func(txCtx context.Context, tx bun.Tx) error {
		_, insertErr := tx.NewInsert().
			Model(row).
			ModelTableExpr(`schedule.student_arrival_schedules`).
			Exec(txCtx)
		return insertErr
	})
	require.NoError(t, err, "Wochenplan konnte nicht angelegt werden")

	t.Cleanup(func() {
		cleanupCtx := tenant.WithTenantID(context.Background(), tenantID)
		_ = tenant.WithTenantTx(cleanupCtx, db, tenantID, func(txCtx context.Context, tx bun.Tx) error {
			_, delErr := tx.NewDelete().
				Model((*scheduleModels.StudentArrivalSchedule)(nil)).
				ModelTableExpr(`schedule.student_arrival_schedules`).
				Where("student_id = ?", studentID).
				Exec(txCtx)
			return delErr
		})
	})
	return true
}

// openAttendanceToday schreibt eine offene Anwesenheitszeile fuer heute. Sie
// laeuft im Tenant-Kontext, damit RLS greift wie im Betrieb.
func openAttendanceToday(t *testing.T, db *bun.DB, tenantID, studentID int64, checkIn time.Time) {
	t.Helper()
	// Echtes Geraet statt einer geratenen ID: active.attendance haelt einen
	// zusammengesetzten Fremdschluessel auf (tenant_id, device_id).
	device := testpkg.CreateTestDeviceForTenant(t, db, tenantID, "today-status-fixture")

	row := &activeModels.Attendance{
		StudentID:   studentID,
		Date:        timezone.TodayDate(),
		CheckInTime: checkIn,
		DeviceID:    device.ID,
	}
	row.SetTenantID(tenantID)

	ctx := tenant.WithTenantID(context.Background(), tenantID)
	err := tenant.WithTenantTx(ctx, db, tenantID, func(txCtx context.Context, tx bun.Tx) error {
		_, insertErr := tx.NewInsert().
			Model(row).
			ModelTableExpr(`active.attendance`).
			Exec(txCtx)
		return insertErr
	})
	require.NoError(t, err, "Anwesenheitszeile konnte nicht angelegt werden")

	t.Cleanup(func() {
		cleanupCtx := tenant.WithTenantID(context.Background(), tenantID)
		_ = tenant.WithTenantTx(cleanupCtx, db, tenantID, func(txCtx context.Context, tx bun.Tx) error {
			_, delErr := tx.NewDelete().
				Model((*activeModels.Attendance)(nil)).
				ModelTableExpr(`active.attendance`).
				Where("student_id = ?", studentID).
				Exec(txCtx)
			return delErr
		})
	})
}

// TestGetChildTodayStatusRejectsForeignChild ist die Mandantengrenze: ein
// Elternkonto darf ausschliesslich verknuepfte Kinder lesen.
func TestGetChildTodayStatusRejectsForeignChild(t *testing.T) {
	svc, db := buildTodayStatusService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	_, err := svc.GetChildTodayStatus(context.Background(), chain.AccountID, chain.StudentID+999999)

	require.Error(t, err, "ein nicht verknuepftes Kind muss abgewiesen werden")
}

// TestGetChildTodayStatusPresent prueft den Gutfall Ende zu Ende gegen die
// Datenbank: eine offene Anwesenheit ergibt die Ja-Aussage plus die Uhrzeit.
func TestGetChildTodayStatusPresent(t *testing.T) {
	svc, db := buildTodayStatusService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	checkIn := timezone.Now().Add(-90 * time.Minute)
	openAttendanceToday(t, db, chain.TenantID, chain.StudentID, checkIn)

	status, err := svc.GetChildTodayStatus(context.Background(), chain.AccountID, chain.StudentID)

	require.NoError(t, err)
	require.NotNil(t, status.AtOgs, "eine offene Anwesenheit belegt die Ja-Aussage")
	assert.True(t, *status.AtOgs)
	assert.Equal(t, parentService.DayStatePresent, status.State)
	assert.Equal(t, timezone.WallClock(checkIn).Format("15:04"), status.Since)
	assert.Empty(t, status.Until)
}

// TestGetChildTodayStatusWithoutAttendanceIsUnknown deckt den Rueckfall ab:
// ohne jede Anwesenheitszeile und ohne lesbaren Betreuungsplan trifft die
// Antwort keine Ja/Nein-Aussage, statt "nicht angekommen" zu behaupten und
// Eltern grundlos zu beunruhigen.
func TestGetChildTodayStatusWithoutAttendanceIsUnknown(t *testing.T) {
	svc, db := buildTodayStatusService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	status, err := svc.GetChildTodayStatus(context.Background(), chain.AccountID, chain.StudentID)

	require.NoError(t, err)
	assert.Nil(t, status.AtOgs, "ohne Beleg darf keine Ja/Nein-Aussage entstehen")
	assert.Equal(t, parentService.DayStateUnknown, status.State)
}

// TestGetChildTodayStatusCareDayWithoutAttendance ist der Fall, der Eltern am
// meisten interessiert: heute ist ein Betreuungstag, das Kind ist aber noch
// nicht da. Die Antwort darf jetzt "nein" sagen, weil ein Betreuungstag belegt
// ist, und nennt die erwartete Ankunftszeit.
func TestGetChildTodayStatusCareDayWithoutAttendance(t *testing.T) {
	svc, db := buildTodayStatusServiceWithSchedule(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	arrival := timezone.WallClock(time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC))
	seeded := seedArrivalScheduleForToday(t, db, chain.TenantID, chain.StudentID, arrival)

	status, err := svc.GetChildTodayStatus(context.Background(), chain.AccountID, chain.StudentID)

	require.NoError(t, err)
	if !seeded {
		assert.Empty(t, status.ExpectedFrom, "das Wochenende ist nie ein Betreuungstag")
		return
	}
	assert.Equal(t, "08:00", status.ExpectedFrom, "die erwartete Ankunft kommt aus dem Wochenplan")
}

// TestGetChildTodayStatusPresentOnCareDay haelt fest, dass eine offene
// Anwesenheit den Betreuungstag nicht verdraengt: beide Angaben stehen
// nebeneinander in derselben Antwort.
func TestGetChildTodayStatusPresentOnCareDay(t *testing.T) {
	svc, db := buildTodayStatusServiceWithSchedule(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	arrival := timezone.WallClock(time.Date(2026, 1, 1, 7, 30, 0, 0, time.UTC))
	seeded := seedArrivalScheduleForToday(t, db, chain.TenantID, chain.StudentID, arrival)
	checkIn := timezone.Now().Add(-30 * time.Minute)
	openAttendanceToday(t, db, chain.TenantID, chain.StudentID, checkIn)

	status, err := svc.GetChildTodayStatus(context.Background(), chain.AccountID, chain.StudentID)

	require.NoError(t, err)
	require.NotNil(t, status.AtOgs, "eine offene Anwesenheit belegt die Ja-Aussage")
	assert.True(t, *status.AtOgs)
	assert.Equal(t, parentService.DayStatePresent, status.State)
	if seeded {
		assert.Equal(t, "07:30", status.ExpectedFrom)
	}
}

// TestGetChildTodayStatusWithoutPlanIsNoCareDay: ein Kind ohne Eintrag fuer
// heute hat keinen Betreuungstag, und ohne Betreuungstag gibt es auch keine
// erwartete Ankunftszeit zu melden.
func TestGetChildTodayStatusWithoutPlanIsNoCareDay(t *testing.T) {
	svc, db := buildTodayStatusServiceWithSchedule(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	status, err := svc.GetChildTodayStatus(context.Background(), chain.AccountID, chain.StudentID)

	require.NoError(t, err)
	assert.Empty(t, status.ExpectedFrom, "ohne Wochenplan gibt es keine Ankunftszeit")
}
