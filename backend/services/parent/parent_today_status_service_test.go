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
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
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
