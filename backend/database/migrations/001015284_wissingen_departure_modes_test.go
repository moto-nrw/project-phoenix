package migrations

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

const wissingenDepartureTestFields = `[
	{"key":"schedule_pickup","type":"weekday_schedule","target":"schedule.pickup"},
	{"key":"darf_ihr_kind_alleine_nach_hause_gehen","type":"boolean"},
	{"key":"fahrt_ihr_kind_mit_dem_bus_nach_hause","type":"boolean"},
	{"key":"soll_ihr_kind_nur_gemeinsam_mit_einem_anderen_kind_gehen_durfen","type":"boolean"},
	{"key":"mit_welchem_kind_soll_ihr_kind_zusammen_gehen","type":"text"}
]`

type wissingenDepartureTestRow struct {
	AllowedDepartureModes  json.RawMessage `bun:"allowed_departure_modes"`
	DepartureDays          json.RawMessage `bun:"departure_days"`
	BusDays                json.RawMessage `bun:"bus_days"`
	PickupDays             json.RawMessage `bun:"pickup_days"`
	PickupStatus           *string         `bun:"pickup_status"`
	DepartureCompanionNote *string         `bun:"departure_companion_note"`
}

func insertWissingenDepartureTestSchema(t *testing.T, db *bun.DB, tenantID, createdBy int64, version int) int64 {
	t.Helper()
	var id int64
	require.NoError(t, db.QueryRowContext(context.Background(), `
		INSERT INTO enrollment.form_schemas
			(tenant_id, name, version, fields, core_requirements, legal_blocks, is_active, created_by)
		VALUES (?, 'Schuljahr 2026/2027', ?, ?::jsonb, '{}'::jsonb, '[]'::jsonb, false, ?)
		RETURNING id
	`, tenantID, version, wissingenDepartureTestFields, createdBy).Scan(&id))
	return id
}

func insertWissingenDepartureTestPhase(t *testing.T, db *bun.DB, tenantID, schemaID int64) int64 {
	t.Helper()
	var id int64
	require.NoError(t, db.QueryRowContext(context.Background(), `
		INSERT INTO enrollment.phases
			(tenant_id, name, service_start_date, service_end_date, form_schema_id)
		VALUES (?, 'Schuljahr 2026/2027', CURRENT_DATE, CURRENT_DATE + 365, ?)
		RETURNING id
	`, tenantID, schemaID).Scan(&id))
	return id
}

func insertWissingenDepartureTestChild(t *testing.T, db *bun.DB, tenantID, schemaID, phaseID, studentID int64, customData string) {
	t.Helper()
	ctx := context.Background()
	var requestID int64
	require.NoError(t, db.QueryRowContext(ctx, `
		INSERT INTO enrollment.requests
			(tenant_id, schema_id, phase_id, guardian_first_name, guardian_last_name, guardian_email, status_token)
		VALUES (?, ?, ?, 'Test', 'Guardian', ?, ?)
		RETURNING id
	`, tenantID, schemaID, phaseID,
		fmt.Sprintf("guardian-%d-%d@example.test", studentID, time.Now().UnixNano()),
		fmt.Sprintf("wissingen-departure-%d-%d", studentID, time.Now().UnixNano()),
	).Scan(&requestID))

	_, err := db.ExecContext(ctx, `
		INSERT INTO enrollment.request_children
			(tenant_id, request_id, first_name, last_name, date_of_birth, custom_data, status, created_student_id)
		VALUES (?, ?, 'Test', 'Child', CURRENT_DATE - 3650, ?::jsonb, 'approved', ?)
	`, tenantID, requestID, customData, studentID)
	require.NoError(t, err)
}

func loadWissingenDepartureTestRow(t *testing.T, db *bun.DB, studentID int64) wissingenDepartureTestRow {
	t.Helper()
	var row wissingenDepartureTestRow
	require.NoError(t, db.NewRaw(`
		SELECT allowed_departure_modes, departure_days, bus_days, pickup_days,
			pickup_status, departure_companion_note
		FROM users.students
		WHERE id = ?
	`, studentID).Scan(context.Background(), &row))
	return row
}

