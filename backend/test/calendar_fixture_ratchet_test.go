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
	"api/school/school_supervisions_test.go:TestSchoolSupervisionsFollowTheAssignment":                                                                                                    "52b9c5462b0f6672",
	"database/repositories/active/work_session_test.go:TestWorkSessionRepository_ListOverlappingByStaffID_KeepsEarlierStarts":                                                             "265f38a640646464",
	"database/repositories/education/group_substitution_repository_test.go:TestGroupSubstitutionRepository_FindOverlapping":                                                               "1c119c4171de4af3",
	"database/repositories/enrollment/request_child_offering_repository_grade_levels_test.go:TestRequestChildOfferingRepository_CountActiveGradeLevels_CountsAChildOncePerOffering":       "63fac136aa98ff6c",
	"database/repositories/enrollment/request_child_offering_repository_grade_levels_test.go:TestRequestChildOfferingRepository_CountActiveGradeLevels_EmptyInputSkipsTheQuery":           "d73d1f1f6d558252",
	"database/repositories/enrollment/request_child_offering_repository_grade_levels_test.go:TestRequestChildOfferingRepository_CountActiveGradeLevels_ExcludesIntervalsOutsideTheWindow": "ce3b863c70f72db5",
	"database/repositories/enrollment/request_child_offering_repository_grade_levels_test.go:TestRequestChildOfferingRepository_CountActiveGradeLevels_ExcludesTerminalChildren":          "03ef104219b546a8",
	"database/repositories/enrollment/request_child_offering_repository_grade_levels_test.go:TestRequestChildOfferingRepository_CountActiveGradeLevels_GroupsByGrade":                     "454b5499045e72a5",
	"database/repositories/enrollment/request_child_offering_repository_grade_levels_test.go:TestRequestChildOfferingRepository_CountActiveGradeLevels_RejectsAnEmptyWindow":              "71afd64ffad60356",
	"database/repositories/enrollment/request_child_offering_repository_grade_levels_test.go:TestRequestChildOfferingRepository_CountActiveGradeLevels_ReportsMissingGradeSeparately":     "42050203e5a34bdd",
	"services/active/staff_vacation_opening_db_test.go:TestSetVacationOpening_NegativeRemainingAllowsOverdrawnAccount":                                                                    "f8475d175d762626",
	"services/active/staff_vacation_opening_db_test.go:TestSetVacationOpening_RejectsOpenCutoff":                                                                                          "6d5df6c5bf2dd6f2",
	"services/active/staff_vacation_opening_db_test.go:TestSetVacationOpening_RejectsPastVacationYear":                                                                                    "b852b2e168131fbf",
	"services/active/staff_vacation_opening_db_test.go:TestSetVacationOpening_RejectsSecondOpeningForSameYear":                                                                            "9b22f13e49919e13",
	"services/active/staff_vacation_opening_db_test.go:TestSetVacationOpening_RejectsVacationAbsencesBeforeCutoff":                                                                        "bbd07d8308be4afb",
	"services/active/staff_vacation_opening_db_test.go:TestSetVacationOpening_RespectsCustomQuota":                                                                                        "9f8e9d28fde554d9",
	"services/calendar/service_integration_test.go:TestCalendarServiceIntegration_DeleteFeedVisibleLeavesTombstone":                                                                       "c4fdfeb251888c29",
	"services/enrollment/offering_change_request_automatic_test.go:TestOfferingChangeRequestService_Decide_ExclusionKeepsManualAndRequiredLunchDays":                                      "821c0d08769e80c3",
	"services/enrollment/offering_change_request_automatic_test.go:TestOfferingChangeRequestService_Decide_SnapshotMatchesGrandfatheredAutomaticBooking":                                  "b38102eb6dc8339f",
	"services/enrollment/offering_change_request_service_test.go:TestOfferingChangeRequestService_Create_PayloadExcludesAutomaticOfferings":                                               "60817b1776d080c1",
	"services/enrollment/offering_change_request_service_test.go:TestOfferingChangeRequestService_Decide_LogsNoWishWhenTheDateIsKept":                                                     "a7a9b9690e03fc76",
	"services/parent/care_exception_service_test.go:TestDeleteCareExceptionPreservesArrival":                                                                                              "7004a7ee3d5ef204",
	"services/parent/care_exception_service_test.go:TestDeleteCareException_RemovesPickupAndPreservesArrival":                                                                             "f9a05d7c246802b7",
	"services/parent/care_exception_service_test.go:TestListCareExceptions_ArrivalRepoErrorSurfaces":                                                                                      "baf2d1ddb67542ed",
	"services/parent/care_exception_service_test.go:TestListCareExceptions_NotOwnedChild":                                                                                                 "48142d4798179204",
	"services/parent/care_exception_service_test.go:TestListCareExceptions_RepoErrorSurfaces":                                                                                             "ff67c4b8ad34062b",
	"services/parent/care_exception_service_test.go:TestSubmitCareException_ClearingLegRemovesIt":                                                                                         "2ff60c2328b106dd",
	"services/parent/excused_request_test.go:TestSickRequest_ApproveWritesSickStatusAndLiveFlag":                                                                                          "590f9a0b1a75d891",
	"services/parent/parent_care_schedule_service_test.go:TestGetChildCareSchedule_TodayAbsentReflectsStatusDay":                                                                          "3b81e479e9ebe4ec",
	"services/parent/parent_write_service_test.go:TestListSickDays_AllowsPortalAccessWithoutWritePermissions":                                                                             "00b0c557f5cd8683",
	"services/parent/parent_write_service_test.go:TestListSickDays_HidesAnotherGuardiansReason":                                                                                           "8883183b885b60c1",
	"services/parent/parent_write_service_test.go:TestListSickDays_NotOwned":                                                                                                              "225641a4a1a37659",
	"services/schedule/staff_schedule_overview_integration_test.go:TestStaffScheduleOverview_TenantIsolationAcrossEveryProjectionRead":                                                    "fe0776d85ff99812",
	"api/timetable/substitutions_bulk_test.go:TestBulkSubstitution_MultiDayWithSubstitute":                                                                                                "ea2d9c4e76a37c5d",
	"services/enrollment/offering_change_full_withdrawal_test.go:TestOfferingChangeRequestService_ListPending_KeepsUntouchedBookingsOutOfTheWarning":                                      "37fb53c778e186a5",
	"services/enrollment/offering_change_history_test.go:TestOfferingChangeRequestService_ListHistory":                                                                                    "db817bf49c44f4ed",
	"services/enrollment/offering_change_request_automatic_test.go:TestOfferingChangeRequestService_Decide_ExclusionSkipsAutoTargetAndRecordsOverride":                                    "3aa76b83de9ea2e3",
	"services/enrollment/offering_change_request_automatic_test.go:TestOfferingChangeRequestService_Decide_RejectionFallsBackToPayloadSnapshot":                                           "4b5c82525a943bd6",
	"services/enrollment/offering_change_request_automatic_test.go:TestOfferingChangeRequestService_Decide_RejectionFreezesDiffSnapshot":                                                  "279a54101c7af37c",
	"services/enrollment/offering_change_request_automatic_test.go:TestOfferingChangeRequestService_GetForStudent_MarksAutomaticDiffEntries":                                              "aa10416a7a0fd4c9",
	"services/enrollment/offering_change_request_automatic_test.go:TestOfferingChangeRequestService_ListPending_IncludesUnchangedGrandfatheredRuleTarget":                                 "e387446f274e027f",
	"services/enrollment/offering_change_request_automatic_test.go:TestOfferingChangeRequestService_ListPending_MarksAutomaticDiffEntries":                                                "9f8f92ee077f40a7",
	"services/enrollment/request_child_offering_repository_date_test.go:TestRequestChildOfferingRepository_ListAtDates_DoesNotReturnHistoricalSelection":                                  "17fabc254dd1856b",
	"services/schedule/template_end_cascade_integration_test.go:TestTemplateEnd_CascadesToCappedPredecessors":                                                                             "d0667c696fd69ff1",
	"services/schedule/template_end_cascade_integration_test.go:TestTemplateEnd_FromCappedPredecessorAlsoEndsLivingSuccessor":                                                             "92d5b402539dbf2a",
	"services/schedule/template_offering_source_integration_test.go:TestTemplateOfferingSource_PullForwardWidensSourcedRoster":                                                            "04c41649c4cd42a5",
	"services/schedule/template_offering_source_integration_test.go:TestTemplateOfferingSource_SplitAwayFromAngebotClearsSourcedRoster":                                                   "3d204cdd85ff030b",
	"services/schedule/template_series_roster_integration_test.go:TestSeriesRoster_AddsChildAcrossCappedPredecessor":                                                                      "134323794bf3f7a2",
	"services/schedule/template_series_roster_integration_test.go:TestSeriesRoster_KeepsPredecessorOnlyChildOutsideScope":                                                                 "a6b28538d0236956",
	"services/schedule/template_series_roster_integration_test.go:TestSeriesRoster_NarrowedSuccessorLeavesTheOtherWeekdayIntact":                                                          "0c397f7e26a6c6d4",
	"services/schedule/template_series_roster_integration_test.go:TestSeriesRoster_NoopWithoutSeriesRosterFrom":                                                                           "724ae37ef8409f02",
	"services/schedule/template_series_roster_integration_test.go:TestSeriesRoster_PastAnchorClampsToTodayAndSegmentStart":                                                                "77aa8a616027bb4f",
	"services/schedule/template_series_roster_integration_test.go:TestSeriesRoster_PreservesHandRemovedChildOnPredecessorOccurrence":                                                      "6dc67a22926cb604",
	"services/schedule/template_series_roster_integration_test.go:TestSeriesRoster_PrimaryChangeReachesMaterializedOccurrences":                                                           "323e9c56f17d4d03",
	"services/schedule/template_series_roster_integration_test.go:TestSeriesRoster_ReachesTheClickedWeekdayTheSuccessorNoLongerRuns":                                                      "5bdc8667653fade4",
	"services/schedule/template_series_roster_integration_test.go:TestSeriesRoster_ReconcilesSupervisorsAcrossPredecessor":                                                                "7497a359c878bf59",
	"services/schedule/template_series_roster_integration_test.go:TestSeriesRoster_RemovedStaffLosesPlannedOccurrenceRows":                                                                "84f94e2fe9f0a1a2",
	"services/schedule/template_series_roster_integration_test.go:TestSeriesRoster_RemovesChildAcrossCappedPredecessor":                                                                   "9aa66ff221f2a94b",
	"services/schedule/template_series_roster_integration_test.go:TestSeriesRoster_SecondIdenticalSaveKeepsRowsUntouched":                                                                 "6fad9f73b0bdb1ff",
	"services/schedule/template_series_roster_integration_test.go:TestSeriesRoster_SkipsProtectedPredecessorEnrollments":                                                                  "f133f4ba356e040b",
	"services/schedule/template_series_roster_integration_test.go:TestSeriesRoster_WeekdayScopedAddOnlyTouchesThatWeekday":                                                                "433e3ffb28b2bb6c",
	"services/schedule/template_series_roster_integration_test.go:TestSeriesRoster_WeekdayScopedEditLeavesOtherWeekdaysAlone":                                                             "bb00f3ac154265d2",
	"services/schedule/template_source_class_filter_integration_test.go:TestTemplateSourceClassFilter_HonoursBookedWeekdays":                                                              "8f9b9cbf24b1bae8",
	"api/students/attendance_history_handlers_test.go:TestGetStudentAttendanceHistory_FutureEndClampsToToday":                                                                             "fdde9edfa31177f5",
	"api/students/change_request_list_handlers_test.go:TestAggregatedChangeRequests_EmitsCurrentStatusPerDate":                                                                            "ee40f4c85bb7f415",
	"api/students/change_request_list_handlers_test.go:TestAggregatedChangeRequests_GroupsContradictingAbsencesOnOneDay":                                                                  "59db00db10558bbc",
	"api/students/status_day_internal_test.go:TestStaffAbsenceNotificationCallbacks":                                                                                                      "ea2b6def1e60b646",
	"api/timetable/exception_conflicts_test.go:TestExceptionConflicts_CancelledInstance_EmitsWarningPerExpectedStudent":                                                                   "ecc5f20534936e60",
	"api/timetable/exception_conflicts_test.go:TestExceptionConflicts_Empty_NoExceptions":                                                                                                 "0db0aaa36e8290e4",
	"api/timetable/gaps_test.go:TestGaps_Empty":                                                                                                       "d91746d30b223ea6",
	"api/timetable/instances_list_test.go:TestListInstances_Empty":                                                                                    "6851d91d21a51053",
	"api/timetable/templates_series_test.go:TestGetTemplate_ResolvesCappedPredecessorToLivingSuccessor":                                               "311c87e0e998e3ac",
	"api/timetable/templates_start_pull_test.go:TestTemplateUpdateStartDatePullForward":                                                               "2f0f05c9a48ba535",
	"database/repositories/feedback/entry_repository_test.go:TestEntryRepository_List":                                                                "490c95a0e89108ca",
	"database/repositories/schedule/arrival_schedule_test.go:TestStudentArrivalExceptionRepository_DeletePastExceptions":                              "18faff796d2617ce",
	"database/repositories/schedule/arrival_schedule_test.go:TestStudentArrivalNoteRepository_DeletePastNotes":                                        "23cae5cbe7f896fb",
	"database/repositories/schedule/pickup_schedule_test.go:TestStudentPickupExceptionRepository_DeletePastExceptions":                                "520465af8c6c2547",
	"database/repositories/schedule/pickup_schedule_test.go:TestStudentPickupNoteRepository_DeletePastNotes":                                          "3acdf33bb38552d2",
	"services/absence/excused_request_errorpath_test.go:TestDecide_ApprovalNotifiesAfterCommit":                                                       "4b62deaed38a1be5",
	"services/active/staff_vacation_opening_db_test.go:TestDeleteVacationOpening_WritesTombstoneAndRestoresSummary":                                   "ca9a2604aaacdce8",
	"services/active/staff_vacation_opening_db_test.go:TestSetVacationOpening_AllowsVacationBeginningOnWeekendBeforeCutoff":                           "9a136ca7cf697f7a",
	"services/active/staff_vacation_opening_db_test.go:TestSetVacationOpening_DerivesTakenBeforeFromQuota":                                            "1e617bf2cb61d11d",
	"services/active/staff_vacation_opening_db_test.go:TestVacationOpeningRepository_BatchAndListReads":                                               "392799936402a26b",
	"services/enrollment/offering_adjustment_dated_test.go:TestDecisionService_UpdateChildOfferings_DatedSwitchBeforePhaseStartDropsUnstartedRow":     "ca71a51cb807ba0e",
	"services/enrollment/offering_change_request_service_test.go:TestOfferingChangeRequestService_Catalog_MarksCurrentBookingAndCapacity":             "f90d711626bf3e3d",
	"services/enrollment/offering_change_request_service_test.go:TestOfferingChangeRequestService_Create_StoresPendingRequest":                        "54c1fa08e400a8d7",
	"services/enrollment/offering_change_request_service_test.go:TestOfferingChangeRequestService_Create_StripsChangedCurrentAutomaticOffering":       "ed0bccf2df03c592",
	"services/enrollment/offering_change_request_service_test.go:TestOfferingChangeRequestService_Decide_AppliesTheConfirmedDate":                     "c76f54d6dff04cca",
	"services/enrollment/offering_change_request_service_test.go:TestOfferingChangeRequestService_Decide_ApprovalAppliesTheDatedSwitch":               "7e53720102f50a22",
	"services/enrollment/offering_change_request_service_test.go:TestOfferingChangeRequestService_Decide_CapsRebookingAtPlannedCareEnd":               "833db73d30e77e82",
	"services/enrollment/offering_change_request_service_test.go:TestOfferingChangeRequestService_Decide_LogsTheDateTheFamilyAskedFor":                "38fea3badccbba2a",
	"services/enrollment/offering_change_request_service_test.go:TestOfferingChangeRequestService_Decide_RejectionNeedsAReasonAndChangesNothing":      "1df59c826a380d64",
	"services/enrollment/offering_change_request_service_test.go:TestOfferingChangeRequestService_GetForStudent_ReportsRecentDecision":                "afea880ba7c2ab30",
	"services/enrollment/offering_change_request_service_test.go:TestOfferingChangeRequestService_PreviewDecision_ReportsOnlyUncoveredManualPlanning": "d58cfbc85dd2dd28",
	"services/enrollment/pickup_adjustment_service_test.go:TestPickupAdjustmentAppliesArrivalSchedulesOnlyForImmediateExceptions":                     "8a3f9afa1b72b8ab",
	"services/import/student_import_config_test.go:TestEnrollmentStartsInFuture_UsesBusinessDate":                                                     "68877e59248fbcfd",
	"services/parent/care_ended_child_test.go:TestParentPortal_CareEndedChildIsReadOnly":                                                              "d7ad935d37c624fa",
	"services/parent/care_exception_service_test.go:TestListCareExceptions_MergesBothLegsAndFlagsStaffSource":                                         "edb8a1da710a0b49",
	"services/schedule/bulk_substitution_unit_test.go:TestNormalizeBulkDates_DedupesAndSortsAscending":                                                "e5af2bff17941726",
	"services/schedule/calendar_period_integration_test.go:TestCalendarPeriodService_ConcurrentBootstrapVsCreate":                                     "38529821f1618e53",
	"services/schedule/calendar_period_integration_test.go:TestCalendarPeriodService_EnsureDefaultSchoolYear":                                         "d8fa8e4d5c0b13df",
	"services/schedule/care_request_history_test.go:TestListHistory_IncludesPickupChangeWithPayloadSummary":                                           "e6513e8012e60a3d",
	"services/schedule/pickup_auto_excusal_test.go:TestAutoExcusal_BulkWeeklyUpsertResyncsExceptions":                                                 "012c31c15b692585",
	"services/schedule/pickup_auto_excusal_test.go:TestAutoExcusal_DeletingTheExceptionRestoresBlocks":                                                "d5fe0f9da86343eb",
	"services/schedule/pickup_auto_excusal_test.go:TestAutoExcusal_FullDayStatusCoexistsAndReleaseReplays":                                            "83fcf9e539a1325b",
	"services/schedule/pickup_auto_excusal_test.go:TestAutoExcusal_LaterThanBaselineMeansNoCoupling":                                                  "2528b84a1efb7ced",
	"services/schedule/pickup_auto_excusal_test.go:TestAutoExcusal_ManualCreateConvertsAutoToManual":                                                  "a16e3bd6b2e7b25a",
	"services/schedule/pickup_auto_excusal_test.go:TestAutoExcusal_ManualDeleteOfConvertedRowRederivesAuto":                                           "8a7363c3336b6ac1",
	"services/schedule/pickup_auto_excusal_test.go:TestAutoExcusal_ManualDeleteRefusesAutoRows":                                                       "ad8bdcc69806d1c7",
	"services/schedule/pickup_auto_excusal_test.go:TestAutoExcusal_ManualPartialAbsenceIsNeverTouched":                                                "78adef2fbf30e55c",
	"services/schedule/pickup_auto_excusal_test.go:TestAutoExcusal_MovingPickupBackReleasesBlocks":                                                    "1e956c1d7e884532",
	"services/schedule/pickup_auto_excusal_test.go:TestAutoExcusal_MovingPickupEarlierWidensTheExcusal":                                               "2b1e22d57fa74ba3",
	"services/schedule/pickup_auto_excusal_test.go:TestAutoExcusal_NoWeeklyBaselineMeansNoCoupling":                                                   "52af80ea93c39b53",
	"services/schedule/pickup_auto_excusal_test.go:TestAutoExcusal_PulledForwardPickupExcusesLaterBlocks":                                             "6477ca82b7c59736",
	"services/schedule/pickup_auto_excusal_test.go:TestAutoExcusal_WeeklyBaselineAddedCouplesExistingException":                                       "86b28c7820f64737",
	"services/schedule/pickup_auto_excusal_test.go:TestAutoExcusal_WeeklyBaselineDeletedReleasesCoupling":                                             "15ca7506d5ab0453",
	"services/schedule/pickup_auto_excusal_test.go:TestAutoExcusal_WeeklyBaselineMovedEarlierReleasesCoupling":                                        "5916d763707f3b06",
	"services/schedule/shift_plan_sync_service_test.go:TestSickCascade_HalfDayRules":                                                                  "ef1f306724e8300d",
	"services/schedule/shift_plan_sync_service_test.go:TestSickCascade_PastShiftsRemainHistoricalDuringMarkAndReconcile":                              "1696bbb1b27dff63",
	"services/schedule/template_source_class_filter_integration_test.go:TestTemplateSourceClassFilter_ClassChangeMovesTheChild":                       "99a8843e8be560f7",
	"services/schedule/template_source_class_filter_integration_test.go:TestTemplateSourceClassFilter_DeregistrationLimitsTheAssignment":              "19d69fc5104ea9ac",
	"services/schedule/template_source_class_filter_integration_test.go:TestTemplateSourceClassFilter_LaterApprovalJoinsTheTermin":                    "bfe78f75b83795fd",
	"services/schedule/template_source_class_filter_integration_test.go:TestTemplateSourceClassFilter_MatchesCaseInsensitively":                       "409ec7c09d944e68",
	"services/schedule/template_source_class_filter_integration_test.go:TestTemplateSourceClassFilter_OfferingDayChangeReshapesTheRoster":             "d592f7dbb505288b",
	"services/schedule/template_source_class_filter_integration_test.go:TestTemplateSourceClassFilter_SeedsOnlyTheFilteredClass":                      "e3bbadd9e4fd1eae",
	"services/schedule/template_source_class_filter_integration_test.go:TestTemplateSourceClassFilter_UpdateSwitchesFromGradeToClass":                 "725e0a48af778ad5",
	"services/schedule/template_split_service_test.go:TestTemplateEndFromDate_CapsTemplateAndProtectsHistory":                                         "400a35732eacfb8b",
	"services/schedule/template_split_service_test.go:TestTemplateEnd_ConcurrentTemplateUpdatePreservesCommittedCap":                                  "30e36b9061c4f9f1",
	"services/schedule/template_split_service_test.go:TestTemplateMutations_RejectCareOfferingSeriesConflictsWithoutPersisting":                       "002b4ef18d244827",
	"services/schedule/template_split_service_test.go:TestTemplateSplitAndEnd_RespectCurrentSegmentEnvelope":                                          "3ec839afc7b142bc",
	"services/schedule/template_split_service_test.go:TestTemplateSplit_ExplicitRosterAndWeekPattern":                                                 "918472ac05b08607",
	"services/schedule/template_split_service_test.go:TestTemplateSplit_HappyPath_CarriesRosterAndProtectsHistory":                                    "63298016d3a04a9c",
	"services/schedule/template_split_service_test.go:TestTemplateSplit_RejectsResplittingBoundedPredecessor":                                         "6ed30bdc1d8d4f41",
	"services/schedule/template_split_service_test.go:TestTemplateSplit_SingleEditThenSuccessorUpdateDoesNotDuplicate":                                "4479d16dd4a224f5",
	"services/schedule/template_split_service_test.go:TestTemplateSplit_SuccessorValidFrom_NoPhantomBeforeEffective":                                  "eb6553b57c5d6441",
	"services/schedule/template_split_service_test.go:TestTemplateSplit_UpdateSegmentsPreservesBoundsDuringMaterialization":                           "9777f46b4329093f",
	"services/schedule/template_start_pull_forward_test.go:TestUpdateTemplate_StartDatePullForward_MovesEnvelopeRosterAndMaterializesGapOnly":         "0a0c70f45c2ad507",
	"services/schedule/template_start_pull_forward_test.go:TestUpdateTemplate_StartDatePullForward_MovesWeekdayScopedRoster":                          "0b03af601e07acdc",
	"services/schedule/template_start_pull_forward_test.go:TestUpdateTemplate_StartDatePullForward_RejectsPredecessorOverlap":                         "65ff67499201167a",
	"services/users/care_booking_authority_integration_test.go:TestBookingParticipationRangeExcludesAlumniWithoutDateBoundary":                        "58716a7b6ead2295",
	"services/users/care_booking_authority_integration_test.go:TestNaturalBookingEndSchedulerIsIdempotent":                                            "4b91b8afde26b3e5",
	"services/users/care_lifecycle_integration_test.go:TestCareExit_BinarySchoolWithNfcAndGroups":                                                     "2b5d7bfdc7e22da9",
	"services/users/care_withdrawal_lifecycle_test.go:TestCareWithdrawalLifecycle_CompletionEndsBookingsFromEveryEnrollmentRequest":                   "3fa65b61dcb9facd",
	"api/active/handlers_unit_test.go:TestNewActiveGroupResponse_WithActiveSupervisors":                                                               "4eed519b40c0f25a",
	"api/active/handlers_unit_test.go:TestNewSupervisorResponse_ActiveSupervisor":                                                                     "d7cfdb570d72960f",
	"api/active/handlers_unit_test.go:TestNewSupervisorResponse_WithActiveGroup":                                                                      "e5220d80ab636f56",
	"api/active/handlers_unit_test.go:TestNewSupervisorResponse_WithStaff":                                                                            "8ff2a05ca483a84f",
	"api/display/api_test.go:TestDisplayDashboardPickupBuckets":                                                                                       "c70e53077ea9602c",
	"api/iot/checkin/attendance_internal_test.go:TestAttendanceInfo_Fields":                                                                           "9c08c140636a697f",
	"api/students/care_exit_handlers_test.go:TestStudentList_CareStatusDecidesWhichSideIsShown":                                                       "1342b119321e9578",
	"api/students/care_exit_handlers_test.go:TestStudentList_UsesBookingParticipationButKeepsAdministrationAndLivePresence":                           "4389bdce1a49e52a",
	"api/timetable/deviation_log_test.go:TestApplyDeviations_ActiveInstance_EndsAndCreatesSupervisor":                                                 "19cd99cb509a9e2d",
	"api/timetable/instances_create_test.go:TestCreateInstance_Validation":                                                                            "15ec225f20e1950f",
	"database/repositories/active/attendance_repository_test.go:TestAttendanceRepository_CloseOpenForToday":                                           "cc06fa47ada96749",
	"database/repositories/active/group_repository_test.go:TestActiveGroupRepository_FindWithSupervisors":                                             "bf0dcd6ecff7a406",
	"database/repositories/active/student_status_day_test.go:TestStudentStatusDayRepository_ClearByIDAndDates":                                        "5437670e6e4457ed",
	"database/repositories/active/work_session_test.go:TestWorkSessionRepository_GetTodayPresenceMap":                                                 "7a522fe724bf2206",
	"database/repositories/schedule/activity_instance_repo_test.go:TestActivityInstanceRepository_DeletePlannedMaterializedWeekendInstances":          "e33cba0d4dcf154a",
	"database/repositories/users/parent_announcement_test.go:TestParentAnnouncementAudience_WeekdayScopedEnrollmentMatchesToday":                      "6af8c98385c45178",
	"models/active/attendance_test.go:TestAttendance_CompleteLifecycle":                                                                               "d32e3f1bc4f8a007",
	"models/active/attendance_test.go:TestAttendance_Fields":                                                                                          "f7f986912dfaa5b5",
	"models/active/attendance_test.go:TestAttendance_GetCreatedAt":                                                                                    "981b7a7ccc295511",
	"models/active/attendance_test.go:TestAttendance_GetUpdatedAt":                                                                                    "5e4837479b3f841b",
	"models/active/attendance_test.go:TestAttendance_IsCheckedIn_WhenCheckedIn":                                                                       "171509f4e67ffe6c",
	"models/active/attendance_test.go:TestAttendance_IsCheckedIn_WhenCheckedOut":                                                                      "8792add230a90c02",
	"models/active/attendance_test.go:TestAttendance_MultipleRecords":                                                                                 "b7245de0ebbd1522",
	"services/active/active_service_wrappers_internal_test.go:TestActiveServiceThinDelegates":                                                         "74b1a687bfceb697",
	"services/active/analytics_service_test.go:TestGetDashboardAnalytics":                                                                             "1dd38daafa9b76ff",
	"services/active/update_visit_mock_test.go:TestUpdateVisitLocksAttendanceBeforeClosingIt":                                                         "5e806ff8a0e12020",
	"services/active/visit_helpers_test.go:TestCreateVisit_ClearsParentStatusForToday":                                                                "e25bcc762ba39622",
	"services/active/visit_helpers_test.go:TestCreateVisit_ClearsPlannedStatusForToday":                                                               "b3c8f4b906be213f",
	"services/active/work_session_export_test.go:TestWSGetHistory_AuditCountError":                                                                    "f8611b6547ca4720",
	"services/active/work_session_export_test.go:TestWSGetHistory_BreaksError":                                                                        "fd8c5539673432c9",
	"services/active/work_session_service_test.go:TestWSGetHistory_ClosedSessionKeepsCachedBreaks":                                                    "279289b4da67548e",
	"services/active/work_session_service_test.go:TestWSGetHistory_DeductsRunningBreakFromNetMinutes":                                                 "230b369dfe9a7f28",
	"services/active/work_session_service_test.go:TestWSGetHistory_RepoError":                                                                         "f9281cf10a2f96b8",
	"services/active/work_session_service_test.go:TestWSGetHistory_RunningBreakIsCappedAtTheLiveLimit":                                                "e90f252b1d4011d9",
	"services/active/work_session_service_test.go:TestWSGetHistory_SerializesRunningBreakInBreakMinutes":                                              "77f8ac278c3ebd48",
	"services/calendar/service_integration_test.go:TestCalendarServiceIntegration_CleanupExpiredFeedTombstonesCascadesChildren":                       "df1e5073e16c5b91",
	"services/calendar/service_integration_test.go:TestCalendarServiceIntegration_StaffSubscriptionPublishesOccurrenceAndDeletionCancellations":       "f3b84ba79643f3ee",
	"services/calendar/service_integration_test.go:TestCalendarServiceIntegration_SubscriptionFeed":                                                   "5b0b7f19c479ba6a",
	"services/schedule/care_request_decision_snapshot_test.go:TestDecide_PickupChangeFreezesDiff":                                                     "2302a18146b6db75",
	"services/schedule/instance_service_integration_test.go:TestInstance_ReplanWeek_RemovesFutureLegacyWeekendInstances":                              "099158bbf85f2740",
	"services/schedule/instance_service_integration_test.go:TestInstance_Start_ConflictWarning_Staff":                                                 "12410a9ab669ee8a",
	"services/schedule/instance_service_integration_test.go:TestInstance_Start_StaffBridgedOverrideDifferentRoom_Conflict":                            "e5493636852c19b8",
	"services/schedule/instance_service_integration_test.go:TestInstance_Start_StaffBridgedOverrideSameRoom_NoConflict":                               "2ab669912c01eea2",
	"services/schedule/instance_service_integration_test.go:TestInstance_Start_StaffBridgedWithoutRosterRow_Conflict":                                 "d1a0446880cffafa",
	"services/schedule/instance_service_integration_test.go:TestInstance_Start_StaffSameRoomIsNotAConflict":                                           "1c9e1b369ce5d462",
	"services/schedule/staff_schedule_overview_integration_test.go:TestShiftCoverageProjection_BatchesEffectiveSeriesReadsAndIsolatesTenant":          "d5782103eb6eec2a",
	"services/schedule/staff_schedule_overview_integration_test.go:TestStaffScheduleOverview_WeeklySummariesIncludeShiftsOutsideViewport":             "49fba938841ac958",
	"services/schedule/staff_schedule_overview_integration_test.go:TestStaffScheduleOverview_WeeklySummariesResolveSollAndIsolateTenant":              "812605b57bf0a209",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_MoveConsumesOriginalDateBeforeRematerialization":                   "9a9a399c25c7f047",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_RepeatedMoveKeepsOriginalOccurrenceIdentity":                       "b7c67b5738cfc70b",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_WeekPatternARespectsCycle":                                         "624e4b4072c86d74",
	"services/schedule/staff_shift_series_rule_edit_test.go:TestStaffShiftSeries_SplitRejectsExtensionWithoutRecurrenceOccurrence":                    "eabd0713d336149a",
	"services/schedule/staff_shift_series_rule_edit_test.go:TestStaffShiftSeries_SplitRejectsWhenNextSegmentLeavesNoOccurrence":                       "d74c4845f893d883",
	// Source patterns added to the ratchet still need an exact baseline for
	// tests that predate #2571. Keep this list shrink-only.
	"api/active/checkin_test.go:TestAttendance_Fields":                                                                                                                                                                "1a1ee2f363b5121d",
	"api/birthdays/api_test.go:TestOverviewListsTodaysChildren":                                                                                                                                                       "53c1db9a764f7fce",
	"api/display/api_test.go:TestDisplayDashboardPublic":                                                                                                                                                              "345e03a570148e5d",
	"api/staff/absence_admin_test.go:TestAdminCreateStaffAbsence_CompTimeAllowedForManager":                                                                                                                           "aaef5f80484e1511",
	"api/staff/absence_admin_test.go:TestAdminCreateStaffAbsence_SickCascades":                                                                                                                                        "21d9e5ce0f52162d",
	"api/staff/time_tracking_handlers_internal_test.go:TestParseYearQuery_DefaultsToBerlinCalendarYear":                                                                                                               "88b96820e28b8680",
	"api/students/attendance_history_export_test.go:TestParseAttendanceExportOptions_AcceptsToday":                                                                                                                    "746032c58cbc2ac0",
	"api/students/care_exit_handlers_test.go:TestCareExitHandlers_PreviewThenConfirm":                                                                                                                                 "cc6c960f6c638e74",
	"api/students/care_exit_handlers_test.go:TestStudentList_MarksRecordedExitsOnly":                                                                                                                                  "33630dd1671dbd5f",
	"api/students/day_log_handlers_test.go:TestGetStudentsDayLog_AdminSeesStatuses":                                                                                                                                   "4ba61d957699db8e",
	"api/students/day_log_logic_test.go:TestParseDayLogDateRejectsHistoryWithoutDatedGroupAssignments":                                                                                                                "dece8336c09a3a01",
	"api/students/ogs_group_live_test.go:TestOGSGroupLive_AggregatesGroupData":                                                                                                                                        "bcee1639051e561c",
	"api/students/status_day_internal_test.go:TestStudentStatusDayHandlers_TodayUpdatesLiveStatusAndClearsOpposite":                                                                                                   "e9b5bb78f5f1ce01",
	"api/students/status_day_overview_test.go:TestGetStudentStatusDaysOverview_AdminSeesEntries":                                                                                                                      "c4e33f8e3d03bf9a",
	"api/students/update_class_resync_test.go:TestUpdateStudent_ClassChangeResyncsOfferingSourcedTemplates":                                                                                                           "77c912c322ec5517",
	"api/timetable/instances_create_test.go:TestCreateInstance_DuplicateTemplateBoundReturnsConflict":                                                                                                                 "0e8326b13e838e5a",
	"api/timetable/templates_series_test.go:TestUpdateTemplate_SeriesRosterFromReachesPredecessor":                                                                                                                    "1202a744e8a40ab8",
	"api/timetable/templates_split_test.go:TestTemplateEndHandler_HappyPath":                                                                                                                                          "e1f996517f98b54a",
	"api/timetable/templates_split_test.go:TestTemplateSplitHandler_UpdateSuccessorPreservesValidFrom":                                                                                                                "375b9d383288bc1d",
	"api/timetable/templates_split_test.go:TestTemplateUpdateHandler_RejectsInconsistentValidityEnvelopeWithoutMutation":                                                                                              "ff830ecfbe9f5228",
	"api/timetable/templates_test.go:TestListTemplates_CapacityFields":                                                                                                                                                "7174a634bbaa6c64",
	"database/migrations/001015314_template_source_school_classes_test.go:TestTemplateSourceSchoolClassesDownPreservesSourcedEnrollmentHistory":                                                                       "78973faba2674a8c",
	"database/repositories/active/attendance_date_range_test.go:TestAttendanceRepository_FindByStudentAndDateRange":                                                                                                   "5b3252aad4ccb6ab",
	"database/repositories/active/bulk_readers_test.go:TestGroupSupervisorRepository_ListActiveSupervisedRooms":                                                                                                       "4a84319ecaaa2636",
	"database/repositories/active/staff_absence_test.go:TestStaffAbsenceRepository_GetByStaffAndDateRange":                                                                                                            "76bf7b2e9b89f770",
	"database/repositories/active/student_status_day_test.go:TestStudentStatusDayRepository_NoteOnReReport":                                                                                                           "57f427a2c609a31b",
	"database/repositories/active/student_status_day_test.go:TestStudentStatusDayRepository_TenantScope":                                                                                                              "17f70a9cf3ac5296",
	"database/repositories/active/student_status_day_test.go:TestStudentStatusDayRepository_UpsertAndFind":                                                                                                            "70e900fd24b06cfb",
	"database/repositories/active/work_session_test.go:TestWorkSessionRepository_GetHistoryByStaffID":                                                                                                                 "791e613f720e1e80",
	"database/repositories/active/work_session_test.go:TestWorkSessionRepository_GetHistoryByStaffIDWrapsDatabaseError":                                                                                               "db69486f59e62660",
	"database/repositories/enrollment/request_child_offering_repository_capacity_range_test.go:TestRequestChildOfferingRepository_CountMaxActiveByCareOfferingInRangeExcludingRequestChild_ExcludesReplacedIntervals": "a7b44a3033cc4fbc",
	"database/repositories/enrollment/request_child_offering_repository_capacity_range_test.go:TestRequestChildOfferingRepository_CountMaxActiveByCareOfferingInRange_IncludesFutureBookings":                         "ee3f61ed3524d1c1",
	"database/repositories/enrollment/request_child_offering_repository_grade_levels_test.go:TestRequestChildOfferingRepository_Aggregates_CountEveryPhaseLikeTheGate":                                                "c4d926e13c00013f",
	"database/repositories/enrollment/request_child_offering_repository_grade_levels_test.go:TestRequestChildOfferingRepository_CountMaxActiveByIDsInRange_GuardsItsInput":                                            "3e0123b5b121f7e5",
	"database/repositories/enrollment/request_child_offering_repository_grade_levels_test.go:TestRequestChildOfferingRepository_CountMaxActiveByIDsInRange_MatchesTheSingleOfferingVariant":                           "9c28aab076159978",
	"database/repositories/enrollment/request_child_offering_repository_grade_levels_test.go:TestRequestChildOfferingRepository_CountMaxActiveByIDsInRange_SeparatesOfferings":                                        "eb87c8e3d99d7d97",
	"database/repositories/feedback/entry_repository_test.go:TestEntryRepository_FindByDateRange":                                                                                                                     "3a987c811c920c35",
	"database/repositories/feedback/entry_repository_test.go:TestEntryRepository_FindByStudentAndDateRange":                                                                                                           "a3966028a459c28c",
	"database/repositories/schedule/staff_shift_repo_test.go:TestStaffShiftRepository_DeleteUpcomingByStaffID":                                                                                                        "7a44bd73fba181fe",
	"database/repositories/users/care_withdrawal_completion_test.go:TestCareWithdrawalCompletionRepository_ParticipationBoundaryUsesPendingCompletionWhenEnrollmentIsOpen":                                            "35f8324c27b61758",
	"database/repositories/users/care_withdrawal_completion_test.go:TestCareWithdrawalCompletionRepository_UpsertUsesIncomingBoundary":                                                                                "40cd605546a7e013",
	"services/absence/excused_request_service_test.go:TestDecide_ApproveRefusedWhenPartialAbsenceExists":                                                                                                              "bea844e2b210c7b3",
	"services/active/attendance_service_test.go:TestGetStudentAttendanceStatus_NotCheckedIn":                                                                                                                          "33ca603438f00e6d",
	"services/active/cleanup_service_test.go:TestCleanupStaleAttendance_CheckOutTimeIsBerlinEndOfDay":                                                                                                                 "867efc4787c5cf40",
	"services/active/cleanup_supervisors_test.go:TestCleanupStaleSupervisors_ClosesYesterdayRecords":                                                                                                                  "ccf9bf1c069dc79e",
	"services/active/staff_absence_service_test.go:TestAbsCreateAbsenceFor_RejectsCompTimeAgainstLaterLedgerCapacity":                                                                                                 "cb52ce0e3c797dc6",
	"services/active/staff_opening_balance_mock_test.go:TestStaffBalanceAdjustmentService_OpeningAllowsNegativeTarget":                                                                                                "c22b55376229cb66",
	"services/active/student_status_day_write_bulk_test.go:TestBulkCreateForDates_RejectsConflictWithoutPartialWrites":                                                                                                "ad043a1aae2b0898",
	"services/active/student_status_day_write_bulk_test.go:TestCreateForDates_RejectsConflictWithoutPartialWrites":                                                                                                    "81c21301241e828b",
	"services/active/work_session_autocheckout_mock_test.go:TestAutoCheckout_QueriesOpenSessionsIncludingToday":                                                                                                       "e5f8944d4021a284",
	"services/active/work_session_service_test.go:TestWSApplyCustomScheduleRows_StampsAnchorForFirstRotation":                                                                                                         "2347d9d0a37427e6",
	"services/education/grade_transition_offering_resync_test.go:TestGradeTransitionService_ApplyAndRevert_ResyncOfferingSourcedRosters":                                                                              "87758be42fc5d4b6",
	"services/education/grade_transition_roster_reconcile_test.go:TestGradeTransitionService_Revert_FillsBackdatedInstance":                                                                                           "9bda6e2d66560233",
	"services/education/grade_transition_roster_reconcile_test.go:TestGradeTransitionService_Revert_FillsTodaysInstanceMaterializedWhileAlumnus":                                                                      "c230eb320cc571bc",
	"services/enrollment/care_offering_booking_stats_internal_test.go:TestBookingStatsWindow_DefaultsToTodayWithoutPhaseDates":                                                                                        "baed039a1339ab3a",
	"services/enrollment/care_offering_booking_stats_internal_test.go:TestListBookingStats_CountsInTheCapacityGatesWindow":                                                                                            "095f3718e8b38730",
	"services/enrollment/class_roster_care_end_test.go:TestClassRosterFiltersCareDate":                                                                                                                                "0f9af60cc1b16754",
	"services/enrollment/decision_service_test.go:TestDecisionService_Decide_ApprovedScheduledPastStartActivatesStudent":                                                                                              "a22a709ea60f0361",
	"services/enrollment/decision_service_test.go:TestDecisionService_ListChildOfferings_CarriesAttributesAndFutureBookings":                                                                                          "0877a2d0a0447e8a",
	"services/enrollment/offering_change_request_service_test.go:TestOfferingChangeRequestService_ListPending_ReportsDateClampedToThePhaseStart":                                                                      "4e614d5011180e1a",
	"services/enrollment/offering_change_request_service_test.go:TestOfferingChangeRequestService_ListPending_ReportsTheSelectableDateRange":                                                                          "a95581fce52662fb",
	"services/enrollment/offering_source_service_test.go:TestDecide_MultiSourceFanOutSeedsFromPhaseStart":                                                                                                             "478ae3a831bb5e10",
	"services/enrollment/offering_source_service_test.go:TestListOfferingSourceOptions_CountsScopedToSelectedPeriod":                                                                                                  "4c8acc8ee6b8e59d",
	"services/enrollment/offering_source_service_test.go:TestUpdateChildOfferings_UndatedCorrectionKeepsPhaseStartOnMultiSource":                                                                                      "ed9fc54180765d67",
	"services/enrollment/report_service_test.go:TestCareUsageEnrichesGuardiansAndSchedulePickup":                                                                                                                      "b2cde36a6c78a2a8",
	"services/enrollment/report_service_test.go:TestClassRosterUsesOfferingDateForPickupProjection":                                                                                                                   "7116b1aa5e568165",
	"services/feedback/feedback_service_test.go:TestFeedbackService_GetEntriesByDateRange":                                                                                                                            "4be832fee2bdf672",
	"services/feedback/feedback_service_test.go:TestFeedbackService_GetEntriesByStudentAndDateRange":                                                                                                                  "45ad2f40c29ca3a1",
	"services/parent/care_exception_service_test.go:TestListCareExceptions_FlagsAbsentArrivalRow":                                                                                                                     "61b3143c47371951",
	"services/parent/care_exception_service_test.go:TestListCareExceptions_FlagsAbsentPickupRow":                                                                                                                      "306c3731175216b3",
	"services/parent/excused_request_test.go:TestExcusedRequest_ApproveWritesStatusDays":                                                                                                                              "aadd2f7d1acc006b",
	"services/parent/parent_care_offerings_service_test.go:TestGetChildCareOfferingsReturnsCompleteSortedView":                                                                                                        "fa10de1db56febb4",
	"services/parent/parent_meal_plan_test.go:TestMealPlanWeek_AllowsNextWeek":                                                                                                                                        "802dcbc083f9e87d",
	"services/parent/parent_meal_plan_test.go:TestMealPlanWeek_DisabledReturnsSentinel":                                                                                                                               "c0e80a5ce78bfdba",
	"services/parent/parent_meal_plan_test.go:TestMealPlanWeek_FarFutureWeekOutOfRange":                                                                                                                               "65fd9bcc2584890e",
	"services/parent/parent_meal_plan_test.go:TestMealPlanWeek_NotOwnedChildRejected":                                                                                                                                 "a6569656bd50162b",
	"services/parent/parent_meal_plan_test.go:TestMealPlanWeek_PastWeekOutOfRange":                                                                                                                                    "76e1b1b8b86d040f",
	"services/parent/parent_meal_plan_test.go:TestMealPlanWeek_ReturnsCurrentWeekEntries":                                                                                                                             "97e62a23823950fd",
	"services/parent/parent_meal_plan_test.go:TestMealPlanWeek_SettingErrorPropagates":                                                                                                                                "8de2f66a37c70596",
	"services/parent/parent_request_edit_test.go:TestEditExcusedRequestReplacesWithdrawal":                                                                                                                            "f3403224702f5441",
	"services/parent/parent_write_service_test.go:TestListSickDays_ExcludesStaffCreatedExcused":                                                                                                                       "36ef7107cdd6cb42",
	"services/parent/parent_write_service_test.go:TestListSickDays_ReturnsSickAndExcused":                                                                                                                             "d3e6330f791b7719",
	"services/parent/parent_write_service_test.go:TestSubmitSickNote_ClearsClassTripForSubmittedDate":                                                                                                                 "d84cc8ee337d9031",
	"services/parent/parent_write_service_test.go:TestSubmitSickNote_FutureWriteSerializesWithStaffConflictCheck":                                                                                                     "89ecd146f1df92e9",
	"services/parent/parent_write_service_test.go:TestSubmitSickNote_NonContiguousExcludesUnrelatedRows":                                                                                                              "1861b11458bef8ff",
	"services/parent/parent_write_service_test.go:TestSubmitSickNote_RefusesPartialAbsenceConflict":                                                                                                                   "e01d133da033a22c",
	"services/parent/sick_note_gate_pin_test.go:TestSickNoteStaysImmediateWhenApprovalDisabled":                                                                                                                       "9ecf71fcf9d35732",
	"services/schedule/partial_absence_pending_request_test.go:TestPartialAbsenceCreate_RefusesPendingFullDayRequest":                                                                                                 "695b07546479650a",
	"services/schedule/shift_plan_sync_service_test.go:TestSickCascade_ClearSickForRange":                                                                                                                             "140c17bde3f20bdf",
	"services/schedule/shift_plan_sync_service_test.go:TestSickCascade_ConcurrentOverlappingReportsSerializeBeforeOverlapRead":                                                                                        "09e52a2bd60d6a30",
	"services/schedule/shift_plan_sync_service_test.go:TestSickCascade_UpdateRangeRollsBackWhenRemovedShiftCannotReactivate":                                                                                          "343c055a17d0ef5e",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffScheduleOverview_SeriesFieldsRideExistingReads":                                                                                                "f7a12630cacf8249",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_CapAllByStaffIDClampsFutureSeries":                                                                                                 "da7136ecd814b12f",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_CollisionSkipsAndReports":                                                                                                          "c437e02dfeae0d00",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_CreateMaterializesFromTomorrow":                                                                                                    "d58966626e3869d3",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_CreateRejectsBadReferences":                                                                                                        "97eeed472f9f4a60",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_EditDetachesAndDeleteRecordsException":                                                                                             "1f05e28cdec12c97",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_EndAtFirstOccurrence":                                                                                                              "d4c67aba8a21da6e",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_EndSeriesKeepsDetachedAndPast":                                                                                                     "194198242280bcb4",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_SplitAtFirstOccurrence":                                                                                                            "551d0569c8c007e1",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_SplitOutsideSegmentRejected":                                                                                                       "1ac1cd884d9fff33",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_SplitPreservesDeviationsOnSuccessor":                                                                                               "d270a2055873d560",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_SplitTodayUpdatesOccurrenceAndReplansTomorrow":                                                                                     "09d6c197dab9e006",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_WeekPatternRequiresCycle":                                                                                                          "83ab726a80cc5ca0",
	"services/schedule/staff_shift_series_mock_test.go:TestEndSeriesUnit_ErrorBranches":                                                                                                                               "6d7654771d91914e",
	"services/schedule/staff_shift_series_mock_test.go:TestGetSeriesUnit":                                                                                                                                             "4e580640f0edfa79",
	"services/schedule/staff_shift_series_mock_test.go:TestSplitSeriesUnit_ErrorBranches":                                                                                                                             "43b7f15a816a989d",
	"services/schedule/staff_shift_series_rule_edit_test.go:TestStaffShiftSeries_SplitAppliesNewWeekdaysFromEffectiveDate":                                                                                            "4ff4dac59c71eb43",
	"services/schedule/staff_shift_series_rule_edit_test.go:TestStaffShiftSeries_SplitBoundsEarlierSegmentAtNextSuccessor":                                                                                            "ae884f0c1e8d4e94",
	"services/schedule/staff_shift_series_rule_edit_test.go:TestStaffShiftSeries_SplitExtendsSeriesEndingToday":                                                                                                       "0799163b7db523cb",
	"services/schedule/staff_shift_series_rule_edit_test.go:TestStaffShiftSeries_SplitKeepsStoredValidityWhenUnset":                                                                                                   "d87cf41a3aa234d4",
	"services/schedule/staff_shift_series_rule_edit_test.go:TestStaffShiftSeries_SplitRejectsSupersededSegment":                                                                                                       "dbe6a8b0ff4753f3",
	"services/schedule/staff_shift_series_rule_edit_test.go:TestStaffShiftSeries_SplitRejectsValidityBeyondCalendarPeriod":                                                                                            "a571de5c0130b0f1",
	"services/schedule/staff_shift_series_rule_edit_test.go:TestStaffShiftSeries_SplitRejectsWhenNoOccurrenceRemains":                                                                                                 "9c107f28bfa816b6",
	"services/schedule/staff_shift_series_rule_edit_test.go:TestStaffShiftSeries_SplitShortensValidityAndDropsLaterShifts":                                                                                            "39afd98877ebbfaa",
	"services/schedule/template_end_service_unit_test.go:TestTemplateEndFromDate_ReturnsSummaryAndDeletesOpenEndedWindow":                                                                                             "899c95a8695dd10f",
	"services/schedule/template_offering_source_unit_test.go:TestResyncUpdatedTemplateOfferingRoster":                                                                                                                 "d2a7291797217399",
	"services/schedule/template_series_roster_mock_test.go:TestReconcileSeriesPredecessorRoster_CreatesBoundedRows":                                                                                                   "f8eb0964ce415493",
	"services/users/care_booking_authority_integration_test.go:TestBookingMutationPlansFutureNaturalEndImmediately":                                                                                                   "31b10f4d9ea7b782",
	"services/users/care_booking_authority_integration_test.go:TestOverdueRebookingReplacesTheStaleCompletion":                                                                                                        "11b33fdd50f6f8a0",
	"services/users/care_lifecycle_service_test.go:TestCareLifecycle_CancelPutsThePlanBack":                                                                                                                           "2d7c37b2ee7146bb",
	"services/users/care_lifecycle_service_test.go:TestCareLifecycle_CancelRefusesOrdinaryEnrollmentEnd":                                                                                                              "e6e6a25a7cf2a476",
	"services/users/care_lifecycle_service_test.go:TestCareLifecycle_CancelRestoresPreviousEnrollmentEnd":                                                                                                             "09362b7f5354072b",
	"services/users/care_lifecycle_service_test.go:TestCareLifecycle_LastCareDayIsInclusive":                                                                                                                          "06f557c1db7436c9",
	"services/users/care_lifecycle_service_test.go:TestCareLifecycle_Resume":                                                                                                                                          "3d114b2f4244432f",
	"services/users/care_withdrawal_lifecycle_test.go:TestCareWithdrawalLifecycle_AllowsRetroactiveExitButNotBeforeAttendance":                                                                                        "3eb57e6ca0290064",
	"services/users/care_withdrawal_lifecycle_test.go:TestCareWithdrawalLifecycle_CancellingPlannedExitRestoresTask":                                                                                                  "f9c359247aea2198",
	"services/users/person_service_eligibility_test.go:TestFilterStudentsEligibleOnDate_IncludesImmediatelyActiveFutureStudentToday":                                                                                  "d2a6258a1b9d14a6",
	"api/iot/checkin/checkin_test.go:TestDeviceCheckin_ResponseIncludesPickupTime":                                                                                                                                    "67027fca5312e9dc",
	"api/iot/checkin/checkin_test.go:TestDevicePickupQuery_ReturnsPickupInfoWithoutCreatingVisit":                                                                                                                     "873e521eaef54557",
	"api/iot/checkin/checkin_test.go:TestDevicePickupQuery_PrefersDayNotesOverRecurringNotes":                                                                                                                         "695a8628534c8d15",
	"api/iot/checkin/checkin_test.go:TestDevicePickupQuery_PreservesRecurringNotesWhenExceptionReasonIsBlank":                                                                                                         "54010f6b9ccf2685",
	"api/students/students_test.go:TestListStudents_WithPickupTimes":                                                                                                                                                  "d4232768aee9267a",
	"api/students/students_test.go:TestListStudents_WithArrivalTimes":                                                                                                                                                 "882be1a21e280f56",
	"services/active/visit_helpers_test.go:TestCreateVisit_WithDevice":                                                                                                                                                "1baf71bdf2cd69dc",
	"services/active/visit_helpers_test.go:TestCreateVisit_CompletedVisitCreatesClosedAttendance":                                                                                                                     "de747d6b7efaf4eb",
	"services/active/visit_helpers_test.go:TestUpdateVisit_ReconcilesMatchingAttendanceSession":                                                                                                                       "1d55b07434ea1737",
	"services/active/visit_helpers_test.go:TestUpdateVisit_GroupMoveWithCheckoutClosesAttendanceSession":                                                                                                              "4f5f1d2b3b16c181",
	"services/active/visit_helpers_test.go:TestCreateVisit_ReEntry":                                                                                                                                                   "574708e65294d24c",
	"services/schedule/instance_service_integration_test.go:TestInstance_Reopen_RestoresAbsenceProvenance":                                                                                                            "58b5a2e372859608",
	"services/schedule/instance_service_integration_test.go:TestInstance_Complete_SkipsChildrenNotInCareThatDay":                                                                                                      "a5cd19d18f4c131f",
	"services/schedule/instance_service_integration_test.go:TestInstance_Complete_RecordsAbsenceForCancelledDay":                                                                                                      "ded72e75a6feab6e",
	"services/users/staff_document_service_test.go:TestStaffDocumentService_CreateHydratesGeneratedTimestamps":                                                                                                        "7bc8a6e65fb072bd",
	"services/users/staff_document_service_test.go:TestStaffDocumentService_RetentionSchedule":                                                                                                                        "9fd606fbe5d635d2",
	"simulate/feed_tombstone_test.go:TestRunFullDaySeedsStaffFeedTombstone":                                                                                                                                           "9de0fc1219436ec7",
	"database/repositories/users/care_withdrawal_completion_test.go:TestCareWithdrawalCompletionRepository_OnePendingTaskPerChild":                                                                                    "4a84dc3114fe62bb",
	"services/schedule/shift_plan_sync_service_test.go:TestSickCascade_ShiftOnlyChangesBroadcastTenantInvalidation":                                                                                                   "b74151a3386dbed1",
	"services/schedule/shift_plan_sync_service_test.go:TestSickCascade_ReconcileSickRangeAppliesOnlyDateDelta":                                                                                                        "ef491ef68f3de3e2",
	"database/repositories/active/group_supervisor_repository_test.go:TestGroupSupervisorRepository_FindByID":                                                                                                         "e390a916b4dd2a08",
	"database/repositories/active/group_supervisor_repository_test.go:TestGroupSupervisorRepository_Update":                                                                                                           "34a88928f2ef96f6",
	"database/repositories/active/group_supervisor_repository_test.go:TestGroupSupervisorRepository_Delete":                                                                                                           "f3aa66c435f2f414",
	"database/repositories/active/group_supervisor_repository_test.go:TestGroupSupervisorRepository_EndSupervision":                                                                                                   "2cef41393cada2df",
	"database/repositories/active/group_supervisor_repository_test.go:TestGroupSupervisorRepository_EndAllActiveByStaffID":                                                                                            "5230b9d1baa861e6",
	"database/repositories/active/group_supervisor_repository_test.go:TestGroupSupervisorRepository_EndByActiveGroupAndStaffID":                                                                                       "6e0408bd298800fa",
	"services/active/supervisor_service_test.go:TestActiveService_GetGroupSupervisor":                                                                                                                                 "009a5d506efb354c",
	"services/active/supervisor_service_test.go:TestActiveService_UpdateGroupSupervisor":                                                                                                                              "a42781a3d4c5a8b5",
	"services/active/supervisor_service_test.go:TestActiveService_DeleteGroupSupervisor":                                                                                                                              "aee0464810f033ce",
	"services/active/supervisor_service_test.go:TestActiveService_EndSupervision":                                                                                                                                     "f46a63e14525833e",
	"services/scheduler/scheduler_test.go:TestCheckAndRunMaterialization_EnabledByDefault":                                                                                                                            "4a7fc1dc1f14c956",
	"services/scheduler/scheduler_test.go:TestCheckAndRunMaterialization_WasRunToday":                                                                                                                                 "585a4b874a228472",
	"services/scheduler/scheduler_test.go:TestCheckAndRunMaterialization_HappyPath":                                                                                                                                   "3ebd9269f35721d3",
	"services/scheduler/scheduler_test.go:TestCheckAndRunMaterialization_ZeroCounters":                                                                                                                                "f52945f289265919",
	"services/scheduler/scheduler_test.go:TestCheckAndRunMaterialization_MaterializerError":                                                                                                                           "0c17996e78855cb9",
	"services/scheduler/scheduler_test.go:TestCheckAndRunMaterialization_OnlyRacedCounter":                                                                                                                            "e23a47bd6a4560fc",
	"services/scheduler/scheduler_test.go:TestIsoWeekdayMatchesNow_NonSundayMismatch":                                                                                                                                 "5355bfe0662b3f3a",
	"database/repositories/active/group_supervisor_repository_test.go:TestGroupSupervisorRepository_List":                                                                                                             "7f1dc43991d00dc1",
	"database/repositories/active/group_supervisor_repository_test.go:TestGroupSupervisorRepository_FindActiveByStaffID":                                                                                              "4db3aa662cd90a34",
	"database/repositories/education/group_substitution_repository_test.go:TestGroupSubstitutionRepository_List_WithFilters":                                                                                          "5242eac453815f6f",
	"services/active/supervisor_service_test.go:TestActiveService_CreateGroupSupervisor":                                                                                                                              "ac44cb878951b3a5",
	"services/enrollment/offering_pickup_projection_test.go:TestOfferingPickupProjection_FutureBookingEndIsNotVisibleOnEffectiveDate":                                                                                 "205b896b6ef368d2",
	"services/enrollment/offering_pickup_projection_test.go:TestOfferingPickupProjection_FutureReplacementStartsExactlyOnEffectiveDate":                                                                               "bf7d6901386408f4",
	"services/feedback/errors_test.go:TestInvalidDateRangeError_Unwrap":                                                                                                                                               "22cbed94a3cc1dc0",
	"services/feedback/feedback_service_test.go:TestFeedbackErrors":                                                                                                                                                   "38269b5bfaab0693",
	"services/parent/parent_today_status_service_test.go:TestGetChildTodayStatusCareDayWithoutAttendance":                                                                                                             "625fd1953b5eed6b",
	"services/parent/parent_today_status_service_test.go:TestGetChildTodayStatusCareDayWithoutArrivalTime":                                                                                                            "190056881768d993",
	"services/parent/parent_today_status_service_test.go:TestGetChildTodayStatusPresentOnCareDay":                                                                                                                     "c4dc9057b6853130",
	"services/parent/parent_today_status_service_test.go:TestGetChildTodayStatusPickupOnlyDoesNotClaimNoCare":                                                                                                         "1326fe190a94b83d",
	"services/parent/parent_today_status_service_test.go:TestGetChildTodayStatusTracksAttendanceSchoolWide":                                                                                                           "575e40b29eb1d11f",
	"services/parent/parent_today_status_service_test.go:TestGetChildTodayStatusAbsentArrivalExceptionOverridesWeeklyPlan":                                                                                            "c40f644b7a617169",
	"services/schedule/arrival_baseline_service_test.go:TestArrivalBaselineCareDayWithoutAnyClassTime":                                                                                                                "6d3dcc9e800bb42e",
	"services/schedule/pickup_change_request_service_test.go:TestPickupChangeRequestAppliesOnlyAfterStaffApproval":                                                                                                    "837c78b44e53112b",
	"services/schedule/pickup_change_request_service_test.go:TestPickupChangeApprovalExcusesBlocksAfterEarlierPickup":                                                                                                 "9c4bdf5d0381f49b",
	"services/schedule/pickup_change_request_service_test.go:TestPendingPickupChangeNamesBlocksRemovedByApproval":                                                                                                     "c193f9b98dc66717",
	"services/schedule/pickup_change_request_service_test.go:TestPendingPickupChangeMarksUnavailableImpact":                                                                                                           "ae44373a3f021eba",
	"services/schedule/pickup_change_request_service_test.go:TestPickupApprovalRejectsStaleAffectedBlockList":                                                                                                         "07f236cf6f5d6dd8",
	"services/schedule/pickup_change_request_service_test.go:TestPickupApprovalRequiresImpactTokenForHTTPDecision":                                                                                                    "c09e0b59f44a2aba",
	"services/schedule/pickup_change_request_service_test.go:TestPickupChangeApprovalReleasesAutoExcusalAfterLaterPickup":                                                                                             "dd8cccdb35841837",
	"services/scheduler/scheduler_test.go:TestCheckAndRunMaterialization_EnabledWrongWeekday":                                                                                                                         "535bf5a12cbe8f9c",
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
type liveClock struct{}
func (liveClock) Now() time.Time { return time.Now() }

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
	methodHistory := GetHistory(WorkSession{CheckInTime: liveClock{}.Now()}, from, to)
	_ = methodHistory.WeeklySummaries
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
func unrelatedHelper() int { return 1 }
`)
	before, err := scanCalendarFixtureClockRisks(root)
	if err != nil || len(before) == 0 {
		t.Fatalf("initial scan = %v, %v", before, err)
	}
	baseline := map[string]string{"sample/history_test.go:TestLegacy": before[0].fingerprint}
	writeCalendarFixtureSourceAt(t, root, "sample/helper_test.go", `package sample
