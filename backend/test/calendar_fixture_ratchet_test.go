package test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// calendarFixtureClockLegacyBaseline is the exact pre-#2571 debt. It is not an
// exception process: fingerprints make edits fail, no new entries belong here,
// and the baseline may only shrink.
var calendarFixtureClockLegacyBaseline = map[string]string{
	"database/repositories/active/work_session_test.go:TestWorkSessionRepository_ListOverlappingByStaffID_KeepsEarlierStarts":                                                             "5ea9243eec4e82f4",
	"database/repositories/education/group_substitution_repository_test.go:TestGroupSubstitutionRepository_FindOverlapping":                                                               "c41ff44ced1fe7b5",
	"database/repositories/enrollment/request_child_offering_repository_grade_levels_test.go:TestRequestChildOfferingRepository_CountActiveGradeLevels_CountsAChildOncePerOffering":       "7a086446c714a6f3",
	"database/repositories/enrollment/request_child_offering_repository_grade_levels_test.go:TestRequestChildOfferingRepository_CountActiveGradeLevels_EmptyInputSkipsTheQuery":           "8ca869aa01f63fed",
	"database/repositories/enrollment/request_child_offering_repository_grade_levels_test.go:TestRequestChildOfferingRepository_CountActiveGradeLevels_ExcludesIntervalsOutsideTheWindow": "2827f4f2074c477a",
	"database/repositories/enrollment/request_child_offering_repository_grade_levels_test.go:TestRequestChildOfferingRepository_CountActiveGradeLevels_ExcludesTerminalChildren":          "7f28ff40f9e8d244",
	"database/repositories/enrollment/request_child_offering_repository_grade_levels_test.go:TestRequestChildOfferingRepository_CountActiveGradeLevels_GroupsByGrade":                     "04da14393eb372bc",
	"database/repositories/enrollment/request_child_offering_repository_grade_levels_test.go:TestRequestChildOfferingRepository_CountActiveGradeLevels_RejectsAnEmptyWindow":              "714dc40297d0cc41",
	"database/repositories/enrollment/request_child_offering_repository_grade_levels_test.go:TestRequestChildOfferingRepository_CountActiveGradeLevels_ReportsMissingGradeSeparately":     "54363e40e91cc672",
	"services/active/staff_vacation_opening_db_test.go:TestSetVacationOpening_NegativeRemainingAllowsOverdrawnAccount":                                                                    "f91b11dc913fca33",
	"services/active/staff_vacation_opening_db_test.go:TestSetVacationOpening_RejectsOpenCutoff":                                                                                          "102ceeb9fc508971",
	"services/active/staff_vacation_opening_db_test.go:TestSetVacationOpening_RejectsPastVacationYear":                                                                                    "c24cf699c70fa918",
	"services/active/staff_vacation_opening_db_test.go:TestSetVacationOpening_RejectsSecondOpeningForSameYear":                                                                            "70d6cb9fc7f3e2ee",
	"services/active/staff_vacation_opening_db_test.go:TestSetVacationOpening_RejectsVacationAbsencesBeforeCutoff":                                                                        "152f2d583cb589c2",
	"services/active/staff_vacation_opening_db_test.go:TestSetVacationOpening_RespectsCustomQuota":                                                                                        "17896692800d3d7c",
	"services/calendar/service_integration_test.go:TestCalendarServiceIntegration_DeleteFeedVisibleLeavesTombstone":                                                                       "f9a986c8cf661d60",
	"services/education/education_service_test.go:TestUpdateSubstitution_DateValidation":                                                                                                  "9d7b62f626b81cdb",
	"services/enrollment/offering_change_request_automatic_test.go:TestOfferingChangeRequestService_Decide_ExclusionKeepsManualAndRequiredLunchDays":                                      "d30d8787509c52f5",
	"services/enrollment/offering_change_request_automatic_test.go:TestOfferingChangeRequestService_Decide_SnapshotMatchesGrandfatheredAutomaticBooking":                                  "04e5cf1408ecb4f1",
	"services/enrollment/offering_change_request_service_test.go:TestOfferingChangeRequestService_Create_PayloadExcludesAutomaticOfferings":                                               "241c252e3a5f0b6a",
	"services/enrollment/offering_change_request_service_test.go:TestOfferingChangeRequestService_Decide_LogsNoWishWhenTheDateIsKept":                                                     "aaab0eaa93b6f875",
	"services/parent/care_exception_service_test.go:TestDeleteCareExceptionPreservesArrival":                                                                                              "d45c103932c13f47",
	"services/parent/care_exception_service_test.go:TestDeleteCareException_RemovesPickupAndPreservesArrival":                                                                             "9224360811c77868",
	"services/parent/care_exception_service_test.go:TestListCareExceptions_ArrivalRepoErrorSurfaces":                                                                                      "c348bc666c185225",
	"services/parent/care_exception_service_test.go:TestListCareExceptions_NotOwnedChild":                                                                                                 "4bcf8c00cfccc99b",
	"services/parent/care_exception_service_test.go:TestListCareExceptions_RepoErrorSurfaces":                                                                                             "93c190aa90014e4e",
	"services/parent/care_exception_service_test.go:TestSubmitCareException_ClearingLegRemovesIt":                                                                                         "ad842726b3956cda",
	"services/parent/excused_request_test.go:TestSickRequest_ApproveWritesSickStatusAndLiveFlag":                                                                                          "9683e32668ca20c3",
	"services/parent/parent_care_schedule_service_test.go:TestGetChildCareSchedule_TodayAbsentReflectsStatusDay":                                                                          "372d2d97d7891236",
	"services/parent/parent_write_service_test.go:TestListSickDays_AllowsPortalAccessWithoutWritePermissions":                                                                             "dbc657adc9a54f66",
	"services/parent/parent_write_service_test.go:TestListSickDays_HidesAnotherGuardiansReason":                                                                                           "8f9f9da20e2a9698",
	"services/parent/parent_write_service_test.go:TestListSickDays_NotOwned":                                                                                                              "c725fd5e50aeaa70",
	"services/schedule/staff_schedule_overview_integration_test.go:TestStaffScheduleOverview_TenantIsolationAcrossEveryProjectionRead":                                                    "a265295739107fcc",
	"api/timetable/substitutions_bulk_test.go:TestBulkSubstitution_MultiDayWithSubstitute":                                                                                                "123a2520b962f835",
	"services/enrollment/offering_change_full_withdrawal_test.go:TestOfferingChangeRequestService_ListPending_KeepsUntouchedBookingsOutOfTheWarning":                                      "7ff500a2e1b65edb",
	"services/enrollment/offering_change_history_test.go:TestOfferingChangeRequestService_ListHistory":                                                                                    "9236ba268563a02e",
	"services/enrollment/offering_change_request_automatic_test.go:TestOfferingChangeRequestService_Decide_ExclusionSkipsAutoTargetAndRecordsOverride":                                    "0d8a45b210498e8c",
	"services/enrollment/offering_change_request_automatic_test.go:TestOfferingChangeRequestService_Decide_RejectionFallsBackToPayloadSnapshot":                                           "5474ea6e705761ed",
	"services/enrollment/offering_change_request_automatic_test.go:TestOfferingChangeRequestService_Decide_RejectionFreezesDiffSnapshot":                                                  "df9780d79bd51373",
	"services/enrollment/offering_change_request_automatic_test.go:TestOfferingChangeRequestService_GetForStudent_MarksAutomaticDiffEntries":                                              "99cd2d215b82db6a",
	"services/enrollment/offering_change_request_automatic_test.go:TestOfferingChangeRequestService_ListPending_IncludesUnchangedGrandfatheredRuleTarget":                                 "73f401572ac5edd1",
	"services/enrollment/offering_change_request_automatic_test.go:TestOfferingChangeRequestService_ListPending_MarksAutomaticDiffEntries":                                                "16393ddbadd32448",
	"services/enrollment/request_child_offering_repository_date_test.go:TestRequestChildOfferingRepository_ListAtDates_DoesNotReturnHistoricalSelection":                                  "60174e9d7c243b46",
	"services/schedule/template_end_cascade_integration_test.go:TestTemplateEnd_CascadesToCappedPredecessors":                                                                             "ceb836a5b82f47ad",
	"services/schedule/template_end_cascade_integration_test.go:TestTemplateEnd_FromCappedPredecessorAlsoEndsLivingSuccessor":                                                             "25387c4916833dd1",
	"services/schedule/template_offering_source_integration_test.go:TestTemplateOfferingSource_PullForwardWidensSourcedRoster":                                                            "e2bdbfaeb72c2553",
	"services/schedule/template_offering_source_integration_test.go:TestTemplateOfferingSource_SplitAwayFromAngebotClearsSourcedRoster":                                                   "e97b99e4eaca8ad3",
	"services/schedule/template_series_roster_integration_test.go:TestSeriesRoster_AddsChildAcrossCappedPredecessor":                                                                      "405bbb22e9d4777b",
	"services/schedule/template_series_roster_integration_test.go:TestSeriesRoster_KeepsPredecessorOnlyChildOutsideScope":                                                                 "b16b15c8e76701d7",
	"services/schedule/template_series_roster_integration_test.go:TestSeriesRoster_NarrowedSuccessorLeavesTheOtherWeekdayIntact":                                                          "8f429b548e43ca49",
	"services/schedule/template_series_roster_integration_test.go:TestSeriesRoster_NoopWithoutSeriesRosterFrom":                                                                           "120c39b9fe19b05f",
	"services/schedule/template_series_roster_integration_test.go:TestSeriesRoster_PastAnchorClampsToTodayAndSegmentStart":                                                                "115fe2c4125bd82e",
	"services/schedule/template_series_roster_integration_test.go:TestSeriesRoster_PreservesHandRemovedChildOnPredecessorOccurrence":                                                      "100e929bfe512d20",
	"services/schedule/template_series_roster_integration_test.go:TestSeriesRoster_PrimaryChangeReachesMaterializedOccurrences":                                                           "0c57387f48fa14a1",
	"services/schedule/template_series_roster_integration_test.go:TestSeriesRoster_ReachesTheClickedWeekdayTheSuccessorNoLongerRuns":                                                      "7241144350584f79",
	"services/schedule/template_series_roster_integration_test.go:TestSeriesRoster_ReconcilesSupervisorsAcrossPredecessor":                                                                "99200a493a6092aa",
	"services/schedule/template_series_roster_integration_test.go:TestSeriesRoster_RemovedStaffLosesPlannedOccurrenceRows":                                                                "f330c4f56360dc1c",
	"services/schedule/template_series_roster_integration_test.go:TestSeriesRoster_RemovesChildAcrossCappedPredecessor":                                                                   "0a984d93639bc032",
	"services/schedule/template_series_roster_integration_test.go:TestSeriesRoster_SecondIdenticalSaveKeepsRowsUntouched":                                                                 "ebdb93c3f609fca6",
	"services/schedule/template_series_roster_integration_test.go:TestSeriesRoster_SkipsProtectedPredecessorEnrollments":                                                                  "27d248ac539211eb",
	"services/schedule/template_series_roster_integration_test.go:TestSeriesRoster_WeekdayScopedAddOnlyTouchesThatWeekday":                                                                "2bcc53815c7cfb31",
	"services/schedule/template_series_roster_integration_test.go:TestSeriesRoster_WeekdayScopedEditLeavesOtherWeekdaysAlone":                                                             "cc315b9e3dc3d7fb",
	"services/schedule/template_source_class_filter_integration_test.go:TestTemplateSourceClassFilter_HonoursBookedWeekdays":                                                              "2729aa108baacf3a",
	"api/school/school_supervisions_test.go:TestSchoolSupervisionsFollowTheAssignment":                                                                                                    "9b5315fa788a3a57",
	"api/students/attendance_history_handlers_test.go:TestGetStudentAttendanceHistory_FutureEndClampsToToday":                                                                             "e98cfe72f49abcaa",
	"api/students/change_request_list_handlers_test.go:TestAggregatedChangeRequests_EmitsCurrentStatusPerDate":                                                                            "893d374e14864032",
	"api/students/change_request_list_handlers_test.go:TestAggregatedChangeRequests_GroupsContradictingAbsencesOnOneDay":                                                                  "51a99f7f75db249f",
	"api/students/status_day_internal_test.go:TestStaffAbsenceNotificationCallbacks":                                                                                                      "717ccb0849800c29",
	"api/timetable/exception_conflicts_test.go:TestExceptionConflicts_CancelledInstance_EmitsWarningPerExpectedStudent":                                                                   "a63f3706bcb6596f",
	"api/timetable/exception_conflicts_test.go:TestExceptionConflicts_Empty_NoExceptions":                                                                                                 "8bec43d76ce53e05",
	"api/timetable/gaps_test.go:TestGaps_Empty":                                                                                                       "ac7ac1ff97ddf3a9",
	"api/timetable/instances_list_test.go:TestListInstances_Empty":                                                                                    "b8cb8ab4b16b6032",
	"api/timetable/templates_series_test.go:TestGetTemplate_ResolvesCappedPredecessorToLivingSuccessor":                                               "030a75b1dc83a9a5",
	"api/timetable/templates_start_pull_test.go:TestTemplateUpdateStartDatePullForward":                                                               "365b9d09ecc54406",
	"database/repositories/feedback/entry_repository_test.go:TestEntryRepository_List":                                                                "b371c41004ab6018",
	"database/repositories/schedule/arrival_schedule_test.go:TestStudentArrivalExceptionRepository_DeletePastExceptions":                              "be92b91efcdce476",
	"database/repositories/schedule/arrival_schedule_test.go:TestStudentArrivalNoteRepository_DeletePastNotes":                                        "64966f6318884223",
	"database/repositories/schedule/pickup_schedule_test.go:TestStudentPickupExceptionRepository_DeletePastExceptions":                                "6d93a11351a5090f",
	"database/repositories/schedule/pickup_schedule_test.go:TestStudentPickupNoteRepository_DeletePastNotes":                                          "b958bafd0e287c9a",
	"services/absence/excused_request_errorpath_test.go:TestDecide_ApprovalNotifiesAfterCommit":                                                       "80c7a905f48e76fe",
	"services/active/staff_vacation_opening_db_test.go:TestDeleteVacationOpening_WritesTombstoneAndRestoresSummary":                                   "d6a1b62a12921dfd",
	"services/active/staff_vacation_opening_db_test.go:TestSetVacationOpening_AllowsVacationBeginningOnWeekendBeforeCutoff":                           "b1897e16329f47c3",
	"services/active/staff_vacation_opening_db_test.go:TestSetVacationOpening_DerivesTakenBeforeFromQuota":                                            "037b7323bbb2a9e8",
	"services/active/staff_vacation_opening_db_test.go:TestVacationOpeningRepository_BatchAndListReads":                                               "bab275694c151a14",
	"services/enrollment/offering_adjustment_dated_test.go:TestDecisionService_UpdateChildOfferings_DatedSwitchBeforePhaseStartDropsUnstartedRow":     "1659d683d8730fa1",
	"services/enrollment/offering_change_request_service_test.go:TestOfferingChangeRequestService_Catalog_MarksCurrentBookingAndCapacity":             "2ed3bee1ea31c77e",
	"services/enrollment/offering_change_request_service_test.go:TestOfferingChangeRequestService_Create_StoresPendingRequest":                        "46a4250436d5dad8",
	"services/enrollment/offering_change_request_service_test.go:TestOfferingChangeRequestService_Create_StripsChangedCurrentAutomaticOffering":       "8fd356753b17c729",
	"services/enrollment/offering_change_request_service_test.go:TestOfferingChangeRequestService_Decide_AppliesTheConfirmedDate":                     "24996c8c66feff9e",
	"services/enrollment/offering_change_request_service_test.go:TestOfferingChangeRequestService_Decide_ApprovalAppliesTheDatedSwitch":               "054ea6d9b99f76e6",
	"services/enrollment/offering_change_request_service_test.go:TestOfferingChangeRequestService_Decide_CapsRebookingAtPlannedCareEnd":               "4c5125d4a15526c6",
	"services/enrollment/offering_change_request_service_test.go:TestOfferingChangeRequestService_Decide_LogsTheDateTheFamilyAskedFor":                "380202db92507221",
	"services/enrollment/offering_change_request_service_test.go:TestOfferingChangeRequestService_Decide_RejectionNeedsAReasonAndChangesNothing":      "0c99c758bf1714cc",
	"services/enrollment/offering_change_request_service_test.go:TestOfferingChangeRequestService_GetForStudent_ReportsRecentDecision":                "9af7b2eed98393e7",
	"services/enrollment/offering_change_request_service_test.go:TestOfferingChangeRequestService_PreviewDecision_ReportsOnlyUncoveredManualPlanning": "3e6a1eda14edb403",
	"services/enrollment/pickup_adjustment_service_test.go:TestPickupAdjustmentAppliesArrivalSchedulesOnlyForImmediateExceptions":                     "f78a6c0050e56344",
	"services/import/student_import_config_test.go:TestEnrollmentStartsInFuture_UsesBusinessDate":                                                     "d3c4048ea395347a",
	"services/parent/care_ended_child_test.go:TestParentPortal_CareEndedChildIsReadOnly":                                                              "73e351526486456e",
	"services/parent/care_exception_service_test.go:TestListCareExceptions_MergesBothLegsAndFlagsStaffSource":                                         "50fa000725caf0ee",
	"services/schedule/bulk_substitution_unit_test.go:TestNormalizeBulkDates_DedupesAndSortsAscending":                                                "4ef3c229bd90057c",
	"services/schedule/calendar_period_integration_test.go:TestCalendarPeriodService_ConcurrentBootstrapVsCreate":                                     "f83834f85bfc921c",
	"services/schedule/calendar_period_integration_test.go:TestCalendarPeriodService_EnsureDefaultSchoolYear":                                         "003436e2f7d65478",
	"services/schedule/care_request_history_test.go:TestListHistory_IncludesPickupChangeWithPayloadSummary":                                           "d61d71c17b8e2da7",
	"services/schedule/pickup_auto_excusal_test.go:TestAutoExcusal_BulkWeeklyUpsertResyncsExceptions":                                                 "7c180b25b464e7d5",
	"services/schedule/pickup_auto_excusal_test.go:TestAutoExcusal_DeletingTheExceptionRestoresBlocks":                                                "a2e0b9afe2bc4eae",
	"services/schedule/pickup_auto_excusal_test.go:TestAutoExcusal_FullDayStatusCoexistsAndReleaseReplays":                                            "122d216e75bb78de",
	"services/schedule/pickup_auto_excusal_test.go:TestAutoExcusal_LaterThanBaselineMeansNoCoupling":                                                  "e4aee723b89daf04",
	"services/schedule/pickup_auto_excusal_test.go:TestAutoExcusal_ManualCreateConvertsAutoToManual":                                                  "452217b0a35abad3",
	"services/schedule/pickup_auto_excusal_test.go:TestAutoExcusal_ManualDeleteOfConvertedRowRederivesAuto":                                           "527e2a5746c0e129",
	"services/schedule/pickup_auto_excusal_test.go:TestAutoExcusal_ManualDeleteRefusesAutoRows":                                                       "5b3f1d71933d71f2",
	"services/schedule/pickup_auto_excusal_test.go:TestAutoExcusal_ManualPartialAbsenceIsNeverTouched":                                                "7e121bc8119e7169",
	"services/schedule/pickup_auto_excusal_test.go:TestAutoExcusal_MovingPickupBackReleasesBlocks":                                                    "f38ee181f94bad6b",
	"services/schedule/pickup_auto_excusal_test.go:TestAutoExcusal_MovingPickupEarlierWidensTheExcusal":                                               "100052be2f0583e6",
	"services/schedule/pickup_auto_excusal_test.go:TestAutoExcusal_NoWeeklyBaselineMeansNoCoupling":                                                   "cb4b6f61dd262dd6",
	"services/schedule/pickup_auto_excusal_test.go:TestAutoExcusal_PulledForwardPickupExcusesLaterBlocks":                                             "848610c6ddb71f77",
	"services/schedule/pickup_auto_excusal_test.go:TestAutoExcusal_WeeklyBaselineAddedCouplesExistingException":                                       "e209de391ad32647",
	"services/schedule/pickup_auto_excusal_test.go:TestAutoExcusal_WeeklyBaselineDeletedReleasesCoupling":                                             "b251529065f3f66b",
	"services/schedule/pickup_auto_excusal_test.go:TestAutoExcusal_WeeklyBaselineMovedEarlierReleasesCoupling":                                        "87e5cc85a5abb84e",
	"services/schedule/shift_plan_sync_service_test.go:TestSickCascade_HalfDayRules":                                                                  "bca8ec7e805fc677",
	"services/schedule/shift_plan_sync_service_test.go:TestSickCascade_PastShiftsRemainHistoricalDuringMarkAndReconcile":                              "9a0d2f7cb5305f61",
	"services/schedule/template_source_class_filter_integration_test.go:TestTemplateSourceClassFilter_ClassChangeMovesTheChild":                       "2cdb0b776254481c",
	"services/schedule/template_source_class_filter_integration_test.go:TestTemplateSourceClassFilter_DeregistrationLimitsTheAssignment":              "fcf4432a3e1cc5d3",
	"services/schedule/template_source_class_filter_integration_test.go:TestTemplateSourceClassFilter_LaterApprovalJoinsTheTermin":                    "881a48104ec3ca76",
	"services/schedule/template_source_class_filter_integration_test.go:TestTemplateSourceClassFilter_MatchesCaseInsensitively":                       "068e53b7795f27fa",
	"services/schedule/template_source_class_filter_integration_test.go:TestTemplateSourceClassFilter_OfferingDayChangeReshapesTheRoster":             "40923ff1f1c59b41",
	"services/schedule/template_source_class_filter_integration_test.go:TestTemplateSourceClassFilter_SeedsOnlyTheFilteredClass":                      "056fb0721af7a411",
	"services/schedule/template_source_class_filter_integration_test.go:TestTemplateSourceClassFilter_UpdateSwitchesFromGradeToClass":                 "185af4ecda4704f1",
	"services/schedule/template_split_service_test.go:TestTemplateEndFromDate_CapsTemplateAndProtectsHistory":                                         "7c0079d897a97d43",
	"services/schedule/template_split_service_test.go:TestTemplateEnd_ConcurrentTemplateUpdatePreservesCommittedCap":                                  "7decc7d7dfe57cea",
	"services/schedule/template_split_service_test.go:TestTemplateMutations_RejectCareOfferingSeriesConflictsWithoutPersisting":                       "242869222cc4cc2d",
	"services/schedule/template_split_service_test.go:TestTemplateSplitAndEnd_RespectCurrentSegmentEnvelope":                                          "fae3be15b5c63cd8",
	"services/schedule/template_split_service_test.go:TestTemplateSplit_ExplicitRosterAndWeekPattern":                                                 "19eba9ba7e5845ff",
	"services/schedule/template_split_service_test.go:TestTemplateSplit_HappyPath_CarriesRosterAndProtectsHistory":                                    "eeb7e401a8f49745",
	"services/schedule/template_split_service_test.go:TestTemplateSplit_RejectsResplittingBoundedPredecessor":                                         "3ec9aea1e02244e5",
	"services/schedule/template_split_service_test.go:TestTemplateSplit_SingleEditThenSuccessorUpdateDoesNotDuplicate":                                "9938686ed1fa6110",
	"services/schedule/template_split_service_test.go:TestTemplateSplit_SuccessorValidFrom_NoPhantomBeforeEffective":                                  "1e6714fa5c434874",
	"services/schedule/template_split_service_test.go:TestTemplateSplit_UpdateSegmentsPreservesBoundsDuringMaterialization":                           "59fc6c134558fe0f",
	"services/schedule/template_start_pull_forward_test.go:TestUpdateTemplate_StartDatePullForward_MovesEnvelopeRosterAndMaterializesGapOnly":         "300b78e3812fb078",
	"services/schedule/template_start_pull_forward_test.go:TestUpdateTemplate_StartDatePullForward_MovesWeekdayScopedRoster":                          "f351a4b64f9dc220",
	"services/schedule/template_start_pull_forward_test.go:TestUpdateTemplate_StartDatePullForward_RejectsPredecessorOverlap":                         "6e59cc1dcb1c4248",
	"services/users/care_booking_authority_integration_test.go:TestBookingParticipationRangeExcludesAlumniWithoutDateBoundary":                        "9080722ca38159ee",
	"services/users/care_booking_authority_integration_test.go:TestNaturalBookingEndSchedulerIsIdempotent":                                            "ce16ab392ebe2797",
	"services/users/care_lifecycle_integration_test.go:TestCareExit_BinarySchoolWithNfcAndGroups":                                                     "d7c7ded74e1fdb59",
	"services/users/care_withdrawal_lifecycle_test.go:TestCareWithdrawalLifecycle_CompletionEndsBookingsFromEveryEnrollmentRequest":                   "f3c2d4f9f49d0e66",
	"api/active/handlers_unit_test.go:TestNewActiveGroupResponse_WithActiveSupervisors":                                                               "f860de8458062986",
	"api/active/handlers_unit_test.go:TestNewSupervisorResponse_ActiveSupervisor":                                                                     "c766ad753dbdc4df",
	"api/active/handlers_unit_test.go:TestNewSupervisorResponse_WithActiveGroup":                                                                      "5eb60c52cb5129de",
	"api/active/handlers_unit_test.go:TestNewSupervisorResponse_WithStaff":                                                                            "593d29c25da62c9f",
	"api/display/api_test.go:TestDisplayDashboardPickupBuckets":                                                                                       "494ba1c344a1e81a",
	"api/iot/checkin/attendance_internal_test.go:TestAttendanceInfo_Fields":                                                                           "0fd644e7f499ce29",
	"api/students/care_exit_handlers_test.go:TestStudentList_CareStatusDecidesWhichSideIsShown":                                                       "d2f88bbd6702f28c",
	"api/students/care_exit_handlers_test.go:TestStudentList_UsesBookingParticipationButKeepsAdministrationAndLivePresence":                           "a5c71c12c485ffd3",
	"api/timetable/deviation_log_test.go:TestApplyDeviations_ActiveInstance_EndsAndCreatesSupervisor":                                                 "4cdc795369bb0686",
	"api/timetable/instances_create_test.go:TestCreateInstance_Validation":                                                                            "28f4c3f5cd897a2e",
	"database/repositories/active/attendance_repository_test.go:TestAttendanceRepository_CloseOpenForToday":                                           "749da188f625861a",
	"database/repositories/active/group_repository_test.go:TestActiveGroupRepository_FindWithSupervisors":                                             "ff3035932749a7fa",
	"database/repositories/active/student_status_day_test.go:TestStudentStatusDayRepository_ClearByIDAndDates":                                        "c345815488821eed",
	"database/repositories/active/work_session_test.go:TestWorkSessionRepository_GetTodayPresenceMap":                                                 "ba74980b7c1125fa",
	"database/repositories/schedule/activity_instance_repo_test.go:TestActivityInstanceRepository_DeletePlannedMaterializedWeekendInstances":          "575d5893529c8aef",
	"database/repositories/users/parent_announcement_test.go:TestParentAnnouncementAudience_WeekdayScopedEnrollmentMatchesToday":                      "fcedd893d1d0d0b2",
	"models/active/attendance_test.go:TestAttendance_CompleteLifecycle":                                                                               "1b4cb283b2d64611",
	"models/active/attendance_test.go:TestAttendance_Fields":                                                                                          "7cb5dc87c9dd1d57",
	"models/active/attendance_test.go:TestAttendance_GetCreatedAt":                                                                                    "742b093d23733a63",
	"models/active/attendance_test.go:TestAttendance_GetUpdatedAt":                                                                                    "e3e71418bad9cbe5",
	"models/active/attendance_test.go:TestAttendance_IsCheckedIn_WhenCheckedIn":                                                                       "1c39e06b60abaa5e",
	"models/active/attendance_test.go:TestAttendance_IsCheckedIn_WhenCheckedOut":                                                                      "14e6354df810ebcc",
	"models/active/attendance_test.go:TestAttendance_MultipleRecords":                                                                                 "13b30b9278737741",
	"services/active/active_service_wrappers_internal_test.go:TestActiveServiceThinDelegates":                                                         "70358f4494c0a88d",
	"services/active/analytics_service_test.go:TestGetDashboardAnalytics":                                                                             "c0aa0f725cdba3ad",
	"services/active/update_visit_mock_test.go:TestUpdateVisitLocksAttendanceBeforeClosingIt":                                                         "0716e9bfb550beee",
	"services/active/visit_helpers_test.go:TestCreateVisit_ClearsParentStatusForToday":                                                                "937fbb3ffde375ce",
	"services/active/visit_helpers_test.go:TestCreateVisit_ClearsPlannedStatusForToday":                                                               "4c15d85b1dfd38bf",
	"services/active/work_session_export_test.go:TestWSGetHistory_AuditCountError":                                                                    "455996c94d7d4ded",
	"services/active/work_session_export_test.go:TestWSGetHistory_BreaksError":                                                                        "52868b8397826110",
	"services/active/work_session_service_test.go:TestWSGetHistory_ClosedSessionKeepsCachedBreaks":                                                    "1700cb032bc7e143",
	"services/active/work_session_service_test.go:TestWSGetHistory_DeductsRunningBreakFromNetMinutes":                                                 "07edc93d98ba7515",
	"services/active/work_session_service_test.go:TestWSGetHistory_RepoError":                                                                         "f7f1803c0a36c745",
	"services/active/work_session_service_test.go:TestWSGetHistory_RunningBreakIsCappedAtTheLiveLimit":                                                "648cf9916010d340",
	"services/active/work_session_service_test.go:TestWSGetHistory_SerializesRunningBreakInBreakMinutes":                                              "2d1ec1f66ce9deb5",
	"services/calendar/service_integration_test.go:TestCalendarServiceIntegration_CleanupExpiredFeedTombstonesCascadesChildren":                       "95a96f88db753e5e",
	"services/calendar/service_integration_test.go:TestCalendarServiceIntegration_StaffSubscriptionPublishesOccurrenceAndDeletionCancellations":       "9e5301cfd8c3c15d",
	"services/calendar/service_integration_test.go:TestCalendarServiceIntegration_SubscriptionFeed":                                                   "960b24930f2ff331",
	"services/schedule/care_request_decision_snapshot_test.go:TestDecide_PickupChangeFreezesDiff":                                                     "c6d78f5311321b1a",
	"services/schedule/instance_service_integration_test.go:TestInstance_ReplanWeek_RemovesFutureLegacyWeekendInstances":                              "c6128c0e2bba645f",
	"services/schedule/instance_service_integration_test.go:TestInstance_Start_ConflictWarning_Staff":                                                 "75c1bfb193fc3f78",
	"services/schedule/instance_service_integration_test.go:TestInstance_Start_StaffBridgedOverrideDifferentRoom_Conflict":                            "b4801e7ba9df79ea",
	"services/schedule/instance_service_integration_test.go:TestInstance_Start_StaffBridgedOverrideSameRoom_NoConflict":                               "48365afcff40d588",
	"services/schedule/instance_service_integration_test.go:TestInstance_Start_StaffBridgedWithoutRosterRow_Conflict":                                 "a09c9246046da070",
	"services/schedule/instance_service_integration_test.go:TestInstance_Start_StaffSameRoomIsNotAConflict":                                           "061634984d144fd0",
	"services/schedule/staff_schedule_overview_integration_test.go:TestShiftCoverageProjection_BatchesEffectiveSeriesReadsAndIsolatesTenant":          "8acdcfe741cd46a2",
	"services/schedule/staff_schedule_overview_integration_test.go:TestStaffScheduleOverview_WeeklySummariesIncludeShiftsOutsideViewport":             "1f93d930b5a5dbe6",
	"services/schedule/staff_schedule_overview_integration_test.go:TestStaffScheduleOverview_WeeklySummariesResolveSollAndIsolateTenant":              "9bba2901f491ae2e",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_MoveConsumesOriginalDateBeforeRematerialization":                   "8e168ba3dedf6410",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_RepeatedMoveKeepsOriginalOccurrenceIdentity":                       "6705638dd28d4ee4",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_WeekPatternARespectsCycle":                                         "9d297f27bbea31eb",
	"services/schedule/staff_shift_series_rule_edit_test.go:TestStaffShiftSeries_SplitRejectsExtensionWithoutRecurrenceOccurrence":                    "acb872debb60167b",
	"services/schedule/staff_shift_series_rule_edit_test.go:TestStaffShiftSeries_SplitRejectsWhenNextSegmentLeavesNoOccurrence":                       "15206d7e96a62e7c",
	// Source patterns added to the ratchet still need an exact baseline for
	// tests that predate #2571. Keep this list shrink-only.
	"api/active/checkin_test.go:TestAttendance_Fields":                                                                                                                                                                "90934e19fb6f5550",
	"api/birthdays/api_test.go:TestOverviewListsTodaysChildren":                                                                                                                                                       "f8dda4e746b3eb76",
	"api/display/api_test.go:TestDisplayDashboardPublic":                                                                                                                                                              "14e79bfad3819023",
	"api/staff/absence_admin_test.go:TestAdminCreateStaffAbsence_CompTimeAllowedForManager":                                                                                                                           "1152e16310582f62",
	"api/staff/absence_admin_test.go:TestAdminCreateStaffAbsence_SickCascades":                                                                                                                                        "0d46e1fe9f83d5d6",
	"api/staff/time_tracking_handlers_internal_test.go:TestParseYearQuery_DefaultsToBerlinCalendarYear":                                                                                                               "fd93be8deb3eb67b",
	"api/students/attendance_history_export_test.go:TestParseAttendanceExportOptions_AcceptsToday":                                                                                                                    "12b4e1d7a4c8c79f",
	"api/students/care_exit_handlers_test.go:TestCareExitHandlers_PreviewThenConfirm":                                                                                                                                 "ce964a231ce1c893",
	"api/students/care_exit_handlers_test.go:TestStudentList_MarksRecordedExitsOnly":                                                                                                                                  "acbf003b177dd126",
	"api/students/day_log_handlers_test.go:TestGetStudentsDayLog_AdminSeesStatuses":                                                                                                                                   "93c4dc3c03edf2a5",
	"api/students/day_log_logic_test.go:TestParseDayLogDateRejectsHistoryWithoutDatedGroupAssignments":                                                                                                                "608b575121081962",
	"api/students/ogs_group_live_test.go:TestOGSGroupLive_AggregatesGroupData":                                                                                                                                        "d9b7bdd8c81928b8",
	"api/students/status_day_internal_test.go:TestStudentStatusDayHandlers_TodayUpdatesLiveStatusAndClearsOpposite":                                                                                                   "14742d43ceb983bd",
	"api/students/status_day_overview_test.go:TestGetStudentStatusDaysOverview_AdminSeesEntries":                                                                                                                      "a07d4c93e088f775",
	"api/students/update_class_resync_test.go:TestUpdateStudent_ClassChangeResyncsOfferingSourcedTemplates":                                                                                                           "f271ad11e6ff8b32",
	"api/timetable/instances_create_test.go:TestCreateInstance_DuplicateTemplateBoundReturnsConflict":                                                                                                                 "61fb4952979a396d",
	"api/timetable/templates_series_test.go:TestUpdateTemplate_SeriesRosterFromReachesPredecessor":                                                                                                                    "5a4b0b07fdc7bf19",
	"api/timetable/templates_split_test.go:TestTemplateEndHandler_HappyPath":                                                                                                                                          "9b7588e3c29a0cdb",
	"api/timetable/templates_split_test.go:TestTemplateSplitHandler_UpdateSuccessorPreservesValidFrom":                                                                                                                "f1885c7eac4ed472",
	"api/timetable/templates_split_test.go:TestTemplateUpdateHandler_RejectsInconsistentValidityEnvelopeWithoutMutation":                                                                                              "318d5e2c592a741d",
	"api/timetable/templates_test.go:TestListTemplates_CapacityFields":                                                                                                                                                "402409737d3f103a",
	"database/migrations/001015314_template_source_school_classes_test.go:TestTemplateSourceSchoolClassesDownPreservesSourcedEnrollmentHistory":                                                                       "1d35c2fb7f4512d0",
	"database/repositories/active/attendance_date_range_test.go:TestAttendanceRepository_FindByStudentAndDateRange":                                                                                                   "90b9d0c754d791c4",
	"database/repositories/active/bulk_readers_test.go:TestGroupSupervisorRepository_ListActiveSupervisedRooms":                                                                                                       "4868b12e7bdeeaca",
	"database/repositories/active/group_supervisor_repository_test.go:TestGroupSupervisorRepository_EndAllActiveByStaffID":                                                                                            "ed8579b6d01f4ab4",
	"database/repositories/active/staff_absence_test.go:TestStaffAbsenceRepository_GetByStaffAndDateRange":                                                                                                            "1c721e3c02d2f270",
	"database/repositories/active/student_status_day_test.go:TestStudentStatusDayRepository_NoteOnReReport":                                                                                                           "1d55b22e1976e0c7",
	"database/repositories/active/student_status_day_test.go:TestStudentStatusDayRepository_TenantScope":                                                                                                              "c09292f9ae67e02a",
	"database/repositories/active/student_status_day_test.go:TestStudentStatusDayRepository_UpsertAndFind":                                                                                                            "39306a079ccce5a7",
	"database/repositories/active/work_session_test.go:TestWorkSessionRepository_GetHistoryByStaffID":                                                                                                                 "7d58f8bbbfe6b620",
	"database/repositories/active/work_session_test.go:TestWorkSessionRepository_GetHistoryByStaffIDWrapsDatabaseError":                                                                                               "5ad96c310cdea900",
	"database/repositories/enrollment/request_child_offering_repository_capacity_range_test.go:TestRequestChildOfferingRepository_CountMaxActiveByCareOfferingInRangeExcludingRequestChild_ExcludesReplacedIntervals": "71906c018f79f003",
	"database/repositories/enrollment/request_child_offering_repository_capacity_range_test.go:TestRequestChildOfferingRepository_CountMaxActiveByCareOfferingInRange_IncludesFutureBookings":                         "d31abc153c6fc846",
	"database/repositories/enrollment/request_child_offering_repository_grade_levels_test.go:TestRequestChildOfferingRepository_Aggregates_CountEveryPhaseLikeTheGate":                                                "2127d20ce914886f",
	"database/repositories/enrollment/request_child_offering_repository_grade_levels_test.go:TestRequestChildOfferingRepository_CountMaxActiveByIDsInRange_GuardsItsInput":                                            "0c6a99aadd2ea66a",
	"database/repositories/enrollment/request_child_offering_repository_grade_levels_test.go:TestRequestChildOfferingRepository_CountMaxActiveByIDsInRange_MatchesTheSingleOfferingVariant":                           "3c7caef81bead1f4",
	"database/repositories/enrollment/request_child_offering_repository_grade_levels_test.go:TestRequestChildOfferingRepository_CountMaxActiveByIDsInRange_SeparatesOfferings":                                        "76310ca6d780df8c",
	"database/repositories/feedback/entry_repository_test.go:TestEntryRepository_FindByDateRange":                                                                                                                     "eaed1b1e3078cbc3",
	"database/repositories/feedback/entry_repository_test.go:TestEntryRepository_FindByStudentAndDateRange":                                                                                                           "32fc789676b40b2a",
	"database/repositories/schedule/staff_shift_repo_test.go:TestStaffShiftRepository_DeleteUpcomingByStaffID":                                                                                                        "376b6e85f1e52593",
	"database/repositories/users/care_withdrawal_completion_test.go:TestCareWithdrawalCompletionRepository_ParticipationBoundaryUsesPendingCompletionWhenEnrollmentIsOpen":                                            "a6c7ad09ace7191e",
	"database/repositories/users/care_withdrawal_completion_test.go:TestCareWithdrawalCompletionRepository_UpsertUsesIncomingBoundary":                                                                                "692fa445b9a0fd29",
	"services/absence/excused_request_service_test.go:TestDecide_ApproveRefusedWhenPartialAbsenceExists":                                                                                                              "fdec85ffa9a92b34",
	"services/active/attendance_service_test.go:TestGetStudentAttendanceStatus_NotCheckedIn":                                                                                                                          "d225b2b93aa1676b",
	"services/active/cleanup_service_test.go:TestCleanupStaleAttendance_CheckOutTimeIsBerlinEndOfDay":                                                                                                                 "183fee96f095e985",
	"services/active/cleanup_supervisors_test.go:TestCleanupStaleSupervisors_ClosesYesterdayRecords":                                                                                                                  "9997c0891441e103",
	"services/active/staff_absence_service_test.go:TestAbsCreateAbsenceFor_RejectsCompTimeAgainstLaterLedgerCapacity":                                                                                                 "2016799c6b5187bd",
	"services/active/staff_opening_balance_mock_test.go:TestStaffBalanceAdjustmentService_OpeningAllowsNegativeTarget":                                                                                                "9e2ed887ac0968af",
	"services/active/student_status_day_write_bulk_test.go:TestBulkCreateForDates_RejectsConflictWithoutPartialWrites":                                                                                                "4c835b1a9a3d1e16",
	"services/active/student_status_day_write_bulk_test.go:TestCreateForDates_RejectsConflictWithoutPartialWrites":                                                                                                    "76cda5e1786a7701",
	"services/active/work_session_autocheckout_mock_test.go:TestAutoCheckout_QueriesOpenSessionsIncludingToday":                                                                                                       "5e7c2bab706e6544",
	"services/active/work_session_service_test.go:TestWSApplyCustomScheduleRows_StampsAnchorForFirstRotation":                                                                                                         "163e86d9629ae7db",
	"services/education/grade_transition_offering_resync_test.go:TestGradeTransitionService_ApplyAndRevert_ResyncOfferingSourcedRosters":                                                                              "015e7eb13c529220",
	"services/education/grade_transition_roster_reconcile_test.go:TestGradeTransitionService_Revert_FillsBackdatedInstance":                                                                                           "639d0ca32869afa6",
	"services/education/grade_transition_roster_reconcile_test.go:TestGradeTransitionService_Revert_FillsTodaysInstanceMaterializedWhileAlumnus":                                                                      "9723866e26be20b9",
	"services/enrollment/care_offering_booking_stats_internal_test.go:TestBookingStatsWindow_DefaultsToTodayWithoutPhaseDates":                                                                                        "0cb3598aefe5f539",
	"services/enrollment/care_offering_booking_stats_internal_test.go:TestListBookingStats_CountsInTheCapacityGatesWindow":                                                                                            "d98c9229a572c7df",
	"services/enrollment/class_roster_care_end_test.go:TestClassRosterFiltersCareDate":                                                                                                                                "1ee05c6289bc4bc7",
	"services/enrollment/decision_service_test.go:TestDecisionService_Decide_ApprovedScheduledPastStartActivatesStudent":                                                                                              "4fe6002810702da4",
	"services/enrollment/decision_service_test.go:TestDecisionService_ListChildOfferings_CarriesAttributesAndFutureBookings":                                                                                          "a0673560c88a67a4",
	"services/enrollment/offering_change_request_service_test.go:TestOfferingChangeRequestService_ListPending_ReportsDateClampedToThePhaseStart":                                                                      "33a8e77ed1334e11",
	"services/enrollment/offering_change_request_service_test.go:TestOfferingChangeRequestService_ListPending_ReportsTheSelectableDateRange":                                                                          "60cfd47d353c7b04",
	"services/enrollment/offering_source_service_test.go:TestDecide_MultiSourceFanOutSeedsFromPhaseStart":                                                                                                             "5ed3f6809d60dd9f",
	"services/enrollment/offering_source_service_test.go:TestListOfferingSourceOptions_CountsScopedToSelectedPeriod":                                                                                                  "7f825a5b681976d8",
	"services/enrollment/offering_source_service_test.go:TestUpdateChildOfferings_UndatedCorrectionKeepsPhaseStartOnMultiSource":                                                                                      "25205b2f7bdebe20",
	"services/enrollment/report_service_test.go:TestCareUsageEnrichesGuardiansAndSchedulePickup":                                                                                                                      "9c13ed3728a85093",
	"services/enrollment/report_service_test.go:TestClassRosterUsesOfferingDateForPickupProjection":                                                                                                                   "6af21490fede9f3d",
	"services/feedback/feedback_service_test.go:TestFeedbackService_GetEntriesByDateRange":                                                                                                                            "4be832fee2bdf672",
	"services/feedback/feedback_service_test.go:TestFeedbackService_GetEntriesByStudentAndDateRange":                                                                                                                  "45ad2f40c29ca3a1",
	"services/parent/care_exception_service_test.go:TestListCareExceptions_FlagsAbsentArrivalRow":                                                                                                                     "3d86ffa166c687b6",
	"services/parent/care_exception_service_test.go:TestListCareExceptions_FlagsAbsentPickupRow":                                                                                                                      "6015df3240d99f78",
	"services/parent/excused_request_test.go:TestExcusedRequest_ApproveWritesStatusDays":                                                                                                                              "f24bdcfdf15d74df",
	"services/parent/parent_care_offerings_service_test.go:TestGetChildCareOfferingsReturnsCompleteSortedView":                                                                                                        "45a37057efb136ef",
	"services/parent/parent_meal_plan_test.go:TestMealPlanWeek_AllowsNextWeek":                                                                                                                                        "631a9a11c001cc14",
	"services/parent/parent_meal_plan_test.go:TestMealPlanWeek_DisabledReturnsSentinel":                                                                                                                               "4a9737400d04d725",
	"services/parent/parent_meal_plan_test.go:TestMealPlanWeek_FarFutureWeekOutOfRange":                                                                                                                               "71c6f09ea6305ec1",
	"services/parent/parent_meal_plan_test.go:TestMealPlanWeek_NotOwnedChildRejected":                                                                                                                                 "093fbca92fa3fa20",
	"services/parent/parent_meal_plan_test.go:TestMealPlanWeek_PastWeekOutOfRange":                                                                                                                                    "4abca0ffb669facc",
	"services/parent/parent_meal_plan_test.go:TestMealPlanWeek_ReturnsCurrentWeekEntries":                                                                                                                             "7ad2f874be413494",
	"services/parent/parent_meal_plan_test.go:TestMealPlanWeek_SettingErrorPropagates":                                                                                                                                "77e6fd4ae6450f59",
	"services/parent/parent_request_edit_test.go:TestEditExcusedRequestReplacesWithdrawal":                                                                                                                            "c5ea46ceb4cc4688",
	"services/parent/parent_write_service_test.go:TestListSickDays_ExcludesStaffCreatedExcused":                                                                                                                       "cb58ee6eaaf9ccdf",
	"services/parent/parent_write_service_test.go:TestListSickDays_ReturnsSickAndExcused":                                                                                                                             "387e0f2bc67c94b0",
	"services/parent/parent_write_service_test.go:TestSubmitSickNote_ClearsClassTripForSubmittedDate":                                                                                                                 "554adb5339dde068",
	"services/parent/parent_write_service_test.go:TestSubmitSickNote_FutureWriteSerializesWithStaffConflictCheck":                                                                                                     "ae18fc53b13a985e",
	"services/parent/parent_write_service_test.go:TestSubmitSickNote_NonContiguousExcludesUnrelatedRows":                                                                                                              "8150535c76bb1bba",
	"services/parent/parent_write_service_test.go:TestSubmitSickNote_RefusesPartialAbsenceConflict":                                                                                                                   "95f1dbc7a9e0418d",
	"services/parent/sick_note_gate_pin_test.go:TestSickNoteStaysImmediateWhenApprovalDisabled":                                                                                                                       "c81be21e2fa90ffb",
	"services/schedule/partial_absence_pending_request_test.go:TestPartialAbsenceCreate_RefusesPendingFullDayRequest":                                                                                                 "0e6a9c48226abc0a",
	"services/schedule/shift_plan_sync_service_test.go:TestSickCascade_ClearSickForRange":                                                                                                                             "8fb11146cfb91997",
	"services/schedule/shift_plan_sync_service_test.go:TestSickCascade_ConcurrentOverlappingReportsSerializeBeforeOverlapRead":                                                                                        "4afe81f995a23572",
	"services/schedule/shift_plan_sync_service_test.go:TestSickCascade_UpdateRangeRollsBackWhenRemovedShiftCannotReactivate":                                                                                          "6322b6a6340d76dc",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffScheduleOverview_SeriesFieldsRideExistingReads":                                                                                                "b26fe9cc6f190f9e",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_CapAllByStaffIDClampsFutureSeries":                                                                                                 "2c8c4868de4aad30",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_CollisionSkipsAndReports":                                                                                                          "54ae42432f8310f7",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_CreateMaterializesFromTomorrow":                                                                                                    "c6515401f8cb4d9c",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_CreateRejectsBadReferences":                                                                                                        "6c9658ebb80fd5d7",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_EditDetachesAndDeleteRecordsException":                                                                                             "77f4999f64fdd3a0",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_EndAtFirstOccurrence":                                                                                                              "86fc8270d70a5ad1",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_EndSeriesKeepsDetachedAndPast":                                                                                                     "04345df89707e82b",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_SplitAtFirstOccurrence":                                                                                                            "cfc4c9c349338ad6",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_SplitOutsideSegmentRejected":                                                                                                       "ff8df106dcd1001a",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_SplitPreservesDeviationsOnSuccessor":                                                                                               "21a4ebc94ae75257",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_SplitTodayUpdatesOccurrenceAndReplansTomorrow":                                                                                     "da3a17f82c6584f9",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_WeekPatternRequiresCycle":                                                                                                          "1d778c3c368bda97",
	"services/schedule/staff_shift_series_mock_test.go:TestEndSeriesUnit_ErrorBranches":                                                                                                                               "5ff0604254a175cd",
	"services/schedule/staff_shift_series_mock_test.go:TestGetSeriesUnit":                                                                                                                                             "abbbcc3a349c40f6",
	"services/schedule/staff_shift_series_mock_test.go:TestSplitSeriesUnit_ErrorBranches":                                                                                                                             "16970d570f3a081b",
	"services/schedule/staff_shift_series_rule_edit_test.go:TestStaffShiftSeries_SplitAppliesNewWeekdaysFromEffectiveDate":                                                                                            "3e8ed130c790314c",
	"services/schedule/staff_shift_series_rule_edit_test.go:TestStaffShiftSeries_SplitBoundsEarlierSegmentAtNextSuccessor":                                                                                            "776ca517504dd20f",
	"services/schedule/staff_shift_series_rule_edit_test.go:TestStaffShiftSeries_SplitExtendsSeriesEndingToday":                                                                                                       "ba458d67a47a30a4",
	"services/schedule/staff_shift_series_rule_edit_test.go:TestStaffShiftSeries_SplitKeepsStoredValidityWhenUnset":                                                                                                   "9b901f9df8e1c04b",
	"services/schedule/staff_shift_series_rule_edit_test.go:TestStaffShiftSeries_SplitRejectsSupersededSegment":                                                                                                       "965b1d9f083aaeda",
	"services/schedule/staff_shift_series_rule_edit_test.go:TestStaffShiftSeries_SplitRejectsValidityBeyondCalendarPeriod":                                                                                            "dab502a501fa47c8",
	"services/schedule/staff_shift_series_rule_edit_test.go:TestStaffShiftSeries_SplitRejectsWhenNoOccurrenceRemains":                                                                                                 "e65244793b99ea67",
	"services/schedule/staff_shift_series_rule_edit_test.go:TestStaffShiftSeries_SplitShortensValidityAndDropsLaterShifts":                                                                                            "451e8a956d7430fd",
	"services/schedule/template_end_service_unit_test.go:TestTemplateEndFromDate_ReturnsSummaryAndDeletesOpenEndedWindow":                                                                                             "3ad05c18f64ace9c",
	"services/schedule/template_offering_source_unit_test.go:TestResyncUpdatedTemplateOfferingRoster":                                                                                                                 "bbcc3beea8e5da8f",
	"services/schedule/template_series_roster_mock_test.go:TestReconcileSeriesPredecessorRoster_CreatesBoundedRows":                                                                                                   "73d17f070892e459",
	"services/users/care_booking_authority_integration_test.go:TestBookingMutationPlansFutureNaturalEndImmediately":                                                                                                   "e3d60925ebd79c58",
	"services/users/care_booking_authority_integration_test.go:TestOverdueRebookingReplacesTheStaleCompletion":                                                                                                        "621efdf9cbf477b9",
	"services/users/care_lifecycle_service_test.go:TestCareLifecycle_CancelPutsThePlanBack":                                                                                                                           "11613a76e13ff800",
	"services/users/care_lifecycle_service_test.go:TestCareLifecycle_CancelRefusesOrdinaryEnrollmentEnd":                                                                                                              "34c699e347a490eb",
	"services/users/care_lifecycle_service_test.go:TestCareLifecycle_CancelRestoresPreviousEnrollmentEnd":                                                                                                             "8316fbf5ed9d3dfc",
	"services/users/care_lifecycle_service_test.go:TestCareLifecycle_LastCareDayIsInclusive":                                                                                                                          "033b5c729a4009ae",
	"services/users/care_lifecycle_service_test.go:TestCareLifecycle_Resume":                                                                                                                                          "05a2fa403dccdafa",
	"services/users/care_withdrawal_lifecycle_test.go:TestCareWithdrawalLifecycle_AllowsRetroactiveExitButNotBeforeAttendance":                                                                                        "132d8915cb744615",
	"services/users/care_withdrawal_lifecycle_test.go:TestCareWithdrawalLifecycle_CancellingPlannedExitRestoresTask":                                                                                                  "e3dc9ad9688e918d",
	"services/users/person_service_eligibility_test.go:TestFilterStudentsEligibleOnDate_IncludesImmediatelyActiveFutureStudentToday":                                                                                  "ed289271228085f9",
	"api/iot/checkin/checkin_test.go:TestDeviceCheckin_ResponseIncludesPickupTime":                                                                                                                                    "93ea1e7afb0190c5",
	"api/iot/checkin/checkin_test.go:TestDevicePickupQuery_ReturnsPickupInfoWithoutCreatingVisit":                                                                                                                     "5a25f476e09ff822",
	"api/iot/checkin/checkin_test.go:TestDevicePickupQuery_PrefersDayNotesOverRecurringNotes":                                                                                                                         "fbbc9407fb588524",
	"api/iot/checkin/checkin_test.go:TestDevicePickupQuery_PreservesRecurringNotesWhenExceptionReasonIsBlank":                                                                                                         "b9b1b5c92a62faf4",
	"api/students/students_test.go:TestListStudents_WithPickupTimes":                                                                                                                                                  "7c6e372f5c483946",
	"api/students/students_test.go:TestListStudents_WithArrivalTimes":                                                                                                                                                 "b95bda54d6d6e189",
	"services/active/visit_helpers_test.go:TestCreateVisit_WithDevice":                                                                                                                                                "88b69ab42643bf19",
	"services/active/visit_helpers_test.go:TestCreateVisit_CompletedVisitCreatesClosedAttendance":                                                                                                                     "c8b7f27d3e24216a",
	"services/active/visit_helpers_test.go:TestUpdateVisit_ReconcilesMatchingAttendanceSession":                                                                                                                       "27f57966f99807de",
	"services/active/visit_helpers_test.go:TestUpdateVisit_GroupMoveWithCheckoutClosesAttendanceSession":                                                                                                              "5f7bbe20b1d999f0",
	"services/active/visit_helpers_test.go:TestCreateVisit_ReEntry":                                                                                                                                                   "73916490ff0daa09",
	"services/schedule/instance_service_integration_test.go:TestInstance_Reopen_RestoresAbsenceProvenance":                                                                                                            "3e4a5e68d29ed222",
	"services/schedule/instance_service_integration_test.go:TestInstance_Complete_SkipsChildrenNotInCareThatDay":                                                                                                      "b0859c7c5aa78b19",
	"services/schedule/instance_service_integration_test.go:TestInstance_Complete_RecordsAbsenceForCancelledDay":                                                                                                      "144d56d758bf0fe0",
	"services/users/staff_document_service_test.go:TestStaffDocumentService_CreateHydratesGeneratedTimestamps":                                                                                                        "b9979ac2591b8e3b",
	"services/users/staff_document_service_test.go:TestStaffDocumentService_RetentionSchedule":                                                                                                                        "d4235227b88e1b04",
	"simulate/feed_tombstone_test.go:TestRunFullDaySeedsStaffFeedTombstone":                                                                                                                                           "50018ffb9a00c84d",
	"database/repositories/users/care_withdrawal_completion_test.go:TestCareWithdrawalCompletionRepository_OnePendingTaskPerChild":                                                                                    "8876220d397e4fe1",
	"services/schedule/shift_plan_sync_service_test.go:TestSickCascade_ShiftOnlyChangesBroadcastTenantInvalidation":                                                                                                   "fcaac75824ad5122",
	"services/schedule/shift_plan_sync_service_test.go:TestSickCascade_ReconcileSickRangeAppliesOnlyDateDelta":                                                                                                        "7c460cc232334c76",
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
	updated := WorkSession{}
	updated.CheckInTime = time.Now()
	history := GetHistory(session, from, to)
	require.Len(t, history.WeeklySummaries, 1)
	updatedHistory := GetHistory(updated, from, to)
	_ = updatedHistory.WeeklySummaries
	other := GetHistory(NewWorkSession(fixtureNow()), from, to)
	_ = other.WeeklySummaries
	structured := GetHistory(fixtureSession(), from, to)
	_ = structured.WeeklySummaries
}
`)
	writeLiveInstantHelper(t, root)

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

func writeLiveInstantHelper(t *testing.T, root string) {
	t.Helper()

	writeCalendarFixtureSourceAt(t, root, "active/clock_helpers_test.go", `package active