func assertWissingenDepartureJSON(t *testing.T, expected string, actual json.RawMessage) {
	t.Helper()
	assert.JSONEq(t, expected, string(actual))
}

func assertWissingenPickupStatus(t *testing.T, expected string, actual *string) {
	t.Helper()
	require.NotNil(t, actual)
	assert.Equal(t, expected, *actual)
}

func TestWissingenDepartureModes_DerivesConfirmedAnswersAndPreservesExistingPlans(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	require.NoError(t, studentsDepartureAccompaniedUp(ctx, db))

	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("wissingen-departure-%d@example.test", time.Now().UnixNano()))
	t.Cleanup(func() { testpkg.CleanupAccount(t, db, account.ID) })
	tenantID, _ := testpkg.CreateTestTenant(t, db)
	otherTenantID, _ := testpkg.CreateTestTenant(t, db)
	t.Cleanup(func() {
		testpkg.CleanupTenantTestData(t, db, tenantID, otherTenantID)
		testpkg.CleanupTestTenant(t, db, tenantID, otherTenantID)
	})
	_, err := db.NewRaw(`
		UPDATE platform.schools
		SET slug = 'ogs-wissingen', subdomain = 'ogs-wissingen'
		WHERE id = ?
	`, tenantID).Exec(ctx)
	require.NoError(t, err)
	_, err = db.NewRaw(`UPDATE platform.schools SET slug = 'ogs-wissingen' WHERE id = ?`, otherTenantID).Exec(ctx)
	require.NoError(t, err)

	schemaID := insertWissingenDepartureTestSchema(t, db, tenantID, account.ID, 20)
	phaseID := insertWissingenDepartureTestPhase(t, db, tenantID, schemaID)

	pickup := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Pickup", "Child", "1a")
	insertWissingenDepartureTestChild(t, db, tenantID, schemaID, phaseID, pickup.ID, `{
		"schedule_pickup":{"mon":"15:00","thu":"16:00","fri":""},
		"darf_ihr_kind_alleine_nach_hause_gehen":false,
		"fahrt_ihr_kind_mit_dem_bus_nach_hause":true,
		"soll_ihr_kind_nur_gemeinsam_mit_einem_anderen_kind_gehen_durfen":true,
		"mit_welchem_kind_soll_ihr_kind_zusammen_gehen":"Wird wegen Abholung ignoriert"
	}`)

	alone := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Alone", "Child", "1a")
	insertWissingenDepartureTestChild(t, db, tenantID, schemaID, phaseID, alone.ID, `{
		"schedule_pickup":{"mon":"15:00","wed":"15:00"},
		"darf_ihr_kind_alleine_nach_hause_gehen":true,
		"fahrt_ihr_kind_mit_dem_bus_nach_hause":false,
		"soll_ihr_kind_nur_gemeinsam_mit_einem_anderen_kind_gehen_durfen":false
	}`)

	bus := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Bus", "Child", "1a")
	insertWissingenDepartureTestChild(t, db, tenantID, schemaID, phaseID, bus.ID, `{
		"schedule_pickup":{"tue":"15:00"},
		"darf_ihr_kind_alleine_nach_hause_gehen":true,
		"fahrt_ihr_kind_mit_dem_bus_nach_hause":true
	}`)

	accompanied := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Accompanied", "Child", "1a")
	insertWissingenDepartureTestChild(t, db, tenantID, schemaID, phaseID, accompanied.ID, `{
		"schedule_pickup":{"fri":"15:00"},
		"darf_ihr_kind_alleine_nach_hause_gehen":true,
		"fahrt_ihr_kind_mit_dem_bus_nach_hause":false,
		"soll_ihr_kind_nur_gemeinsam_mit_einem_anderen_kind_gehen_durfen":true,
		"mit_welchem_kind_soll_ihr_kind_zusammen_gehen":"  Begleitkind  "
	}`)

	busAndAccompanied := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Combined", "Child", "1a")
	insertWissingenDepartureTestChild(t, db, tenantID, schemaID, phaseID, busAndAccompanied.ID, `{
		"schedule_pickup":{"mon":"15:00","thu":"16:00"},
		"darf_ihr_kind_alleine_nach_hause_gehen":true,
		"fahrt_ihr_kind_mit_dem_bus_nach_hause":true,
		"soll_ihr_kind_nur_gemeinsam_mit_einem_anderen_kind_gehen_durfen":true,
		"mit_welchem_kind_soll_ihr_kind_zusammen_gehen":"Begleitkind"
	}`)

	manual := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Manual", "Child", "1a")
	insertWissingenDepartureTestChild(t, db, tenantID, schemaID, phaseID, manual.ID, `{
		"schedule_pickup":{"mon":"15:00","wed":"15:00"},
		"darf_ihr_kind_alleine_nach_hause_gehen":false
	}`)
	_, err = db.NewRaw(`
		UPDATE users.students
		SET allowed_departure_modes = '{"mon":["alone"],"wed":["alone"]}'::jsonb,
			pickup_status = 'Geht alleine nach Hause'
		WHERE id = ?
	`, manual.ID).Exec(ctx)
	require.NoError(t, err)

	duplicate := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Duplicate", "Child", "1a")
	duplicateData := `{
		"schedule_pickup":{"tue":"15:00"},
		"darf_ihr_kind_alleine_nach_hause_gehen":false
	}`
	insertWissingenDepartureTestChild(t, db, tenantID, schemaID, phaseID, duplicate.ID, duplicateData)
	insertWissingenDepartureTestChild(t, db, tenantID, schemaID, phaseID, duplicate.ID, `{
		"schedule_pickup":"malformed",
		"darf_ihr_kind_alleine_nach_hause_gehen":"malformed"
	}`)

	invalidTime := testpkg.CreateTestStudentForTenant(t, db, tenantID, "InvalidTime", "Child", "1a")
	insertWissingenDepartureTestChild(t, db, tenantID, schemaID, phaseID, invalidTime.ID, `{
		"schedule_pickup":{"mon":"15:00","tue":"25:99"},
		"darf_ihr_kind_alleine_nach_hause_gehen":false
	}`)

	unknownWeekday := testpkg.CreateTestStudentForTenant(t, db, tenantID, "UnknownWeekday", "Child", "1a")
	insertWissingenDepartureTestChild(t, db, tenantID, schemaID, phaseID, unknownWeekday.ID, `{
		"schedule_pickup":{"mon":"15:00","sat":"15:00"},
		"darf_ihr_kind_alleine_nach_hause_gehen":false
	}`)

	malformedSchedule := testpkg.CreateTestStudentForTenant(t, db, tenantID, "MalformedSchedule", "Child", "1a")
	insertWissingenDepartureTestChild(t, db, tenantID, schemaID, phaseID, malformedSchedule.ID, `{
		"schedule_pickup":"malformed",
		"darf_ihr_kind_alleine_nach_hause_gehen":false
	}`)

	require.NoError(t, wissingenDepartureModesUp(ctx, db))
	require.NoError(t, wissingenDepartureModesUp(ctx, db), "migration must be idempotent")

	pickupRow := loadWissingenDepartureTestRow(t, db, pickup.ID)
	assertWissingenDepartureJSON(t, `{"mon":["pickup"],"thu":["pickup"]}`, pickupRow.AllowedDepartureModes)
	assertWissingenDepartureJSON(t, `{"mon":"pickup","thu":"pickup"}`, pickupRow.DepartureDays)
	assertWissingenDepartureJSON(t, `{}`, pickupRow.BusDays)
	assertWissingenDepartureJSON(t, `{"mon":true,"thu":true}`, pickupRow.PickupDays)
	assertWissingenPickupStatus(t, "Wird abgeholt", pickupRow.PickupStatus)
	assert.Nil(t, pickupRow.DepartureCompanionNote)

	aloneRow := loadWissingenDepartureTestRow(t, db, alone.ID)
	assertWissingenDepartureJSON(t, `{"mon":["alone"],"wed":["alone"]}`, aloneRow.AllowedDepartureModes)
	assertWissingenDepartureJSON(t, `{}`, aloneRow.DepartureDays)
	assertWissingenDepartureJSON(t, `{}`, aloneRow.BusDays)
	assertWissingenDepartureJSON(t, `{}`, aloneRow.PickupDays)
	assertWissingenPickupStatus(t, "Geht alleine nach Hause", aloneRow.PickupStatus)

	busRow := loadWissingenDepartureTestRow(t, db, bus.ID)
	assertWissingenDepartureJSON(t, `{"tue":["alone","bus"]}`, busRow.AllowedDepartureModes)
	assertWissingenDepartureJSON(t, `{"tue":"bus"}`, busRow.DepartureDays)
	assertWissingenDepartureJSON(t, `{"tue":true}`, busRow.BusDays)
	assertWissingenPickupStatus(t, "Geht alleine nach Hause", busRow.PickupStatus)

	accompaniedRow := loadWissingenDepartureTestRow(t, db, accompanied.ID)
	assertWissingenDepartureJSON(t, `{"fri":["alone","accompanied"]}`, accompaniedRow.AllowedDepartureModes)
	assertWissingenDepartureJSON(t, `{"fri":"accompanied"}`, accompaniedRow.DepartureDays)
	assertWissingenPickupStatus(t, "Geht mit anderem Kind", accompaniedRow.PickupStatus)
	require.NotNil(t, accompaniedRow.DepartureCompanionNote)
	assert.Equal(t, "Begleitkind", *accompaniedRow.DepartureCompanionNote)

	combinedRow := loadWissingenDepartureTestRow(t, db, busAndAccompanied.ID)
	assertWissingenDepartureJSON(t, `{"mon":["alone","bus","accompanied"],"thu":["alone","bus","accompanied"]}`, combinedRow.AllowedDepartureModes)
	assertWissingenDepartureJSON(t, `{"mon":"bus","thu":"bus"}`, combinedRow.DepartureDays)
	assertWissingenDepartureJSON(t, `{"mon":true,"thu":true}`, combinedRow.BusDays)
	assertWissingenPickupStatus(t, "Geht mit anderem Kind", combinedRow.PickupStatus)

	manualRow := loadWissingenDepartureTestRow(t, db, manual.ID)
	assertWissingenDepartureJSON(t, `{"mon":["alone"],"wed":["alone"]}`, manualRow.AllowedDepartureModes)
	assertWissingenPickupStatus(t, "Geht alleine nach Hause", manualRow.PickupStatus)

	duplicateRow := loadWissingenDepartureTestRow(t, db, duplicate.ID)
	assertWissingenDepartureJSON(t, `{}`, duplicateRow.AllowedDepartureModes)
	assert.Nil(t, duplicateRow.PickupStatus)

	invalidTimeRow := loadWissingenDepartureTestRow(t, db, invalidTime.ID)
	assertWissingenDepartureJSON(t, `{}`, invalidTimeRow.AllowedDepartureModes)
	assert.Nil(t, invalidTimeRow.PickupStatus)

	unknownWeekdayRow := loadWissingenDepartureTestRow(t, db, unknownWeekday.ID)
	assertWissingenDepartureJSON(t, `{}`, unknownWeekdayRow.AllowedDepartureModes)
	assert.Nil(t, unknownWeekdayRow.PickupStatus)

	malformedScheduleRow := loadWissingenDepartureTestRow(t, db, malformedSchedule.ID)
	assertWissingenDepartureJSON(t, `{}`, malformedScheduleRow.AllowedDepartureModes)
	assert.Nil(t, malformedScheduleRow.PickupStatus)
}
