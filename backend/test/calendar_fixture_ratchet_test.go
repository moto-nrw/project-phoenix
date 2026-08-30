package test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const legacyCalendarClockReason = "pre-#2571 current-day test; rewriting its fixture is outside this ratchet ticket"

// calendarFixtureClockExceptions is keyed by exact file and top-level test.
// The shared reason is reserved for the pre-#2571 baseline below. New entries
// must instead explain why observing the live clock is load-bearing; entries
// fail when their finding disappears, so this list can only shrink unnoticed.
var calendarFixtureClockExceptions = map[string]string{
	"api/active/handlers_unit_test.go:TestNewActiveGroupResponse_WithActiveSupervisors":                                                         legacyCalendarClockReason,
	"api/active/handlers_unit_test.go:TestNewSupervisorResponse_ActiveSupervisor":                                                               legacyCalendarClockReason,
	"api/active/handlers_unit_test.go:TestNewSupervisorResponse_WithActiveGroup":                                                                legacyCalendarClockReason,
	"api/active/handlers_unit_test.go:TestNewSupervisorResponse_WithStaff":                                                                      legacyCalendarClockReason,
	"api/display/api_test.go:TestDisplayDashboardPickupBuckets":                                                                                 legacyCalendarClockReason,
	"api/iot/checkin/attendance_internal_test.go:TestAttendanceInfo_Fields":                                                                     legacyCalendarClockReason,
	"api/students/care_exit_handlers_test.go:TestStudentList_CareStatusDecidesWhichSideIsShown":                                                 legacyCalendarClockReason,
	"api/students/care_exit_handlers_test.go:TestStudentList_UsesBookingParticipationButKeepsAdministrationAndLivePresence":                     legacyCalendarClockReason,
	"api/timetable/deviation_log_test.go:TestApplyDeviations_ActiveInstance_EndsAndCreatesSupervisor":                                           legacyCalendarClockReason,
	"api/timetable/instances_create_test.go:TestCreateInstance_Validation":                                                                      legacyCalendarClockReason,
	"database/repositories/active/attendance_repository_test.go:TestAttendanceRepository_CloseOpenForToday":                                     legacyCalendarClockReason,
	"database/repositories/active/group_repository_test.go:TestActiveGroupRepository_FindWithSupervisors":                                       legacyCalendarClockReason,
	"database/repositories/active/student_status_day_test.go:TestStudentStatusDayRepository_ClearByIDAndDates":                                  legacyCalendarClockReason,
	"database/repositories/active/work_session_test.go:TestWorkSessionRepository_GetTodayPresenceMap":                                           legacyCalendarClockReason,
	"database/repositories/schedule/activity_instance_repo_test.go:TestActivityInstanceRepository_DeletePlannedMaterializedWeekendInstances":    legacyCalendarClockReason,
	"database/repositories/users/parent_announcement_test.go:TestParentAnnouncementAudience_WeekdayScopedEnrollmentMatchesToday":                legacyCalendarClockReason,
	"models/active/attendance_test.go:TestAttendance_CompleteLifecycle":                                                                         legacyCalendarClockReason,
	"models/active/attendance_test.go:TestAttendance_Fields":                                                                                    legacyCalendarClockReason,
	"models/active/attendance_test.go:TestAttendance_GetCreatedAt":                                                                              legacyCalendarClockReason,
	"models/active/attendance_test.go:TestAttendance_GetUpdatedAt":                                                                              legacyCalendarClockReason,
	"models/active/attendance_test.go:TestAttendance_IsCheckedIn_WhenCheckedIn":                                                                 legacyCalendarClockReason,
	"models/active/attendance_test.go:TestAttendance_IsCheckedIn_WhenCheckedOut":                                                                legacyCalendarClockReason,
	"models/active/attendance_test.go:TestAttendance_MultipleRecords":                                                                           legacyCalendarClockReason,
	"services/active/active_service_wrappers_internal_test.go:TestActiveServiceThinDelegates":                                                   legacyCalendarClockReason,
	"services/active/analytics_service_test.go:TestGetDashboardAnalytics":                                                                       legacyCalendarClockReason,
	"services/active/update_visit_mock_test.go:TestUpdateVisitLocksAttendanceBeforeClosingIt":                                                   legacyCalendarClockReason,
	"services/active/visit_helpers_test.go:TestCreateVisit_ClearsParentStatusForToday":                                                          legacyCalendarClockReason,
	"services/active/visit_helpers_test.go:TestCreateVisit_ClearsPlannedStatusForToday":                                                         legacyCalendarClockReason,
	"services/active/work_session_export_test.go:TestWSGetHistory_AuditCountError":                                                              legacyCalendarClockReason,
	"services/active/work_session_export_test.go:TestWSGetHistory_BreaksError":                                                                  legacyCalendarClockReason,
	"services/active/work_session_service_test.go:TestWSGetHistory_ClosedSessionKeepsCachedBreaks":                                              legacyCalendarClockReason,
	"services/active/work_session_service_test.go:TestWSGetHistory_DeductsRunningBreakFromNetMinutes":                                           legacyCalendarClockReason,
	"services/active/work_session_service_test.go:TestWSGetHistory_RepoError":                                                                   legacyCalendarClockReason,
	"services/active/work_session_service_test.go:TestWSGetHistory_RunningBreakIsCappedAtTheLiveLimit":                                          legacyCalendarClockReason,
	"services/active/work_session_service_test.go:TestWSGetHistory_SerializesRunningBreakInBreakMinutes":                                        legacyCalendarClockReason,
	"services/calendar/service_integration_test.go:TestCalendarServiceIntegration_CleanupExpiredFeedTombstonesCascadesChildren":                 legacyCalendarClockReason,
	"services/calendar/service_integration_test.go:TestCalendarServiceIntegration_StaffSubscriptionPublishesOccurrenceAndDeletionCancellations": legacyCalendarClockReason,
	"services/calendar/service_integration_test.go:TestCalendarServiceIntegration_SubscriptionFeed":                                             legacyCalendarClockReason,
	"services/schedule/care_request_decision_snapshot_test.go:TestDecide_PickupChangeFreezesDiff":                                               legacyCalendarClockReason,
	"services/schedule/instance_service_integration_test.go:TestInstance_ReplanWeek_RemovesFutureLegacyWeekendInstances":                        legacyCalendarClockReason,
	"services/schedule/instance_service_integration_test.go:TestInstance_Start_ConflictWarning_Staff":                                           legacyCalendarClockReason,
	"services/schedule/instance_service_integration_test.go:TestInstance_Start_StaffBridgedOverrideDifferentRoom_Conflict":                      legacyCalendarClockReason,
	"services/schedule/instance_service_integration_test.go:TestInstance_Start_StaffBridgedOverrideSameRoom_NoConflict":                         legacyCalendarClockReason,
	"services/schedule/instance_service_integration_test.go:TestInstance_Start_StaffBridgedWithoutRosterRow_Conflict":                           legacyCalendarClockReason,
	"services/schedule/instance_service_integration_test.go:TestInstance_Start_StaffSameRoomIsNotAConflict":                                     legacyCalendarClockReason,
	"services/schedule/schedule_service_test.go:TestScheduleService_GenerateEvents":                                                             legacyCalendarClockReason,
	"services/schedule/staff_schedule_overview_integration_test.go:TestShiftCoverageProjection_BatchesEffectiveSeriesReadsAndIsolatesTenant":    legacyCalendarClockReason,
	"services/schedule/staff_schedule_overview_integration_test.go:TestStaffScheduleOverview_WeeklySummariesIncludeShiftsOutsideViewport":       legacyCalendarClockReason,
	"services/schedule/staff_schedule_overview_integration_test.go:TestStaffScheduleOverview_WeeklySummariesResolveSollAndIsolateTenant":        legacyCalendarClockReason,
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_MoveConsumesOriginalDateBeforeRematerialization":             legacyCalendarClockReason,
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_RepeatedMoveKeepsOriginalOccurrenceIdentity":                 legacyCalendarClockReason,
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_WeekPatternARespectsCycle":                                   legacyCalendarClockReason,
	"services/schedule/staff_shift_series_rule_edit_test.go:TestStaffShiftSeries_SplitRejectsExtensionWithoutRecurrenceOccurrence":              legacyCalendarClockReason,
	"services/schedule/staff_shift_series_rule_edit_test.go:TestStaffShiftSeries_SplitRejectsWhenNextSegmentLeavesNoOccurrence":                 legacyCalendarClockReason,
	"services/scheduler/scheduler_test.go:TestIsoWeekdayMatchesNow":                                                                             "the test explicitly compares the scheduler's live ISO weekday helper with time.Now",
}

