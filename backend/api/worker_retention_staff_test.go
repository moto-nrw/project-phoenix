package api

import (
	"context"
	"testing"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"

	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// retiredStaff is a staff member of a school with one record of each kind
// the automatic deletion removes, and the stored name of their document.
type retiredStaff struct {
	staffID  int64
	personID int64
	document string
}

// staffWithRecords stages a staff member of the school with master,
// financial and employment data, a qualification, a teacher record with a
// group assignment, a document, a work session, a supervision record, planned
// supervision assignment and historical timetable assignment.
func (s retentionSuite) staffWithRecords(t *testing.T, tenantID int64, name string) retiredStaff {
	t.Helper()
	staff := testpkg.CreateTestStaffForTenant(t, s.admin, tenantID, name, "Personal")
	ctx := context.Background()
	for _, query := range []string{
		`INSERT INTO users.staff_master_data (tenant_id, staff_id, address_city) VALUES (?0, ?1, 'Münster')`,
		`INSERT INTO users.staff_financial_data (tenant_id, staff_id, iban) VALUES (?0, ?1, 'DE02120300000000202051')`,
		`INSERT INTO users.staff_qualifications (tenant_id, staff_id, name, sort_order) VALUES (?0, ?1, 'Erzieherin', 0)`,
		`INSERT INTO users.teachers (tenant_id, staff_id, specialization) VALUES (?0, ?1, 'Musik')`,
		`INSERT INTO active.work_sessions (tenant_id, staff_id, date, check_in_time, created_by)
			VALUES (?0, ?1, DATE '2020-01-06', TIMESTAMPTZ '2020-01-06 08:00:00+01', ?1)`,
	} {
		_, err := s.admin.ExecContext(ctx, query, tenantID, staff.ID)
		require.NoError(t, err)
	}
	var teacherID int64
	require.NoError(t, s.admin.NewRaw(`SELECT id FROM users.teachers WHERE tenant_id = ? AND staff_id = ?`, tenantID, staff.ID).Scan(ctx, &teacherID))
	educationGroup := testpkg.CreateTestEducationGroupForTenant(t, s.admin, tenantID, name+" Lehrkraftgruppe")
	_, err := s.admin.ExecContext(ctx, `INSERT INTO education.group_teacher (tenant_id, group_id, teacher_id) VALUES (?, ?, ?)`, tenantID, educationGroup.ID, teacherID)
	require.NoError(t, err)
	group := testpkg.CreateTestActiveGroupForTenant(t, s.admin, tenantID)
	_, err = s.admin.ExecContext(ctx, `INSERT INTO active.group_supervisors (tenant_id, staff_id, group_id, role, start_date)
		VALUES (?, ?, ?, 'supervisor', DATE '2020-01-06')`, tenantID, staff.ID, group.ID)
	require.NoError(t, err)
	plannedGroup := testpkg.CreateTestActivityGroupForTenant(t, s.admin, tenantID, name+" Planung")
	_, err = s.admin.ExecContext(ctx, `INSERT INTO activities.supervisors (tenant_id, staff_id, group_id, is_primary, valid_from)
		VALUES (?, ?, ?, true, DATE '2020-01-06')`, tenantID, staff.ID, plannedGroup.ID)
	require.NoError(t, err)
	room := testpkg.CreateTestRoomForTenant(t, s.admin, tenantID, name+" Raum")
	instanceDate := testpkg.Date(2020, 1, 6)
	instance := testpkg.CreateTestActivityInstanceForTenant(t, s.admin, tenantID, instanceDate, room.ID, testpkg.ActivityInstanceOpts{})
	testpkg.CreateTestInstanceStaffForTenant(t, s.admin, tenantID, instance.ID, staff.ID, testpkg.InstanceStaffOpts{})
	var document string
	require.NoError(t, s.admin.NewRaw(`INSERT INTO users.staff_documents
		(tenant_id, staff_id, category, filename_display, filename_stored, size_bytes, content_type, uploaded_by)
		VALUES (?, ?, 'arbeitsvertrag', 'Vertrag.pdf', gen_random_uuid()::text || '.pdf', 1024, 'application/pdf', 1)
		RETURNING filename_stored`, tenantID, staff.ID).Scan(ctx, &document))
	return retiredStaff{staffID: staff.ID, personID: staff.PersonID, document: document}
}

// stagedStaffRecords is the number of rows staffWithRecords stages.
const stagedStaffRecords = 11

// retire records the exit of a staff member at noon daysAgo Berlin days back,
// as the offboarding does when it retires the membership.
func (s retentionSuite) retire(t *testing.T, staffID int64, daysAgo int) {
	t.Helper()
	_, err := s.admin.ExecContext(context.Background(), `UPDATE users.staff_school_memberships
		SET deleted_at = (((CURRENT_TIMESTAMP AT TIME ZONE 'Europe/Berlin')::date - ?)::timestamp + INTERVAL '12 hours') AT TIME ZONE 'Europe/Berlin'
		WHERE id = ?`, daysAgo, staffID)
	require.NoError(t, err)
}

// staffRecords counts the rows of the staff member the erasure removes.
func (s retentionSuite) staffRecords(t *testing.T, staffID int64) int {
	t.Helper()
	return s.count(t, `SELECT
		(SELECT COUNT(*) FROM users.staff_master_data WHERE staff_id = ?0)
		+ (SELECT COUNT(*) FROM users.staff_financial_data WHERE staff_id = ?0)
		+ (SELECT COUNT(*) FROM users.staff_qualifications WHERE staff_id = ?0)
		+ (SELECT COUNT(*) FROM users.staff_documents WHERE staff_id = ?0)
		+ (SELECT COUNT(*) FROM users.staff_employment_profiles WHERE membership_id = ?0)
		+ (SELECT COUNT(*) FROM users.teachers WHERE staff_id = ?0)
		+ (SELECT COUNT(*) FROM education.group_teacher WHERE teacher_id IN (SELECT id FROM users.teachers WHERE staff_id = ?0))
		+ (SELECT COUNT(*) FROM active.work_sessions WHERE staff_id = ?0)
		+ (SELECT COUNT(*) FROM active.group_supervisors WHERE staff_id = ?0)
		+ (SELECT COUNT(*) FROM activities.supervisors WHERE staff_id = ?0)
		+ (SELECT COUNT(*) FROM schedule.instance_staff WHERE staff_id = ?0)`, staffID)
}

type staffPerson struct {
	FirstName  string `bun:"first_name"`
	LastName   string `bun:"last_name"`
	Tombstoned bool   `bun:"tombstoned"`
}

func (s retentionSuite) staffPerson(t *testing.T, personID int64) staffPerson {
	t.Helper()
	var person staffPerson
	require.NoError(t, s.admin.NewRaw(`SELECT first_name, last_name, deleted_at IS NOT NULL AS tombstoned
		FROM users.persons WHERE id = ?`, personID).Scan(context.Background(), &person))
	return person
}

// staffDeletionLog is what the deletion log holds for one automatically
// deleted staff member.
type staffDeletionLog struct {
	DeletionType        string `bun:"deletion_type"`
	RecordsDeleted      int    `bun:"records_deleted"`
	DeletedBy           string `bun:"deleted_by"`
	Reason              string `bun:"deletion_reason"`
	MasterData          int    `bun:"master_data"`
	Documents           int    `bun:"documents"`
	GroupTeachers       int    `bun:"group_teacher"`
	PlannedSupervisors  int    `bun:"planned_supervisors"`
	InstanceAssignments int    `bun:"timetable_instance_staff"`
	PersonAnonymized    bool   `bun:"person_anonymized"`
}

func (s retentionSuite) staffDeletionLog(t *testing.T, tenantID, staffID int64) []staffDeletionLog {
	t.Helper()
	var rows []staffDeletionLog
	require.NoError(t, s.admin.NewRaw(`
		SELECT deletion_type, records_deleted, deleted_by, deletion_reason,
			(metadata->>'master_data')::integer AS master_data,
			(metadata->>'documents')::integer AS documents,
			(metadata->>'group_teacher')::integer AS group_teacher,
			(metadata->>'planned_supervisors')::integer AS planned_supervisors,
			(metadata->>'timetable_instance_staff')::integer AS timetable_instance_staff,
			(metadata->>'person_anonymized')::boolean AS person_anonymized
		FROM audit.data_deletions
		WHERE tenant_id = ? AND staff_id = ? AND deletion_type = 'staff_retention'
		ORDER BY id`, tenantID, staffID).Scan(context.Background(), &rows))
	return rows
}

func (s retentionSuite) tombstoneErased(t *testing.T, staffID int64) bool {
	t.Helper()
	return s.count(t, `SELECT COUNT(*) FROM users.staff_school_memberships
		WHERE id = ? AND deleted_at IS NOT NULL AND personal_data_erased_at IS NOT NULL`, staffID) == 1
}

func TestNightlyRetentionDeletesStaffAfterTheTimeTrackingWindow(t *testing.T) {
	t.Parallel()
	suite := newRetentionSuite(t)
	school := testpkg.Tenant(t)
	otherSchool, _ := testpkg.CreateTestTenant(t, suite.admin)

	expired := suite.staffWithRecords(t, school, "Abgelaufen")
	suite.retire(t, expired.staffID, 731)
	kept := suite.staffWithRecords(t, school, "Frist")
	suite.retire(t, kept.staffID, 730)
	active := suite.staffWithRecords(t, school, "Aktiv")
	otherExpired := suite.staffWithRecords(t, otherSchool, "Andere")
	suite.retire(t, otherExpired.staffID, 900)

	// The offboarding's deletion log entry names the staff member and must
	// survive their deletion.
	offboardingLog := suite.insertID(t, `INSERT INTO audit.data_deletions (tenant_id, staff_id, deletion_type, records_deleted, deleted_by, deletion_reason)
		VALUES (?, ?, 'manual', 3, 'leitung', 'staff offboarding') RETURNING id`, school, expired.staffID)
	// A record of another person names the staff member as its actor; its
	// foreign key does not cascade.
	student := testpkg.CreateTestStudentForTenant(t, suite.admin, school, "Kind", "Notiz", "2b")
	note := suite.insertID(t, `INSERT INTO schedule.student_pickup_notes (tenant_id, student_id, content, created_by)
		VALUES (?, ?, 'Abholung durch Oma', ?) RETURNING id`, school, student.ID, expired.staffID)

	suite.run(t, school)

	assert.Zero(t, suite.staffRecords(t, expired.staffID), "a staff member gone for more than the window loses every record")
	assert.True(t, suite.fileCleanupIntent(t, school, expired.document), "the document file waits in the staff document cleanup queue")
	assert.Equal(t, staffPerson{FirstName: "Gelöscht", LastName: "Benutzer", Tombstoned: true}, suite.staffPerson(t, expired.personID),
		"the person is anonymized like a deleted child's")
	assert.True(t, suite.tombstoneErased(t, expired.staffID), "the membership stays as an erased tombstone")
	assert.True(t, suite.exists(t, "schedule.student_pickup_notes", note), "records of other people keep their actor")
	assert.True(t, suite.exists(t, "audit.data_deletions", offboardingLog), "the deletion log of the staff member is kept")
	assert.Equal(t, []staffDeletionLog{{
		DeletionType: "staff_retention", RecordsDeleted: 12, DeletedBy: "system", Reason: "retention_expired",
		MasterData: 1, Documents: 1, GroupTeachers: 1, PlannedSupervisors: 1, InstanceAssignments: 1, PersonAnonymized: true,
	}}, suite.staffDeletionLog(t, school, expired.staffID), "the deletion is logged per staff member, attributed to the system")
	assert.Empty(t, suite.loggedDeletions(t, school, auditModels.DeletionTypeStaffRetention), "each staff member is logged on its own, not as a data kind")

	for name, member := range map[string]retiredStaff{"the last day of the window": kept, "a staff member without an exit": active} {
		assert.Equal(t, stagedStaffRecords, suite.staffRecords(t, member.staffID), "%s keeps every record", name)
		assert.Equal(t, "Personal", suite.staffPerson(t, member.personID).LastName, "%s keeps the name", name)
		assert.Empty(t, suite.staffDeletionLog(t, school, member.staffID), name)
	}
	assert.Equal(t, stagedStaffRecords, suite.staffRecords(t, otherExpired.staffID), "the run of one school never touches another")
	assert.Empty(t, suite.staffDeletionLog(t, otherSchool, otherExpired.staffID))

	suite.run(t, school)

	assert.Len(t, suite.staffDeletionLog(t, school, expired.staffID), 1, "a second run erases and logs nothing")
}

func TestNightlyRetentionKeepsTheNameOfARehiredStaffMember(t *testing.T) {
	t.Parallel()
	suite := newRetentionSuite(t)
	school := testpkg.Tenant(t)
	former := suite.staffWithRecords(t, school, "Zurück")
	suite.retire(t, former.staffID, 800)
	rehired := suite.insertID(t, `INSERT INTO users.staff_school_memberships (tenant_id, person_id) VALUES (?, ?) RETURNING id`,
		school, former.personID)

	suite.run(t, school)

	assert.Zero(t, suite.staffRecords(t, former.staffID), "the records of the former membership go")
	assert.Equal(t, staffPerson{FirstName: "Zurück", LastName: "Personal"}, suite.staffPerson(t, former.personID),
		"the person stays named while a newer membership is on the staff")
	assert.True(t, suite.exists(t, "users.staff_school_memberships", rehired))
	logged := suite.staffDeletionLog(t, school, former.staffID)
	require.Len(t, logged, 1)
	assert.False(t, logged[0].PersonAnonymized)
}

func TestNightlyRetentionKeepsTheNameOfAStaffMemberWhoIsStillAStudent(t *testing.T) {
	t.Parallel()
	suite := newRetentionSuite(t)
	school := testpkg.Tenant(t)
	former := suite.staffWithRecords(t, school, "Doppelrolle")
	suite.retire(t, former.staffID, 800)
	student := suite.insertID(t, `INSERT INTO users.student_profiles (tenant_id, person_id) VALUES (?, ?) RETURNING id`,
		school, former.personID)
	membership := suite.insertID(t, `INSERT INTO users.student_school_memberships (tenant_id, student_profile_id, school_class)
		VALUES (?, ?, '4a') RETURNING id`, school, student)
	_, err := suite.admin.ExecContext(context.Background(), `INSERT INTO users.student_care_profiles (tenant_id, membership_id) VALUES (?, ?)`, school, membership)
	require.NoError(t, err)

	suite.run(t, school)

	assert.Zero(t, suite.staffRecords(t, former.staffID), "the retired staff records still expire")
	assert.Equal(t, staffPerson{FirstName: "Doppelrolle", LastName: "Personal"}, suite.staffPerson(t, former.personID),
		"the active student keeps the shared identity")
	assert.True(t, suite.exists(t, "users.student_school_memberships", membership))
	logged := suite.staffDeletionLog(t, school, former.staffID)
	require.Len(t, logged, 1)
	assert.False(t, logged[0].PersonAnonymized)
}

func TestNightlyStaffRetentionKeepsSickNotesUntilTheirOwnWindowEnds(t *testing.T) {
	t.Parallel()
	suite := newRetentionSuite(t)
	school := testpkg.Tenant(t)
	_, err := suite.admin.ExecContext(context.Background(), `INSERT INTO config.setting_values (tenant_id, setting_key, value)
		VALUES (?, 'gdpr.staff_sick_note_retention_days', '1095'::jsonb)`, school)
	require.NoError(t, err)

	former := suite.staffWithRecords(t, school, "Bescheinigung")
	uploader := testpkg.CreateTestAccount(t, suite.admin, "staff-retention-sick-note")
	sickNote, _ := suite.staffDocument(t, school, former.staffID, uploader.ID, "au_bescheinigung", 800)
	suite.retire(t, former.staffID, 731)

	suite.run(t, school)

	assert.True(t, suite.exists(t, "users.staff_documents", sickNote), "the sick-note workflow owns this document's longer retention period")
	assert.Equal(t, 1, suite.staffRecords(t, former.staffID), "only the sick note remains after the former staff member's records are erased")
}

func TestNightlyRetentionErasesEveryFormerMembershipOfOnePerson(t *testing.T) {
	t.Parallel()
	suite := newRetentionSuite(t)
	school := testpkg.Tenant(t)
	first := suite.staffWithRecords(t, school, "Zweimal")
	suite.retire(t, first.staffID, 1200)
	second := suite.insertID(t, `INSERT INTO users.staff_school_memberships (tenant_id, person_id) VALUES (?, ?) RETURNING id`,
		school, first.personID)
	suite.retire(t, second, 900)

	suite.run(t, school)

	assert.True(t, suite.tombstoneErased(t, first.staffID))
	assert.True(t, suite.tombstoneErased(t, second), "the person anonymized by the first erasure does not hold up the second")
	assert.Equal(t, "Gelöscht", suite.staffPerson(t, first.personID).FirstName)
	logged := suite.staffDeletionLog(t, school, second)
	require.Len(t, logged, 1)
	assert.False(t, logged[0].PersonAnonymized, "the person is counted once, with the first membership")
}

func TestNightlyRetentionFollowsTheSchoolsTimeTrackingWindow(t *testing.T) {
	t.Parallel()
	suite := newRetentionSuite(t)
	school := testpkg.Tenant(t)
	_, err := suite.admin.ExecContext(context.Background(), `INSERT INTO config.setting_values (tenant_id, setting_key, value)
		VALUES (?, 'gdpr.time_tracking_retention_days', '1095'::jsonb)`, school)
	require.NoError(t, err)

	kept := suite.staffWithRecords(t, school, "Lange")
	suite.retire(t, kept.staffID, 800)
	expired := suite.staffWithRecords(t, school, "Lange")
	suite.retire(t, expired.staffID, 1096)

	suite.run(t, school)

	assert.Equal(t, stagedStaffRecords, suite.staffRecords(t, kept.staffID), "a longer contractual window keeps the staff member")
	assert.Zero(t, suite.staffRecords(t, expired.staffID))
}

func TestNightlyRetentionRefusesAStaffWindowOutsideTheFrame(t *testing.T) {
	t.Parallel()
	suite := newRetentionSuite(t)
	school := testpkg.Tenant(t)
	_, err := suite.admin.ExecContext(context.Background(), `INSERT INTO config.setting_values (tenant_id, setting_key, value)
		VALUES (?, 'gdpr.time_tracking_retention_days', '365'::jsonb)`, school)
	require.NoError(t, err)
	member := suite.staffWithRecords(t, school, "Zu")
	suite.retire(t, member.staffID, 900)

	err = testpkg.WithinTenantContext(t, context.Background(), suite.serve, school, func(ctx context.Context) error {
		return suite.retention.runSteps(ctx)
	})

	require.ErrorContains(t, err, "outside 730 to 2920", "a window outside the frame deletes nothing and fails the run")
	assert.Equal(t, stagedStaffRecords, suite.staffRecords(t, member.staffID))
}

func TestStaffRetentionFunctionRefusesStaffWhoAreNotDue(t *testing.T) {
	t.Parallel()
	suite := newRetentionSuite(t)
	school := testpkg.Tenant(t)
	otherSchool, _ := testpkg.CreateTestTenant(t, suite.admin)
	recent := suite.staffWithRecords(t, school, "Kurz")
	suite.retire(t, recent.staffID, 100)
	active := suite.staffWithRecords(t, school, "Aktiv")
	foreign := suite.staffWithRecords(t, otherSchool, "Fremd")
	suite.retire(t, foreign.staffID, 900)

	erase := func(tenantID, staffID int64, cutoffDaysAgo int) error {
		return testpkg.WithinTenantContext(t, context.Background(), suite.serve, school, func(ctx context.Context) error {
			transaction, _ := tenant.TransactionFromContext(ctx)
			tx, ok := transaction.(bun.Tx)
			require.True(t, ok, "the school's tenant transaction")
			_, err := tx.ExecContext(ctx, `SELECT users.erase_retired_staff_records(?, ?,
				(((CURRENT_TIMESTAMP AT TIME ZONE 'Europe/Berlin')::date - ?)::timestamp) AT TIME ZONE 'Europe/Berlin')`,
				tenantID, staffID, cutoffDaysAgo)
			return err
		})
	}

	assert.ErrorContains(t, erase(school, recent.staffID, 50), "exceeds the time-tracking window",
		"a cutoff younger than the shortest window is refused")
	assert.ErrorContains(t, erase(school, recent.staffID, 730), "has not left before the cutoff",
		"a staff member who left after the cutoff is refused")
	assert.ErrorContains(t, erase(school, active.staffID, 730), "has not left before the cutoff",
		"a staff member still on the staff is refused")
	assert.ErrorContains(t, erase(otherSchool, foreign.staffID, 730), "does not match transaction tenant",
		"another school's staff member is refused")
	for _, member := range []retiredStaff{recent, active, foreign} {
		assert.Equal(t, stagedStaffRecords, suite.staffRecords(t, member.staffID))
	}
}
