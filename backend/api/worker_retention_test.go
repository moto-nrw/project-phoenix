package api

import (
	"context"
	"fmt"
	"testing"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"

	settingsModule "github.com/moto-nrw/project-phoenix/modules/settings"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// retentionSuite runs the nightly retention of one school the way the Worker
// does (#3950): the steps the root composes, inside the school's tenant
// transaction on the least-privilege Serve pool, and seeds and inspects rows
// as the superuser.
type retentionSuite struct {
	admin     *bun.DB
	serve     *bun.DB
	retention workerRetention
}

func newRetentionSuite(t *testing.T) retentionSuite {
	t.Helper()
	admin := testpkg.SetupTestDB(t)
	serve := testpkg.SetupServeTestDB(t)
	t.Cleanup(func() { require.NoError(t, serve.Close()) })
	db, err := transactionruntime.NewDatabase(admin)
	require.NoError(t, err)
	owners, err := NewRetentionTestOwners(db, testpkg.TenantRuntime(t, admin))
	require.NoError(t, err)
	students, err := NewStudentTestRetention(db, owners.Audit, owners.Settings)
	require.NoError(t, err)
	staff, err := newStaffRetention(db, owners.Audit, owners.Settings, nil)
	require.NoError(t, err)
	attendance, err := presenceCompose.New(presenceCompose.Dependencies{DB: admin, Observe: func(presenceCompose.Observation) {}})
	require.NoError(t, err)
	retention, err := newWorkerRetention(retentionOwners{
		deletionLog:          owners.Audit,
		settings:             owners.Settings,
		unregisteredTagScans: owners.UnregisteredTagScans,
		statusDays:           owners.StatusDays,
		parentMessages:       owners.ParentMessages,
		settledEnrollments:   owners.SettledEnrollments,
		staffSickNotes:       owners.StaffSickNotes,
		auditLedgers:         owners.Audit,
		deliveries:           owners.Deliveries,
		fileCleanups:         owners.FileCleanups,
		staffCleanups:        owners.StaffCleanups,
		carePlanCleanups:     owners.CarePlanCleanups,
		submissionLimits:     owners.SubmissionLimits,
		settingAudit:         owners.SettingAudit,
		exportTransfers:      owners.ExportTransfers,
	}, students, staff, attendance)
	require.NoError(t, err)
	return retentionSuite{admin: admin, serve: serve, retention: retention}
}

func (s retentionSuite) run(t *testing.T, tenantID int64) {
	t.Helper()
	require.NoError(t, testpkg.WithinTenantContext(t, context.Background(), s.serve, tenantID, func(ctx context.Context) error {
		return s.retention.runSteps(ctx)
	}))
}

// runSteps runs the owners' deletion steps of one school in order and logs
// each deletion, as the Worker's nightly run does (services/scheduler).
func (r workerRetention) runSteps(ctx context.Context) error {
	for _, step := range r.steps {
		outcome, err := step.Run(ctx)
		if err != nil {
			return fmt.Errorf("retention step %s: %w", step.Kind, err)
		}
		if outcome.Deleted == 0 || step.LogsSubjects {
			continue
		}
		if err := r.log(ctx, step.Kind, outcome); err != nil {
			return fmt.Errorf("log retention step %s: %w", step.Kind, err)
		}
	}
	return nil
}

func (s retentionSuite) insertID(t *testing.T, query string, args ...any) int64 {
	t.Helper()
	var id int64
	require.NoError(t, s.admin.NewRaw(query, args...).Scan(context.Background(), &id))
	return id
}

func (s retentionSuite) exists(t *testing.T, table string, id int64) bool {
	t.Helper()
	var found bool
	require.NoError(t, s.admin.NewRaw("SELECT EXISTS (SELECT 1 FROM "+table+" WHERE id = ?)", id).Scan(context.Background(), &found))
	return found
}

// loggedDeletions returns the records_deleted of each deletion-log row of the
// data kind for the school; such a row names no child and no staff member.
func (s retentionSuite) loggedDeletions(t *testing.T, tenantID int64, kind string) []int {
	t.Helper()
	var counts []int
	require.NoError(t, s.admin.NewRaw(`
		SELECT records_deleted FROM audit.data_deletions
		WHERE tenant_id = ? AND deletion_type = ? AND student_id IS NULL AND staff_id IS NULL
		ORDER BY id`, tenantID, kind).Scan(context.Background(), &counts))
	return counts
}

type studentDeletionAudit struct {
	RecordsDeleted       int `bun:"records_deleted"`
	EnrollmentReferences int `bun:"enrollment_references"`
}

func (s retentionSuite) studentDeletionAudit(t *testing.T, tenantID, studentID int64) studentDeletionAudit {
	t.Helper()
	var audit studentDeletionAudit
	require.NoError(t, s.admin.NewRaw(`
		SELECT records_deleted, (metadata->'counts'->>'enrollment_references')::integer AS enrollment_references
		FROM audit.data_deletions
		WHERE tenant_id = ? AND student_id = ? AND deletion_type = 'student_retention'`, tenantID, studentID).
		Scan(context.Background(), &audit))
	return audit
}

func TestWorkerRetentionPreloadsRetentionWindows(t *testing.T) {
	t.Parallel()

	assert.ElementsMatch(t, []string{
		settingsModule.KeyGDPRStatusDayRetentionDays,
		settingsModule.KeyGDPRAttendanceRetentionDays,
		settingsModule.KeyGDPRParentMessageRetentionDays,
		settingsModule.KeyGDPRWithdrawnEnrollmentRetentionDays,
		settingsModule.KeyGDPRStaffSickNoteRetentionDays,
		settingsModule.KeyGDPRStudentRetentionDays,
		settingsModule.KeyGDPRTimeTrackingRetentionDays,
	}, settingsModule.RetentionWindowKeys())
}

func TestNightlyRetentionDeletesUnregisteredTagScansAfterNinetyDays(t *testing.T) {
	t.Parallel()
	suite := newRetentionSuite(t)
	school := testpkg.Tenant(t)
	otherSchool, _ := testpkg.CreateTestTenant(t, suite.admin)

	const insertScan = `INSERT INTO audit.unregistered_tag_scans (tenant_id, tag_uid, scanned_at)
		VALUES (?, ?, CURRENT_TIMESTAMP - make_interval(days => ?)) RETURNING id`
	expired := suite.insertID(t, insertScan, school, "retention-expired", 91)
	kept := suite.insertID(t, insertScan, school, "retention-kept", 89)
	otherExpired := suite.insertID(t, insertScan, otherSchool, "retention-other", 91)

	suite.run(t, school)

	assert.False(t, suite.exists(t, "audit.unregistered_tag_scans", expired), "a scan older than 90 days is deleted")
	assert.True(t, suite.exists(t, "audit.unregistered_tag_scans", kept), "a scan younger than 90 days stays")
	assert.True(t, suite.exists(t, "audit.unregistered_tag_scans", otherExpired), "the run of one school never touches another")
	assert.Equal(t, []int{1}, suite.loggedDeletions(t, school, auditModels.DeletionTypeUnregisteredTagScanRetention))
	assert.Empty(t, suite.loggedDeletions(t, otherSchool, auditModels.DeletionTypeUnregisteredTagScanRetention))
}

// insertStatusDay stores a status day daysAgo Berlin calendar days back.
func (s retentionSuite) insertStatusDay(t *testing.T, tenantID, studentID int64, status string, daysAgo int) int64 {
	t.Helper()
	return s.insertID(t, `INSERT INTO active.student_status_days (tenant_id, student_id, date, status, reported_at)
		VALUES (?, ?, (CURRENT_TIMESTAMP AT TIME ZONE 'Europe/Berlin')::date - ?, ?, CURRENT_TIMESTAMP - make_interval(days => ?))
		RETURNING id`, tenantID, studentID, daysAgo, status, daysAgo)
}

func (s retentionSuite) count(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	require.NoError(t, s.admin.NewRaw(query, args...).Scan(context.Background(), &n))
	return n
}

func TestNightlyRetentionDeletesStatusDaysAfterTheirWindow(t *testing.T) {
	t.Parallel()
	suite := newRetentionSuite(t)
	school := testpkg.Tenant(t)
	otherSchool, _ := testpkg.CreateTestTenant(t, suite.admin)
	child := testpkg.CreateTestStudent(t, suite.admin, "Frist", "Kind", "2a")
	otherChild := testpkg.CreateTestStudentForTenant(t, suite.admin, otherSchool, "Andere", "Schule", "2a")

	expiredSick := suite.insertStatusDay(t, school, child.ID, "sick", 366)
	expiredTrip := suite.insertStatusDay(t, school, child.ID, "class_trip", 400)
	keptExcused := suite.insertStatusDay(t, school, child.ID, "excused", 364)
	keptOnCutoff := suite.insertStatusDay(t, school, child.ID, "sick", 365)
	otherExpired := suite.insertStatusDay(t, otherSchool, otherChild.ID, "sick", 400)
	mealHistory := suite.count(t, `SELECT COUNT(*) FROM schedule.meal_sickness_status_history WHERE tenant_id = ?`, school)

	suite.run(t, school)

	assert.False(t, suite.exists(t, "active.student_status_days", expiredSick), "a sick day older than 365 days is deleted")
	assert.False(t, suite.exists(t, "active.student_status_days", expiredTrip), "every status of the day kind is deleted")
	assert.True(t, suite.exists(t, "active.student_status_days", keptExcused), "a younger day stays")
	assert.True(t, suite.exists(t, "active.student_status_days", keptOnCutoff), "the day exactly 365 days back is still inside the window")
	assert.True(t, suite.exists(t, "active.student_status_days", otherExpired), "the run of one school never touches another")
	assert.Equal(t, []int{2}, suite.loggedDeletions(t, school, auditModels.DeletionTypeStatusDayRetention))
	assert.Empty(t, suite.loggedDeletions(t, otherSchool, auditModels.DeletionTypeStatusDayRetention))
	assert.Equal(t, mealHistory,
		suite.count(t, `SELECT COUNT(*) FROM schedule.meal_sickness_status_history WHERE tenant_id = ?`, school),
		"the purge must not copy the deleted sick day into the meal sickness history")
}

func TestNightlyRetentionFollowsTheSchoolsRecordedStatusDayWindow(t *testing.T) {
	t.Parallel()
	suite := newRetentionSuite(t)
	school := testpkg.Tenant(t)
	child := testpkg.CreateTestStudent(t, suite.admin, "Lange", "Frist", "3b")
	_, err := suite.admin.ExecContext(context.Background(), `INSERT INTO config.setting_values (tenant_id, setting_key, value)
		VALUES (?, 'gdpr.status_day_retention_days', '730'::jsonb)`, school)
	require.NoError(t, err)

	kept := suite.insertStatusDay(t, school, child.ID, "sick", 400)
	expired := suite.insertStatusDay(t, school, child.ID, "sick", 731)

	suite.run(t, school)

	assert.True(t, suite.exists(t, "active.student_status_days", kept), "a longer contractual window keeps the day")
	assert.False(t, suite.exists(t, "active.student_status_days", expired))
}

// insertAttendance stores a closed attendance daysAgo Berlin calendar days
// back, checked in on the device.
func (s retentionSuite) insertAttendance(t *testing.T, tenantID, studentID, deviceID int64, daysAgo int) int64 {
	t.Helper()
	return s.insertID(t, `INSERT INTO active.attendance (tenant_id, student_id, date, check_in_time, check_out_time, device_id)
		VALUES (?, ?, (CURRENT_TIMESTAMP AT TIME ZONE 'Europe/Berlin')::date - ?,
			CURRENT_TIMESTAMP - make_interval(days => ?, hours => 2), CURRENT_TIMESTAMP - make_interval(days => ?), ?)
		RETURNING id`, tenantID, studentID, daysAgo, daysAgo, daysAgo, deviceID)
}

func TestNightlyRetentionDeletesAttendanceAfterItsWindow(t *testing.T) {
	t.Parallel()
	suite := newRetentionSuite(t)
	school := testpkg.Tenant(t)
	otherSchool, _ := testpkg.CreateTestTenant(t, suite.admin)
	child := testpkg.CreateTestStudent(t, suite.admin, "Anwesenheit", "Frist", "2a")
	otherChild := testpkg.CreateTestStudentForTenant(t, suite.admin, otherSchool, "Andere", "Schule", "2a")
	device := testpkg.CreateTestDevice(t, suite.admin, "attendance-retention")
	otherDevice := testpkg.CreateTestDeviceForTenant(t, suite.admin, otherSchool, "attendance-retention-other")

	expired := suite.insertAttendance(t, school, child.ID, device.ID, 366)
	expiredLong := suite.insertAttendance(t, school, child.ID, device.ID, 900)
	keptOnCutoff := suite.insertAttendance(t, school, child.ID, device.ID, 365)
	kept := suite.insertAttendance(t, school, child.ID, device.ID, 10)
	otherExpired := suite.insertAttendance(t, otherSchool, otherChild.ID, otherDevice.ID, 400)

	suite.run(t, school)

	assert.False(t, suite.exists(t, "active.attendance", expired), "attendance older than 365 days is deleted")
	assert.False(t, suite.exists(t, "active.attendance", expiredLong))
	assert.True(t, suite.exists(t, "active.attendance", keptOnCutoff), "the day exactly 365 days back is still inside the window")
	assert.True(t, suite.exists(t, "active.attendance", kept))
	assert.True(t, suite.exists(t, "active.attendance", otherExpired), "the run of one school never touches another")
	assert.Equal(t, []int{2}, suite.loggedDeletions(t, school, auditModels.DeletionTypeAttendanceRetention))
	assert.Empty(t, suite.loggedDeletions(t, otherSchool, auditModels.DeletionTypeAttendanceRetention))
}

func TestNightlyRetentionFollowsTheSchoolsRecordedAttendanceWindow(t *testing.T) {
	t.Parallel()
	suite := newRetentionSuite(t)
	school := testpkg.Tenant(t)
	child := testpkg.CreateTestStudent(t, suite.admin, "Kurze", "Frist", "3b")
	device := testpkg.CreateTestDevice(t, suite.admin, "attendance-retention-window")
	_, err := suite.admin.ExecContext(context.Background(), `INSERT INTO config.setting_values (tenant_id, setting_key, value)
		VALUES (?, 'gdpr.attendance_retention_days', '30'::jsonb)`, school)
	require.NoError(t, err)

	kept := suite.insertAttendance(t, school, child.ID, device.ID, 30)
	expired := suite.insertAttendance(t, school, child.ID, device.ID, 31)

	suite.run(t, school)

	assert.True(t, suite.exists(t, "active.attendance", kept))
	assert.False(t, suite.exists(t, "active.attendance", expired), "a shorter contractual window deletes earlier")
	assert.Equal(t, []int{1}, suite.loggedDeletions(t, school, auditModels.DeletionTypeAttendanceRetention))
}

type parentThreadFixture struct {
	suite    retentionSuite
	tenantID int64
	student  int64
	guardian int64
	id       int64
}

// parentThread stores a conversation opened daysAgo days back between a
// fresh guardian account and the child.
func (s retentionSuite) parentThread(t *testing.T, tenantID, studentID int64, daysAgo int) parentThreadFixture {
	t.Helper()
	guardian := testpkg.CreateTestAccount(t, s.admin, "retention-guardian@example.com").ID
	id := s.insertID(t, `INSERT INTO users.parent_message_threads (tenant_id, student_id, guardian_account_id, created_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP - make_interval(days => ?)) RETURNING id`, tenantID, studentID, guardian, daysAgo)
	return parentThreadFixture{suite: s, tenantID: tenantID, student: studentID, guardian: guardian, id: id}
}

// message stores a message sent daysAgo days back and makes it the thread's
// preview when it is the newest, as sending does.
func (f parentThreadFixture) message(t *testing.T, body string, daysAgo int) int64 {
	t.Helper()
	id := f.suite.insertID(t, `INSERT INTO users.parent_messages
		(tenant_id, thread_id, student_id, sender_account_id, sender_kind, sender_name, body, created_at)
		VALUES (?, ?, ?, ?, 'guardian', 'Elternteil', ?, CURRENT_TIMESTAMP - make_interval(days => ?)) RETURNING id`,
		f.tenantID, f.id, f.student, f.guardian, body, daysAgo)
	f.touch(t, id)
	return id
}

// openRequest stores a request the OGS has not decided yet.
func (f parentThreadFixture) openRequest(t *testing.T, body string, daysAgo int) int64 {
	t.Helper()
	return f.suite.insertID(t, `INSERT INTO users.parent_messages
		(tenant_id, thread_id, student_id, sender_account_id, sender_kind, sender_name, body, created_at, kind, request_type, request_status)
		VALUES (?, ?, ?, ?, 'guardian', 'Elternteil', ?, CURRENT_TIMESTAMP - make_interval(days => ?), 'request', 'care_change', 'offen') RETURNING id`,
		f.tenantID, f.id, f.student, f.guardian, body, daysAgo)
}

// requestWithoutStatus exercises the nullable request status. Only the
// explicit status "offen" remains protected from retention; NULL is not open.
func (f parentThreadFixture) requestWithoutStatus(t *testing.T, body string, daysAgo int) int64 {
	t.Helper()
	return f.suite.insertID(t, `INSERT INTO users.parent_messages
		(tenant_id, thread_id, student_id, sender_account_id, sender_kind, sender_name, body, created_at, kind, request_type)
		VALUES (?, ?, ?, ?, 'guardian', 'Elternteil', ?, CURRENT_TIMESTAMP - make_interval(days => ?), 'request', 'care_change') RETURNING id`,
		f.tenantID, f.id, f.student, f.guardian, body, daysAgo)
}

func (f parentThreadFixture) touch(t *testing.T, messageID int64) {
	t.Helper()
	_, err := f.suite.admin.ExecContext(context.Background(), `UPDATE users.parent_message_threads AS thread
		SET last_message_id = message.id, last_message_at = message.created_at,
		    last_sender_kind = message.sender_kind, last_message_body = message.body
		FROM users.parent_messages AS message
		WHERE thread.id = ? AND message.id = ?
		  AND (thread.last_message_at IS NULL OR thread.last_message_at < message.created_at)`, f.id, messageID)
	require.NoError(t, err)
}

func TestNightlyRetentionDeletesParentMessagesAfterTheirWindow(t *testing.T) {
	t.Parallel()
	suite := newRetentionSuite(t)
	school := testpkg.Tenant(t)
	otherSchool, _ := testpkg.CreateTestTenant(t, suite.admin)
	child := testpkg.CreateTestStudent(t, suite.admin, "Nachricht", "Kind", "1a")
	otherChild := testpkg.CreateTestStudentForTenant(t, suite.admin, otherSchool, "Andere", "Schule", "1a")

	// An active conversation: its old message goes, the recent one stays.
	active := suite.parentThread(t, school, child.ID, 500)
	expired := active.message(t, "Alte Nachricht", 366)
	recent := active.message(t, "Neue Nachricht", 364)

	// A conversation whose only message expired disappears with its read state.
	finished := suite.parentThread(t, school, child.ID, 500)
	finished.message(t, "Längst erledigt", 400)
	_, err := suite.admin.ExecContext(context.Background(), `INSERT INTO users.parent_message_reads (tenant_id, thread_id, account_id)
		VALUES (?, ?, ?)`, school, finished.id, finished.guardian)
	require.NoError(t, err)

	// An undecided request keeps its conversation; the preview must not keep
	// the text of the deleted newer message.
	waiting := suite.parentThread(t, school, child.ID, 500)
	request := waiting.openRequest(t, "Bitte Betreuung ändern", 450)
	waiting.message(t, "Vertrauliche alte Antwort", 400)
	withoutStatus := suite.parentThread(t, school, child.ID, 500)
	requestWithoutStatus := withoutStatus.requestWithoutStatus(t, "Alte Anfrage", 400)

	otherThread := suite.parentThread(t, otherSchool, otherChild.ID, 500)
	otherExpired := otherThread.message(t, "Andere Schule", 400)

	suite.run(t, school)

	assert.False(t, suite.exists(t, "users.parent_messages", expired), "a message older than 365 days is deleted")
	assert.True(t, suite.exists(t, "users.parent_messages", recent), "a younger message stays")
	assert.True(t, suite.exists(t, "users.parent_message_threads", active.id))
	assert.False(t, suite.exists(t, "users.parent_message_threads", finished.id), "a conversation without messages disappears")
	assert.Zero(t, suite.count(t, `SELECT COUNT(*) FROM users.parent_message_reads WHERE thread_id = ?`, finished.id))
	assert.True(t, suite.exists(t, "users.parent_messages", request), "an undecided request stays")
	assert.False(t, suite.exists(t, "users.parent_messages", requestWithoutStatus), "a request without the explicit open status is deleted")
	assert.False(t, suite.exists(t, "users.parent_message_threads", withoutStatus.id), "the thread of a deleted request disappears")
	var preview string
	require.NoError(t, suite.admin.NewRaw(`SELECT last_message_body FROM users.parent_message_threads WHERE id = ?`, waiting.id).
		Scan(context.Background(), &preview))
	assert.Equal(t, "Bitte Betreuung ändern", preview, "the preview shows the newest remaining message, never the deleted one")
	assert.True(t, suite.exists(t, "users.parent_messages", otherExpired), "the run of one school never touches another")
	assert.Equal(t, []int{6}, suite.loggedDeletions(t, school, auditModels.DeletionTypeParentMessageRetention), "expired messages and their empty threads are logged")
	assert.Empty(t, suite.loggedDeletions(t, otherSchool, auditModels.DeletionTypeParentMessageRetention))
}

func TestNightlyRetentionDeletesExpiredAuditLedgerRows(t *testing.T) {
	t.Parallel()
	suite := newRetentionSuite(t)
	school := testpkg.Tenant(t)
	otherSchool, _ := testpkg.CreateTestTenant(t, suite.admin)
	account := testpkg.CreateTestAccount(t, suite.admin, "retention-ledger@example.com").ID

	const insertAuthEvent = `INSERT INTO audit.auth_events (tenant_id, account_id, event_type, success, ip_address, created_at)
		VALUES (?, ?, 'login', true, '127.0.0.1', CURRENT_TIMESTAMP - make_interval(days => ?)) RETURNING id`
	expiredLogin := suite.insertID(t, insertAuthEvent, school, account, 91)
	keptLogin := suite.insertID(t, insertAuthEvent, school, account, 89)
	otherLogin := suite.insertID(t, insertAuthEvent, otherSchool, account, 91)

	const lastYearStart = `((date_trunc('year', CURRENT_TIMESTAMP AT TIME ZONE 'Europe/Berlin') - INTERVAL '1 year') AT TIME ZONE 'Europe/Berlin')`
	const insertAccess = `INSERT INTO audit.data_access_log (tenant_id, actor_account_id, actor_role, resource_type, range_start, range_end, accessed_at)
		SELECT ?, ?, 'staff', 'attendance', at, at, at FROM (SELECT ` + lastYearStart + ` + make_interval(days => ?) AS at) AS access RETURNING id`
	expiredAccess := suite.insertID(t, insertAccess, school, account, -1)
	keptAccess := suite.insertID(t, insertAccess, school, account, 1)

	const insertDeletion = `INSERT INTO audit.data_deletions (tenant_id, deletion_type, records_deleted, deleted_by, deleted_at)
		VALUES (?, 'unregistered_tag_scan_retention', 1, 'system', CURRENT_TIMESTAMP - make_interval(years => 3, days => ?)) RETURNING id`
	expiredDeletion := suite.insertID(t, insertDeletion, school, 1)
	keptDeletion := suite.insertID(t, insertDeletion, school, -1)

	suite.run(t, school)

	assert.False(t, suite.exists(t, "audit.auth_events", expiredLogin), "a sign-in event older than 90 days is deleted")
	assert.True(t, suite.exists(t, "audit.auth_events", keptLogin))
	assert.True(t, suite.exists(t, "audit.auth_events", otherLogin), "the run of one school never touches another")
	assert.False(t, suite.exists(t, "audit.data_access_log", expiredAccess), "an access from the year before last is deleted")
	assert.True(t, suite.exists(t, "audit.data_access_log", keptAccess), "an access from last year stays until its end")
	assert.False(t, suite.exists(t, "audit.data_deletions", expiredDeletion), "a deletion-log row older than three years is deleted")
	assert.True(t, suite.exists(t, "audit.data_deletions", keptDeletion))
	assert.Equal(t, []int{1}, suite.loggedDeletions(t, school, auditModels.DeletionTypeAuthEventRetention))
	assert.Equal(t, []int{1}, suite.loggedDeletions(t, school, auditModels.DeletionTypeDataAccessLogRetention))
	assert.Equal(t, []int{1}, suite.loggedDeletions(t, school, auditModels.DeletionTypeDeletionLogRetention))
	assert.Empty(t, suite.loggedDeletions(t, otherSchool, auditModels.DeletionTypeAuthEventRetention))
}

// deliveryFixture plants delivery rows of one school as the Delivery worker
// leaves them: the end of an intent is its sending, final failure or
// cancellation, daysAgo days back.
type deliveryFixture struct {
	suite    retentionSuite
	tenantID int64
}

func (f deliveryFixture) email(t *testing.T, kind, status string, daysAgo int) int64 {
	t.Helper()
	return f.suite.insertID(t, `INSERT INTO platform.email_outbox
		(tenant_id, kind, recipient, payload, status, attempts, created_at, updated_at, sent_at, dead_letter_at, cancelled_at,
		 lease_token, lease_expires_at)
		SELECT ?, ?::text, '{"address":"eltern@example.com","name":"Eltern"}'::jsonb, '{"body":"Inhalt"}'::jsonb, ?::text, 1,
		       ended - INTERVAL '1 day', ended,
		       CASE WHEN ?::text = 'sent' THEN ended END,
		       CASE WHEN ?::text = 'dead_letter' THEN ended END,
		       CASE WHEN ?::text = 'cancelled' THEN ended END,
		       CASE WHEN ?::text = 'claimed' THEN 'lease' END,
		       CASE WHEN ?::text = 'claimed' THEN CURRENT_TIMESTAMP + INTERVAL '5 minutes' END
		FROM (SELECT CURRENT_TIMESTAMP - make_interval(days => ?) AS ended) AS intent
		RETURNING id`, f.tenantID, kind, status, status, status, status, status, status, daysAgo)
}

func (f deliveryFixture) push(t *testing.T, status string, daysAgo int) int64 {
	t.Helper()
	return f.suite.insertID(t, `INSERT INTO platform.push_outbox
		(tenant_id, kind, recipient, payload, status, created_at, updated_at, sent_at, dead_letter_at)
		SELECT ?, 'notification', '{"endpoint":"https://push.example.com/1"}'::jsonb, '{"title":"Hinweis"}'::jsonb, ?::text,
		       ended - INTERVAL '1 day', ended,
		       CASE WHEN ?::text = 'sent' THEN ended END,
		       CASE WHEN ?::text = 'dead_letter' THEN ended END
		FROM (SELECT CURRENT_TIMESTAMP - make_interval(days => ?) AS ended) AS intent
		RETURNING id`, f.tenantID, status, status, status, daysAgo)
}

// deliveryLog stores the delivery-log row of a parent letter for one
// recipient; outboxID is 0 when no mail was queued.
func (f deliveryFixture) deliveryLog(t *testing.T, outboxID int64, reachability string, daysAgo int) int64 {
	t.Helper()
	var outbox any
	email := any(nil)
	if outboxID > 0 {
		outbox = outboxID
	}
	if reachability == "ok" {
		email = "eltern@example.com"
	}
	return f.suite.insertID(t, `INSERT INTO platform.email_delivery
		(tenant_id, related_entity_type, related_entity_id, outbox_id, recipient_email, reachability, created_at)
		VALUES (?, 'parent_announcement', ?, ?, ?, ?, CURRENT_TIMESTAMP - make_interval(days => ?)) RETURNING id`,
		f.tenantID, f.tenantID, outbox, email, reachability, daysAgo)
}

func TestNightlyRetentionDeletesDeliveriesThirtyDaysAfterTheirEnd(t *testing.T) {
	t.Parallel()
	suite := newRetentionSuite(t)
	school := testpkg.Tenant(t)
	otherSchool, _ := testpkg.CreateTestTenant(t, suite.admin)
	mails := deliveryFixture{suite: suite, tenantID: school}
	others := deliveryFixture{suite: suite, tenantID: otherSchool}

	sentOld := mails.email(t, "parent_announcement", "sent", 31)
	sentRecent := mails.email(t, "parent_announcement", "sent", 29)
	failedOld := mails.email(t, "parent_announcement", "dead_letter", 31)
	cancelledOld := mails.email(t, "parent_announcement", "cancelled", 31)
	pendingOld := mails.email(t, "parent_announcement", "pending", 60)
	claimedOld := mails.email(t, "parent_announcement", "claimed", 60)
	welcomeOld := mails.email(t, "guardian_welcome", "sent", 31)
	otherSchoolSentOld := others.email(t, "parent_announcement", "sent", 31)

	logOfSentOld := mails.deliveryLog(t, sentOld, "ok", 32)
	logOfSentRecent := mails.deliveryLog(t, sentRecent, "ok", 30)
	logOfPending := mails.deliveryLog(t, pendingOld, "ok", 60)
	logWithoutMailOld := mails.deliveryLog(t, 0, "no_email", 31)
	logWithoutMailRecent := mails.deliveryLog(t, 0, "no_email", 29)

	pushSentOld := mails.push(t, "sent", 31)
	pushFailedRecent := mails.push(t, "dead_letter", 29)
	pushPendingOld := mails.push(t, "pending", 60)

	suite.run(t, school)
	suite.run(t, school)

	assert.False(t, suite.exists(t, "platform.email_outbox", sentOld), "a mail sent more than 30 days ago is deleted")
	assert.False(t, suite.exists(t, "platform.email_outbox", failedOld), "a mail that finally failed more than 30 days ago is deleted")
	assert.False(t, suite.exists(t, "platform.email_outbox", cancelledOld), "a cancelled mail is deleted 30 days later")
	assert.True(t, suite.exists(t, "platform.email_outbox", sentRecent), "a mail sent less than 30 days ago stays")
	assert.True(t, suite.exists(t, "platform.email_outbox", pendingOld), "a mail still being tried stays")
	assert.True(t, suite.exists(t, "platform.email_outbox", claimedOld), "a mail the worker holds stays")
	assert.True(t, suite.exists(t, "platform.email_outbox", otherSchoolSentOld), "the run of one school never touches another")

	var recipient, payload string
	require.NoError(t, suite.admin.NewRaw(`SELECT recipient::text, payload::text FROM platform.email_outbox WHERE id = ?`, welcomeOld).
		Scan(context.Background(), &recipient, &payload))
	assert.Equal(t, "{}", recipient, "the welcome mail keeps no recipient")
	assert.Equal(t, "{}", payload, "the welcome mail keeps no content, only its key so nobody is welcomed twice")

	assert.False(t, suite.exists(t, "platform.email_delivery", logOfSentOld), "the delivery log of an old mail goes with it")
	assert.False(t, suite.exists(t, "platform.email_delivery", logWithoutMailOld), "a delivery log without a mail goes 30 days after it was written")
	assert.True(t, suite.exists(t, "platform.email_delivery", logOfSentRecent), "the window starts with the sending, not with the log row")
	assert.True(t, suite.exists(t, "platform.email_delivery", logOfPending), "the log of a mail still being tried stays")
	assert.True(t, suite.exists(t, "platform.email_delivery", logWithoutMailRecent))

	assert.False(t, suite.exists(t, "platform.push_outbox", pushSentOld), "a push sent more than 30 days ago is deleted")
	assert.True(t, suite.exists(t, "platform.push_outbox", pushFailedRecent))
	assert.True(t, suite.exists(t, "platform.push_outbox", pushPendingOld), "a push still being tried stays")

	assert.Equal(t, []int{7}, suite.loggedDeletions(t, school, auditModels.DeletionTypeDeliveryRetention),
		"three mails, the cleared welcome, two delivery logs and one push, logged once")
	assert.Empty(t, suite.loggedDeletions(t, otherSchool, auditModels.DeletionTypeDeliveryRetention))
}

// cleanupIntent stores a cleanup intent of a stored file in table, finished
// finishedDaysAgo days back; a negative value leaves it waiting for its bytes.
func (s retentionSuite) cleanupIntent(t *testing.T, table string, tenantID int64, finishedDaysAgo int) int64 {
	t.Helper()
	owner := "owner_id"
	if table == "users.staff_document_file_cleanup" {
		owner = "staff_id"
	}
	return s.insertID(t, `INSERT INTO `+table+` (tenant_id, `+owner+`, filename_stored, retry_after, cleaned_at, created_at)
		SELECT ?, 4046, gen_random_uuid()::text || '.pdf', at, CASE WHEN ? >= 0 THEN at END, at - INTERVAL '1 day'
		FROM (SELECT CURRENT_TIMESTAMP - make_interval(days => abs(?)) AS at) AS intent
		RETURNING id`, tenantID, finishedDaysAgo, finishedDaysAgo)
}

func TestNightlyRetentionDeletesFinishedCleanupIntentsThirtyDaysLater(t *testing.T) {
	t.Parallel()
	suite := newRetentionSuite(t)
	school := testpkg.Tenant(t)
	otherSchool, _ := testpkg.CreateTestTenant(t, suite.admin)

	type intents struct{ finishedOld, finishedRecent, pendingOld, otherSchoolOld int64 }
	tables := map[string]string{
		"documents.file_cleanup":                    auditModels.DeletionTypeFileCleanupRetention,
		"documents.announcement_attachment_cleanup": auditModels.DeletionTypeFileCleanupRetention,
		"users.staff_document_file_cleanup":         auditModels.DeletionTypeStaffCleanupRetention,
		"users.student_document_file_cleanup":       auditModels.DeletionTypeStudentDocumentCleanupRetention,
	}
	planted := map[string]intents{}
	for table := range tables {
		planted[table] = intents{
			finishedOld:    suite.cleanupIntent(t, table, school, 31),
			finishedRecent: suite.cleanupIntent(t, table, school, 29),
			pendingOld:     suite.cleanupIntent(t, table, school, -60),
			otherSchoolOld: suite.cleanupIntent(t, table, otherSchool, 31),
		}
	}

	suite.run(t, school)
	suite.run(t, school)

	for table, ids := range planted {
		assert.False(t, suite.exists(t, table, ids.finishedOld), "%s: an intent finished more than 30 days ago is deleted", table)
		assert.True(t, suite.exists(t, table, ids.finishedRecent), "%s: an intent finished less than 30 days ago stays", table)
		assert.True(t, suite.exists(t, table, ids.pendingOld), "%s: an intent still waiting for its bytes stays", table)
		assert.True(t, suite.exists(t, table, ids.otherSchoolOld), "%s: the run of one school never touches another", table)
	}
	assert.Equal(t, []int{2}, suite.loggedDeletions(t, school, auditModels.DeletionTypeFileCleanupRetention), "the file storage and the attachments, logged once")
	assert.Equal(t, []int{1}, suite.loggedDeletions(t, school, auditModels.DeletionTypeStudentDocumentCleanupRetention))
	assert.Empty(t, suite.loggedDeletions(t, otherSchool, auditModels.DeletionTypeFileCleanupRetention))
}

func TestCleanupIntentRetentionRefusesAYoungCutoff(t *testing.T) {
	t.Parallel()
	suite := newRetentionSuite(t)
	school := testpkg.Tenant(t)
	recent := suite.cleanupIntent(t, "documents.file_cleanup", school, 1)

	inSchool := func(query string, args ...any) error {
		return testpkg.WithinTenantContext(t, context.Background(), suite.serve, school, func(ctx context.Context) error {
			transaction, _ := tenant.TransactionFromContext(ctx)
			tx, ok := transaction.(bun.Tx)
			require.True(t, ok, "the school's tenant transaction")
			_, err := tx.ExecContext(ctx, query, args...)
			return err
		})
	}
	require.ErrorContains(t, inSchool(`SELECT documents.delete_finished_file_cleanups(?, CURRENT_TIMESTAMP)`, school),
		"cleanup intent cutoff exceeds retention window")
	require.ErrorContains(t, inSchool(`DELETE FROM documents.file_cleanup WHERE id = ?`, recent),
		"permission denied for table file_cleanup", "the tenant role keeps no direct DELETE on the intents")
	assert.True(t, suite.exists(t, "documents.file_cleanup", recent))
}

func (s retentionSuite) offboardingJob(t *testing.T, tenantID, staffID int64, completedDaysAgo int) {
	t.Helper()
	_, err := s.admin.NewRaw(`INSERT INTO users.staff_offboarding_cleanup (tenant_id, staff_id, completed_at, created_at)
		SELECT ?, ?, CASE WHEN ? >= 0 THEN at END, at - INTERVAL '1 day'
		FROM (SELECT CURRENT_TIMESTAMP - make_interval(days => abs(?)) AS at) AS job`,
		tenantID, staffID, completedDaysAgo, completedDaysAgo).Exec(context.Background())
	require.NoError(t, err)
}

func TestNightlyRetentionDeletesCompletedOffboardingJobsThirtyDaysLater(t *testing.T) {
	t.Parallel()
	suite := newRetentionSuite(t)
	school := testpkg.Tenant(t)
	otherSchool, _ := testpkg.CreateTestTenant(t, suite.admin)
	suite.offboardingJob(t, school, 1, 31)
	suite.offboardingJob(t, school, 2, 29)
	suite.offboardingJob(t, school, 3, -60)
	suite.offboardingJob(t, otherSchool, 1, 31)
	fileCleanup := suite.cleanupIntent(t, "users.staff_document_file_cleanup", school, 31)

	suite.run(t, school)

	staffIDs := func(tenantID int64) []int64 {
		var ids []int64
		require.NoError(t, suite.admin.NewRaw(`SELECT staff_id FROM users.staff_offboarding_cleanup WHERE tenant_id = ? ORDER BY staff_id`, tenantID).
			Scan(context.Background(), &ids))
		return ids
	}
	assert.Equal(t, []int64{2, 3}, staffIDs(school), "a completed job goes 30 days later; a recent or running job stays")
	assert.Equal(t, []int64{1}, staffIDs(otherSchool), "the run of one school never touches another")
	assert.False(t, suite.exists(t, "users.staff_document_file_cleanup", fileCleanup))
	assert.Equal(t, []int{2}, suite.loggedDeletions(t, school, auditModels.DeletionTypeStaffCleanupRetention), "the offboarding job and the file cleanup, logged once")
}

// withdrawalCompletion stores a care-withdrawal completion of the child (0
// for a deleted child) in state, finished finishedDaysAgo days back.
func (s retentionSuite) withdrawalCompletion(t *testing.T, tenantID, studentID int64, state string, finishedDaysAgo int) int64 {
	t.Helper()
	var student any
	if studentID > 0 {
		student = studentID
	}
	return s.insertID(t, `INSERT INTO users.care_withdrawal_completions
		(tenant_id, student_id, first_bookingless_day, trigger, withdrawal_confirmed_role, withdrawal_confirmed_at,
		 state, outcome, obsolete_reason, resolved_at)
		SELECT ?, ?, (at AT TIME ZONE 'Europe/Berlin')::date, 'direct_school', 'staff', at - INTERVAL '1 day', ?::text,
		       CASE ?::text WHEN 'resolved' THEN 'deleted' END,
		       CASE ?::text WHEN 'obsolete' THEN 'rebooked_without_gap' END,
		       CASE WHEN ?::text <> 'pending' THEN at END
		FROM (SELECT CURRENT_TIMESTAMP - make_interval(days => ?) AS at) AS completion
		RETURNING id`, tenantID, student, state, state, state, state, finishedDaysAgo)
}

func TestNightlyRetentionDeletesFinishedWithdrawalCompletionsAfterAYear(t *testing.T) {
	t.Parallel()
	suite := newRetentionSuite(t)
	school := testpkg.Tenant(t)
	otherSchool, _ := testpkg.CreateTestTenant(t, suite.admin)
	child := testpkg.CreateTestStudent(t, suite.admin, "Abmeldung", "Kind", "3b")

	resolvedOld := suite.withdrawalCompletion(t, school, 0, "resolved", 366)
	obsoleteOld := suite.withdrawalCompletion(t, school, 0, "obsolete", 366)
	resolvedRecent := suite.withdrawalCompletion(t, school, 0, "resolved", 364)
	pendingOld := suite.withdrawalCompletion(t, school, child.ID, "pending", 400)
	otherSchoolOld := suite.withdrawalCompletion(t, otherSchool, 0, "resolved", 366)
	exit := suite.insertID(t, `INSERT INTO users.student_care_exits (tenant_id, student_id, reason, withdrawal_completion_id)
		VALUES (?, ?, 'moved_away', ?) RETURNING id`, school, child.ID, obsoleteOld)

	suite.run(t, school)

	assert.False(t, suite.exists(t, "users.care_withdrawal_completions", resolvedOld), "a completion resolved more than 365 days ago is deleted")
	assert.False(t, suite.exists(t, "users.care_withdrawal_completions", obsoleteOld), "an obsolete completion goes after the same window")
	assert.True(t, suite.exists(t, "users.care_withdrawal_completions", resolvedRecent), "the history of the last year stays")
	assert.True(t, suite.exists(t, "users.care_withdrawal_completions", pendingOld), "a pending completion stays however old")
	assert.True(t, suite.exists(t, "users.care_withdrawal_completions", otherSchoolOld), "the run of one school never touches another")
	assert.True(t, suite.exists(t, "users.student_care_exits", exit), "the care exit stays and only loses its link")
	assert.Zero(t, suite.count(t, `SELECT COUNT(*) FROM users.student_care_exits WHERE id = ? AND withdrawal_completion_id IS NOT NULL`, exit))
	assert.Equal(t, []int{2}, suite.loggedDeletions(t, school, auditModels.DeletionTypeCareWithdrawalRetention))
	assert.Empty(t, suite.loggedDeletions(t, otherSchool, auditModels.DeletionTypeCareWithdrawalRetention))
}

func TestNightlyRetentionDeletesSubmissionRateLimitsAfterTheirWindow(t *testing.T) {
	t.Parallel()
	suite := newRetentionSuite(t)
	school := testpkg.Tenant(t)
	otherSchool, _ := testpkg.CreateTestTenant(t, suite.admin)
	const insertBucket = `INSERT INTO enrollment.submission_rate_limits (tenant_id, key_type, key_value, attempts, window_start)
		VALUES (?, ?, ?, 1, CURRENT_TIMESTAMP - make_interval(hours => ?)) RETURNING id`
	expiredMail := suite.insertID(t, insertBucket, school, "email", "eltern@example.com", 25)
	expiredIP := suite.insertID(t, insertBucket, school, "ip", "203.0.113.7", 25)
	runningMail := suite.insertID(t, insertBucket, school, "email", "noch@example.com", 23)
	otherSchoolExpired := suite.insertID(t, insertBucket, otherSchool, "email", "eltern@example.com", 25)

	suite.run(t, school)

	assert.False(t, suite.exists(t, "enrollment.submission_rate_limits", expiredMail), "an e-mail bucket goes once its 24-hour window is over")
	assert.False(t, suite.exists(t, "enrollment.submission_rate_limits", expiredIP))
	assert.True(t, suite.exists(t, "enrollment.submission_rate_limits", runningMail), "a running window keeps its count")
	assert.True(t, suite.exists(t, "enrollment.submission_rate_limits", otherSchoolExpired), "the run of one school never touches another")
	assert.Equal(t, []int{2}, suite.loggedDeletions(t, school, auditModels.DeletionTypeSubmissionRateLimitRetention))
	assert.Empty(t, suite.loggedDeletions(t, otherSchool, auditModels.DeletionTypeSubmissionRateLimitRetention))
}

// retentionProtocol is one protocol of #4044 with a seed of one row for the
// school ?0 that names the account ?1, dated by the SQL it is formatted with.
type retentionProtocol struct {
	kind          string
	table         string
	seed          string
	followingYear bool
}

var retentionProtocols = []retentionProtocol{
	{"student_deletion_log_retention", "audit.student_deletions", `INSERT INTO audit.student_deletions (tenant_id, student_id, actor_account_id, reason, counts, deleted_at)
		VALUES (?0, ?1, ?1, 'privacy_request', '{}'::jsonb, %s) RETURNING id`, false},
	{"enrollment_deletion_log_retention", "audit.enrollment_deletions", `INSERT INTO audit.enrollment_deletions (tenant_id, request_id, actor_account_id, actor_type, scope, reason, counts, deleted_at)
		VALUES (?0, ?1, ?1, 'admin', 'request', 'Doppelte Anmeldung', '{}'::jsonb, %s) RETURNING id`, false},
	{"enrollment_restoration_log_retention", "audit.enrollment_restorations", `INSERT INTO audit.enrollment_restorations (tenant_id, request_id, child_ids, actor_account_id, restored_at)
		VALUES (?0, ?1, '[]'::jsonb, ?1, %s) RETURNING id`, false},
	{"time_tracking_deletion_log_retention", "audit.time_tracking_deletions", `INSERT INTO audit.time_tracking_deletions (tenant_id, staff_id, source, source_id, deleted_by, payload, note, occurred_at)
		VALUES (?0, ?1, 'balance_adjustment', ?1, ?1, '{}'::jsonb, 'Korrektur', %s) RETURNING id`, false},
	{"staff_master_data_change_retention", "audit.staff_master_data_changes", `INSERT INTO audit.staff_master_data_changes (tenant_id, staff_id, changed_by, section, field_name, note, occurred_at)
		VALUES (?0, ?1, ?1, 'contact', 'phone', 'Neue Nummer', %s) RETURNING id`, false},
	{"personnel_number_change_retention", "audit.personnel_number_changes", `INSERT INTO audit.personnel_number_changes (tenant_id, staff_id, changed_by, new_value, occurred_at)
		VALUES (?0, ?1, ?1, '4711', %s) RETURNING id`, false},
	{"guardian_financial_change_retention", "audit.guardian_financial_changes", `INSERT INTO audit.guardian_financial_changes (tenant_id, guardian_profile_id, student_id, changed_by, field_name, occurred_at)
		VALUES (?0, ?1, ?1, ?1, 'iban', %s) RETURNING id`, false},
	{"class_list_change_retention", "audit.class_list_entry_changes", `INSERT INTO audit.class_list_entry_changes (tenant_id, entry_id, action, matched_student_id, changed_by, occurred_at)
		VALUES (?0, ?1, 'assigned', ?1, ?1, %s) RETURNING id`, true},
	{"substitution_change_retention", "audit.substitution_changes", `INSERT INTO audit.substitution_changes (tenant_id, substitution_id, target_type, action, group_id, target_staff_id, actor_account_id, start_date, end_date, created_at)
		VALUES (?0, ?1, 'group_handover', 'assigned', ?1, ?1, ?1, CURRENT_DATE, CURRENT_DATE, %s) RETURNING id`, true},
	{"file_event_retention", "audit.file_events", `INSERT INTO audit.file_events (tenant_id, action, actor_account_id, actor_name, detail, created_at)
		VALUES (?0, 'file_uploaded', ?1, 'Testkraft', 'elternbrief.pdf', %s) RETURNING id`, true},
	{"export_transfer_retention", "audit.export_transfers", `INSERT INTO audit.export_transfers (tenant_id, actor_account_id, actor_name, export_kind, export_format, filename,
			byte_size, target_host, target_port, target_directory, status, created_at)
		VALUES (?0, ?1, 'Testkraft', 'staff_time_tracking', 'csv', 'zeiten.csv', 1, 'sftp.example.org', 22, '/', 'success', %s) RETURNING id`, true},
	{"setting_audit_retention", "config.setting_audit", `INSERT INTO config.setting_audit (tenant_id, setting_key, new_value, action, changed_by, changed_at)
		VALUES (?0, 'gdpr.data_cleanup_time', '"03:00"'::jsonb, 'set', ?1, %s) RETURNING id`, false},
}

func TestNightlyRetentionDeletesExpiredProtocolRows(t *testing.T) {
	t.Parallel()
	suite := newRetentionSuite(t)
	school := testpkg.Tenant(t)
	otherSchool, _ := testpkg.CreateTestTenant(t, suite.admin)
	account := testpkg.CreateTestAccount(t, suite.admin, "retention-protocol@example.com").ID

	const lastYearStart = `((date_trunc('year', CURRENT_TIMESTAMP AT TIME ZONE 'Europe/Berlin') - INTERVAL '1 year') AT TIME ZONE 'Europe/Berlin')`
	type seeded struct{ expired, kept, other int64 }
	rows := map[string]seeded{}
	for _, protocol := range retentionProtocols {
		expired, kept := "CURRENT_TIMESTAMP - INTERVAL '3 years 1 day'", "CURRENT_TIMESTAMP - INTERVAL '3 years' + INTERVAL '1 day'"
		if protocol.followingYear {
			expired, kept = lastYearStart+" - INTERVAL '1 day'", lastYearStart+" + INTERVAL '1 day'"
		}
		rows[protocol.kind] = seeded{
			expired: suite.insertID(t, fmt.Sprintf(protocol.seed, expired), school, account),
			kept:    suite.insertID(t, fmt.Sprintf(protocol.seed, kept), school, account),
			other:   suite.insertID(t, fmt.Sprintf(protocol.seed, expired), otherSchool, account),
		}
	}

	suite.run(t, school)

	for _, protocol := range retentionProtocols {
		row := rows[protocol.kind]
		assert.False(t, suite.exists(t, protocol.table, row.expired), "%s: a row past its window is deleted", protocol.table)
		assert.True(t, suite.exists(t, protocol.table, row.kept), "%s: a row inside its window stays", protocol.table)
		assert.True(t, suite.exists(t, protocol.table, row.other), "%s: the run of one school never touches another", protocol.table)
		assert.Equal(t, []int{1}, suite.loggedDeletions(t, school, protocol.kind), "%s logs its own data kind", protocol.table)
		assert.Empty(t, suite.loggedDeletions(t, otherSchool, protocol.kind))
	}
	assert.Len(t, retentionProtocols, len(auditModels.ProtocolRetentionTypes())+2, "every protocol of #4044 is covered")
}
