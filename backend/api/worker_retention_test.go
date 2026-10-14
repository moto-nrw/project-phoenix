package api

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
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
	owners, err := testutil.NewRetentionTestOwners(admin, testpkg.TenantRuntime(t, admin))
	require.NoError(t, err)
	retention, err := newWorkerRetention(retentionOwners{
		deletionLog:          owners.Audit,
		settings:             owners.Settings,
		unregisteredTagScans: owners.UnregisteredTagScans,
		statusDays:           owners.StatusDays,
		parentMessages:       owners.ParentMessages,
	}, nil)
	require.NoError(t, err)
	return retentionSuite{admin: admin, serve: serve, retention: retention}
}

func (s retentionSuite) run(t *testing.T, tenantID int64) {
	t.Helper()
	require.NoError(t, testpkg.WithinTenantContext(t, context.Background(), s.serve, tenantID, func(ctx context.Context) error {
		return s.retention.run(ctx, tenantID)
	}))
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
	assert.Equal(t, []int{1}, suite.loggedDeletions(t, school, retentionUnregisteredTagScans))
	assert.Empty(t, suite.loggedDeletions(t, otherSchool, retentionUnregisteredTagScans))
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
	assert.Equal(t, []int{2}, suite.loggedDeletions(t, school, retentionStatusDays))
	assert.Empty(t, suite.loggedDeletions(t, otherSchool, retentionStatusDays))
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

	otherThread := suite.parentThread(t, otherSchool, otherChild.ID, 500)
	otherExpired := otherThread.message(t, "Andere Schule", 400)

	suite.run(t, school)

	assert.False(t, suite.exists(t, "users.parent_messages", expired), "a message older than 365 days is deleted")
	assert.True(t, suite.exists(t, "users.parent_messages", recent), "a younger message stays")
	assert.True(t, suite.exists(t, "users.parent_message_threads", active.id))
	assert.False(t, suite.exists(t, "users.parent_message_threads", finished.id), "a conversation without messages disappears")
	assert.Zero(t, suite.count(t, `SELECT COUNT(*) FROM users.parent_message_reads WHERE thread_id = ?`, finished.id))
	assert.True(t, suite.exists(t, "users.parent_messages", request), "an undecided request stays")
	var preview string
	require.NoError(t, suite.admin.NewRaw(`SELECT last_message_body FROM users.parent_message_threads WHERE id = ?`, waiting.id).
		Scan(context.Background(), &preview))
	assert.Equal(t, "Bitte Betreuung ändern", preview, "the preview shows the newest remaining message, never the deleted one")
	assert.True(t, suite.exists(t, "users.parent_messages", otherExpired), "the run of one school never touches another")
	assert.Equal(t, []int{3}, suite.loggedDeletions(t, school, retentionParentMessages))
	assert.Empty(t, suite.loggedDeletions(t, otherSchool, retentionParentMessages))
}
