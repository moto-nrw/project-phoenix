package integration

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	importModels "github.com/moto-nrw/project-phoenix/models/import"
	"github.com/moto-nrw/project-phoenix/services"
	importService "github.com/moto-nrw/project-phoenix/services/import"
	importPorts "github.com/moto-nrw/project-phoenix/services/import/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Cutover tests for the data import (#2708): every accepted row is committed
// through the owner commands of People Directory, School Membership, Care
// Plan, Student Presence and the Audit platform. The assertions read the
// owner tables directly so they prove what was persisted, not what the
// import reports.

type importCounts struct {
	persons, students, guardians, links, phones, consents, arrivals, pickups, audits, consentHistory int
}

func countImportRows(t *testing.T, db *bun.DB, tenantID int64) importCounts {
	t.Helper()
	ctx := context.Background()
	count := func(table string) int {
		var n int
		require.NoError(t, db.NewSelect().TableExpr(table).ColumnExpr("count(*)").Where("tenant_id = ?", tenantID).Scan(ctx, &n))
		return n
	}
	return importCounts{
		persons: count("users.persons"), students: count("users.students"), guardians: count("users.guardian_profiles"),
		links: count("users.students_guardians"), phones: count("users.guardian_phone_numbers"), consents: count("users.privacy_consents"),
		arrivals: count("schedule.student_arrival_schedules"), pickups: count("schedule.student_pickup_schedules"),
		audits: count("audit.data_imports"), consentHistory: count("audit.student_consent_changes"),
	}
}

// importer carries the two identities an import run needs: the staff row
// that owns created schedules and the account the GDPR audit record names.
type importer struct{ staffID, accountID int64 }

func newImporter(t *testing.T, db *bun.DB) importer {
	t.Helper()
	staff, account := testpkg.CreateTestStaffWithAccount(t, db, "Import", "Admin")
	return importer{staffID: staff.ID, accountID: account.ID}
}

func runStudentImport(t *testing.T, db *bun.DB, module services.ImportTestModule, actor importer, mode importModels.ImportMode, rows []importModels.StudentImportRow, after func(context.Context, *importModels.ImportResult[importModels.StudentImportRow]) error) (*importModels.ImportResult[importModels.StudentImportRow], error) {
	t.Helper()
	var result *importModels.ImportResult[importModels.StudentImportRow]
	err := testpkg.WithTenantTx(t, testpkg.Ctx(t), db, testpkg.Tenant(t), func(ctx context.Context, _ bun.Tx) error {
		var err error
		result, err = module.Import.Import(ctx, importModels.ImportRequest[importModels.StudentImportRow]{
			Rows: rows, Mode: mode, UserID: actor.staffID, SkipInvalidRows: true,
		})
		if err != nil {
			return err
		}
		if err := module.Import.RecordAuditInTransaction(ctx, "student", "cutover.csv", result, actor.accountID, false, testpkg.Tenant(t)); err != nil {
			return err
		}
		if after != nil {
			return after(ctx, result)
		}
		return nil
	})
	return result, err
}

func requireNoRowErrors(t *testing.T, result *importModels.ImportResult[importModels.StudentImportRow]) {
	t.Helper()
	for _, rowErr := range result.Errors {
		for _, e := range rowErr.Errors {
			assert.NotEqual(t, importModels.ErrorSeverityError, e.Severity, "row %d: %s", rowErr.RowNumber, e.Message)
		}
	}
	require.Zero(t, result.ErrorCount)
}

