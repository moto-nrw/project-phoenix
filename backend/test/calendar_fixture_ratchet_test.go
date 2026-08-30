package test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const legacyCalendarClockReason = "pre-#2571 finding"

// calendarFixtureClockLegacyBaseline is the exact pre-#2571 debt. It is not an
// exception process: stale entries fail, no new entries belong here, and the
// baseline may only shrink.
var calendarFixtureClockLegacyBaseline = map[string]string{
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
	// Source patterns added to the ratchet still need an exact baseline for
	// tests that predate #2571. Keep this list shrink-only.
	"api/active/checkin_test.go:TestAttendance_Fields":                                                                                                                                                                legacyCalendarClockReason,
	"api/birthdays/api_test.go:TestOverviewListsTodaysChildren":                                                                                                                                                       legacyCalendarClockReason,
	"api/display/api_test.go:TestDisplayDashboardPublic":                                                                                                                                                              legacyCalendarClockReason,
	"api/staff/absence_admin_test.go:TestAdminCreateStaffAbsence_CompTimeAllowedForManager":                                                                                                                           legacyCalendarClockReason,
	"api/staff/absence_admin_test.go:TestAdminCreateStaffAbsence_SickCascades":                                                                                                                                        legacyCalendarClockReason,
	"api/staff/time_tracking_handlers_internal_test.go:TestParseYearQuery_DefaultsToBerlinCalendarYear":                                                                                                               legacyCalendarClockReason,
	"api/students/attendance_history_export_test.go:TestParseAttendanceExportOptions_AcceptsToday":                                                                                                                    legacyCalendarClockReason,
	"api/students/care_exit_handlers_test.go:TestCareExitHandlers_PreviewThenConfirm":                                                                                                                                 legacyCalendarClockReason,
	"api/students/care_exit_handlers_test.go:TestStudentList_MarksRecordedExitsOnly":                                                                                                                                  legacyCalendarClockReason,
	"api/students/day_log_handlers_test.go:TestGetStudentsDayLog_AdminSeesStatuses":                                                                                                                                   legacyCalendarClockReason,
	"api/students/day_log_logic_test.go:TestParseDayLogDateRejectsHistoryWithoutDatedGroupAssignments":                                                                                                                legacyCalendarClockReason,
	"api/students/ogs_group_live_test.go:TestOGSGroupLive_AggregatesGroupData":                                                                                                                                        legacyCalendarClockReason,
	"api/students/status_day_internal_test.go:TestStudentStatusDayHandlers_TodayUpdatesLiveStatusAndClearsOpposite":                                                                                                   legacyCalendarClockReason,
	"api/students/status_day_overview_test.go:TestGetStudentStatusDaysOverview_AdminSeesEntries":                                                                                                                      legacyCalendarClockReason,
	"api/students/update_class_resync_test.go:TestUpdateStudent_ClassChangeResyncsOfferingSourcedTemplates":                                                                                                           legacyCalendarClockReason,
	"api/timetable/instances_create_test.go:TestCreateInstance_DuplicateTemplateBoundReturnsConflict":                                                                                                                 legacyCalendarClockReason,
	"api/timetable/templates_series_test.go:TestUpdateTemplate_SeriesRosterFromReachesPredecessor":                                                                                                                    legacyCalendarClockReason,
	"api/timetable/templates_split_test.go:TestTemplateEndHandler_HappyPath":                                                                                                                                          legacyCalendarClockReason,
	"api/timetable/templates_split_test.go:TestTemplateSplitHandler_UpdateSuccessorPreservesValidFrom":                                                                                                                legacyCalendarClockReason,
	"api/timetable/templates_split_test.go:TestTemplateUpdateHandler_RejectsInconsistentValidityEnvelopeWithoutMutation":                                                                                              legacyCalendarClockReason,
	"api/timetable/templates_test.go:TestListTemplates_CapacityFields":                                                                                                                                                legacyCalendarClockReason,
	"database/migrations/001015314_template_source_school_classes_test.go:TestTemplateSourceSchoolClassesDownPreservesSourcedEnrollmentHistory":                                                                       legacyCalendarClockReason,
	"database/repositories/active/attendance_date_range_test.go:TestAttendanceRepository_FindByStudentAndDateRange":                                                                                                   legacyCalendarClockReason,
	"database/repositories/active/bulk_readers_test.go:TestGroupSupervisorRepository_ListActiveSupervisedRooms":                                                                                                       legacyCalendarClockReason,
	"database/repositories/active/group_supervisor_repository_test.go:TestGroupSupervisorRepository_EndAllActiveByStaffID":                                                                                            legacyCalendarClockReason,
	"database/repositories/active/staff_absence_test.go:TestStaffAbsenceRepository_GetByStaffAndDateRange":                                                                                                            legacyCalendarClockReason,
	"database/repositories/active/student_status_day_test.go:TestStudentStatusDayRepository_NoteOnReReport":                                                                                                           legacyCalendarClockReason,
	"database/repositories/active/student_status_day_test.go:TestStudentStatusDayRepository_TenantScope":                                                                                                              legacyCalendarClockReason,
	"database/repositories/active/student_status_day_test.go:TestStudentStatusDayRepository_UpsertAndFind":                                                                                                            legacyCalendarClockReason,
	"database/repositories/active/work_session_test.go:TestWorkSessionRepository_GetHistoryByStaffID":                                                                                                                 legacyCalendarClockReason,
	"database/repositories/active/work_session_test.go:TestWorkSessionRepository_GetHistoryByStaffIDWrapsDatabaseError":                                                                                               legacyCalendarClockReason,
	"database/repositories/enrollment/request_child_offering_repository_capacity_range_test.go:TestRequestChildOfferingRepository_CountMaxActiveByCareOfferingInRangeExcludingRequestChild_ExcludesReplacedIntervals": legacyCalendarClockReason,
	"database/repositories/enrollment/request_child_offering_repository_capacity_range_test.go:TestRequestChildOfferingRepository_CountMaxActiveByCareOfferingInRange_IncludesFutureBookings":                         legacyCalendarClockReason,
	"database/repositories/enrollment/request_child_offering_repository_grade_levels_test.go:TestRequestChildOfferingRepository_Aggregates_CountEveryPhaseLikeTheGate":                                                legacyCalendarClockReason,
	"database/repositories/enrollment/request_child_offering_repository_grade_levels_test.go:TestRequestChildOfferingRepository_CountMaxActiveByIDsInRange_GuardsItsInput":                                            legacyCalendarClockReason,
	"database/repositories/enrollment/request_child_offering_repository_grade_levels_test.go:TestRequestChildOfferingRepository_CountMaxActiveByIDsInRange_MatchesTheSingleOfferingVariant":                           legacyCalendarClockReason,
	"database/repositories/enrollment/request_child_offering_repository_grade_levels_test.go:TestRequestChildOfferingRepository_CountMaxActiveByIDsInRange_SeparatesOfferings":                                        legacyCalendarClockReason,
	"database/repositories/feedback/entry_repository_test.go:TestEntryRepository_FindByDateRange":                                                                                                                     legacyCalendarClockReason,
	"database/repositories/feedback/entry_repository_test.go:TestEntryRepository_FindByStudentAndDateRange":                                                                                                           legacyCalendarClockReason,
	"database/repositories/schedule/staff_shift_repo_test.go:TestStaffShiftRepository_DeleteUpcomingByStaffID":                                                                                                        legacyCalendarClockReason,
	"database/repositories/users/care_withdrawal_completion_test.go:TestCareWithdrawalCompletionRepository_ParticipationBoundaryUsesPendingCompletionWhenEnrollmentIsOpen":                                            legacyCalendarClockReason,
	"database/repositories/users/care_withdrawal_completion_test.go:TestCareWithdrawalCompletionRepository_UpsertUsesIncomingBoundary":                                                                                legacyCalendarClockReason,
	"services/absence/excused_request_service_test.go:TestDecide_ApproveRefusedWhenPartialAbsenceExists":                                                                                                              legacyCalendarClockReason,
	"services/active/attendance_service_test.go:TestGetStudentAttendanceStatus_NotCheckedIn":                                                                                                                          legacyCalendarClockReason,
	"services/active/cleanup_service_test.go:TestCleanupStaleAttendance_CheckOutTimeIsBerlinEndOfDay":                                                                                                                 legacyCalendarClockReason,
	"services/active/cleanup_supervisors_test.go:TestCleanupStaleSupervisors_ClosesYesterdayRecords":                                                                                                                  legacyCalendarClockReason,
	"services/active/staff_absence_service_test.go:TestAbsCreateAbsenceFor_RejectsCompTimeAgainstLaterLedgerCapacity":                                                                                                 legacyCalendarClockReason,
	"services/active/staff_opening_balance_mock_test.go:TestStaffBalanceAdjustmentService_OpeningAllowsNegativeTarget":                                                                                                legacyCalendarClockReason,
	"services/active/student_status_day_write_bulk_test.go:TestBulkCreateForDates_RejectsConflictWithoutPartialWrites":                                                                                                legacyCalendarClockReason,
	"services/active/student_status_day_write_bulk_test.go:TestCreateForDates_RejectsConflictWithoutPartialWrites":                                                                                                    legacyCalendarClockReason,
	"services/active/work_session_autocheckout_mock_test.go:TestAutoCheckout_QueriesOpenSessionsIncludingToday":                                                                                                       legacyCalendarClockReason,
	"services/active/work_session_service_test.go:TestWSApplyCustomScheduleRows_StampsAnchorForFirstRotation":                                                                                                         legacyCalendarClockReason,
	"services/education/grade_transition_offering_resync_test.go:TestGradeTransitionService_ApplyAndRevert_ResyncOfferingSourcedRosters":                                                                              legacyCalendarClockReason,
	"services/education/grade_transition_roster_reconcile_test.go:TestGradeTransitionService_Revert_FillsBackdatedInstance":                                                                                           legacyCalendarClockReason,
	"services/education/grade_transition_roster_reconcile_test.go:TestGradeTransitionService_Revert_FillsTodaysInstanceMaterializedWhileAlumnus":                                                                      legacyCalendarClockReason,
	"services/enrollment/care_offering_booking_stats_internal_test.go:TestBookingStatsWindow_DefaultsToTodayWithoutPhaseDates":                                                                                        legacyCalendarClockReason,
	"services/enrollment/care_offering_booking_stats_internal_test.go:TestListBookingStats_CountsInTheCapacityGatesWindow":                                                                                            legacyCalendarClockReason,
	"services/enrollment/class_roster_care_end_test.go:TestClassRosterFiltersCareDate":                                                                                                                                legacyCalendarClockReason,
	"services/enrollment/decision_service_test.go:TestDecisionService_Decide_ApprovedScheduledPastStartActivatesStudent":                                                                                              legacyCalendarClockReason,
	"services/enrollment/decision_service_test.go:TestDecisionService_ListChildOfferings_CarriesAttributesAndFutureBookings":                                                                                          legacyCalendarClockReason,
	"services/enrollment/offering_change_request_service_test.go:TestOfferingChangeRequestService_ListPending_ReportsDateClampedToThePhaseStart":                                                                      legacyCalendarClockReason,
	"services/enrollment/offering_change_request_service_test.go:TestOfferingChangeRequestService_ListPending_ReportsTheSelectableDateRange":                                                                          legacyCalendarClockReason,
	"services/enrollment/offering_source_service_test.go:TestDecide_MultiSourceFanOutSeedsFromPhaseStart":                                                                                                             legacyCalendarClockReason,
	"services/enrollment/offering_source_service_test.go:TestListOfferingSourceOptions_CountsScopedToSelectedPeriod":                                                                                                  legacyCalendarClockReason,
	"services/enrollment/offering_source_service_test.go:TestUpdateChildOfferings_UndatedCorrectionKeepsPhaseStartOnMultiSource":                                                                                      legacyCalendarClockReason,
	"services/enrollment/report_service_test.go:TestCareUsageEnrichesGuardiansAndSchedulePickup":                                                                                                                      legacyCalendarClockReason,
	"services/enrollment/report_service_test.go:TestClassRosterUsesOfferingDateForPickupProjection":                                                                                                                   legacyCalendarClockReason,
	"services/feedback/errors_test.go:TestInvalidDateRangeError_Unwrap":                                                                                                                                               legacyCalendarClockReason,
	"services/feedback/feedback_service_test.go:TestFeedbackErrors":                                                                                                                                                   legacyCalendarClockReason,
	"services/feedback/feedback_service_test.go:TestFeedbackService_GetEntriesByDateRange":                                                                                                                            legacyCalendarClockReason,
	"services/feedback/feedback_service_test.go:TestFeedbackService_GetEntriesByStudentAndDateRange":                                                                                                                  legacyCalendarClockReason,
	"services/parent/care_exception_service_test.go:TestListCareExceptions_FlagsAbsentArrivalRow":                                                                                                                     legacyCalendarClockReason,
	"services/parent/care_exception_service_test.go:TestListCareExceptions_FlagsAbsentPickupRow":                                                                                                                      legacyCalendarClockReason,
	"services/parent/excused_request_test.go:TestExcusedRequest_ApproveWritesStatusDays":                                                                                                                              legacyCalendarClockReason,
	"services/parent/parent_care_offerings_service_test.go:TestGetChildCareOfferingsReturnsCompleteSortedView":                                                                                                        legacyCalendarClockReason,
	"services/parent/parent_meal_plan_test.go:TestMealPlanWeek_AllowsNextWeek":                                                                                                                                        legacyCalendarClockReason,
	"services/parent/parent_meal_plan_test.go:TestMealPlanWeek_DisabledReturnsSentinel":                                                                                                                               legacyCalendarClockReason,
	"services/parent/parent_meal_plan_test.go:TestMealPlanWeek_FarFutureWeekOutOfRange":                                                                                                                               legacyCalendarClockReason,
	"services/parent/parent_meal_plan_test.go:TestMealPlanWeek_NotOwnedChildRejected":                                                                                                                                 legacyCalendarClockReason,
	"services/parent/parent_meal_plan_test.go:TestMealPlanWeek_PastWeekOutOfRange":                                                                                                                                    legacyCalendarClockReason,
	"services/parent/parent_meal_plan_test.go:TestMealPlanWeek_ReturnsCurrentWeekEntries":                                                                                                                             legacyCalendarClockReason,
	"services/parent/parent_meal_plan_test.go:TestMealPlanWeek_SettingErrorPropagates":                                                                                                                                legacyCalendarClockReason,
	"services/parent/parent_request_edit_test.go:TestEditExcusedRequestReplacesWithdrawal":                                                                                                                            legacyCalendarClockReason,
	"services/parent/parent_write_service_test.go:TestListSickDays_ExcludesStaffCreatedExcused":                                                                                                                       legacyCalendarClockReason,
	"services/parent/parent_write_service_test.go:TestListSickDays_ReturnsSickAndExcused":                                                                                                                             legacyCalendarClockReason,
	"services/parent/parent_write_service_test.go:TestSubmitSickNote_ClearsClassTripForSubmittedDate":                                                                                                                 legacyCalendarClockReason,
	"services/parent/parent_write_service_test.go:TestSubmitSickNote_FutureWriteSerializesWithStaffConflictCheck":                                                                                                     legacyCalendarClockReason,
	"services/parent/parent_write_service_test.go:TestSubmitSickNote_NonContiguousExcludesUnrelatedRows":                                                                                                              legacyCalendarClockReason,
	"services/parent/parent_write_service_test.go:TestSubmitSickNote_RefusesPartialAbsenceConflict":                                                                                                                   legacyCalendarClockReason,
	"services/parent/sick_note_gate_pin_test.go:TestSickNoteStaysImmediateWhenApprovalDisabled":                                                                                                                       legacyCalendarClockReason,
	"services/schedule/partial_absence_pending_request_test.go:TestPartialAbsenceCreate_RefusesPendingFullDayRequest":                                                                                                 legacyCalendarClockReason,
	"services/schedule/shift_plan_sync_service_test.go:TestSickCascade_ClearSickForRange":                                                                                                                             legacyCalendarClockReason,
	"services/schedule/shift_plan_sync_service_test.go:TestSickCascade_ConcurrentOverlappingReportsSerializeBeforeOverlapRead":                                                                                        legacyCalendarClockReason,
	"services/schedule/shift_plan_sync_service_test.go:TestSickCascade_UpdateRangeRollsBackWhenRemovedShiftCannotReactivate":                                                                                          legacyCalendarClockReason,
	"services/schedule/staff_shift_series_integration_test.go:TestStaffScheduleOverview_SeriesFieldsRideExistingReads":                                                                                                legacyCalendarClockReason,
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_CapAllByStaffIDClampsFutureSeries":                                                                                                 legacyCalendarClockReason,
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_CollisionSkipsAndReports":                                                                                                          legacyCalendarClockReason,
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_CreateMaterializesFromTomorrow":                                                                                                    legacyCalendarClockReason,
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_CreateRejectsBadReferences":                                                                                                        legacyCalendarClockReason,
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_EditDetachesAndDeleteRecordsException":                                                                                             legacyCalendarClockReason,
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_EndAtFirstOccurrence":                                                                                                              legacyCalendarClockReason,
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_EndSeriesKeepsDetachedAndPast":                                                                                                     legacyCalendarClockReason,
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_SplitAtFirstOccurrence":                                                                                                            legacyCalendarClockReason,
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_SplitOutsideSegmentRejected":                                                                                                       legacyCalendarClockReason,
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_SplitPreservesDeviationsOnSuccessor":                                                                                               legacyCalendarClockReason,
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_SplitTodayUpdatesOccurrenceAndReplansTomorrow":                                                                                     legacyCalendarClockReason,
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_WeekPatternRequiresCycle":                                                                                                          legacyCalendarClockReason,
	"services/schedule/staff_shift_series_mock_test.go:TestEndSeriesUnit_ErrorBranches":                                                                                                                               legacyCalendarClockReason,
	"services/schedule/staff_shift_series_mock_test.go:TestGetSeriesUnit":                                                                                                                                             legacyCalendarClockReason,
	"services/schedule/staff_shift_series_mock_test.go:TestSplitSeriesUnit_ErrorBranches":                                                                                                                             legacyCalendarClockReason,
	"services/schedule/staff_shift_series_rule_edit_test.go:TestStaffShiftSeries_SplitAppliesNewWeekdaysFromEffectiveDate":                                                                                            legacyCalendarClockReason,
	"services/schedule/staff_shift_series_rule_edit_test.go:TestStaffShiftSeries_SplitBoundsEarlierSegmentAtNextSuccessor":                                                                                            legacyCalendarClockReason,
	"services/schedule/staff_shift_series_rule_edit_test.go:TestStaffShiftSeries_SplitExtendsSeriesEndingToday":                                                                                                       legacyCalendarClockReason,
	"services/schedule/staff_shift_series_rule_edit_test.go:TestStaffShiftSeries_SplitKeepsStoredValidityWhenUnset":                                                                                                   legacyCalendarClockReason,
	"services/schedule/staff_shift_series_rule_edit_test.go:TestStaffShiftSeries_SplitRejectsSupersededSegment":                                                                                                       legacyCalendarClockReason,
	"services/schedule/staff_shift_series_rule_edit_test.go:TestStaffShiftSeries_SplitRejectsValidityBeyondCalendarPeriod":                                                                                            legacyCalendarClockReason,
	"services/schedule/staff_shift_series_rule_edit_test.go:TestStaffShiftSeries_SplitRejectsWhenNoOccurrenceRemains":                                                                                                 legacyCalendarClockReason,
	"services/schedule/staff_shift_series_rule_edit_test.go:TestStaffShiftSeries_SplitShortensValidityAndDropsLaterShifts":                                                                                            legacyCalendarClockReason,
	"services/schedule/template_end_service_unit_test.go:TestTemplateEndFromDate_ReturnsSummaryAndDeletesOpenEndedWindow":                                                                                             legacyCalendarClockReason,
	"services/schedule/template_offering_source_unit_test.go:TestResyncUpdatedTemplateOfferingRoster":                                                                                                                 legacyCalendarClockReason,
	"services/schedule/template_series_roster_mock_test.go:TestReconcileSeriesPredecessorRoster_CreatesBoundedRows":                                                                                                   legacyCalendarClockReason,
	"services/users/care_booking_authority_integration_test.go:TestBookingMutationPlansFutureNaturalEndImmediately":                                                                                                   legacyCalendarClockReason,
	"services/users/care_booking_authority_integration_test.go:TestOverdueRebookingReplacesTheStaleCompletion":                                                                                                        legacyCalendarClockReason,
	"services/users/care_lifecycle_service_test.go:TestCareLifecycle_CancelPutsThePlanBack":                                                                                                                           legacyCalendarClockReason,
	"services/users/care_lifecycle_service_test.go:TestCareLifecycle_CancelRefusesOrdinaryEnrollmentEnd":                                                                                                              legacyCalendarClockReason,
	"services/users/care_lifecycle_service_test.go:TestCareLifecycle_CancelRestoresPreviousEnrollmentEnd":                                                                                                             legacyCalendarClockReason,
	"services/users/care_lifecycle_service_test.go:TestCareLifecycle_LastCareDayIsInclusive":                                                                                                                          legacyCalendarClockReason,
	"services/users/care_lifecycle_service_test.go:TestCareLifecycle_Resume":                                                                                                                                          legacyCalendarClockReason,
	"services/users/care_withdrawal_lifecycle_test.go:TestCareWithdrawalLifecycle_AllowsRetroactiveExitButNotBeforeAttendance":                                                                                        legacyCalendarClockReason,
	"services/users/care_withdrawal_lifecycle_test.go:TestCareWithdrawalLifecycle_CancellingPlannedExitRestoresTask":                                                                                                  legacyCalendarClockReason,
	"services/users/person_service_eligibility_test.go:TestFilterStudentsEligibleOnDate_IncludesImmediatelyActiveFutureStudentToday":                                                                                  legacyCalendarClockReason,
}