import "time"
func fixtureTime() time.Time { return time.Time{} }
func unrelatedHelper() int { return 2 }
`)
	unrelated, err := scanCalendarFixtureClockRisks(root)
	if err != nil {
		t.Fatal(err)
	}
	if remaining, err := applyCalendarClockLegacyBaseline(unrelated, baseline); err != nil || len(remaining) != 0 {
		t.Fatalf("unrelated helper edit changed baseline: %v, %v", remaining, err)
	}
	writeCalendarFixtureSourceAt(t, root, "sample/helper_test.go", `package sample
import "time"
func fixtureTime() time.Time { return time.Now() }
func unrelatedHelper() int { return 2 }
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
func freshnessSession() WorkSession {
	return WorkSession{CheckInTime: time.Date(2026, 8, 19, 8, 0, 0, 0, tz.Berlin), UpdatedAt: time.Now()}
}
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
	freshHistory := GetHistory(WorkSession{CheckInTime: checkIn, UpdatedAt: time.Now()})
	helperHistory := GetHistory(freshnessSession())
	time := fakeClock{}
	_ = []any{base, from, to, checkIn, elapsedStart, history.WeeklySummaries, freshHistory.WeeklySummaries, helperHistory.WeeklySummaries, time.Now(), "time.Now().Add(-2h)"}
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