func TestDataImportCutover_StudentCreateUpdateAndReplay(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.OwnTenant(t)
	module, err := services.NewImportTestModule(db, testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	actor := newImporter(t, db)
	card := testpkg.CreateTestRFIDCard(t, db, "CUT")
	guardianEmail := fmt.Sprintf("erz.%s@example.test", card.ID)
	ctx := testpkg.Ctx(t)

	row := importModels.StudentImportRow{
		FirstName: "Max", LastName: "Cutover", SchoolClass: "1A", Birthday: "2018-02-03", TagID: card.ID,
		AddressStreet: "Kinderweg 3", AddressPostalCode: "50667", AddressCity: "Köln",
		DepartureDays: map[string]string{"mon": "bus", "tue": "pickup", "wed": "alone"},
		Guardians: []importModels.GuardianImportData{{
			FirstName: "Maria", LastName: "Cutover", Email: guardianEmail, RelationshipType: "Mutter",
			GuardianRole: "Nur Abholung", PickupNotes: "nur dienstags", EmergencyPriority: 2, CanPickup: true,
			PhoneNumbers: []importModels.PhoneImportData{{PhoneNumber: "0171 1234567", PhoneType: "mobile", IsPrimary: true}},
		}},
		ArrivalSchedules:  []importModels.ArrivalScheduleImportData{{Weekday: 1, ExpectedArrival: "08:00"}},
		PickupSchedules:   []importModels.PickupScheduleImportData{{Weekday: 2, PickupTime: "15:30", Notes: "Oma"}},
		AGBAcceptedAt:     "2026-08-01",
		PrivacyAccepted:   true,
		DataRetentionDays: 14,
	}
	before := countImportRows(t, db, tenantID)

	result, err := runStudentImport(t, db, module, actor, importModels.ImportModeCreate, []importModels.StudentImportRow{row}, nil)
	require.NoError(t, err)
	requireNoRowErrors(t, result)
	assert.Equal(t, 1, result.CreatedCount)

	after := countImportRows(t, db, tenantID)
	assert.Equal(t, before.persons+1, after.persons)
	assert.Equal(t, before.students+1, after.students)
	assert.Equal(t, before.guardians+1, after.guardians)
	assert.Equal(t, before.links+1, after.links)
	assert.Equal(t, before.phones+1, after.phones)
	assert.Equal(t, before.consents+1, after.consents)
	assert.Equal(t, before.arrivals+1, after.arrivals)
	assert.Equal(t, before.pickups+1, after.pickups)
	assert.Equal(t, before.audits+1, after.audits, "the GDPR import record is committed with the batch")
	assert.Equal(t, before.consentHistory+1, after.consentHistory, "the AGB consent date is recorded in the history")

	var student struct {
		ID                int64   `bun:"id"`
		PersonID          int64   `bun:"person_id"`
		SchoolClass       string  `bun:"school_class"`
		AddressStreet     *string `bun:"address_street"`
		AddressCity       *string `bun:"address_city"`
		PickupStatus      *string `bun:"pickup_status"`
		DepartureDays     string  `bun:"departure_days"`
		Status            string  `bun:"status"`
		AGBAcceptedAtNull bool    `bun:"agb_null"`
	}
	require.NoError(t, db.NewSelect().TableExpr("users.students AS s").
		ColumnExpr("s.id, s.person_id, s.school_class, s.address_street, s.address_city, s.pickup_status, s.departure_days::text AS departure_days, s.status, s.agb_accepted_at IS NULL AS agb_null").
		Join("JOIN users.persons p ON p.id = s.person_id").Where("s.tenant_id = ? AND p.tag_id = ?", tenantID, card.ID).Scan(ctx, &student))
	assert.Equal(t, "1A", student.SchoolClass)
	assert.Equal(t, "Kinderweg 3", *student.AddressStreet)
	assert.Equal(t, "active", student.Status)
	assert.False(t, student.AGBAcceptedAtNull)
	assert.Contains(t, student.DepartureDays, `"mon": "bus"`)
	assert.Contains(t, student.DepartureDays, `"tue": "pickup"`)
	var link struct {
		GuardianRole      string  `bun:"guardian_role"`
		PickupNotes       *string `bun:"pickup_notes"`
		EmergencyPriority int     `bun:"emergency_priority"`
		Permissions       string  `bun:"permissions"`
	}
	require.NoError(t, db.NewSelect().TableExpr("users.students_guardians").ColumnExpr("guardian_role, pickup_notes, emergency_priority, permissions::text AS permissions").
		Where("tenant_id = ? AND student_id = ?", tenantID, student.ID).Scan(ctx, &link))
	assert.Equal(t, authorize.GuardianRolePickupOnly, link.GuardianRole)
	assert.Equal(t, "nur dienstags", *link.PickupNotes)
	assert.Equal(t, 2, link.EmergencyPriority)
	assert.NotContains(t, link.Permissions, authorize.GuardianPermissionPortalAccess, "pickup-only presets grant no portal access")
	var retention int
	require.NoError(t, db.NewSelect().TableExpr("users.privacy_consents").Column("data_retention_days").Where("tenant_id = ? AND student_id = ?", tenantID, student.ID).Scan(ctx, &retention))
	assert.Equal(t, 14, retention)

	// Replay of the same file in create mode: every row is a duplicate, no
	// owner row is written twice.
	result, err = runStudentImport(t, db, module, actor, importModels.ImportModeCreate, []importModels.StudentImportRow{row}, nil)
	require.NoError(t, err)
	assert.Equal(t, 0, result.CreatedCount)
	assert.Equal(t, 1, result.ErrorCount)
	replayed := countImportRows(t, db, tenantID)
	replayed.audits = after.audits
	assert.Equal(t, after, replayed, "a replayed batch is idempotent")

	// Upsert of the same file: the card resolves the child, nothing is
	// duplicated and the guardian link stays one.
	result, err = runStudentImport(t, db, module, actor, importModels.ImportModeUpsert, []importModels.StudentImportRow{row}, nil)
	require.NoError(t, err)
	requireNoRowErrors(t, result)
	assert.Equal(t, 1, result.UpdatedCount)
	upserted := countImportRows(t, db, tenantID)
	upserted.audits = after.audits
	assert.Equal(t, after, upserted, "an upsert replay is idempotent")

	// Update across a class change, matched via the card; only given cells
	// change, the guardian is merged and its role preset patched.
	update := importModels.StudentImportRow{
		FirstName: "Max", LastName: "Cutover", SchoolClass: "2A", TagID: card.ID, AddressCity: "Bonn",
		Guardians:         []importModels.GuardianImportData{{Email: guardianEmail, GuardianRole: "Sorgeberechtigt"}},
		PickupSchedules:   []importModels.PickupScheduleImportData{{Weekday: 2, PickupTime: "16:00"}, {Weekday: 4, PickupTime: "15:00"}},
		DataRetentionDays: 14,
	}
	result, err = runStudentImport(t, db, module, actor, importModels.ImportModeUpdate, []importModels.StudentImportRow{update}, nil)
	require.NoError(t, err)
	requireNoRowErrors(t, result)
	assert.Equal(t, 1, result.UpdatedCount)
	require.NoError(t, db.NewSelect().TableExpr("users.students AS s").
		ColumnExpr("s.id, s.person_id, s.school_class, s.address_street, s.address_city, s.pickup_status, s.departure_days::text AS departure_days, s.status, s.agb_accepted_at IS NULL AS agb_null").
		Where("s.tenant_id = ? AND s.id = ?", tenantID, student.ID).Scan(ctx, &student))
	assert.Equal(t, "2A", student.SchoolClass)
	assert.Equal(t, "Bonn", *student.AddressCity)
	assert.Equal(t, "Kinderweg 3", *student.AddressStreet, "empty cell keeps the street")
	assert.Contains(t, student.DepartureDays, `"mon": "bus"`, "the plan survives an update without Gehweise columns")
	require.NoError(t, db.NewSelect().TableExpr("users.students_guardians").ColumnExpr("guardian_role, pickup_notes, emergency_priority, permissions::text AS permissions").
		Where("tenant_id = ? AND student_id = ?", tenantID, student.ID).Scan(ctx, &link))
	assert.Equal(t, authorize.GuardianRoleLegalGuardian, link.GuardianRole)
	assert.Equal(t, "nur dienstags", *link.PickupNotes)
	updated := countImportRows(t, db, tenantID)
	assert.Equal(t, after.links, updated.links, "existing guardian is merged, not duplicated")
	assert.Equal(t, after.pickups+1, updated.pickups, "one pickup day updated, one added")
	var pickupHours []int
	require.NoError(t, db.NewSelect().TableExpr("schedule.student_pickup_schedules").ColumnExpr("extract(hour from pickup_time)::int").
		Where("tenant_id = ? AND student_id = ?", tenantID, student.ID).OrderExpr("weekday").Scan(ctx, &pickupHours))
	assert.Equal(t, []int{16, 15}, pickupHours)
}

// Phone-only guardians (no e-mail) are recognised on re-import by their stored
// number, then by a unique name, so a second file never duplicates them.
func TestDataImportCutover_ReusesPhoneOnlyGuardian(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.OwnTenant(t)
	module, err := services.NewImportTestModule(db, testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	actor := newImporter(t, db)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano()%1_000_000)
	phone := "0151 " + suffix
	ctx := testpkg.Ctx(t)

	row := importModels.StudentImportRow{
		FirstName: "Lena", LastName: "Ohnemail" + suffix, SchoolClass: "1B", Birthday: "2018-05-06",
		Guardians: []importModels.GuardianImportData{{
			FirstName: "Karin", LastName: "Ohnemail", RelationshipType: "Mutter",
			PhoneNumbers: []importModels.PhoneImportData{{PhoneNumber: phone, PhoneType: "mobile", IsPrimary: true}},
		}},
		PrivacyAccepted: true, DataRetentionDays: 30,
	}
	result, err := runStudentImport(t, db, module, actor, importModels.ImportModeCreate, []importModels.StudentImportRow{row}, nil)
	require.NoError(t, err)
	requireNoRowErrors(t, result)
	before := countImportRows(t, db, tenantID)

	update := row
	update.Guardians = []importModels.GuardianImportData{{
		FirstName: "Karin", LastName: "Ohnemail", GuardianRole: "Sorgeberechtigt",
		PhoneNumbers: []importModels.PhoneImportData{{PhoneNumber: "0151-" + suffix, PhoneType: "mobile"}},
	}}
	result, err = runStudentImport(t, db, module, actor, importModels.ImportModeUpdate, []importModels.StudentImportRow{update}, nil)
	require.NoError(t, err)
	requireNoRowErrors(t, result)
	after := countImportRows(t, db, tenantID)
	assert.Equal(t, before.guardians, after.guardians, "phone-only guardian is merged, not duplicated")
	assert.Equal(t, before.links, after.links)
	assert.Equal(t, before.phones+1, after.phones, "differently formatted number is stored as given; the profile stays one")
	assert.Equal(t, before.consents, after.consents, "an existing consent is never rewritten")
	var role string
	require.NoError(t, db.NewSelect().TableExpr("users.students_guardians").Column("guardian_role").Where("tenant_id = ?", tenantID).OrderExpr("id DESC").Limit(1).Scan(ctx, &role))
	assert.Equal(t, authorize.GuardianRoleLegalGuardian, role)

	// A row whose numbers match nothing stored still resolves the guardian by
	// its unique name among the child's own relationships.
	byName := row
	byName.Guardians = []importModels.GuardianImportData{{
		FirstName: "Karin", LastName: "Ohnemail", PickupNotes: "ab 15 Uhr",
		PhoneNumbers: []importModels.PhoneImportData{{PhoneNumber: "0202 " + suffix, PhoneType: "work"}},
	}}
	result, err = runStudentImport(t, db, module, actor, importModels.ImportModeUpdate, []importModels.StudentImportRow{byName}, nil)
	require.NoError(t, err)
	requireNoRowErrors(t, result)
	assert.Equal(t, after.guardians, countImportRows(t, db, tenantID).guardians)
	var notes string
	require.NoError(t, db.NewSelect().TableExpr("users.students_guardians").Column("pickup_notes").Where("tenant_id = ?", tenantID).OrderExpr("id DESC").Limit(1).Scan(ctx, &notes))
	assert.Equal(t, "ab 15 Uhr", notes)
}

// The import resolves an existing guardian by the exact e-mail, however many
// other guardians contain that address and sort before it: the legacy lookup
// compared LOWER(email) without a page limit.
func TestDataImportCutover_FindsGuardianByExactEmailAmongSubstringMatches(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.OwnTenant(t)
	module, err := services.NewImportTestModule(db, testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	actor := newImporter(t, db)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano()%1_000_000)
	target := "k" + suffix + "@import.test"
	setEmail := func(id int64, email string) {
		t.Helper()
		_, err := db.NewUpdate().TableExpr("users.guardian_profiles").Set("email = ?", email).Where("id = ?", id).Exec(context.Background())
		require.NoError(t, err)
	}
	for i := range 201 {
		other := testpkg.CreateTestGuardianProfileForTenant(t, db, tenantID, "Anna", "Aaron", "substring")
		setEmail(other.ID, fmt.Sprintf("x%03d%s", i, target))
	}
	existing := testpkg.CreateTestGuardianProfileForTenant(t, db, tenantID, "Karin", "Zimmer", "exact")
	setEmail(existing.ID, target)
	before := countImportRows(t, db, tenantID)

	row := importModels.StudentImportRow{
		FirstName: "Mila", LastName: "Mailtreffer" + suffix, SchoolClass: "2A", Birthday: "2017-03-04",
		Guardians: []importModels.GuardianImportData{{
			FirstName: "Karin", LastName: "Zimmer", Email: strings.ToUpper(target), RelationshipType: "Mutter",
		}},
		PrivacyAccepted: true, DataRetentionDays: 30,
	}
	result, err := runStudentImport(t, db, module, actor, importModels.ImportModeCreate, []importModels.StudentImportRow{row}, nil)
	require.NoError(t, err)
	requireNoRowErrors(t, result)

	after := countImportRows(t, db, tenantID)
	assert.Equal(t, before.guardians, after.guardians, "the existing guardian is reused, not duplicated")
	var linked int64
	require.NoError(t, db.NewSelect().TableExpr("users.students_guardians").Column("guardian_profile_id").
		Where("tenant_id = ?", tenantID).OrderExpr("id DESC").Limit(1).Scan(context.Background(), &linked))
	assert.Equal(t, existing.ID, linked)
}

// A batch is validated completely before any owner command runs, a row that
// fails during the write phase is rolled back to its savepoint while the
// other rows survive, and a failure after the last owner command (the audit
// record) rolls back the whole batch across every owner.
func TestDataImportCutover_BatchValidationRowSavepointAndRollback(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.OwnTenant(t)
	failLinkFor := ""
	module, err := services.NewImportTestModuleWithOptions(db, testpkg.TenantRuntime(t, db), services.ImportTestOptions{
		WrapGuardians: func(inner importPorts.GuardianDirectory) importPorts.GuardianDirectory {
			return failingGuardians{GuardianDirectory: inner, failFor: &failLinkFor}
		},
	})
	require.NoError(t, err)
	actor := newImporter(t, db)
	otherTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenant)
	foreign := testpkg.CreateTestStudentForTenant(t, db, otherTenant, "Fremd", "Schule", "1a")
	foreignBefore := countImportRows(t, db, otherTenant)

	valid := func(name string) importModels.StudentImportRow {
		return importModels.StudentImportRow{FirstName: name, LastName: "Batch", SchoolClass: "3C", Birthday: "2017-01-02", DataRetentionDays: 30,
			Guardians: []importModels.GuardianImportData{{FirstName: "Eltern", LastName: name, Email: fmt.Sprintf("%s.batch@example.test", name), RelationshipType: "Vater"}}}
	}
	invalid := importModels.StudentImportRow{FirstName: "", LastName: "Batch", SchoolClass: "3C", DataRetentionDays: 30}

	// The People Directory link command refuses one row after its person,
	// student and guardian profile were already written. The row rolls back
	// to its savepoint; the rest of the batch survives.
	failLinkFor = "Klaus.batch@example.test"
	before := countImportRows(t, db, tenantID)
	result, err := runStudentImport(t, db, module, actor, importModels.ImportModeCreate, []importModels.StudentImportRow{valid("Karla"), invalid, valid("Klaus"), valid("Ben")}, nil)
	require.NoError(t, err)
	failLinkFor = ""
	assert.Equal(t, 2, result.CreatedCount)
	assert.Equal(t, 2, result.ErrorCount)
	codes := map[int][]string{}
	for _, rowErr := range result.Errors {
		for _, e := range rowErr.Errors {
			codes[rowErr.RowNumber] = append(codes[rowErr.RowNumber], e.Code)
		}
	}
	assert.Contains(t, codes[3], "required", "the empty first name is rejected during validation")
	assert.Contains(t, codes[4], "creation_failed", "the refused owner command becomes a stable row error")
	after := countImportRows(t, db, tenantID)
	assert.Equal(t, before.persons+2, after.persons, "the failed row's person does not survive its savepoint rollback")
	assert.Equal(t, before.students+2, after.students)
	assert.Equal(t, before.guardians+2, after.guardians, "and neither does its guardian profile")
	assert.Equal(t, before.links+2, after.links)

	// Failure after the last owner command: the audit append fails, the
	// batch rolls back across People Directory, Care Plan and Student
	// Presence, and stable row-level errors remain readable.
	injected := errors.New("audit sink unavailable")
	rollbackRow := valid("Carla")
	rollbackRow.PrivacyAccepted, rollbackRow.DataRetentionDays = true, 30
	rollbackRow.PickupSchedules = []importModels.PickupScheduleImportData{{Weekday: 1, PickupTime: "15:00"}}
	var observed *importModels.ImportResult[importModels.StudentImportRow]
	_, err = runStudentImport(t, db, module, actor, importModels.ImportModeCreate, []importModels.StudentImportRow{rollbackRow, invalid}, func(ctx context.Context, result *importModels.ImportResult[importModels.StudentImportRow]) error {
		observed = result
		pending := countImportRowsInTx(t, ctx, tenantID)
		assert.Equal(t, after.students+1, pending.students, "the owner writes are pending inside the batch transaction")
		assert.Equal(t, after.consents+1, pending.consents)
		assert.Equal(t, after.pickups+1, pending.pickups)
		return injected
	})
	require.ErrorIs(t, err, injected)
	require.NotNil(t, observed)
	assert.Equal(t, 1, observed.CreatedCount)
	assert.Equal(t, 1, observed.ErrorCount)
	assert.Equal(t, 3, observed.Errors[0].RowNumber, "row numbers stay stable across the rollback")
	assert.Equal(t, after, countImportRows(t, db, tenantID), "the whole batch rolled back across every owner")

	// The retry of the same batch succeeds and the foreign tenant is untouched.
	result, err = runStudentImport(t, db, module, actor, importModels.ImportModeCreate, []importModels.StudentImportRow{rollbackRow}, nil)
	require.NoError(t, err)
	requireNoRowErrors(t, result)
	assert.Equal(t, 1, result.CreatedCount)
	retried := countImportRows(t, db, tenantID)
	assert.Equal(t, after.students+1, retried.students)
	assert.Equal(t, after.consents+1, retried.consents)
	assert.Equal(t, foreignBefore, countImportRows(t, db, otherTenant), "the other tenant's rows are untouched")
	var foreignClass string
	require.NoError(t, db.NewSelect().TableExpr("users.students").Column("school_class").Where("id = ?", foreign.ID).Scan(context.Background(), &foreignClass))
	assert.Equal(t, "1a", foreignClass)

	// The same name in the other tenant is not a duplicate for this tenant:
	// tenant RLS scopes every owner lookup.
	crossTenant := importModels.StudentImportRow{FirstName: "Fremd", LastName: "Schule", SchoolClass: "1a", Birthday: "2017-03-04", DataRetentionDays: 30}
	result, err = runStudentImport(t, db, module, actor, importModels.ImportModeCreate, []importModels.StudentImportRow{crossTenant}, nil)
	require.NoError(t, err)
	requireNoRowErrors(t, result)
	assert.Equal(t, 1, result.CreatedCount)
	assert.Equal(t, foreignBefore, countImportRows(t, db, otherTenant))

	// Runtime evidence: every run reported its counters without names.
	require.NotEmpty(t, *module.Observations)
	last := (*module.Observations)[len(*module.Observations)-1]
	assert.Equal(t, "student", last.Entity)
	assert.Equal(t, 1, last.Rows)
	assert.Equal(t, 1, last.Created)
	assert.False(t, last.DryRun)
}