// calendarFixtureClockExceptions contains only tests whose purpose requires
// the system clock. Every exact function key needs its own reviewed reason.
var calendarFixtureClockExceptions = map[string]string{
	"services/scheduler/scheduler_test.go:TestIsoWeekdayMatchesNow": "the test explicitly compares the scheduler's live ISO weekday helper with time.Now",
}

func TestCalendarFixtureClockRatchet(t *testing.T) {
	t.Parallel()

	backendRoot, err := findBackendRoot()
	if err != nil {
		t.Skipf("Could not find backend root: %v", err)
	}
	findings, err := scanCalendarFixtureClockRisks(backendRoot)
	if err == nil {
		findings, err = applyCalendarClockLegacyBaseline(findings, calendarFixtureClockLegacyBaseline)
	}
	if err == nil {
		findings, err = applyCalendarClockExceptions(findings, calendarFixtureClockExceptions)
	}
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

	findings, err := scanCalendarFixtureClockRisks(root)
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

	findings, err := scanCalendarFixtureClockRisks(root)
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
type DateRange struct { From, To tz.Date }
func TestHistoryRange(t *testing.T) {
	t.Parallel()
	from := tz.TodayDate().AddDays(-7)
	to := tz.TodayDate()
	fixed := tz.NewDate(2026, 8, 30)
	_ = GetHistory(from, fixed)
	_ = FindByDateRange(fixed, to)
	_ = DateRange{From: from, To: fixed}
}
`)

	findings, err := scanCalendarFixtureClockRisks(root)
	if err != nil {
		t.Fatal(err)
	}
	requireCalendarFinding(t, findings, "TestHistoryRange", "live clock defines a calendar range")
}

func TestCalendarFixtureRatchetFollowsLiveDateHelperIntoAssertion(t *testing.T) {
	t.Parallel()

	root := writeCalendarFixtureSource(t, "sample/helper_test.go", `package sample
import (
	"testing"
	assertpkg "github.com/stretchr/testify/assert"
	tz "github.com/moto-nrw/project-phoenix/internal/timezone"
)
func fixtureDate() tz.Date { return tz.TodayDate() }
func TestCalendarExpectation(t *testing.T) {
	t.Parallel()
	got := struct{ Date tz.Date }{}
	assertpkg.Equal(t, fixtureDate(), got.Date)
}
`)

	findings, err := scanCalendarFixtureClockRisks(root)
	if err != nil {
		t.Fatal(err)
	}
	requireCalendarFinding(t, findings, "TestCalendarExpectation", "live calendar date used as an expectation")
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

	findings, err := scanCalendarFixtureClockRisks(root)
	if err != nil {
		t.Fatal(err)
	}
	findings, err = applyCalendarClockExceptions(findings, map[string]string{
		key: "the production contract is explicitly relative to the current Berlin day",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("reviewed exception did not suppress its exact test: %v", findings)
	}

	findings, err = scanCalendarFixtureClockRisks(root)
	if err != nil {
		t.Fatal(err)
	}
	_, err = applyCalendarClockExceptions(findings, map[string]string{key: ""})
	if err == nil || !strings.Contains(err.Error(), "non-empty reason") {
		t.Fatalf("empty exception reason must fail, got %v", err)
	}

	findings, err = scanCalendarFixtureClockRisks(root)
	if err != nil {
		t.Fatal(err)
	}
	_, err = applyCalendarClockExceptions(findings, map[string]string{"sample/range_test.go:TestOther": "typo"})
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
	elapsedStart := time.Now()
	history := struct{ WeeklySummaries []int }{}
	time := fakeClock{}
	_ = []any{base, from, to, checkIn, elapsedStart, history.WeeklySummaries, time.Now(), "time.Now().Add(-2h)"}
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

	findings, err := scanCalendarFixtureClockRisks(root)
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