import "time"
func fixtureNow() time.Time {
	now := time.Now()
	return now
}
func fixtureSession() WorkSession {
	session := WorkSession{}
	session.CheckInTime = time.Now()
	return session
}
`)
}

func TestCalendarFixtureRatchetDetectsLiveDateRange(t *testing.T) {
	t.Parallel()

	root := writeCalendarFixtureSource(t, "sample/history_test.go", `package sample
import (
	"testing"
	tz "github.com/moto-nrw/project-phoenix/internal/timezone"
)
type DateRange struct { From, To tz.Date }
func TestHistoryRange(t *testing.T) {
	t.Parallel()
	from := tz.TodayDate().AddDays(-7)
	to := tz.TodayDate()
	fixed := tz.NewDate(2026, 8, 30)
	_ = GetHistory(from, fixed)
	_ = FindByDateRange(fixed, to)
	_ = FindByDateRange(DateRange{From: from, To: fixed})
	_ = List(from, to)
	dateRange := DateRange{From: fixed, To: fixed}
	dateRange.From = tz.TodayDate()
	_ = List(dateRange)
	alias := dateRange
	_ = List(alias)
	_ = List(&DateRange{From: tz.TodayDate(), To: fixed})
}
func BenchmarkHistoryRange(b *testing.B) { _ = GetHistory(tz.TodayDate(), tz.NewDate(2026, 8, 30)) }
func FuzzHistoryRange(f *testing.F) { _ = FindByDateRange(tz.TodayDate(), tz.NewDate(2026, 8, 30)) }
func ExampleHistoryRange() { _ = GetHistory(tz.TodayDate(), tz.NewDate(2026, 8, 30)) }
`)

	findings, err := scanCalendarFixtureClockRisks(root)
	if err != nil {
		t.Fatal(err)
	}
	requireCalendarFinding(t, findings,
		"TestHistoryRange",
		"BenchmarkHistoryRange",
		"FuzzHistoryRange",
		"ExampleHistoryRange",
		"live clock defines a calendar range",
	)
}

func TestCalendarFixtureRatchetFollowsLiveDateHelperIntoAssertion(t *testing.T) {
	t.Parallel()

	root := writeCalendarFixtureSource(t, "sample/helper_test.go", `package sample