// failingGuardians refuses to link the guardian whose e-mail the test names,
// so an owner command fails after earlier owners already wrote their rows.
type failingGuardians struct {
	importPorts.GuardianDirectory
	failFor *string
}

func (g failingGuardians) LinkGuardianToStudent(ctx context.Context, input importPorts.LinkGuardian) (importPorts.GuardianLink, error) {
	if *g.failFor != "" {
		guardians, err := g.ListGuardiansByID(ctx, []int64{input.GuardianProfileID})
		if err != nil {
			return importPorts.GuardianLink{}, err
		}
		for _, guardian := range guardians {
			if guardian.Email != nil && strings.EqualFold(*guardian.Email, *g.failFor) {
				return importPorts.GuardianLink{}, errors.New("guardian link refused by the owner")
			}
		}
	}
	return g.GuardianDirectory.LinkGuardianToStudent(ctx, input)
}

func countImportRowsInTx(t *testing.T, ctx context.Context, tenantID int64) importCounts {
	t.Helper()
	raw, ok := tenant.TransactionFromContext(ctx)
	require.True(t, ok)
	tx, ok := raw.(bun.Tx)
	require.True(t, ok)
	count := func(table string) int {
		var n int
		require.NoError(t, tx.NewSelect().TableExpr(table).ColumnExpr("count(*)").Where("tenant_id = ?", tenantID).Scan(ctx, &n))
		return n
	}
	return importCounts{
		persons: count("users.persons"), students: count("users.students"), guardians: count("users.guardian_profiles"),
		links: count("users.students_guardians"), phones: count("users.guardian_phone_numbers"), consents: count("users.privacy_consents"),
		arrivals: count("schedule.student_arrival_schedules"), pickups: count("schedule.student_pickup_schedules"),
		audits: count("audit.data_imports"), consentHistory: count("audit.student_consent_changes"),
	}
}

