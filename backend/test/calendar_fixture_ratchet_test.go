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
	"database/repositories/active/work_session_test.go:TestWorkSessionRepository_ListOverlappingByStaffID_KeepsEarlierStarts":                                                             "d5123d0c83b631ea",
	"database/repositories/education/group_substitution_repository_test.go:TestGroupSubstitutionRepository_FindOverlapping":                                                               "7bc5a2677e76e106",
	"database/repositories/enrollment/request_child_offering_repository_grade_levels_test.go:TestRequestChildOfferingRepository_CountActiveGradeLevels_CountsAChildOncePerOffering":       "b870b6c3895f5992",
	"database/repositories/enrollment/request_child_offering_repository_grade_levels_test.go:TestRequestChildOfferingRepository_CountActiveGradeLevels_EmptyInputSkipsTheQuery":           "5d20c3a5f8f56c5f",
	"database/repositories/enrollment/request_child_offering_repository_grade_levels_test.go:TestRequestChildOfferingRepository_CountActiveGradeLevels_ExcludesIntervalsOutsideTheWindow": "c32d2d406eafb3d2",
	"database/repositories/enrollment/request_child_offering_repository_grade_levels_test.go:TestRequestChildOfferingRepository_CountActiveGradeLevels_ExcludesTerminalChildren":          "5ffbda9b86b1227d",
	"database/repositories/enrollment/request_child_offering_repository_grade_levels_test.go:TestRequestChildOfferingRepository_CountActiveGradeLevels_GroupsByGrade":                     "2dfa0a4fd890f0ee",
	"database/repositories/enrollment/request_child_offering_repository_grade_levels_test.go:TestRequestChildOfferingRepository_CountActiveGradeLevels_RejectsAnEmptyWindow":              "2c5f91a2ed517eb0",
	"database/repositories/enrollment/request_child_offering_repository_grade_levels_test.go:TestRequestChildOfferingRepository_CountActiveGradeLevels_ReportsMissingGradeSeparately":     "eb2c10d13501d6e5",
	"services/active/staff_vacation_opening_db_test.go:TestSetVacationOpening_NegativeRemainingAllowsOverdrawnAccount":                                                                    "edc6c0efdae8e188",
	"services/active/staff_vacation_opening_db_test.go:TestSetVacationOpening_RejectsOpenCutoff":                                                                                          "03b13a8c11bf16a9",
	"services/active/staff_vacation_opening_db_test.go:TestSetVacationOpening_RejectsPastVacationYear":                                                                                    "e24d1802267cd6fe",
	"services/active/staff_vacation_opening_db_test.go:TestSetVacationOpening_RejectsSecondOpeningForSameYear":                                                                            "4fab2fdc8d83e45b",
	"services/active/staff_vacation_opening_db_test.go:TestSetVacationOpening_RejectsVacationAbsencesBeforeCutoff":                                                                        "1364cdf3dd41e14d",
	"services/active/staff_vacation_opening_db_test.go:TestSetVacationOpening_RespectsCustomQuota":                                                                                        "f5f9637333cd0551",
	"services/calendar/service_integration_test.go:TestCalendarServiceIntegration_DeleteFeedVisibleLeavesTombstone":                                                                       "4ca0fc4f31672e07",
	"services/education/education_service_test.go:TestUpdateSubstitution_DateValidation":                                                                                                  "3b02e4bf12a5f3e1",
	"services/enrollment/offering_change_request_automatic_test.go:TestOfferingChangeRequestService_Decide_ExclusionKeepsManualAndRequiredLunchDays":                                      "c84ae13116264daf",
	"services/enrollment/offering_change_request_automatic_test.go:TestOfferingChangeRequestService_Decide_SnapshotMatchesGrandfatheredAutomaticBooking":                                  "816e21be5ed8ba27",
	"services/enrollment/offering_change_request_service_test.go:TestOfferingChangeRequestService_Create_PayloadExcludesAutomaticOfferings":                                               "1d380061a1344c62",
	"services/enrollment/offering_change_request_service_test.go:TestOfferingChangeRequestService_Decide_LogsNoWishWhenTheDateIsKept":                                                     "461cd713143e264d",
	"services/parent/care_exception_service_test.go:TestDeleteCareExceptionPreservesArrival":                                                                                              "e0224f1002c6e241",
	"services/parent/care_exception_service_test.go:TestDeleteCareException_RemovesPickupAndPreservesArrival":                                                                             "f012268c995553b6",
	"services/parent/care_exception_service_test.go:TestListCareExceptions_ArrivalRepoErrorSurfaces":                                                                                      "aa4f2ef313a870ef",
	"services/parent/care_exception_service_test.go:TestListCareExceptions_NotOwnedChild":                                                                                                 "21c25dd44f062293",
	"services/parent/care_exception_service_test.go:TestListCareExceptions_RepoErrorSurfaces":                                                                                             "f21d530180a3316f",
	"services/parent/care_exception_service_test.go:TestSubmitCareException_ClearingLegRemovesIt":                                                                                         "1dca7db03991ed7b",
	"services/parent/excused_request_test.go:TestSickRequest_ApproveWritesSickStatusAndLiveFlag":                                                                                          "3a281e961e955b14",
	"services/parent/parent_care_schedule_service_test.go:TestGetChildCareSchedule_TodayAbsentReflectsStatusDay":                                                                          "565c20165f06e3cf",
	"services/parent/parent_write_service_test.go:TestListSickDays_AllowsPortalAccessWithoutWritePermissions":                                                                             "8d64ea53d945a210",
	"services/parent/parent_write_service_test.go:TestListSickDays_HidesAnotherGuardiansReason":                                                                                           "cd3ec6f8ac84a5b8",
	"services/parent/parent_write_service_test.go:TestListSickDays_NotOwned":                                                                                                              "fa7921abb8b32b77",
	"services/schedule/staff_schedule_overview_integration_test.go:TestStaffScheduleOverview_TenantIsolationAcrossEveryProjectionRead":                                                    "268c72416f3baa58",
	"api/timetable/substitutions_bulk_test.go:TestBulkSubstitution_MultiDayWithSubstitute":                                                                                                "928da340ac671b4f",
	"services/enrollment/offering_change_full_withdrawal_test.go:TestOfferingChangeRequestService_ListPending_KeepsUntouchedBookingsOutOfTheWarning":                                      "717d9189e827d7e8",
	"services/enrollment/offering_change_history_test.go:TestOfferingChangeRequestService_ListHistory":                                                                                    "a735d6420819fef2",
	"services/enrollment/offering_change_request_automatic_test.go:TestOfferingChangeRequestService_Decide_ExclusionSkipsAutoTargetAndRecordsOverride":                                    "cb9e8bb69dfe7d85",
	"services/enrollment/offering_change_request_automatic_test.go:TestOfferingChangeRequestService_Decide_RejectionFallsBackToPayloadSnapshot":                                           "4b2a648cd8bb5fc2",
	"services/enrollment/offering_change_request_automatic_test.go:TestOfferingChangeRequestService_Decide_RejectionFreezesDiffSnapshot":                                                  "8611eea8c4c2d388",
	"services/enrollment/offering_change_request_automatic_test.go:TestOfferingChangeRequestService_GetForStudent_MarksAutomaticDiffEntries":                                              "599445e3fb16d77b",
	"services/enrollment/offering_change_request_automatic_test.go:TestOfferingChangeRequestService_ListPending_IncludesUnchangedGrandfatheredRuleTarget":                                 "99c48c6e775c63a1",
	"services/enrollment/offering_change_request_automatic_test.go:TestOfferingChangeRequestService_ListPending_MarksAutomaticDiffEntries":                                                "986559fdaa119b7c",
	"services/enrollment/request_child_offering_repository_date_test.go:TestRequestChildOfferingRepository_ListAtDates_DoesNotReturnHistoricalSelection":                                  "42225278ae9a1d2f",
	"services/schedule/template_end_cascade_integration_test.go:TestTemplateEnd_CascadesToCappedPredecessors":                                                                             "86507298f4f40407",
	"services/schedule/template_end_cascade_integration_test.go:TestTemplateEnd_FromCappedPredecessorAlsoEndsLivingSuccessor":                                                             "617093c5b071f30c",
	"services/schedule/template_offering_source_integration_test.go:TestTemplateOfferingSource_PullForwardWidensSourcedRoster":                                                            "7487ba8028db7138",
	"services/schedule/template_offering_source_integration_test.go:TestTemplateOfferingSource_SplitAwayFromAngebotClearsSourcedRoster":                                                   "89f0380462226a34",
	"services/schedule/template_series_roster_integration_test.go:TestSeriesRoster_AddsChildAcrossCappedPredecessor":                                                                      "d25bb75bce227e32",
	"services/schedule/template_series_roster_integration_test.go:TestSeriesRoster_KeepsPredecessorOnlyChildOutsideScope":                                                                 "91c1632dc6b2ab7f",
	"services/schedule/template_series_roster_integration_test.go:TestSeriesRoster_NarrowedSuccessorLeavesTheOtherWeekdayIntact":                                                          "116f0a89c385697e",
	"services/schedule/template_series_roster_integration_test.go:TestSeriesRoster_NoopWithoutSeriesRosterFrom":                                                                           "b61fd665d8d5d4d5",
	"services/schedule/template_series_roster_integration_test.go:TestSeriesRoster_PastAnchorClampsToTodayAndSegmentStart":                                                                "8cbcbb6fd481f99b",
	"services/schedule/template_series_roster_integration_test.go:TestSeriesRoster_PreservesHandRemovedChildOnPredecessorOccurrence":                                                      "24a8778b5289d54c",
	"services/schedule/template_series_roster_integration_test.go:TestSeriesRoster_PrimaryChangeReachesMaterializedOccurrences":                                                           "1a6411ab22ea23e1",
	"services/schedule/template_series_roster_integration_test.go:TestSeriesRoster_ReachesTheClickedWeekdayTheSuccessorNoLongerRuns":                                                      "602088d9440b2feb",
	"services/schedule/template_series_roster_integration_test.go:TestSeriesRoster_ReconcilesSupervisorsAcrossPredecessor":                                                                "9e47eecdc585f127",
	"services/schedule/template_series_roster_integration_test.go:TestSeriesRoster_RemovedStaffLosesPlannedOccurrenceRows":                                                                "b2f35eb261b9a072",
	"services/schedule/template_series_roster_integration_test.go:TestSeriesRoster_RemovesChildAcrossCappedPredecessor":                                                                   "61e5bc1b3a68ce4f",
	"services/schedule/template_series_roster_integration_test.go:TestSeriesRoster_SecondIdenticalSaveKeepsRowsUntouched":                                                                 "b275519e02cb245a",
	"services/schedule/template_series_roster_integration_test.go:TestSeriesRoster_SkipsProtectedPredecessorEnrollments":                                                                  "030bcf54aad614f0",
	"services/schedule/template_series_roster_integration_test.go:TestSeriesRoster_WeekdayScopedAddOnlyTouchesThatWeekday":                                                                "0f01960c1ba0b717",
	"services/schedule/template_series_roster_integration_test.go:TestSeriesRoster_WeekdayScopedEditLeavesOtherWeekdaysAlone":                                                             "f3fcd573590ed1b3",
	"services/schedule/template_source_class_filter_integration_test.go:TestTemplateSourceClassFilter_HonoursBookedWeekdays":                                                              "fac0ed9e5722377a",
	"api/school/school_supervisions_test.go:TestSchoolSupervisionsFollowTheAssignment":                                                                                                    "939ad4d2b478104b",
	"api/students/attendance_history_handlers_test.go:TestGetStudentAttendanceHistory_FutureEndClampsToToday":                                                                             "0d28341a44898ca5",
	"api/students/change_request_list_handlers_test.go:TestAggregatedChangeRequests_EmitsCurrentStatusPerDate":                                                                            "0a06a6c5f34c93bd",
	"api/students/change_request_list_handlers_test.go:TestAggregatedChangeRequests_GroupsContradictingAbsencesOnOneDay":                                                                  "ba230cb3b3077d57",
	"api/students/status_day_internal_test.go:TestStaffAbsenceNotificationCallbacks":                                                                                                      "f001acf18b999c6c",
	"api/timetable/exception_conflicts_test.go:TestExceptionConflicts_CancelledInstance_EmitsWarningPerExpectedStudent":                                                                   "e80ebe0679f0256a",
	"api/timetable/exception_conflicts_test.go:TestExceptionConflicts_Empty_NoExceptions":                                                                                                 "959089d277de5c73",
	"api/timetable/gaps_test.go:TestGaps_Empty":                                                                                                       "0a9ae59b6c13a8da",
	"api/timetable/instances_list_test.go:TestListInstances_Empty":                                                                                    "f0ce70c37c840186",
	"api/timetable/templates_series_test.go:TestGetTemplate_ResolvesCappedPredecessorToLivingSuccessor":                                               "636e10a7917046b1",
	"api/timetable/templates_start_pull_test.go:TestTemplateUpdateStartDatePullForward":                                                               "9986e270826fde69",
	"database/repositories/feedback/entry_repository_test.go:TestEntryRepository_List":                                                                "827fec5306ecec42",
	"database/repositories/schedule/arrival_schedule_test.go:TestStudentArrivalExceptionRepository_DeletePastExceptions":                              "36e68eae586399a7",
	"database/repositories/schedule/arrival_schedule_test.go:TestStudentArrivalNoteRepository_DeletePastNotes":                                        "1de1c5f7cc446e5b",
	"database/repositories/schedule/pickup_schedule_test.go:TestStudentPickupExceptionRepository_DeletePastExceptions":                                "1f45e601bf548bc0",
	"database/repositories/schedule/pickup_schedule_test.go:TestStudentPickupNoteRepository_DeletePastNotes":                                          "39a5c590ab7e536c",
	"services/absence/excused_request_errorpath_test.go:TestDecide_ApprovalNotifiesAfterCommit":                                                       "c8ae49fe62e1a220",
	"services/active/staff_vacation_opening_db_test.go:TestDeleteVacationOpening_WritesTombstoneAndRestoresSummary":                                   "07962d9c00702b07",
	"services/active/staff_vacation_opening_db_test.go:TestSetVacationOpening_AllowsVacationBeginningOnWeekendBeforeCutoff":                           "c3d389ae54ccaaab",
	"services/active/staff_vacation_opening_db_test.go:TestSetVacationOpening_DerivesTakenBeforeFromQuota":                                            "c73a76ee5ed934f4",
	"services/active/staff_vacation_opening_db_test.go:TestVacationOpeningRepository_BatchAndListReads":                                               "d8a0f1a62d4ca9d1",
	"services/enrollment/offering_adjustment_dated_test.go:TestDecisionService_UpdateChildOfferings_DatedSwitchBeforePhaseStartDropsUnstartedRow":     "d0ed1216a03c9308",
	"services/enrollment/offering_change_request_service_test.go:TestOfferingChangeRequestService_Catalog_MarksCurrentBookingAndCapacity":             "997b3e9aea5831fa",
	"services/enrollment/offering_change_request_service_test.go:TestOfferingChangeRequestService_Create_StoresPendingRequest":                        "81971e8887c1a387",
	"services/enrollment/offering_change_request_service_test.go:TestOfferingChangeRequestService_Create_StripsChangedCurrentAutomaticOffering":       "a54ea0a69e4c1643",
	"services/enrollment/offering_change_request_service_test.go:TestOfferingChangeRequestService_Decide_AppliesTheConfirmedDate":                     "a1b4546fd7e10b74",
	"services/enrollment/offering_change_request_service_test.go:TestOfferingChangeRequestService_Decide_ApprovalAppliesTheDatedSwitch":               "b403aefef0eff46f",
	"services/enrollment/offering_change_request_service_test.go:TestOfferingChangeRequestService_Decide_CapsRebookingAtPlannedCareEnd":               "7428d8be0401e0e0",
	"services/enrollment/offering_change_request_service_test.go:TestOfferingChangeRequestService_Decide_LogsTheDateTheFamilyAskedFor":                "8720730cd3062742",
	"services/enrollment/offering_change_request_service_test.go:TestOfferingChangeRequestService_Decide_RejectionNeedsAReasonAndChangesNothing":      "81b9f77bdf9d3aa9",
	"services/enrollment/offering_change_request_service_test.go:TestOfferingChangeRequestService_GetForStudent_ReportsRecentDecision":                "e5a97712d03b2cf3",
	"services/enrollment/offering_change_request_service_test.go:TestOfferingChangeRequestService_PreviewDecision_ReportsOnlyUncoveredManualPlanning": "c595066aadaf82f1",
	"services/enrollment/pickup_adjustment_service_test.go:TestPickupAdjustmentAppliesArrivalSchedulesOnlyForImmediateExceptions":                     "cf8f546fe07fbb9e",
	"services/import/student_import_config_test.go:TestEnrollmentStartsInFuture_UsesBusinessDate":                                                     "398602354bf022e9",
	"services/parent/care_ended_child_test.go:TestParentPortal_CareEndedChildIsReadOnly":                                                              "1affee3ba9303274",
	"services/parent/care_exception_service_test.go:TestListCareExceptions_MergesBothLegsAndFlagsStaffSource":                                         "66c0f3948f645a5d",
	"services/schedule/bulk_substitution_unit_test.go:TestNormalizeBulkDates_DedupesAndSortsAscending":                                                "ae68cd63763e4e61",
	"services/schedule/calendar_period_integration_test.go:TestCalendarPeriodService_ConcurrentBootstrapVsCreate":                                     "e0d22acb6e224c1e",
	"services/schedule/calendar_period_integration_test.go:TestCalendarPeriodService_EnsureDefaultSchoolYear":                                         "2c2fb1250dc1f79d",
	"services/schedule/care_request_history_test.go:TestListHistory_IncludesPickupChangeWithPayloadSummary":                                           "512294192502b909",
	"services/schedule/pickup_auto_excusal_test.go:TestAutoExcusal_BulkWeeklyUpsertResyncsExceptions":                                                 "6dcbcb9bfbe01d9b",
	"services/schedule/pickup_auto_excusal_test.go:TestAutoExcusal_DeletingTheExceptionRestoresBlocks":                                                "286b1e03e2adad6a",
	"services/schedule/pickup_auto_excusal_test.go:TestAutoExcusal_FullDayStatusCoexistsAndReleaseReplays":                                            "35796e456f242058",
	"services/schedule/pickup_auto_excusal_test.go:TestAutoExcusal_LaterThanBaselineMeansNoCoupling":                                                  "aa508f422b8f1601",
	"services/schedule/pickup_auto_excusal_test.go:TestAutoExcusal_ManualCreateConvertsAutoToManual":                                                  "0292c584489c9b7c",
	"services/schedule/pickup_auto_excusal_test.go:TestAutoExcusal_ManualDeleteOfConvertedRowRederivesAuto":                                           "8f824891b64afeaa",
	"services/schedule/pickup_auto_excusal_test.go:TestAutoExcusal_ManualDeleteRefusesAutoRows":                                                       "7425326cb470580a",
	"services/schedule/pickup_auto_excusal_test.go:TestAutoExcusal_ManualPartialAbsenceIsNeverTouched":                                                "43c27efa02d823f4",
	"services/schedule/pickup_auto_excusal_test.go:TestAutoExcusal_MovingPickupBackReleasesBlocks":                                                    "155bafd2b06a537d",
	"services/schedule/pickup_auto_excusal_test.go:TestAutoExcusal_MovingPickupEarlierWidensTheExcusal":                                               "6b66fce44dbd67d0",
	"services/schedule/pickup_auto_excusal_test.go:TestAutoExcusal_NoWeeklyBaselineMeansNoCoupling":                                                   "b28476ad86dc2015",
	"services/schedule/pickup_auto_excusal_test.go:TestAutoExcusal_PulledForwardPickupExcusesLaterBlocks":                                             "1c859bfbb10c42a5",
	"services/schedule/pickup_auto_excusal_test.go:TestAutoExcusal_WeeklyBaselineAddedCouplesExistingException":                                       "ae2639534a2b5c7b",
	"services/schedule/pickup_auto_excusal_test.go:TestAutoExcusal_WeeklyBaselineDeletedReleasesCoupling":                                             "36ccef5af5177732",
	"services/schedule/pickup_auto_excusal_test.go:TestAutoExcusal_WeeklyBaselineMovedEarlierReleasesCoupling":                                        "f9c3532037e78fe5",
	"services/schedule/shift_plan_sync_service_test.go:TestSickCascade_HalfDayRules":                                                                  "cf90f4f5583efcc2",
	"services/schedule/shift_plan_sync_service_test.go:TestSickCascade_PastShiftsRemainHistoricalDuringMarkAndReconcile":                              "0e401a5f106b99c0",
	"services/schedule/template_source_class_filter_integration_test.go:TestTemplateSourceClassFilter_ClassChangeMovesTheChild":                       "acf511c3294768db",
	"services/schedule/template_source_class_filter_integration_test.go:TestTemplateSourceClassFilter_DeregistrationLimitsTheAssignment":              "3ed5a512e78112ae",
	"services/schedule/template_source_class_filter_integration_test.go:TestTemplateSourceClassFilter_LaterApprovalJoinsTheTermin":                    "374163e53d617d73",
	"services/schedule/template_source_class_filter_integration_test.go:TestTemplateSourceClassFilter_MatchesCaseInsensitively":                       "19532edabb24a062",
	"services/schedule/template_source_class_filter_integration_test.go:TestTemplateSourceClassFilter_OfferingDayChangeReshapesTheRoster":             "50ce414e464476f2",
	"services/schedule/template_source_class_filter_integration_test.go:TestTemplateSourceClassFilter_SeedsOnlyTheFilteredClass":                      "5af2e835b4700505",
	"services/schedule/template_source_class_filter_integration_test.go:TestTemplateSourceClassFilter_UpdateSwitchesFromGradeToClass":                 "57e686baa76a1c41",
	"services/schedule/template_split_service_test.go:TestTemplateEndFromDate_CapsTemplateAndProtectsHistory":                                         "ad9577dd87c0afff",
	"services/schedule/template_split_service_test.go:TestTemplateEnd_ConcurrentTemplateUpdatePreservesCommittedCap":                                  "54bb3dc790be9c9f",
	"services/schedule/template_split_service_test.go:TestTemplateMutations_RejectCareOfferingSeriesConflictsWithoutPersisting":                       "3d9c4cc8181bd8ed",
	"services/schedule/template_split_service_test.go:TestTemplateSplitAndEnd_RespectCurrentSegmentEnvelope":                                          "4fde596452c9e9cb",
	"services/schedule/template_split_service_test.go:TestTemplateSplit_ExplicitRosterAndWeekPattern":                                                 "9d0239be7ef0abf7",
	"services/schedule/template_split_service_test.go:TestTemplateSplit_HappyPath_CarriesRosterAndProtectsHistory":                                    "463035a81b0788c1",
	"services/schedule/template_split_service_test.go:TestTemplateSplit_RejectsResplittingBoundedPredecessor":                                         "c10b853728452d06",
	"services/schedule/template_split_service_test.go:TestTemplateSplit_SingleEditThenSuccessorUpdateDoesNotDuplicate":                                "800ca95ea11b6b43",
	"services/schedule/template_split_service_test.go:TestTemplateSplit_SuccessorValidFrom_NoPhantomBeforeEffective":                                  "2855651b427308ab",
	"services/schedule/template_split_service_test.go:TestTemplateSplit_UpdateSegmentsPreservesBoundsDuringMaterialization":                           "9be315fe767b4909",
	"services/schedule/template_start_pull_forward_test.go:TestUpdateTemplate_StartDatePullForward_MovesEnvelopeRosterAndMaterializesGapOnly":         "b4bd2b32b3a279a3",
	"services/schedule/template_start_pull_forward_test.go:TestUpdateTemplate_StartDatePullForward_MovesWeekdayScopedRoster":                          "9f83774a425285bb",
	"services/schedule/template_start_pull_forward_test.go:TestUpdateTemplate_StartDatePullForward_RejectsPredecessorOverlap":                         "7b2a9fd387c0ac43",
	"services/users/care_booking_authority_integration_test.go:TestBookingParticipationRangeExcludesAlumniWithoutDateBoundary":                        "2ecd413b7d7b598a",
	"services/users/care_booking_authority_integration_test.go:TestNaturalBookingEndSchedulerIsIdempotent":                                            "79cf3394e20068e6",
	"services/users/care_lifecycle_integration_test.go:TestCareExit_BinarySchoolWithNfcAndGroups":                                                     "550eda271d226f51",
	"services/users/care_withdrawal_lifecycle_test.go:TestCareWithdrawalLifecycle_CompletionEndsBookingsFromEveryEnrollmentRequest":                   "1fa95f39da162979",
	"api/active/handlers_unit_test.go:TestNewActiveGroupResponse_WithActiveSupervisors":                                                               "40b90e7daf6d5d3c",
	"api/active/handlers_unit_test.go:TestNewSupervisorResponse_ActiveSupervisor":                                                                     "620c843b78709ae1",
	"api/active/handlers_unit_test.go:TestNewSupervisorResponse_WithActiveGroup":                                                                      "c189436ee4212ad3",
	"api/active/handlers_unit_test.go:TestNewSupervisorResponse_WithStaff":                                                                            "94f169f95ab56db1",
	"api/display/api_test.go:TestDisplayDashboardPickupBuckets":                                                                                       "08aa070ceb1ee250",
	"api/iot/checkin/attendance_internal_test.go:TestAttendanceInfo_Fields":                                                                           "914f43610f3b146d",
	"api/students/care_exit_handlers_test.go:TestStudentList_CareStatusDecidesWhichSideIsShown":                                                       "515529c1ec712b0d",
	"api/students/care_exit_handlers_test.go:TestStudentList_UsesBookingParticipationButKeepsAdministrationAndLivePresence":                           "7042838865f084c8",
	"api/timetable/deviation_log_test.go:TestApplyDeviations_ActiveInstance_EndsAndCreatesSupervisor":                                                 "d0d649ea2569c3e8",
	"api/timetable/instances_create_test.go:TestCreateInstance_Validation":                                                                            "fba01e13712e6bea",
	"database/repositories/active/attendance_repository_test.go:TestAttendanceRepository_CloseOpenForToday":                                           "f0c019fc33a4f3eb",
	"database/repositories/active/group_repository_test.go:TestActiveGroupRepository_FindWithSupervisors":                                             "7df8b707e8100538",
	"database/repositories/active/student_status_day_test.go:TestStudentStatusDayRepository_ClearByIDAndDates":                                        "a2454f3b351a2b0e",
	"database/repositories/active/work_session_test.go:TestWorkSessionRepository_GetTodayPresenceMap":                                                 "2586227e18bd589a",
	"database/repositories/schedule/activity_instance_repo_test.go:TestActivityInstanceRepository_DeletePlannedMaterializedWeekendInstances":          "96550626af91fe5b",
	"database/repositories/users/parent_announcement_test.go:TestParentAnnouncementAudience_WeekdayScopedEnrollmentMatchesToday":                      "47cc87c496d805ce",
	"models/active/attendance_test.go:TestAttendance_CompleteLifecycle":                                                                               "0d1d7f3a8726323e",
	"models/active/attendance_test.go:TestAttendance_Fields":                                                                                          "c1abc947f6172d91",
	"models/active/attendance_test.go:TestAttendance_GetCreatedAt":                                                                                    "1ec50bfd1ce56b9e",
	"models/active/attendance_test.go:TestAttendance_GetUpdatedAt":                                                                                    "d4465b5c5e70e0cf",
	"models/active/attendance_test.go:TestAttendance_IsCheckedIn_WhenCheckedIn":                                                                       "12c58528abf8e2ba",
	"models/active/attendance_test.go:TestAttendance_IsCheckedIn_WhenCheckedOut":                                                                      "1a191c6c678c9a13",
	"models/active/attendance_test.go:TestAttendance_MultipleRecords":                                                                                 "d494667f3f79d753",
	"services/active/active_service_wrappers_internal_test.go:TestActiveServiceThinDelegates":                                                         "bab66344c1cc2b4d",
	"services/active/analytics_service_test.go:TestGetDashboardAnalytics":                                                                             "62dd3c87c94335d8",
	"services/active/update_visit_mock_test.go:TestUpdateVisitLocksAttendanceBeforeClosingIt":                                                         "27b6f4c670d1a4e7",
	"services/active/visit_helpers_test.go:TestCreateVisit_ClearsParentStatusForToday":                                                                "f6ec2fc10b7551e2",
	"services/active/visit_helpers_test.go:TestCreateVisit_ClearsPlannedStatusForToday":                                                               "585749b5f5ec3771",
	"services/active/work_session_export_test.go:TestWSGetHistory_AuditCountError":                                                                    "f3f6e6902ed91df4",
	"services/active/work_session_export_test.go:TestWSGetHistory_BreaksError":                                                                        "1132d0a369326a43",
	"services/active/work_session_service_test.go:TestWSGetHistory_ClosedSessionKeepsCachedBreaks":                                                    "d0a8de15d891d581",
	"services/active/work_session_service_test.go:TestWSGetHistory_DeductsRunningBreakFromNetMinutes":                                                 "c737a0eb61ff9cda",
	"services/active/work_session_service_test.go:TestWSGetHistory_RepoError":                                                                         "925e169c500be567",
	"services/active/work_session_service_test.go:TestWSGetHistory_RunningBreakIsCappedAtTheLiveLimit":                                                "6f7f718643f95335",
	"services/active/work_session_service_test.go:TestWSGetHistory_SerializesRunningBreakInBreakMinutes":                                              "c8f053fa0504c9a2",
	"services/calendar/service_integration_test.go:TestCalendarServiceIntegration_CleanupExpiredFeedTombstonesCascadesChildren":                       "b5dc16cbfd46fa1f",
	"services/calendar/service_integration_test.go:TestCalendarServiceIntegration_StaffSubscriptionPublishesOccurrenceAndDeletionCancellations":       "5def5aa43e333f67",
	"services/calendar/service_integration_test.go:TestCalendarServiceIntegration_SubscriptionFeed":                                                   "22b65922fc69603d",
	"services/schedule/care_request_decision_snapshot_test.go:TestDecide_PickupChangeFreezesDiff":                                                     "223fde61b6c85cfc",
	"services/schedule/instance_service_integration_test.go:TestInstance_ReplanWeek_RemovesFutureLegacyWeekendInstances":                              "8ee1007ae52d4fc4",
	"services/schedule/instance_service_integration_test.go:TestInstance_Start_ConflictWarning_Staff":                                                 "ff5b9c9d1eec65f6",
	"services/schedule/instance_service_integration_test.go:TestInstance_Start_StaffBridgedOverrideDifferentRoom_Conflict":                            "2d83a4ba290c007a",
	"services/schedule/instance_service_integration_test.go:TestInstance_Start_StaffBridgedOverrideSameRoom_NoConflict":                               "b9c713861c4f01be",
	"services/schedule/instance_service_integration_test.go:TestInstance_Start_StaffBridgedWithoutRosterRow_Conflict":                                 "5286f1bce0e4541f",
	"services/schedule/instance_service_integration_test.go:TestInstance_Start_StaffSameRoomIsNotAConflict":                                           "aa45b32844f37aea",
	"services/schedule/schedule_service_test.go:TestScheduleService_GenerateEvents":                                                                   "0451f56a85a4af4c",
	"services/schedule/staff_schedule_overview_integration_test.go:TestShiftCoverageProjection_BatchesEffectiveSeriesReadsAndIsolatesTenant":          "0a1c8276be6ba593",
	"services/schedule/staff_schedule_overview_integration_test.go:TestStaffScheduleOverview_WeeklySummariesIncludeShiftsOutsideViewport":             "a68708d4989b8269",
	"services/schedule/staff_schedule_overview_integration_test.go:TestStaffScheduleOverview_WeeklySummariesResolveSollAndIsolateTenant":              "c3be2ad8c1434a52",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_MoveConsumesOriginalDateBeforeRematerialization":                   "a508c3c112ce7ffc",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_RepeatedMoveKeepsOriginalOccurrenceIdentity":                       "1f95ca93501a9cb3",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_WeekPatternARespectsCycle":                                         "bb7b0652aa223920",
	"services/schedule/staff_shift_series_rule_edit_test.go:TestStaffShiftSeries_SplitRejectsExtensionWithoutRecurrenceOccurrence":                    "b90dcbfac3f11517",
	"services/schedule/staff_shift_series_rule_edit_test.go:TestStaffShiftSeries_SplitRejectsWhenNextSegmentLeavesNoOccurrence":                       "4e6b08f748ac4487",
	// Source patterns added to the ratchet still need an exact baseline for
	// tests that predate #2571. Keep this list shrink-only.
	"api/active/checkin_test.go:TestAttendance_Fields":                                                                                                                                                                "b11b7e30c6012218",
	"api/birthdays/api_test.go:TestOverviewListsTodaysChildren":                                                                                                                                                       "f4406fe70301fdbf",
	"api/display/api_test.go:TestDisplayDashboardPublic":                                                                                                                                                              "6d22db70c0c13ca7",
	"api/staff/absence_admin_test.go:TestAdminCreateStaffAbsence_CompTimeAllowedForManager":                                                                                                                           "020dc101798d7ce0",
	"api/staff/absence_admin_test.go:TestAdminCreateStaffAbsence_SickCascades":                                                                                                                                        "80e12d76f774bcb2",
	"api/staff/time_tracking_handlers_internal_test.go:TestParseYearQuery_DefaultsToBerlinCalendarYear":                                                                                                               "3df8a0ec5af51d5f",
	"api/students/attendance_history_export_test.go:TestParseAttendanceExportOptions_AcceptsToday":                                                                                                                    "730ea04b8c1730b9",
	"api/students/care_exit_handlers_test.go:TestCareExitHandlers_PreviewThenConfirm":                                                                                                                                 "ac40aecf5e4a15ce",
	"api/students/care_exit_handlers_test.go:TestStudentList_MarksRecordedExitsOnly":                                                                                                                                  "0c725ee12c367bbf",
	"api/students/day_log_handlers_test.go:TestGetStudentsDayLog_AdminSeesStatuses":                                                                                                                                   "32c0a339b972d91f",
	"api/students/day_log_logic_test.go:TestParseDayLogDateRejectsHistoryWithoutDatedGroupAssignments":                                                                                                                "c884355de3cee4f5",
	"api/students/ogs_group_live_test.go:TestOGSGroupLive_AggregatesGroupData":                                                                                                                                        "531280927e9233a7",
	"api/students/status_day_internal_test.go:TestStudentStatusDayHandlers_TodayUpdatesLiveStatusAndClearsOpposite":                                                                                                   "f1fb857a2245be49",
	"api/students/status_day_overview_test.go:TestGetStudentStatusDaysOverview_AdminSeesEntries":                                                                                                                      "6d3a82a9d07a2b39",
	"api/students/update_class_resync_test.go:TestUpdateStudent_ClassChangeResyncsOfferingSourcedTemplates":                                                                                                           "990e7352c86d11b2",
	"api/timetable/instances_create_test.go:TestCreateInstance_DuplicateTemplateBoundReturnsConflict":                                                                                                                 "a1325e6dc0ef3120",
	"api/timetable/templates_series_test.go:TestUpdateTemplate_SeriesRosterFromReachesPredecessor":                                                                                                                    "40b23c8dd647c122",
	"api/timetable/templates_split_test.go:TestTemplateEndHandler_HappyPath":                                                                                                                                          "02ba9a7a52daad89",
	"api/timetable/templates_split_test.go:TestTemplateSplitHandler_UpdateSuccessorPreservesValidFrom":                                                                                                                "c41fa1a385aa1bb2",
	"api/timetable/templates_split_test.go:TestTemplateUpdateHandler_RejectsInconsistentValidityEnvelopeWithoutMutation":                                                                                              "0b803b149687d3ff",
	"api/timetable/templates_test.go:TestListTemplates_CapacityFields":                                                                                                                                                "c44fd7f5feac00dd",
	"database/migrations/001015314_template_source_school_classes_test.go:TestTemplateSourceSchoolClassesDownPreservesSourcedEnrollmentHistory":                                                                       "df4cbfa3e87f7ff1",
	"database/repositories/active/attendance_date_range_test.go:TestAttendanceRepository_FindByStudentAndDateRange":                                                                                                   "b4ca95fc012da622",
	"database/repositories/active/bulk_readers_test.go:TestGroupSupervisorRepository_ListActiveSupervisedRooms":                                                                                                       "0852c58cb6cc2a4b",
	"database/repositories/active/group_supervisor_repository_test.go:TestGroupSupervisorRepository_EndAllActiveByStaffID":                                                                                            "e213033ae0c964a3",
	"database/repositories/active/staff_absence_test.go:TestStaffAbsenceRepository_GetByStaffAndDateRange":                                                                                                            "c77e35466708f448",
	"database/repositories/active/student_status_day_test.go:TestStudentStatusDayRepository_NoteOnReReport":                                                                                                           "824af29ab92e3fd6",
	"database/repositories/active/student_status_day_test.go:TestStudentStatusDayRepository_TenantScope":                                                                                                              "1081c64220c2a178",
	"database/repositories/active/student_status_day_test.go:TestStudentStatusDayRepository_UpsertAndFind":                                                                                                            "c586f4c6e91ad79c",
	"database/repositories/active/work_session_test.go:TestWorkSessionRepository_GetHistoryByStaffID":                                                                                                                 "5a0e0941d3bde4d2",
	"database/repositories/active/work_session_test.go:TestWorkSessionRepository_GetHistoryByStaffIDWrapsDatabaseError":                                                                                               "8181a0695453e014",
	"database/repositories/enrollment/request_child_offering_repository_capacity_range_test.go:TestRequestChildOfferingRepository_CountMaxActiveByCareOfferingInRangeExcludingRequestChild_ExcludesReplacedIntervals": "f6a13e234ee6a730",
	"database/repositories/enrollment/request_child_offering_repository_capacity_range_test.go:TestRequestChildOfferingRepository_CountMaxActiveByCareOfferingInRange_IncludesFutureBookings":                         "7352dd19b6e43e45",
	"database/repositories/enrollment/request_child_offering_repository_grade_levels_test.go:TestRequestChildOfferingRepository_Aggregates_CountEveryPhaseLikeTheGate":                                                "b6c91399534cbe48",
	"database/repositories/enrollment/request_child_offering_repository_grade_levels_test.go:TestRequestChildOfferingRepository_CountMaxActiveByIDsInRange_GuardsItsInput":                                            "3d1582e3a5c05e3f",
	"database/repositories/enrollment/request_child_offering_repository_grade_levels_test.go:TestRequestChildOfferingRepository_CountMaxActiveByIDsInRange_MatchesTheSingleOfferingVariant":                           "ae76b0d4e18443cf",
	"database/repositories/enrollment/request_child_offering_repository_grade_levels_test.go:TestRequestChildOfferingRepository_CountMaxActiveByIDsInRange_SeparatesOfferings":                                        "4a552e1402cd093a",
	"database/repositories/feedback/entry_repository_test.go:TestEntryRepository_FindByDateRange":                                                                                                                     "00467ea576fdbe3e",
	"database/repositories/feedback/entry_repository_test.go:TestEntryRepository_FindByStudentAndDateRange":                                                                                                           "0967f33f47216e64",
	"database/repositories/schedule/staff_shift_repo_test.go:TestStaffShiftRepository_DeleteUpcomingByStaffID":                                                                                                        "7309f81ba3002bbf",
	"database/repositories/users/care_withdrawal_completion_test.go:TestCareWithdrawalCompletionRepository_ParticipationBoundaryUsesPendingCompletionWhenEnrollmentIsOpen":                                            "6dd231af1db3d660",
	"database/repositories/users/care_withdrawal_completion_test.go:TestCareWithdrawalCompletionRepository_UpsertUsesIncomingBoundary":                                                                                "fc4851ef877e21a5",
	"services/absence/excused_request_service_test.go:TestDecide_ApproveRefusedWhenPartialAbsenceExists":                                                                                                              "227beed8159b256a",
	"services/active/attendance_service_test.go:TestGetStudentAttendanceStatus_NotCheckedIn":                                                                                                                          "738ff6b4d9711512",
	"services/active/cleanup_service_test.go:TestCleanupStaleAttendance_CheckOutTimeIsBerlinEndOfDay":                                                                                                                 "be41ddbfda3d7078",
	"services/active/cleanup_supervisors_test.go:TestCleanupStaleSupervisors_ClosesYesterdayRecords":                                                                                                                  "ceb0098b35d0bb16",
	"services/active/staff_absence_service_test.go:TestAbsCreateAbsenceFor_RejectsCompTimeAgainstLaterLedgerCapacity":                                                                                                 "6bd645a1a9d357f2",
	"services/active/staff_opening_balance_mock_test.go:TestStaffBalanceAdjustmentService_OpeningAllowsNegativeTarget":                                                                                                "48a0a4247e7738d3",
	"services/active/student_status_day_write_bulk_test.go:TestBulkCreateForDates_RejectsConflictWithoutPartialWrites":                                                                                                "feb9901b9750e067",
	"services/active/student_status_day_write_bulk_test.go:TestCreateForDates_RejectsConflictWithoutPartialWrites":                                                                                                    "8463b13752b1b56c",
	"services/active/work_session_autocheckout_mock_test.go:TestAutoCheckout_QueriesOpenSessionsIncludingToday":                                                                                                       "2b8f9c44c916ca19",
	"services/active/work_session_service_test.go:TestWSApplyCustomScheduleRows_StampsAnchorForFirstRotation":                                                                                                         "b21c9894e8279b4a",
	"services/education/grade_transition_offering_resync_test.go:TestGradeTransitionService_ApplyAndRevert_ResyncOfferingSourcedRosters":                                                                              "3a1bece79d1187f4",
	"services/education/grade_transition_roster_reconcile_test.go:TestGradeTransitionService_Revert_FillsBackdatedInstance":                                                                                           "213fc0104129d2de",
	"services/education/grade_transition_roster_reconcile_test.go:TestGradeTransitionService_Revert_FillsTodaysInstanceMaterializedWhileAlumnus":                                                                      "db86e45db3d70235",
	"services/enrollment/care_offering_booking_stats_internal_test.go:TestBookingStatsWindow_DefaultsToTodayWithoutPhaseDates":                                                                                        "fa781731aacf21a2",
	"services/enrollment/care_offering_booking_stats_internal_test.go:TestListBookingStats_CountsInTheCapacityGatesWindow":                                                                                            "745cd9b5e759a948",
	"services/enrollment/class_roster_care_end_test.go:TestClassRosterFiltersCareDate":                                                                                                                                "52c2cdabeeb329e6",
	"services/enrollment/decision_service_test.go:TestDecisionService_Decide_ApprovedScheduledPastStartActivatesStudent":                                                                                              "cb24345738014403",
	"services/enrollment/decision_service_test.go:TestDecisionService_ListChildOfferings_CarriesAttributesAndFutureBookings":                                                                                          "af7d93b13a06f2db",
	"services/enrollment/offering_change_request_service_test.go:TestOfferingChangeRequestService_ListPending_ReportsDateClampedToThePhaseStart":                                                                      "7d0fa0b335413f01",
	"services/enrollment/offering_change_request_service_test.go:TestOfferingChangeRequestService_ListPending_ReportsTheSelectableDateRange":                                                                          "46da4c77f044746d",
	"services/enrollment/offering_source_service_test.go:TestDecide_MultiSourceFanOutSeedsFromPhaseStart":                                                                                                             "077dda01f1afa968",
	"services/enrollment/offering_source_service_test.go:TestListOfferingSourceOptions_CountsScopedToSelectedPeriod":                                                                                                  "2ba44464bc5daedb",
	"services/enrollment/offering_source_service_test.go:TestUpdateChildOfferings_UndatedCorrectionKeepsPhaseStartOnMultiSource":                                                                                      "64f334dfcf46c274",
	"services/enrollment/report_service_test.go:TestCareUsageEnrichesGuardiansAndSchedulePickup":                                                                                                                      "d22ccb8d30c36a6a",
	"services/enrollment/report_service_test.go:TestClassRosterUsesOfferingDateForPickupProjection":                                                                                                                   "7d5c7bf791b25339",
	"services/feedback/feedback_service_test.go:TestFeedbackService_GetEntriesByDateRange":                                                                                                                            "993d5ec48c3688db",
	"services/feedback/feedback_service_test.go:TestFeedbackService_GetEntriesByStudentAndDateRange":                                                                                                                  "2684d17203f7ecda",
	"services/parent/care_exception_service_test.go:TestListCareExceptions_FlagsAbsentArrivalRow":                                                                                                                     "8eb8070542f490a3",
	"services/parent/care_exception_service_test.go:TestListCareExceptions_FlagsAbsentPickupRow":                                                                                                                      "a5cebc5a3dc2da8b",
	"services/parent/excused_request_test.go:TestExcusedRequest_ApproveWritesStatusDays":                                                                                                                              "d7764692655a4f21",
	"services/parent/parent_care_offerings_service_test.go:TestGetChildCareOfferingsReturnsCompleteSortedView":                                                                                                        "d069d6ac783cc0bc",
	"services/parent/parent_meal_plan_test.go:TestMealPlanWeek_AllowsNextWeek":                                                                                                                                        "fb81d013217615f9",
	"services/parent/parent_meal_plan_test.go:TestMealPlanWeek_DisabledReturnsSentinel":                                                                                                                               "277fb2578c2eae0a",
	"services/parent/parent_meal_plan_test.go:TestMealPlanWeek_FarFutureWeekOutOfRange":                                                                                                                               "24a82ea967ed1840",
	"services/parent/parent_meal_plan_test.go:TestMealPlanWeek_NotOwnedChildRejected":                                                                                                                                 "2cb7cd5023dd7309",
	"services/parent/parent_meal_plan_test.go:TestMealPlanWeek_PastWeekOutOfRange":                                                                                                                                    "fe124829903ec21b",
	"services/parent/parent_meal_plan_test.go:TestMealPlanWeek_ReturnsCurrentWeekEntries":                                                                                                                             "8b2c3c51e4637167",
	"services/parent/parent_meal_plan_test.go:TestMealPlanWeek_SettingErrorPropagates":                                                                                                                                "f07ff6198d3f5077",
	"services/parent/parent_request_edit_test.go:TestEditExcusedRequestReplacesWithdrawal":                                                                                                                            "95b378db3e7d9684",
	"services/parent/parent_write_service_test.go:TestListSickDays_ExcludesStaffCreatedExcused":                                                                                                                       "2b2e2b717e11546c",
	"services/parent/parent_write_service_test.go:TestListSickDays_ReturnsSickAndExcused":                                                                                                                             "15f1f1867e4336a7",
	"services/parent/parent_write_service_test.go:TestSubmitSickNote_ClearsClassTripForSubmittedDate":                                                                                                                 "9fe29ebdf9248f25",
	"services/parent/parent_write_service_test.go:TestSubmitSickNote_FutureWriteSerializesWithStaffConflictCheck":                                                                                                     "aa0d1831e55c0867",
	"services/parent/parent_write_service_test.go:TestSubmitSickNote_NonContiguousExcludesUnrelatedRows":                                                                                                              "fd669739343c4656",
	"services/parent/parent_write_service_test.go:TestSubmitSickNote_RefusesPartialAbsenceConflict":                                                                                                                   "ccf4f1814a00a845",
	"services/parent/sick_note_gate_pin_test.go:TestSickNoteStaysImmediateWhenApprovalDisabled":                                                                                                                       "3a31eb999cc50163",
	"services/schedule/partial_absence_pending_request_test.go:TestPartialAbsenceCreate_RefusesPendingFullDayRequest":                                                                                                 "a1300c5395d3b5d3",
	"services/schedule/shift_plan_sync_service_test.go:TestSickCascade_ClearSickForRange":                                                                                                                             "41a3640aa4b1afb5",
	"services/schedule/shift_plan_sync_service_test.go:TestSickCascade_ConcurrentOverlappingReportsSerializeBeforeOverlapRead":                                                                                        "abb397744d316fff",
	"services/schedule/shift_plan_sync_service_test.go:TestSickCascade_UpdateRangeRollsBackWhenRemovedShiftCannotReactivate":                                                                                          "33f8c27f2bb9c900",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffScheduleOverview_SeriesFieldsRideExistingReads":                                                                                                "8cd5147feb9cfb19",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_CapAllByStaffIDClampsFutureSeries":                                                                                                 "eb2dfd94b876c342",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_CollisionSkipsAndReports":                                                                                                          "24c0385ed7ea2ad6",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_CreateMaterializesFromTomorrow":                                                                                                    "20467e00fb4e0b40",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_CreateRejectsBadReferences":                                                                                                        "c7f9512d0935a14b",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_EditDetachesAndDeleteRecordsException":                                                                                             "3403ecad121dac3a",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_EndAtFirstOccurrence":                                                                                                              "019885575469ce4a",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_EndSeriesKeepsDetachedAndPast":                                                                                                     "b9471d98f9c8db57",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_SplitAtFirstOccurrence":                                                                                                            "f11653cd08a2177f",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_SplitOutsideSegmentRejected":                                                                                                       "beb3ef38fcd96da6",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_SplitPreservesDeviationsOnSuccessor":                                                                                               "9e2b0eb1391e972d",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_SplitTodayUpdatesOccurrenceAndReplansTomorrow":                                                                                     "fc3cb9123dfbf8bb",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_WeekPatternRequiresCycle":                                                                                                          "956f83ec988e0a5e",
	"services/schedule/staff_shift_series_mock_test.go:TestEndSeriesUnit_ErrorBranches":                                                                                                                               "38348459e63998f2",
	"services/schedule/staff_shift_series_mock_test.go:TestGetSeriesUnit":                                                                                                                                             "5d8892d6463f6387",
	"services/schedule/staff_shift_series_mock_test.go:TestSplitSeriesUnit_ErrorBranches":                                                                                                                             "ccb0fa5544832e2e",
	"services/schedule/staff_shift_series_rule_edit_test.go:TestStaffShiftSeries_SplitAppliesNewWeekdaysFromEffectiveDate":                                                                                            "f750a1f2f158799d",
	"services/schedule/staff_shift_series_rule_edit_test.go:TestStaffShiftSeries_SplitBoundsEarlierSegmentAtNextSuccessor":                                                                                            "3f1a843a2f4ae6ef",
	"services/schedule/staff_shift_series_rule_edit_test.go:TestStaffShiftSeries_SplitExtendsSeriesEndingToday":                                                                                                       "7b118770a987683b",
	"services/schedule/staff_shift_series_rule_edit_test.go:TestStaffShiftSeries_SplitKeepsStoredValidityWhenUnset":                                                                                                   "f0ffd9af65ebdbc7",
	"services/schedule/staff_shift_series_rule_edit_test.go:TestStaffShiftSeries_SplitRejectsSupersededSegment":                                                                                                       "374a518f02fed0c6",
	"services/schedule/staff_shift_series_rule_edit_test.go:TestStaffShiftSeries_SplitRejectsValidityBeyondCalendarPeriod":                                                                                            "02841c825abfe88b",
	"services/schedule/staff_shift_series_rule_edit_test.go:TestStaffShiftSeries_SplitRejectsWhenNoOccurrenceRemains":                                                                                                 "e470a9b5245d4121",
	"services/schedule/staff_shift_series_rule_edit_test.go:TestStaffShiftSeries_SplitShortensValidityAndDropsLaterShifts":                                                                                            "dfed68ce3f023773",
	"services/schedule/template_end_service_unit_test.go:TestTemplateEndFromDate_ReturnsSummaryAndDeletesOpenEndedWindow":                                                                                             "0445a5edd8b9d3de",
	"services/schedule/template_offering_source_unit_test.go:TestResyncUpdatedTemplateOfferingRoster":                                                                                                                 "848625e599f9f550",
	"services/schedule/template_series_roster_mock_test.go:TestReconcileSeriesPredecessorRoster_CreatesBoundedRows":                                                                                                   "ed40a72a0ac0152d",
	"services/users/care_booking_authority_integration_test.go:TestBookingMutationPlansFutureNaturalEndImmediately":                                                                                                   "b297698516008cdc",
	"services/users/care_booking_authority_integration_test.go:TestOverdueRebookingReplacesTheStaleCompletion":                                                                                                        "f6b2337c7fd37cc5",
	"services/users/care_lifecycle_service_test.go:TestCareLifecycle_CancelPutsThePlanBack":                                                                                                                           "ae8314c40ca6d231",
	"services/users/care_lifecycle_service_test.go:TestCareLifecycle_CancelRefusesOrdinaryEnrollmentEnd":                                                                                                              "ee56fe3c7762ac1c",
	"services/users/care_lifecycle_service_test.go:TestCareLifecycle_CancelRestoresPreviousEnrollmentEnd":                                                                                                             "09ef28ed8288a87f",
	"services/users/care_lifecycle_service_test.go:TestCareLifecycle_LastCareDayIsInclusive":                                                                                                                          "f4ebd95907f4146a",
	"services/users/care_lifecycle_service_test.go:TestCareLifecycle_Resume":                                                                                                                                          "4654cb76a94d674c",
	"services/users/care_withdrawal_lifecycle_test.go:TestCareWithdrawalLifecycle_AllowsRetroactiveExitButNotBeforeAttendance":                                                                                        "6700e799b5a0673e",
	"services/users/care_withdrawal_lifecycle_test.go:TestCareWithdrawalLifecycle_CancellingPlannedExitRestoresTask":                                                                                                  "2b3225a4871b85b8",
	"services/users/person_service_eligibility_test.go:TestFilterStudentsEligibleOnDate_IncludesImmediatelyActiveFutureStudentToday":                                                                                  "12cb16cbe99da7ff",
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
	other := GetHistory(NewWorkSession(fixtureNow()), from, to)
	_ = other.WeeklySummaries
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