import (
	"testing"
	assertpkg "github.com/stretchr/testify/assert"
	tz "github.com/moto-nrw/project-phoenix/internal/timezone"
)
type fixtureClock struct{}
func fixtureDate() tz.Date {
	return crossFileDate()
}
func (fixtureClock) today() tz.Date { return tz.TodayDate() }
func TestCalendarExpectation(t *testing.T) {
	t.Parallel()
	got := struct{ Date tz.Date }{}
	assertpkg.Equal(t, fixtureDate(), got.Date)
	assertpkg.False(t, got.Date.After(fixtureClock{}.today()))
	if got.Date != fixtureDate() { t.Fail() }
}
`)
	writeCalendarFixtureSourceAt(t, root, "sample/date_helpers_test.go", `package sample
import tz "github.com/moto-nrw/project-phoenix/internal/timezone"
func crossFileDate() tz.Date {
	today := tz.TodayDate()
	return today
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
	assertCalendarExceptionAccepted(t, findings, key)
	assertCalendarExceptionRejected(t, findings, map[string]string{key: ""}, "non-empty reason")
	assertCalendarExceptionRejected(t, findings,
		map[string]string{"sample/range_test.go:TestOther": "typo"}, "no matching finding")
}

func assertCalendarExceptionAccepted(t *testing.T, findings []calendarClockFinding, key string) {
	t.Helper()

	remaining, err := applyCalendarClockExceptions(findings, map[string]string{
		key: "the production contract is explicitly relative to the current Berlin day",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 0 {
		t.Fatalf("reviewed exception did not suppress its exact test: %v", remaining)
	}
}

func assertCalendarExceptionRejected(t *testing.T, findings []calendarClockFinding, exceptions map[string]string, want string) {
	t.Helper()

	_, err := applyCalendarClockExceptions(findings, exceptions)
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("exception error %v does not contain %q", err, want)
	}
}

func TestCalendarFixtureLegacyBaselineRejectsChangedFunction(t *testing.T) {
	t.Parallel()

	finding := calendarClockFinding{file: "sample_test.go", function: "TestLegacy", fingerprint: "new"}
	_, err := applyCalendarClockLegacyBaseline([]calendarClockFinding{finding}, map[string]string{
		"sample_test.go:TestLegacy": "old",
	})
	if err == nil || !strings.Contains(err.Error(), "functions changed") {
		t.Fatalf("changed legacy function must fail, got %v", err)
	}
}

func TestCalendarFixtureLegacyBaselineRejectsChangedHelper(t *testing.T) {
	t.Parallel()

	root := writeCalendarFixtureSource(t, "sample/history_test.go", `package sample
import tz "github.com/moto-nrw/project-phoenix/internal/timezone"
func TestLegacy(t *testing.T) {
	t.Parallel()
	_ = tz.TodayDate().Weekday()
	history := GetHistory(fixtureTime())
	_ = history.WeeklySummaries
}
`)
	writeCalendarFixtureSourceAt(t, root, "sample/helper_test.go", `package sample
import "time"
func fixtureTime() time.Time { return time.Time{} }
`)
	before, err := scanCalendarFixtureClockRisks(root)
	if err != nil || len(before) == 0 {
		t.Fatalf("initial scan = %v, %v", before, err)
	}
	baseline := map[string]string{"sample/history_test.go:TestLegacy": before[0].fingerprint}
	writeCalendarFixtureSourceAt(t, root, "sample/helper_test.go", `package sample
import "time"
func fixtureTime() time.Time { return time.Now() }
`)
	after, err := scanCalendarFixtureClockRisks(root)
	if err != nil {
		t.Fatal(err)
	}
	_, err = applyCalendarClockLegacyBaseline(after, baseline)
	if err == nil || !strings.Contains(err.Error(), "remediation:") {
		t.Fatalf("changed helper must produce an actionable baseline error, got %v", err)
	}
}

func TestCalendarFixtureRatchetIgnoresFixedAndNonCodePatterns(t *testing.T) {
	t.Parallel()

	safeRoot := writeCalendarFixtureSource(t, "sample/fixed_test.go", `package sample
import (
	"time"
	"testing"
	tz "github.com/moto-nrw/project-phoenix/internal/timezone"
)
type fakeClock struct{}
type suite struct{}
func (fakeClock) Now() time.Time { return time.Time{} }
func (suite) TestMethod(t *testing.T) {
	t.Parallel()
	_ = tz.TodayDate().Weekday()
}

func Testament(t *testing.T) {
	t.Parallel()
	_ = tz.TodayDate().Weekday()
}
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

func TestCalendarFixtureRatchetKeepsScopesAndNowMethodsSeparate(t *testing.T) {
	t.Parallel()

	root := writeCalendarFixtureSource(t, "sample/scopes_test.go", `package sample
import (
	"testing"
	"time"
)
type liveClock struct{}
type fakeClock struct{}
func (liveClock) Now() time.Time { return time.Now() }
func (fakeClock) Now() time.Time { return time.Time{} }
func TestElapsedAndWeeklyScopes(t *testing.T) {
	t.Parallel()
	t.Run("elapsed", func(t *testing.T) {
		t.Parallel()
		result := measure(time.Now())
		_ = result
	})
	t.Run("weekly", func(t *testing.T) {
		t.Parallel()
		result := GetHistory(fakeClock{}.Now())
		_ = result.WeeklySummaries
	})
}
`)
	assertNoCalendarFindings(t, root)
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
	writeCalendarFixtureSourceAt(t, root, rel, source)
	return root
}

func writeCalendarFixtureSourceAt(t *testing.T, root, rel, source string) {
	t.Helper()

	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
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