func TestCalendarFixtureRatchetQualifiesNowMethodsByReceiver(t *testing.T) {
	t.Parallel()

	root := writeCalendarFixtureSource(t, "sample/methods_test.go", `package sample
import (
	"testing"
	"time"
)
type liveClock struct{}
type fakeClock struct{}
func (liveClock) Now() time.Time { return time.Now() }
func (fakeClock) Now() time.Time { return time.Time{} }
func currentISOWeekday() int { return int(time.Now().Weekday()) }
func delegatedISOWeekday() int { return currentISOWeekday() }
func TestLiveMethod(t *testing.T) {
	t.Parallel()
	history := GetHistory(WorkSession{CheckInTime: liveClock{}.Now()})
	_ = history.WeeklySummaries
}
func TestFakeMethod(t *testing.T) {
	t.Parallel()
	history := GetHistory(WorkSession{CheckInTime: fakeClock{}.Now()})
	_ = history.WeeklySummaries
}
func TestFactoryMethod(t *testing.T) {
	t.Parallel()
	history := GetHistory(WorkSession{CheckInTime: factoryTime()})
	_ = history.WeeklySummaries
}
func TestExplicitReceiverTypeConverges(t *testing.T) {
	t.Parallel()
	var clock Clock
	clock = liveClock{}
	history := GetHistory(WorkSession{CheckInTime: clock.Now()})
	_ = history.WeeklySummaries
}
func TestRepeatedInterfaceAssignmentsUseLastConcreteClock(t *testing.T) {
	t.Parallel()
	var clock Clock
	clock = liveClock{}
	clock = fakeClock{}
	history := GetHistory(WorkSession{CheckInTime: clock.Now()})
	_ = history.WeeklySummaries
}
func TestAnonymousRange(t *testing.T) {
	t.Parallel()
	_ = List(struct{ From, To Date }{From: TodayDate(), To: fixedDate})
}
func TestLiveWeekdayFixture(t *testing.T) {
	t.Parallel()
	_ = map[string]int{"weekday": currentISOWeekday()}
}
func TestIndirectLiveWeekdayFixture(t *testing.T) {
	t.Parallel()
	_ = map[string]int{"weekday": delegatedISOWeekday()}
}
`)
	writeCalendarFixtureSourceAt(t, root, "sample/factory_test.go", `package sample
import "time"
type Clock interface { Now() time.Time }
func newLiveClock() liveClock { return liveClock{} }
func factoryTime() time.Time { return newLiveClock().Now() }
`)
	writeCalendarFixtureSourceAt(t, root, "sample/live_date_test.go", `package sample
import (
	. "time"
	. "github.com/stretchr/testify/assert"
	assertpkg "github.com/stretchr/testify/assert"
	tz "github.com/moto-nrw/project-phoenix/internal/timezone"
)
type Date = tz.Date
var fixedDate = tz.NewDate(2026, 8, 30)
func TodayDate() Date { return tz.TodayDate() }
func liveDate() Date { return tz.DateFromTime(Now()) }
func List(value any) any { return value }
func TestLiveDateConversionHelper(t *testing.T) {
	t.Parallel()
	assertpkg.Equal(t, liveDate(), fixedDate)
}
func normalize(date Date) Date { return date }
func TestWrappedLiveDate(t *testing.T) {
	t.Parallel()
	date := normalize(TodayDate())
	Equal(t, date, fixedDate)
}
func TestDotImportedNow(t *testing.T) {
	t.Parallel()
	history := GetHistory(WorkSession{CheckInTime: Now()})
	_ = history.WeeklySummaries
}
`)
	findings, err := scanCalendarFixtureClockRisks(root)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(formatCalendarClockFindings(findings), "\n")
	if !strings.Contains(joined, "TestLiveMethod") || !strings.Contains(joined, "TestFactoryMethod") ||
		!strings.Contains(joined, "TestExplicitReceiverTypeConverges") || !strings.Contains(joined, "TestDotImportedNow") ||
		!strings.Contains(joined, "TestLiveDateConversionHelper") || !strings.Contains(joined, "TestAnonymousRange") ||
		!strings.Contains(joined, "TestLiveWeekdayFixture") || !strings.Contains(joined, "TestIndirectLiveWeekdayFixture") ||
		!strings.Contains(joined, "TestWrappedLiveDate") || strings.Contains(joined, "TestFakeMethod") ||
		strings.Contains(joined, "TestRepeatedInterfaceAssignmentsUseLastConcreteClock") {
		t.Fatalf("receiver-qualified findings were %q", joined)
	}
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
