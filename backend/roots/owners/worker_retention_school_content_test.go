package owners

import (
	"context"
	"strings"
	"testing"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The content a school creates itself (#4045): each data kind goes after its
// window in the nightly run of its school and is logged once per run.

// berlinDaysAgo is the Berlin calendar day daysAgo days back.
const berlinDaysAgo = `(CURRENT_TIMESTAMP AT TIME ZONE 'Europe/Berlin')::date - ?::integer`

func (s retentionSuite) setWindow(t *testing.T, tenantID int64, key string, days int) {
	t.Helper()
	_, err := s.admin.ExecContext(context.Background(), `INSERT INTO config.setting_values (tenant_id, setting_key, value)
		VALUES (?, ?, to_jsonb(?::integer))`, tenantID, key, days)
	require.NoError(t, err)
}

// --- parent announcements ---

type announcementFixture struct {
	createdDaysAgo   int
	publishedDaysAgo *int
	expiresDaysAgo   *int
	updatedDaysAgo   int
}

func days(n int) *int { return &n }

func (s retentionSuite) insertAnnouncement(t *testing.T, tenantID, accountID int64, f announcementFixture) int64 {
	t.Helper()
	return s.insertID(t, `INSERT INTO users.parent_announcements
		(tenant_id, title, body, created_by, created_at, updated_at, published_at, expires_at)
		VALUES (?, 'Elternbrief', 'Text', ?,
			CURRENT_TIMESTAMP - make_interval(days => ?), CURRENT_TIMESTAMP - make_interval(days => ?),
			CURRENT_TIMESTAMP - make_interval(days => ?::integer), CURRENT_TIMESTAMP - make_interval(days => ?::integer))
		RETURNING id`, tenantID, accountID, f.createdDaysAgo, f.updatedDaysAgo, f.publishedDaysAgo, f.expiresDaysAgo)
}

func TestNightlyRetentionDeletesParentAnnouncementsAfterTheParentMessageWindow(t *testing.T) {
	t.Parallel()
	suite := newRetentionSuite(t)
	school := testpkg.Tenant(t)
	otherSchool, _ := testpkg.CreateTestTenant(t, suite.admin)
	author := testpkg.CreateTestAccount(t, suite.admin, "announcement-retention@example.com")

	expired := suite.insertAnnouncement(t, school, author.ID, announcementFixture{createdDaysAgo: 500, publishedDaysAgo: days(499), expiresDaysAgo: days(400), updatedDaysAgo: 499})
	publishedLongAgo := suite.insertAnnouncement(t, school, author.ID, announcementFixture{createdDaysAgo: 400, publishedDaysAgo: days(400), updatedDaysAgo: 400})
	expiresLater := suite.insertAnnouncement(t, school, author.ID, announcementFixture{createdDaysAgo: 500, publishedDaysAgo: days(499), expiresDaysAgo: days(300), updatedDaysAgo: 499})
	publishedRecently := suite.insertAnnouncement(t, school, author.ID, announcementFixture{createdDaysAgo: 400, publishedDaysAgo: days(300), updatedDaysAgo: 300})
	staleDraft := suite.insertAnnouncement(t, school, author.ID, announcementFixture{createdDaysAgo: 450, updatedDaysAgo: 400})
	recentDraft := suite.insertAnnouncement(t, school, author.ID, announcementFixture{createdDaysAgo: 450, updatedDaysAgo: 300})
	otherExpired := suite.insertAnnouncement(t, otherSchool, author.ID, announcementFixture{createdDaysAgo: 400, publishedDaysAgo: days(400), updatedDaysAgo: 400})
	ackID := suite.insertID(t, `INSERT INTO users.parent_announcement_reads (tenant_id, announcement_id, account_id)
		VALUES (?, ?, ?) RETURNING announcement_id`, school, expired, author.ID)
	require.Equal(t, expired, ackID)
	suite.insertID(t, `INSERT INTO documents.announcement_attachments
		(tenant_id, announcement_id, filename_display, filename_stored, size_bytes, content_type, uploaded_by)
		VALUES (?, ?, 'brief.pdf', 'retention-brief.pdf', 10, 'application/pdf', ?) RETURNING id`, school, expired, author.ID)

	suite.run(t, school)

	assert.False(t, suite.exists(t, "users.parent_announcements", expired), "an announcement expired more than 365 days ago is deleted")
	assert.False(t, suite.exists(t, "users.parent_announcements", publishedLongAgo), "without an expiry the publication starts the window")
	assert.True(t, suite.exists(t, "users.parent_announcements", expiresLater), "the expiry starts the window, not the publication")
	assert.True(t, suite.exists(t, "users.parent_announcements", publishedRecently))
	assert.False(t, suite.exists(t, "users.parent_announcements", staleDraft), "a draft counts from its last change")
	assert.True(t, suite.exists(t, "users.parent_announcements", recentDraft))
	assert.True(t, suite.exists(t, "users.parent_announcements", otherExpired), "the run of one school never touches another")
	assert.Zero(t, suite.count(t, `SELECT COUNT(*) FROM users.parent_announcement_reads WHERE announcement_id = ?`, expired),
		"the reads go with their announcement")
	assert.Equal(t, 1, suite.count(t, `SELECT COUNT(*) FROM documents.announcement_attachment_cleanup
		WHERE tenant_id = ? AND filename_stored = 'retention-brief.pdf'`, school),
		"the attachment file is queued for removal like on a manual delete")
	assert.Equal(t, []int{3}, suite.loggedDeletions(t, school, auditModels.DeletionTypeParentAnnouncementRetention))
	assert.Empty(t, suite.loggedDeletions(t, otherSchool, auditModels.DeletionTypeParentAnnouncementRetention))
}

func TestNightlyRetentionKeepsADeclarationWithSubmissions(t *testing.T) {
	t.Parallel()
	suite := newRetentionSuite(t)
	school := testpkg.Tenant(t)
	author := testpkg.CreateTestAccount(t, suite.admin, "declaration-retention@example.com")
	child := testpkg.CreateTestStudent(t, suite.admin, "Erklärung", "Frist", "2a")
	hash := strings.Repeat("a", 64)

	declaration := func(withSubmission bool) int64 {
		id := suite.insertID(t, `INSERT INTO users.parent_announcements
			(tenant_id, title, body, created_by, created_at, updated_at, published_at, delivery_mode, declaration_kind, declaration_signers)
			VALUES (?, 'Einverständnis', 'Text', ?, CURRENT_TIMESTAMP - INTERVAL '500 days', CURRENT_TIMESTAMP - INTERVAL '500 days',
				CURRENT_TIMESTAMP - INTERVAL '500 days', 'declaration', 'consent', 'any')
			RETURNING id`, school, author.ID)
		version := suite.insertID(t, `INSERT INTO users.parent_announcement_declaration_versions
			(tenant_id, announcement_id, version_no, title, body, declaration_kind, content_hash)
			VALUES (?, ?, 1, 'Einverständnis', 'Text', 'consent', ?) RETURNING id`, school, id, hash)
		if withSubmission {
			suite.insertID(t, `INSERT INTO users.parent_announcement_declaration_submissions
				(tenant_id, announcement_id, version_id, student_id, account_id, signer_name, guardian_role, action, content_hash, record_hash, submitted_at)
				VALUES (?, ?, ?, ?, ?, 'Elternteil', 'primary_guardian', 'agreed', ?, ?, CURRENT_TIMESTAMP - INTERVAL '499 days')
				RETURNING id`, school, id, version, child.ID, author.ID, hash, hash)
		}
		return id
	}
	answered := declaration(true)
	unanswered := declaration(false)

	suite.run(t, school)

	assert.True(t, suite.exists(t, "users.parent_announcements", answered), "submissions prove the consent and keep their declaration")
	assert.False(t, suite.exists(t, "users.parent_announcements", unanswered), "a declaration nobody answered goes with its versions")
	assert.Equal(t, []int{1}, suite.loggedDeletions(t, school, auditModels.DeletionTypeParentAnnouncementRetention))
}

// --- staff notices ---

func (s retentionSuite) insertStaffNotice(t *testing.T, tenantID, accountID int64, validUntilDaysAgo *int, active bool, updatedDaysAgo int) int64 {
	t.Helper()
	return s.insertID(t, `INSERT INTO users.staff_notices (tenant_id, title, valid_from, valid_until, active, created_by, created_at, updated_at)
		VALUES (?, 'Hinweis', `+berlinDaysAgo+`, `+berlinDaysAgo+`, ?, ?,
			CURRENT_TIMESTAMP - INTERVAL '900 days', CURRENT_TIMESTAMP - make_interval(days => ?))
		RETURNING id`, tenantID, 900, validUntilDaysAgo, active, accountID, updatedDaysAgo)
}

func TestNightlyRetentionDeletesStaffNoticesAfterTheTeamChatWindow(t *testing.T) {
	t.Parallel()
	suite := newRetentionSuite(t)
	school := testpkg.Tenant(t)
	otherSchool, _ := testpkg.CreateTestTenant(t, suite.admin)
	author := testpkg.CreateTestAccount(t, suite.admin, "notice-retention@example.com")

	ended := suite.insertStaffNotice(t, school, author.ID, days(366), true, 800)
	keptOnCutoff := suite.insertStaffNotice(t, school, author.ID, days(365), true, 800)
	openEnded := suite.insertStaffNotice(t, school, author.ID, nil, true, 800)
	switchedOff := suite.insertStaffNotice(t, school, author.ID, nil, false, 400)
	switchedOffRecently := suite.insertStaffNotice(t, school, author.ID, nil, false, 300)
	otherEnded := suite.insertStaffNotice(t, otherSchool, author.ID, days(400), true, 800)
	_, err := suite.admin.ExecContext(context.Background(), `INSERT INTO users.staff_notice_acks (notice_id, account_id, tenant_id)
		VALUES (?, ?, ?)`, ended, author.ID, school)
	require.NoError(t, err)

	suite.run(t, school)

	assert.False(t, suite.exists(t, "users.staff_notices", ended), "a notice whose last day lies more than 365 days back is deleted")
	assert.True(t, suite.exists(t, "users.staff_notices", keptOnCutoff), "the day exactly 365 days back is still inside the window")
	assert.True(t, suite.exists(t, "users.staff_notices", openEnded), "an active notice without a last day stays")
	assert.False(t, suite.exists(t, "users.staff_notices", switchedOff), "a switched-off notice counts from its last change")
	assert.True(t, suite.exists(t, "users.staff_notices", switchedOffRecently))
	assert.True(t, suite.exists(t, "users.staff_notices", otherEnded), "the run of one school never touches another")
	assert.Zero(t, suite.count(t, `SELECT COUNT(*) FROM users.staff_notice_acks WHERE notice_id = ?`, ended))
	assert.Equal(t, []int{2}, suite.loggedDeletions(t, school, auditModels.DeletionTypeStaffNoticeRetention))
}

func TestNightlyRetentionFollowsTheSchoolsRecordedTeamChatWindowForStaffNotices(t *testing.T) {
	t.Parallel()
	suite := newRetentionSuite(t)
	school := testpkg.Tenant(t)
	author := testpkg.CreateTestAccount(t, suite.admin, "notice-window@example.com")
	suite.setWindow(t, school, "gdpr.staff_message_retention_days", 30)

	kept := suite.insertStaffNotice(t, school, author.ID, days(30), true, 800)
	expired := suite.insertStaffNotice(t, school, author.ID, days(31), true, 800)

	suite.run(t, school)

	assert.True(t, suite.exists(t, "users.staff_notices", kept))
	assert.False(t, suite.exists(t, "users.staff_notices", expired), "a shorter contractual window deletes earlier")
	assert.Equal(t, []int{1}, suite.loggedDeletions(t, school, auditModels.DeletionTypeStaffNoticeRetention))
}

// --- appointments ---

func (s retentionSuite) insertAppointment(t *testing.T, tenantID, organizerID int64, startDaysAgo, endDaysAgo int) int64 {
	t.Helper()
	return s.insertID(t, `INSERT INTO calendar.appointments
		(tenant_id, organizer_staff_id, title, start_date, end_date, start_time, end_time, delivery_mode)
		VALUES (?, ?, 'Elternabend', `+berlinDaysAgo+`, `+berlinDaysAgo+`, '18:00', '19:00', 'informational')
		RETURNING id`, tenantID, organizerID, startDaysAgo, endDaysAgo)
}

func (s retentionSuite) insertWeeklyRule(t *testing.T, tenantID, appointmentID int64, endsOnDaysAgo, occurrences *int) {
	t.Helper()
	s.insertID(t, `INSERT INTO calendar.recurrence_rules (tenant_id, appointment_id, frequency, interval_count, ends_on, occurrence_count)
		VALUES (?, ?, 'weekly', 1, `+berlinDaysAgo+`, ?) RETURNING id`, tenantID, appointmentID, endsOnDaysAgo, occurrences)
}

func TestNightlyRetentionDeletesEndedAppointmentsAfterTheTimetableWindow(t *testing.T) {
	t.Parallel()
	suite := newRetentionSuite(t)
	school := testpkg.Tenant(t)
	otherSchool, _ := testpkg.CreateTestTenant(t, suite.admin)
	organizer := testpkg.CreateTestStaff(t, suite.admin, "Termin", "Frist")
	otherOrganizer := testpkg.CreateTestStaffForTenant(t, suite.admin, otherSchool, "Andere", "Schule")

	ended := suite.insertAppointment(t, school, organizer.ID, 367, 366)
	keptOnCutoff := suite.insertAppointment(t, school, organizer.ID, 365, 365)
	seriesEnded := suite.insertAppointment(t, school, organizer.ID, 700, 700)
	suite.insertWeeklyRule(t, school, seriesEnded, days(400), nil)
	seriesRunning := suite.insertAppointment(t, school, organizer.ID, 700, 700)
	suite.insertWeeklyRule(t, school, seriesRunning, days(300), nil)
	endless := suite.insertAppointment(t, school, organizer.ID, 900, 900)
	suite.insertWeeklyRule(t, school, endless, nil, nil)
	countedEnded := suite.insertAppointment(t, school, organizer.ID, 500, 500)
	suite.insertWeeklyRule(t, school, countedEnded, nil, days(10)) // last occurrence 437 days back
	countedRunning := suite.insertAppointment(t, school, organizer.ID, 400, 400)
	suite.insertWeeklyRule(t, school, countedRunning, nil, days(10)) // last occurrence 337 days back
	moved := suite.insertAppointment(t, school, organizer.ID, 500, 500)
	suite.insertID(t, `INSERT INTO calendar.appointment_occurrence_overrides (tenant_id, appointment_id, occurrence_date, start_date, end_date)
		VALUES (?, ?, `+berlinDaysAgo+`, `+berlinDaysAgo+`, `+berlinDaysAgo+`) RETURNING id`, school, moved, 500, 100, 100)
	otherEnded := suite.insertAppointment(t, otherSchool, otherOrganizer.ID, 400, 400)
	suite.insertID(t, `INSERT INTO calendar.appointment_recipients (tenant_id, appointment_id, recipient_type, staff_id, status)
		VALUES (?, ?, 'staff', ?, 'info') RETURNING id`, school, ended, organizer.ID)

	suite.run(t, school)

	assert.False(t, suite.exists(t, "calendar.appointments", ended), "an appointment that ended more than 365 days ago is deleted")
	assert.True(t, suite.exists(t, "calendar.appointments", keptOnCutoff), "the day exactly 365 days back is still inside the window")
	assert.False(t, suite.exists(t, "calendar.appointments", seriesEnded), "a series counts from its last day")
	assert.True(t, suite.exists(t, "calendar.appointments", seriesRunning))
	assert.True(t, suite.exists(t, "calendar.appointments", endless), "an endless series never ends")
	assert.False(t, suite.exists(t, "calendar.appointments", countedEnded), "a counted series ends with its last occurrence")
	assert.True(t, suite.exists(t, "calendar.appointments", countedRunning))
	assert.True(t, suite.exists(t, "calendar.appointments", moved), "an occurrence moved to a later day keeps the series")
	assert.True(t, suite.exists(t, "calendar.appointments", otherEnded), "the run of one school never touches another")
	assert.Zero(t, suite.count(t, `SELECT COUNT(*) FROM calendar.recurrence_rules WHERE appointment_id = ?`, seriesEnded))
	assert.Zero(t, suite.count(t, `SELECT COUNT(*) FROM calendar.appointment_recipients WHERE appointment_id = ?`, ended))
	assert.Equal(t, []int{3}, suite.loggedDeletions(t, school, auditModels.DeletionTypeAppointmentRetention))
	assert.Empty(t, suite.loggedDeletions(t, otherSchool, auditModels.DeletionTypeAppointmentRetention))
}

func TestNightlyRetentionFollowsTheSchoolsRecordedTimetableWindowForAppointments(t *testing.T) {
	t.Parallel()
	suite := newRetentionSuite(t)
	school := testpkg.Tenant(t)
	organizer := testpkg.CreateTestStaff(t, suite.admin, "Kurze", "Terminfrist")
	suite.setWindow(t, school, "gdpr.timetable_retention_days", 30)

	kept := suite.insertAppointment(t, school, organizer.ID, 30, 30)
	expired := suite.insertAppointment(t, school, organizer.ID, 31, 31)

	suite.run(t, school)

	assert.True(t, suite.exists(t, "calendar.appointments", kept))
	assert.False(t, suite.exists(t, "calendar.appointments", expired), "a shorter contractual window deletes earlier")
	assert.Equal(t, []int{1}, suite.loggedDeletions(t, school, auditModels.DeletionTypeAppointmentRetention))
}

func TestNightlyRetentionDeletesCancelledAppointmentsAfterTheFeedWindow(t *testing.T) {
	t.Parallel()
	suite := newRetentionSuite(t)
	school := testpkg.Tenant(t)
	organizer := testpkg.CreateTestStaff(t, suite.admin, "Abgesagt", "Frist")

	cancelled := suite.insertAppointment(t, school, organizer.ID, -10, -10)
	deleted := suite.insertAppointment(t, school, organizer.ID, -20, -20)
	suite.insertWeeklyRule(t, school, deleted, days(-200), nil)
	cancelledRecently := suite.insertAppointment(t, school, organizer.ID, -30, -30)
	oldDeletedRecently := suite.insertAppointment(t, school, organizer.ID, 500, 500)
	_, err := suite.admin.ExecContext(context.Background(), `UPDATE calendar.appointments SET
		cancelled_at = CASE WHEN id = ? THEN CURRENT_TIMESTAMP - INTERVAL '91 days'
			WHEN id = ? THEN CURRENT_TIMESTAMP - INTERVAL '89 days' END,
		deleted_at = CASE WHEN id = ? THEN CURRENT_TIMESTAMP - INTERVAL '91 days' END
		WHERE id IN (?, ?, ?)`, cancelled, cancelledRecently, deleted, cancelled, deleted, cancelledRecently)
	require.NoError(t, err)
	_, err = suite.admin.ExecContext(context.Background(), `UPDATE calendar.appointments
		SET deleted_at = CURRENT_TIMESTAMP - INTERVAL '1 day' WHERE id = ?`, oldDeletedRecently)
	require.NoError(t, err)

	suite.run(t, school)

	assert.False(t, suite.exists(t, "calendar.appointments", cancelled), "a cancelled appointment goes 90 days after the cancellation")
	assert.False(t, suite.exists(t, "calendar.appointments", deleted), "a deleted appointment goes 90 days after the deletion")
	assert.True(t, suite.exists(t, "calendar.appointments", cancelledRecently), "calendar feeds still show the cancellation")
	assert.True(t, suite.exists(t, "calendar.appointments", oldDeletedRecently),
		"an old appointment deleted yesterday keeps its feed window instead of going with the timetable window")
	assert.Zero(t, suite.count(t, `SELECT COUNT(*) FROM calendar.recurrence_rules WHERE appointment_id = ?`, deleted),
		"the rule goes with its appointment")
	assert.Equal(t, []int{2}, suite.loggedDeletions(t, school, auditModels.DeletionTypeAppointmentTombstoneRetention))
	assert.Empty(t, suite.loggedDeletions(t, school, auditModels.DeletionTypeAppointmentRetention))
}

// --- grade transitions ---

func (s retentionSuite) insertTransition(t *testing.T, tenantID, accountID int64, year, status string, endedDaysAgo int) int64 {
	t.Helper()
	ended := `CURRENT_TIMESTAMP - make_interval(days => ?)`
	return s.insertID(t, `INSERT INTO education.grade_transitions
		(tenant_id, academic_year, status, created_by, created_at, updated_at, applied_at, applied_by, reverted_at, reverted_by)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP - INTERVAL '900 days', `+ended+`,
			CASE WHEN ? <> 'draft' THEN `+ended+` END, CASE WHEN ? <> 'draft' THEN ?::bigint END,
			CASE WHEN ? = 'reverted' THEN `+ended+` END, CASE WHEN ? = 'reverted' THEN ?::bigint END)
		RETURNING id`, tenantID, year, status, accountID, endedDaysAgo,
		status, endedDaysAgo, status, accountID, status, endedDaysAgo, status, accountID)
}

func TestNightlyRetentionDeletesGradeTransitionsAfterTheStudentWindow(t *testing.T) {
	t.Parallel()
	suite := newRetentionSuite(t)
	school := testpkg.Tenant(t)
	otherSchool, _ := testpkg.CreateTestTenant(t, suite.admin)
	account := testpkg.CreateTestAccount(t, suite.admin, "transition-retention@example.com")

	appliedLongAgo := suite.insertTransition(t, school, account.ID, "2020-2021", "applied", 400)
	appliedRecently := suite.insertTransition(t, school, account.ID, "2021-2022", "applied", 300)
	revertedLongAgo := suite.insertTransition(t, school, account.ID, "2022-2023", "reverted", 400)
	revertedRecently := suite.insertTransition(t, school, account.ID, "2023-2024", "reverted", 300)
	staleDraft := suite.insertTransition(t, school, account.ID, "2024-2025", "draft", 400)
	otherApplied := suite.insertTransition(t, otherSchool, account.ID, "2020-2021", "applied", 400)
	graduate := testpkg.CreateTestStudent(t, suite.admin, "Abgang", "Frist", "4a")
	suite.insertID(t, `INSERT INTO education.grade_transition_history
		(tenant_id, transition_id, student_id, person_name, from_class, to_class, action)
		VALUES (?, ?, ?, 'Abgang Frist', '4a', NULL, 'graduated') RETURNING id`, school, appliedLongAgo, graduate.ID)

	suite.run(t, school)

	assert.False(t, suite.exists(t, "education.grade_transitions", appliedLongAgo), "an application more than 365 days back is deleted")
	assert.True(t, suite.exists(t, "education.grade_transitions", appliedRecently))
	assert.False(t, suite.exists(t, "education.grade_transitions", revertedLongAgo), "a reverted transition counts from its reversal")
	assert.True(t, suite.exists(t, "education.grade_transitions", revertedRecently))
	assert.False(t, suite.exists(t, "education.grade_transitions", staleDraft), "a draft counts from its last change")
	assert.True(t, suite.exists(t, "education.grade_transitions", otherApplied), "the run of one school never touches another")
	assert.Zero(t, suite.count(t, `SELECT COUNT(*) FROM education.grade_transition_history WHERE transition_id = ?`, appliedLongAgo),
		"the history with the children's names goes with its transition")
	assert.Equal(t, []int{3}, suite.loggedDeletions(t, school, auditModels.DeletionTypeGradeTransitionRetention))
}

// --- removed files ---

func TestNightlyRetentionDeletesRemovedFileRowsAfterTheirBytes(t *testing.T) {
	t.Parallel()
	suite := newRetentionSuite(t)
	school := testpkg.Tenant(t)
	otherSchool, _ := testpkg.CreateTestTenant(t, suite.admin)
	uploader := testpkg.CreateTestAccount(t, suite.admin, "file-retention@example.com")
	folder := suite.insertID(t, `INSERT INTO documents.folders (tenant_id, name, visibility, created_by)
		VALUES (?, 'Ablage', 'all_staff', ?) RETURNING id`, school, uploader.ID)
	otherFolder := suite.insertID(t, `INSERT INTO documents.folders (tenant_id, name, visibility, created_by)
		VALUES (?, 'Ablage', 'all_staff', ?) RETURNING id`, otherSchool, uploader.ID)
	file := func(tenantID, folderID int64, name string, deletedDaysAgo, bytesGoneDaysAgo *int) int64 {
		return suite.insertID(t, `INSERT INTO documents.files
			(tenant_id, folder_id, filename_display, filename_stored, size_bytes, content_type, uploaded_by, created_at, deleted_at, deleted_by, file_deleted_at)
			VALUES (?, ?, 'plan.pdf', ?, 10, 'application/pdf', ?, CURRENT_TIMESTAMP - INTERVAL '900 days',
				CURRENT_TIMESTAMP - make_interval(days => ?::integer), CASE WHEN ?::integer IS NOT NULL THEN ?::bigint END,
				CURRENT_TIMESTAMP - make_interval(days => ?::integer))
			RETURNING id`, tenantID, folderID, name, uploader.ID, deletedDaysAgo, deletedDaysAgo, uploader.ID, bytesGoneDaysAgo)
	}

	removed := file(school, folder, "retention-removed.pdf", days(40), days(31))
	removedRecently := file(school, folder, "retention-recent.pdf", days(40), days(29))
	bytesPending := file(school, folder, "retention-pending.pdf", days(400), nil)
	live := file(school, folder, "retention-live.pdf", nil, nil)
	otherRemoved := file(otherSchool, otherFolder, "retention-other.pdf", days(40), days(31))

	suite.run(t, school)

	assert.False(t, suite.exists(t, "documents.files", removed), "a removed file's row goes 30 days after its bytes")
	assert.True(t, suite.exists(t, "documents.files", removedRecently))
	assert.True(t, suite.exists(t, "documents.files", bytesPending), "the row stays until its bytes are gone")
	assert.True(t, suite.exists(t, "documents.files", live), "a file in use stays until the school deletes it")
	assert.True(t, suite.exists(t, "documents.files", otherRemoved), "the run of one school never touches another")
	assert.Equal(t, []int{1}, suite.loggedDeletions(t, school, auditModels.DeletionTypeRemovedFileRetention))
}

func TestRemovedFileRetentionRefusesAYoungCutoffAndAnotherSchool(t *testing.T) {
	t.Parallel()
	suite := newRetentionSuite(t)
	school := testpkg.Tenant(t)
	otherSchool, _ := testpkg.CreateTestTenant(t, suite.admin)
	uploader := testpkg.CreateTestAccount(t, suite.admin, "file-refusal@example.com")
	folder := suite.insertID(t, `INSERT INTO documents.folders (tenant_id, name, visibility, created_by)
		VALUES (?, 'Ablage', 'all_staff', ?) RETURNING id`, school, uploader.ID)
	removed := suite.insertID(t, `INSERT INTO documents.files
		(tenant_id, folder_id, filename_display, filename_stored, size_bytes, content_type, uploaded_by, deleted_at, deleted_by, file_deleted_at)
		VALUES (?, ?, 'plan.pdf', 'retention-refusal.pdf', 10, 'application/pdf', ?, CURRENT_TIMESTAMP - INTERVAL '2 days', ?,
			CURRENT_TIMESTAMP - INTERVAL '1 day')
		RETURNING id`, school, folder, uploader.ID, uploader.ID)

	inSchool := func(query string, args ...any) error {
		return testpkg.WithinTenantContext(t, context.Background(), suite.serve, school, func(ctx context.Context) error {
			transaction, _ := tenant.TransactionFromContext(ctx)
			tx, ok := transaction.(bun.Tx)
			require.True(t, ok, "the school's tenant transaction")
			_, err := tx.ExecContext(ctx, query, args...)
			return err
		})
	}
	require.ErrorContains(t, inSchool(`SELECT documents.delete_removed_files(?, CURRENT_TIMESTAMP)`, school),
		"removed file cutoff exceeds retention window")
	require.Error(t, inSchool(`SELECT documents.delete_removed_files(?, CURRENT_TIMESTAMP - INTERVAL '31 days')`, otherSchool),
		"the function accepts only the tenant of the calling transaction")
	require.ErrorContains(t, inSchool(`DELETE FROM documents.files WHERE id = ?`, removed),
		"permission denied for table files", "the tenant role keeps no direct DELETE on the file rows")
	assert.True(t, suite.exists(t, "documents.files", removed))
}