// The staff import files a full Stammdatensatz through the owners: People
// Directory writes the person, School Membership the staff and caregiver
// rows, Workforce the master data and qualifications, and the update patches
// only the cells the file carries.
func TestDataImportCutover_StaffStammdatenThroughOwners(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.OwnTenant(t)
	module, err := services.NewImportTestModule(db, testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	actor := newImporter(t, db)
	role := testpkg.CreateTestRoleForTenant(t, db, "Betreuungskraft", tenantID)
	ctx := testpkg.Ctx(t)

	row := importModels.StaffImportRow{
		FirstName: "Anna", LastName: "Cutover", RoleName: role.Name, Position: "Gruppenleitung",
		Birthday: "1988-05-12", Gender: "weiblich", PersonnelNumber: "P-2708", EmploymentType: "Teilzeit",
		WeeklyHours: "19,5", EntryDate: "2023-08-01", AddressStreet: "Musterstr. 1", AddressPostalCode: "50667", AddressCity: "Köln",
		Phone: "0221-1234567", EmergencyContactName: "Peter Cutover", EmergencyContactPhone: "0171-9876543",
		Qualifications: "Erste Hilfe (01.03.2024 bis 01.03.2026); Schwimmschein", StaffNotes: "importiert",
	}
	runStaffImport := func(mode importModels.ImportMode, rows []importModels.StaffImportRow) *importModels.ImportResult[importModels.StaffImportRow] {
		t.Helper()
		var result *importModels.ImportResult[importModels.StaffImportRow]
		require.NoError(t, testpkg.WithTenantTx(t, testpkg.Ctx(t), db, tenantID, func(ctx context.Context, _ bun.Tx) error {
			ctx = importService.ContextWithImporterPermissions(ctx, []string{"admin:*"})
			var err error
			result, err = module.StaffImport.Import(ctx, importModels.ImportRequest[importModels.StaffImportRow]{
				Rows: rows, Mode: mode, UserID: actor.staffID, SkipInvalidRows: true,
			})
			if err != nil {
				return err
			}
			return module.StaffImport.RecordAuditInTransaction(ctx, "staff", "staff.csv", result, actor.accountID, false, tenantID)
		}))
		return result
	}

	result := runStaffImport(importModels.ImportModeCreate, []importModels.StaffImportRow{row})
	for _, rowErr := range result.Errors {
		for _, e := range rowErr.Errors {
			assert.NotEqual(t, importModels.ErrorSeverityError, e.Severity, "row %d: %s", rowErr.RowNumber, e.Message)
		}
	}
	require.Equal(t, 1, result.CreatedCount)

	var staff struct {
		ID              int64   `bun:"id"`
		PersonID        int64   `bun:"person_id"`
		StaffNotes      string  `bun:"staff_notes"`
		EmploymentType  *string `bun:"employment_type"`
		PersonnelNumber *string `bun:"personnel_number"`
		FirstName       string  `bun:"first_name"`
		Birthday        string  `bun:"birthday"`
		AccountID       *int64  `bun:"account_id"`
	}
	require.NoError(t, db.NewSelect().TableExpr("users.staff AS s").
		ColumnExpr("s.id, s.person_id, s.staff_notes, s.employment_type, s.personnel_number, p.first_name, p.birthday::text AS birthday, p.account_id").
		Join("JOIN users.persons p ON p.id = s.person_id").Where("s.tenant_id = ? AND p.last_name = ?", tenantID, "Cutover").Scan(ctx, &staff))
	assert.Equal(t, "Anna", staff.FirstName)
	assert.Equal(t, "1988-05-12", staff.Birthday)
	assert.Nil(t, staff.AccountID, "the import must not invent an account")
	assert.Equal(t, "P-2708", *staff.PersonnelNumber)
	assert.Equal(t, "part_time", *staff.EmploymentType)
	assert.Equal(t, "importiert", staff.StaffNotes)

	var teacherRole string
	require.NoError(t, db.NewSelect().TableExpr("users.teachers").Column("role").Where("tenant_id = ? AND staff_id = ?", tenantID, staff.ID).Scan(ctx, &teacherRole))
	assert.Equal(t, "Gruppenleitung", teacherRole, "a caregiver-tier role gets its profile")

	var master struct {
		Gender      *string  `bun:"gender"`
		City        *string  `bun:"address_city"`
		Emergency   *string  `bun:"emergency_contact_name"`
		WeeklyHours *float64 `bun:"weekly_hours"`
		EntryDate   string   `bun:"entry_date"`
	}
	require.NoError(t, db.NewSelect().TableExpr("users.staff_master_data").
		ColumnExpr("gender, address_city, emergency_contact_name, weekly_hours, entry_date::text AS entry_date").
		Where("tenant_id = ? AND staff_id = ?", tenantID, staff.ID).Scan(ctx, &master))
	assert.Equal(t, "female", *master.Gender)
	assert.Equal(t, "Köln", *master.City)
	assert.Equal(t, "Peter Cutover", *master.Emergency)
	assert.InDelta(t, 19.5, *master.WeeklyHours, 0.001)
	assert.Equal(t, "2023-08-01", master.EntryDate)

	var qualifications []struct {
		Name      string `bun:"name"`
		ExpiresOn string `bun:"expires_on"`
	}
	require.NoError(t, db.NewSelect().TableExpr("users.staff_qualifications").ColumnExpr("name, coalesce(expires_on::text, '') AS expires_on").
		Where("tenant_id = ? AND staff_id = ?", tenantID, staff.ID).OrderExpr("id").Scan(ctx, &qualifications))
	require.Len(t, qualifications, 2)
	assert.Equal(t, "Erste Hilfe", qualifications[0].Name)
	assert.Equal(t, "2026-03-01", qualifications[0].ExpiresOn)
	assert.Equal(t, "Schwimmschein", qualifications[1].Name)

	// Update: only the given cells change; the personnel number resolves the
	// record and the empty cells keep their stored values.
	update := importModels.StaffImportRow{
		FirstName: "Anna", LastName: "Cutover", RoleName: role.Name, PersonnelNumber: "P-2708",
		AddressCity: "Bonn", StaffNotes: "aktualisiert",
	}
	result = runStaffImport(importModels.ImportModeUpdate, []importModels.StaffImportRow{update})
	require.Equal(t, 1, result.UpdatedCount)
	require.Zero(t, result.ErrorCount)
	require.NoError(t, db.NewSelect().TableExpr("users.staff AS s").
		ColumnExpr("s.id, s.person_id, s.staff_notes, s.employment_type, s.personnel_number, p.first_name, p.birthday::text AS birthday, p.account_id").
		Join("JOIN users.persons p ON p.id = s.person_id").Where("s.id = ?", staff.ID).Scan(ctx, &staff))
	assert.Equal(t, "aktualisiert", staff.StaffNotes)
	assert.Equal(t, "part_time", *staff.EmploymentType, "an empty cell keeps the employment type")
	require.NoError(t, db.NewSelect().TableExpr("users.staff_master_data").
		ColumnExpr("gender, address_city, emergency_contact_name, weekly_hours, entry_date::text AS entry_date").
		Where("tenant_id = ? AND staff_id = ?", tenantID, staff.ID).Scan(ctx, &master))
	assert.Equal(t, "Bonn", *master.City)
	assert.Equal(t, "Peter Cutover", *master.Emergency, "an empty cell keeps the emergency contact")
	assert.InDelta(t, 19.5, *master.WeeklyHours, 0.001)

	var staffCount int
	require.NoError(t, db.NewSelect().TableExpr("users.staff").ColumnExpr("count(*)").Where("tenant_id = ?", tenantID).Scan(ctx, &staffCount))
	assert.Equal(t, 2, staffCount, "the importing staff member plus the imported one; the update created nobody")
}

// The class-list import writes through School Membership and leaves the same
// audit trail as a manual create; a child that is already a student needs no
// entry, and a replay adds nothing.
func TestDataImportCutover_ClassListThroughMembership(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.OwnTenant(t)
	module, err := services.NewImportTestModule(db, testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	actor := newImporter(t, db)
	testpkg.CreateTestStudent(t, db, "Schon", "Angemeldet", "1a")
	ctx := testpkg.Ctx(t)

	run := func(rows []importModels.ClassListEntryImportRow, dryRun bool) *importModels.ImportResult[importModels.ClassListEntryImportRow] {
		t.Helper()
		var result *importModels.ImportResult[importModels.ClassListEntryImportRow]
		require.NoError(t, testpkg.WithTenantTx(t, testpkg.Ctx(t), db, tenantID, func(ctx context.Context, _ bun.Tx) error {
			var err error
			result, err = module.ClassListImport.Import(ctx, importModels.ImportRequest[importModels.ClassListEntryImportRow]{
				Rows: rows, Mode: importModels.ImportModeCreate, DryRun: dryRun, UserID: actor.staffID, SkipInvalidRows: !dryRun,
			})
			if err != nil {
				return err
			}
			return module.ClassListImport.RecordAuditInTransaction(ctx, "class_list_entries", "class.csv", result, actor.accountID, dryRun, tenantID)
		}))
		return result
	}
	entryCount := func() int {
		var n int
		require.NoError(t, db.NewSelect().TableExpr("users.class_list_entries").ColumnExpr("count(*)").Where("tenant_id = ?", tenantID).Scan(ctx, &n))
		return n
	}
	changeCount := func() int {
		var n int
		require.NoError(t, db.NewSelect().TableExpr("audit.class_list_entry_changes").ColumnExpr("count(*)").Where("tenant_id = ?", tenantID).Scan(ctx, &n))
		return n
	}

	rows := []importModels.ClassListEntryImportRow{
		{FirstName: "Neu", LastName: "Kind", SchoolClass: "1a"},
		{FirstName: "Schon", LastName: "Angemeldet", SchoolClass: "1a"},
	}

	// The preview writes nothing and already reports the existing student.
	preview := run(rows, true)
	assert.Equal(t, 1, preview.CreatedCount)
	assert.Equal(t, 1, preview.ErrorCount)
	assert.Zero(t, entryCount(), "a preview writes no entry")

	result := run(rows, false)
	assert.Equal(t, 1, result.CreatedCount)
	assert.Equal(t, 1, result.ErrorCount, "the child that already is a student needs no list entry")
	assert.Equal(t, 1, entryCount())
	assert.Equal(t, 1, changeCount(), "the create leaves the same audit trail as a manual one")

	var entry struct {
		FirstName string `bun:"first_name"`
		CreatedBy *int64 `bun:"created_by"`
	}
	require.NoError(t, db.NewSelect().TableExpr("users.class_list_entries").ColumnExpr("first_name, created_by").Where("tenant_id = ?", tenantID).Scan(ctx, &entry))
	assert.Equal(t, "Neu", entry.FirstName)
	require.NotNil(t, entry.CreatedBy)
	assert.Equal(t, actor.staffID, *entry.CreatedBy)

	replay := run(rows, false)
	assert.Zero(t, replay.CreatedCount)
	assert.Equal(t, 2, replay.ErrorCount)
	assert.Equal(t, 1, entryCount(), "a replay adds no entry")
	assert.Equal(t, 1, changeCount(), "and no audit row")
}