func TestCalendarFixtureClockRatchet(t *testing.T) {
	t.Parallel()

	backendRoot, err := findBackendRoot()
	if err != nil {
		t.Skipf("Could not find backend root: %v", err)
	}
	findings, err := scanCalendarFixtureClockRisks(backendRoot, calendarFixtureClockExceptions)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) == 0 {
		return
	}
	t.Errorf("Calendar fixture wall-clock ratchet failed (%d finding(s)):\n\n%s\n\n"+
		"These fixtures can cross a Berlin date or ISO-week boundary depending on when CI runs. "+
		"Use timezone.NewDate(...), BerlinMidnight(), or time.Date(...) with a fixed instant. "+
		"If the behavior must observe the live clock, inject it or add the exact file:test key to "+
		"calendarFixtureClockExceptions with a reviewed, non-empty reason.",
		len(findings), strings.Join(formatCalendarClockFindings(findings), "\n"))
}

func TestCalendarFixtureRatchetDetectsEnrollmentPattern(t *testing.T) {
	t.Parallel()

	root := writeCalendarFixtureSource(t, "enrollment/history_test.go", `package enrollment

import (
	stdtime "time"
	tz "github.com/moto-nrw/project-phoenix/internal/timezone"
)

func TestHistoryPeriod(t *testing.T) {
	t.Parallel()
	base := stdtime.Now().UTC().Add(-2 * stdtime.Hour)
	today := tz.DateFromTime(base).String()
	_ = today
}
`)

	findings, err := scanCalendarFixtureClockRisks(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	requireCalendarFinding(t, findings, "enrollment/history_test.go:11", "TestHistoryPeriod")
}

func TestCalendarFixtureRatchetDetectsWorkSessionPattern(t *testing.T) {
	t.Parallel()

	root := writeCalendarFixtureSource(t, "active/work_session_test.go", `package active

import (
	"time"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

func TestHistorySummary(t *testing.T) {
	t.Parallel()
	from := timezone.TodayDate().AddDays(-7)
	to := timezone.TodayDate()
	checkIn := time.Now().Add(-8 * time.Hour)
	checkOut := time.Now().Add(-2 * time.Hour)
	session := WorkSession{CheckInTime: checkIn, CheckOutTime: &checkOut}
	history := GetHistory(session, from, to)
	require.Len(t, history.WeeklySummaries, 1)
}
`)

	findings, err := scanCalendarFixtureClockRisks(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	requireCalendarFinding(t, findings,
		"TestHistorySummary",
		"live calendar date shifted into a range",
		"live instant feeds an ISO-week expectation",
	)
}

func TestCalendarFixtureRatchetDetectsLiveDateRange(t *testing.T) {
	t.Parallel()

	root := writeCalendarFixtureSource(t, "sample/history_test.go", `package sample
import tz "github.com/moto-nrw/project-phoenix/internal/timezone"
func TestHistoryRange(t *testing.T) {
	t.Parallel()
	from := tz.TodayDate().AddDays(-7)
	to := tz.TodayDate()
	_ = GetHistory(from, to)
}
`)

	findings, err := scanCalendarFixtureClockRisks(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	requireCalendarFinding(t, findings, "TestHistoryRange", "live clock defines a calendar range")
}

func TestCalendarFixtureRatchetRequiresReviewedExceptionReason(t *testing.T) {
	t.Parallel()

	root := writeCalendarFixtureSource(t, "sample/range_test.go", `package sample
import tz "github.com/moto-nrw/project-phoenix/internal/timezone"
func TestLiveRange(t *testing.T) {
	t.Parallel()
	from := tz.TodayDate().AddDays(-7)
	_ = from.Weekday()
}
`)
	key := "sample/range_test.go:TestLiveRange"

	findings, err := scanCalendarFixtureClockRisks(root, map[string]string{
		key: "the production contract is explicitly relative to the current Berlin day",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("reviewed exception did not suppress its exact test: %v", findings)
	}

	_, err = scanCalendarFixtureClockRisks(root, map[string]string{key: ""})
	if err == nil || !strings.Contains(err.Error(), "non-empty reason") {
		t.Fatalf("empty exception reason must fail, got %v", err)
	}

	_, err = scanCalendarFixtureClockRisks(root, map[string]string{"sample/range_test.go:TestOther": "typo"})
	if err == nil || !strings.Contains(err.Error(), "no matching finding") {
		t.Fatalf("stale exception must fail, got %v", err)
	}
}

func TestCalendarFixtureRatchetIgnoresFixedAndNonCodePatterns(t *testing.T) {
	t.Parallel()

	safeRoot := writeCalendarFixtureSource(t, "sample/fixed_test.go", `package sample
import (
	"time"
	tz "github.com/moto-nrw/project-phoenix/internal/timezone"
)
type fakeClock struct{}
func (fakeClock) Now() time.Time { return time.Time{} }
func TestFixedFixtures(t *testing.T) {
	t.Parallel()
	base := tz.NewDate(2026, 8, 19).BerlinMidnight().Add(12 * time.Hour)
	from := tz.NewDate(2026, 8, 16)
	to := tz.NewDate(2026, 8, 22)
	checkIn := time.Date(2026, 8, 19, 8, 0, 0, 0, tz.Berlin)
	history := struct{ WeeklySummaries []int }{}
	_ = []any{base, from, to, checkIn, history.WeeklySummaries, fakeClock{}.Now(), "time.Now().Add(-2h)"}
	// timezone.TodayDate().AddDays(-7) is documentation, not syntax.
}
`)
	assertNoCalendarFindings(t, safeRoot)

	productionRoot := writeCalendarFixtureSource(t, "sample/production.go", `package sample
import tz "github.com/moto-nrw/project-phoenix/internal/timezone"
func production() { _ = tz.TodayDate().AddDays(-7).Weekday() }
`)
	assertNoCalendarFindings(t, productionRoot)
}

func assertNoCalendarFindings(t *testing.T, root string) {
	t.Helper()

	findings, err := scanCalendarFixtureClockRisks(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("safe source triggered findings: %v", formatCalendarClockFindings(findings))
	}
}

func writeCalendarFixtureSource(t *testing.T, rel, source string) string {
	t.Helper()

	root := t.TempDir()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	return root
}

func requireCalendarFinding(t *testing.T, findings []calendarClockFinding, wants ...string) {
	t.Helper()

	joined := strings.Join(formatCalendarClockFindings(findings), "\n")
	for _, want := range wants {
		if !strings.Contains(joined, want) {
			t.Errorf("findings %q do not contain %q", joined, want)
		}
	}
}
