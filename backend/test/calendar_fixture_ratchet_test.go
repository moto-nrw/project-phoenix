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
	"database/repositories/active/work_session_test.go:TestWorkSessionRepository_ListOverlappingByStaffID_KeepsEarlierStarts":                                                             "265f38a640646464",
	"database/repositories/education/group_substitution_repository_test.go:TestGroupSubstitutionRepository_FindOverlapping":                                                               "1c119c4171de4af3",
	"database/repositories/enrollment/request_child_offering_repository_grade_levels_test.go:TestRequestChildOfferingRepository_CountActiveGradeLevels_CountsAChildOncePerOffering":       "63fac136aa98ff6c",
	"database/repositories/enrollment/request_child_offering_repository_grade_levels_test.go:TestRequestChildOfferingRepository_CountActiveGradeLevels_EmptyInputSkipsTheQuery":           "d73d1f1f6d558252",
	"database/repositories/enrollment/request_child_offering_repository_grade_levels_test.go:TestRequestChildOfferingRepository_CountActiveGradeLevels_ExcludesIntervalsOutsideTheWindow": "ce3b863c70f72db5",
	"database/repositories/enrollment/request_child_offering_repository_grade_levels_test.go:TestRequestChildOfferingRepository_CountActiveGradeLevels_ExcludesTerminalChildren":          "03ef104219b546a8",
	"database/repositories/enrollment/request_child_offering_repository_grade_levels_test.go:TestRequestChildOfferingRepository_CountActiveGradeLevels_GroupsByGrade":                     "454b5499045e72a5",
	"database/repositories/enrollment/request_child_offering_repository_grade_levels_test.go:TestRequestChildOfferingRepository_CountActiveGradeLevels_RejectsAnEmptyWindow":              "71afd64ffad60356",
	"database/repositories/enrollment/request_child_offering_repository_grade_levels_test.go:TestRequestChildOfferingRepository_CountActiveGradeLevels_ReportsMissingGradeSeparately":     "42050203e5a34bdd",
	"services/active/staff_vacation_opening_db_test.go:TestSetVacationOpening_NegativeRemainingAllowsOverdrawnAccount":                                                                    "15bda522d7245caa",
	"services/active/staff_vacation_opening_db_test.go:TestSetVacationOpening_RejectsOpenCutoff":                                                                                          "6d5df6c5bf2dd6f2",
	"services/active/staff_vacation_opening_db_test.go:TestSetVacationOpening_RejectsPastVacationYear":                                                                                    "b852b2e168131fbf",
	"services/active/staff_vacation_opening_db_test.go:TestSetVacationOpening_RejectsSecondOpeningForSameYear":                                                                            "861ccb82eaf6f6d7",
	"services/active/staff_vacation_opening_db_test.go:TestSetVacationOpening_RejectsVacationAbsencesBeforeCutoff":                                                                        "3d545161f66e3494",
	"services/active/staff_vacation_opening_db_test.go:TestSetVacationOpening_RespectsCustomQuota":                                                                                        "f4d11d559c445bd1",
	"services/calendar/service_integration_test.go:TestCalendarServiceIntegration_DeleteFeedVisibleLeavesTombstone":                                                                       "c4fdfeb251888c29",
	"services/education/education_service_test.go:TestUpdateSubstitution_DateValidation":                                                                                                  "01c823ab61ee91c1",
	"services/enrollment/offering_change_request_automatic_test.go:TestOfferingChangeRequestService_Decide_ExclusionKeepsManualAndRequiredLunchDays":                                      "e9ad2e2ff8240502",
	"services/enrollment/offering_change_request_automatic_test.go:TestOfferingChangeRequestService_Decide_SnapshotMatchesGrandfatheredAutomaticBooking":                                  "c412a2ebdb617246",
	"services/enrollment/offering_change_request_service_test.go:TestOfferingChangeRequestService_Create_PayloadExcludesAutomaticOfferings":                                               "8d9bd1a0ac4cd8c5",
	"services/enrollment/offering_change_request_service_test.go:TestOfferingChangeRequestService_Decide_LogsNoWishWhenTheDateIsKept":                                                     "0c42fdb20541774e",
	"services/parent/care_exception_service_test.go:TestDeleteCareExceptionPreservesArrival":                                                                                              "51015b4c59f461f3",
	"services/parent/care_exception_service_test.go:TestDeleteCareException_RemovesPickupAndPreservesArrival":                                                                             "b15fb9d04c557748",
	"services/parent/care_exception_service_test.go:TestListCareExceptions_ArrivalRepoErrorSurfaces":                                                                                      "baf2d1ddb67542ed",
	"services/parent/care_exception_service_test.go:TestListCareExceptions_NotOwnedChild":                                                                                                 "48142d4798179204",
	"services/parent/care_exception_service_test.go:TestListCareExceptions_RepoErrorSurfaces":                                                                                             "ff67c4b8ad34062b",
	"services/parent/care_exception_service_test.go:TestSubmitCareException_ClearingLegRemovesIt":                                                                                         "fcdad50ca1769681",
	"services/parent/excused_request_test.go:TestSickRequest_ApproveWritesSickStatusAndLiveFlag":                                                                                          "590f9a0b1a75d891",
	"services/parent/parent_care_schedule_service_test.go:TestGetChildCareSchedule_TodayAbsentReflectsStatusDay":                                                                          "3b81e479e9ebe4ec",
	"services/parent/parent_write_service_test.go:TestListSickDays_AllowsPortalAccessWithoutWritePermissions":                                                                             "00b0c557f5cd8683",
	"services/parent/parent_write_service_test.go:TestListSickDays_HidesAnotherGuardiansReason":                                                                                           "8883183b885b60c1",
	"services/parent/parent_write_service_test.go:TestListSickDays_NotOwned":                                                                                                              "225641a4a1a37659",
	"services/schedule/staff_schedule_overview_integration_test.go:TestStaffScheduleOverview_TenantIsolationAcrossEveryProjectionRead":                                                    "5ef8ebe141d07a8a",
	"api/timetable/substitutions_bulk_test.go:TestBulkSubstitution_MultiDayWithSubstitute":                                                                                                "ea999daada850274",
	"services/enrollment/offering_change_full_withdrawal_test.go:TestOfferingChangeRequestService_ListPending_KeepsUntouchedBookingsOutOfTheWarning":                                      "d3b7eeb966eda060",
	"services/enrollment/offering_change_history_test.go:TestOfferingChangeRequestService_ListHistory":                                                                                    "88530dab48130409",
	"services/enrollment/offering_change_request_automatic_test.go:TestOfferingChangeRequestService_Decide_ExclusionSkipsAutoTargetAndRecordsOverride":                                    "35045063ab6808c5",
	"services/enrollment/offering_change_request_automatic_test.go:TestOfferingChangeRequestService_Decide_RejectionFallsBackToPayloadSnapshot":                                           "8c58b5d5f16504ec",
	"services/enrollment/offering_change_request_automatic_test.go:TestOfferingChangeRequestService_Decide_RejectionFreezesDiffSnapshot":                                                  "c5fa08693757e496",
	"services/enrollment/offering_change_request_automatic_test.go:TestOfferingChangeRequestService_GetForStudent_MarksAutomaticDiffEntries":                                              "321beb09c172b692",
	"services/enrollment/offering_change_request_automatic_test.go:TestOfferingChangeRequestService_ListPending_IncludesUnchangedGrandfatheredRuleTarget":                                 "93e62a5c4c524796",
	"services/enrollment/offering_change_request_automatic_test.go:TestOfferingChangeRequestService_ListPending_MarksAutomaticDiffEntries":                                                "281a22f9532e18dc",
	"services/enrollment/request_child_offering_repository_date_test.go:TestRequestChildOfferingRepository_ListAtDates_DoesNotReturnHistoricalSelection":                                  "596a1ea5e50fe739",
	"services/schedule/template_end_cascade_integration_test.go:TestTemplateEnd_CascadesToCappedPredecessors":                                                                             "cdca55aff27c43aa",
	"services/schedule/template_end_cascade_integration_test.go:TestTemplateEnd_FromCappedPredecessorAlsoEndsLivingSuccessor":                                                             "007c8d14d5fbce5e",
	"services/schedule/template_offering_source_integration_test.go:TestTemplateOfferingSource_PullForwardWidensSourcedRoster":                                                            "c0e65fd71ee3c1c6",
	"services/schedule/template_offering_source_integration_test.go:TestTemplateOfferingSource_SplitAwayFromAngebotClearsSourcedRoster":                                                   "5cc64de7e7dd30f7",
	"services/schedule/template_series_roster_integration_test.go:TestSeriesRoster_AddsChildAcrossCappedPredecessor":                                                                      "9b8f91e8d08eb4eb",
	"services/schedule/template_series_roster_integration_test.go:TestSeriesRoster_KeepsPredecessorOnlyChildOutsideScope":                                                                 "da7c01e18f9bfdfa",
	"services/schedule/template_series_roster_integration_test.go:TestSeriesRoster_NarrowedSuccessorLeavesTheOtherWeekdayIntact":                                                          "482af77df1b2fad7",
	"services/schedule/template_series_roster_integration_test.go:TestSeriesRoster_NoopWithoutSeriesRosterFrom":                                                                           "c8810ada1e42cf1e",
	"services/schedule/template_series_roster_integration_test.go:TestSeriesRoster_PastAnchorClampsToTodayAndSegmentStart":                                                                "17bac424b4631146",
	"services/schedule/template_series_roster_integration_test.go:TestSeriesRoster_PreservesHandRemovedChildOnPredecessorOccurrence":                                                      "f64789cf03252989",
	"services/schedule/template_series_roster_integration_test.go:TestSeriesRoster_PrimaryChangeReachesMaterializedOccurrences":                                                           "53db4391f333dcf0",
	"services/schedule/template_series_roster_integration_test.go:TestSeriesRoster_ReachesTheClickedWeekdayTheSuccessorNoLongerRuns":                                                      "813f199e09d3dc67",
	"services/schedule/template_series_roster_integration_test.go:TestSeriesRoster_ReconcilesSupervisorsAcrossPredecessor":                                                                "614b7b8457c7bc84",
	"services/schedule/template_series_roster_integration_test.go:TestSeriesRoster_RemovedStaffLosesPlannedOccurrenceRows":                                                                "38889e5bf2f8c934",
	"services/schedule/template_series_roster_integration_test.go:TestSeriesRoster_RemovesChildAcrossCappedPredecessor":                                                                   "4d70ff7055d7545a",
	"services/schedule/template_series_roster_integration_test.go:TestSeriesRoster_SecondIdenticalSaveKeepsRowsUntouched":                                                                 "36ecfb5792ee7027",
	"services/schedule/template_series_roster_integration_test.go:TestSeriesRoster_SkipsProtectedPredecessorEnrollments":                                                                  "a4a136af0c40db7d",
	"services/schedule/template_series_roster_integration_test.go:TestSeriesRoster_WeekdayScopedAddOnlyTouchesThatWeekday":                                                                "f4652526281c847d",
	"services/schedule/template_series_roster_integration_test.go:TestSeriesRoster_WeekdayScopedEditLeavesOtherWeekdaysAlone":                                                             "f538673be4953496",
	"services/schedule/template_source_class_filter_integration_test.go:TestTemplateSourceClassFilter_HonoursBookedWeekdays":                                                              "e57b32b94e31ae68",
	"api/school/school_supervisions_test.go:TestSchoolSupervisionsFollowTheAssignment":                                                                                                    "a018a778e133d74c",
	"api/students/attendance_history_handlers_test.go:TestGetStudentAttendanceHistory_FutureEndClampsToToday":                                                                             "fdde9edfa31177f5",
	"api/students/change_request_list_handlers_test.go:TestAggregatedChangeRequests_EmitsCurrentStatusPerDate":                                                                            "ee40f4c85bb7f415",
	"api/students/change_request_list_handlers_test.go:TestAggregatedChangeRequests_GroupsContradictingAbsencesOnOneDay":                                                                  "59db00db10558bbc",
	"api/students/status_day_internal_test.go:TestStaffAbsenceNotificationCallbacks":                                                                                                      "ea2b6def1e60b646",
	"api/timetable/exception_conflicts_test.go:TestExceptionConflicts_CancelledInstance_EmitsWarningPerExpectedStudent":                                                                   "ecc5f20534936e60",
	"api/timetable/exception_conflicts_test.go:TestExceptionConflicts_Empty_NoExceptions":                                                                                                 "0db0aaa36e8290e4",
	"api/timetable/gaps_test.go:TestGaps_Empty":                                                                                                       "d91746d30b223ea6",
	"api/timetable/instances_list_test.go:TestListInstances_Empty":                                                                                    "6851d91d21a51053",
	"api/timetable/templates_series_test.go:TestGetTemplate_ResolvesCappedPredecessorToLivingSuccessor":                                               "7951b6bb5c29c3f7",
	"api/timetable/templates_start_pull_test.go:TestTemplateUpdateStartDatePullForward":                                                               "dde5b1f9fd91e210",
	"database/repositories/feedback/entry_repository_test.go:TestEntryRepository_List":                                                                "490c95a0e89108ca",
	"database/repositories/schedule/arrival_schedule_test.go:TestStudentArrivalExceptionRepository_DeletePastExceptions":                              "18faff796d2617ce",
	"database/repositories/schedule/arrival_schedule_test.go:TestStudentArrivalNoteRepository_DeletePastNotes":                                        "23cae5cbe7f896fb",
	"database/repositories/schedule/pickup_schedule_test.go:TestStudentPickupExceptionRepository_DeletePastExceptions":                                "520465af8c6c2547",
	"database/repositories/schedule/pickup_schedule_test.go:TestStudentPickupNoteRepository_DeletePastNotes":                                          "3acdf33bb38552d2",
	"services/absence/excused_request_errorpath_test.go:TestDecide_ApprovalNotifiesAfterCommit":                                                       "6b044473ec2e99bf",
	"services/active/staff_vacation_opening_db_test.go:TestDeleteVacationOpening_WritesTombstoneAndRestoresSummary":                                   "34aa475d62ad80c8",
	"services/active/staff_vacation_opening_db_test.go:TestSetVacationOpening_AllowsVacationBeginningOnWeekendBeforeCutoff":                           "d6a3871b9bc444e6",
	"services/active/staff_vacation_opening_db_test.go:TestSetVacationOpening_DerivesTakenBeforeFromQuota":                                            "b6121ebd586aaced",
	"services/active/staff_vacation_opening_db_test.go:TestVacationOpeningRepository_BatchAndListReads":                                               "392799936402a26b",
	"services/enrollment/offering_adjustment_dated_test.go:TestDecisionService_UpdateChildOfferings_DatedSwitchBeforePhaseStartDropsUnstartedRow":     "f44c49efdfb59665",
	"services/enrollment/offering_change_request_service_test.go:TestOfferingChangeRequestService_Catalog_MarksCurrentBookingAndCapacity":             "e6b97c374d374f39",
	"services/enrollment/offering_change_request_service_test.go:TestOfferingChangeRequestService_Create_StoresPendingRequest":                        "e3fc39310041e6df",
	"services/enrollment/offering_change_request_service_test.go:TestOfferingChangeRequestService_Create_StripsChangedCurrentAutomaticOffering":       "2e5c9b833c46d706",
	"services/enrollment/offering_change_request_service_test.go:TestOfferingChangeRequestService_Decide_AppliesTheConfirmedDate":                     "c0d86b162caf131e",
	"services/enrollment/offering_change_request_service_test.go:TestOfferingChangeRequestService_Decide_ApprovalAppliesTheDatedSwitch":               "4e1e31e7d75e81a6",
	"services/enrollment/offering_change_request_service_test.go:TestOfferingChangeRequestService_Decide_CapsRebookingAtPlannedCareEnd":               "db4b45af70892e31",
	"services/enrollment/offering_change_request_service_test.go:TestOfferingChangeRequestService_Decide_LogsTheDateTheFamilyAskedFor":                "e09c95bc29c9103a",
	"services/enrollment/offering_change_request_service_test.go:TestOfferingChangeRequestService_Decide_RejectionNeedsAReasonAndChangesNothing":      "f546770a5227a9e4",
	"services/enrollment/offering_change_request_service_test.go:TestOfferingChangeRequestService_GetForStudent_ReportsRecentDecision":                "b9ac9787f508bcb8",
	"services/enrollment/offering_change_request_service_test.go:TestOfferingChangeRequestService_PreviewDecision_ReportsOnlyUncoveredManualPlanning": "88743f196fd03e43",
	"services/enrollment/pickup_adjustment_service_test.go:TestPickupAdjustmentAppliesArrivalSchedulesOnlyForImmediateExceptions":                     "8a3f9afa1b72b8ab",
	"services/import/student_import_config_test.go:TestEnrollmentStartsInFuture_UsesBusinessDate":                                                     "68877e59248fbcfd",
	"services/parent/care_ended_child_test.go:TestParentPortal_CareEndedChildIsReadOnly":                                                              "d7ad935d37c624fa",
	"services/parent/care_exception_service_test.go:TestListCareExceptions_MergesBothLegsAndFlagsStaffSource":                                         "0a5f1344d4809272",
	"services/schedule/bulk_substitution_unit_test.go:TestNormalizeBulkDates_DedupesAndSortsAscending":                                                "e5af2bff17941726",
	"services/schedule/calendar_period_integration_test.go:TestCalendarPeriodService_ConcurrentBootstrapVsCreate":                                     "38529821f1618e53",
	"services/schedule/calendar_period_integration_test.go:TestCalendarPeriodService_EnsureDefaultSchoolYear":                                         "d8fa8e4d5c0b13df",
	"services/schedule/care_request_history_test.go:TestListHistory_IncludesPickupChangeWithPayloadSummary":                                           "2b1655136a260704",
	"services/schedule/pickup_auto_excusal_test.go:TestAutoExcusal_BulkWeeklyUpsertResyncsExceptions":                                                 "1e0394f321aa19d1",
	"services/schedule/pickup_auto_excusal_test.go:TestAutoExcusal_DeletingTheExceptionRestoresBlocks":                                                "196d48496a49dfe3",
	"services/schedule/pickup_auto_excusal_test.go:TestAutoExcusal_FullDayStatusCoexistsAndReleaseReplays":                                            "1ebca6e1dd1872f4",
	"services/schedule/pickup_auto_excusal_test.go:TestAutoExcusal_LaterThanBaselineMeansNoCoupling":                                                  "f025b160cd8b6f29",
	"services/schedule/pickup_auto_excusal_test.go:TestAutoExcusal_ManualCreateConvertsAutoToManual":                                                  "936b21e27d1ed70d",
	"services/schedule/pickup_auto_excusal_test.go:TestAutoExcusal_ManualDeleteOfConvertedRowRederivesAuto":                                           "9b40091615f8dbc0",
	"services/schedule/pickup_auto_excusal_test.go:TestAutoExcusal_ManualDeleteRefusesAutoRows":                                                       "72c0f711921e1c57",
	"services/schedule/pickup_auto_excusal_test.go:TestAutoExcusal_ManualPartialAbsenceIsNeverTouched":                                                "0d2b138a0815cf77",
	"services/schedule/pickup_auto_excusal_test.go:TestAutoExcusal_MovingPickupBackReleasesBlocks":                                                    "1d11898a8b96da9e",
	"services/schedule/pickup_auto_excusal_test.go:TestAutoExcusal_MovingPickupEarlierWidensTheExcusal":                                               "5ca015115ff01b1b",
	"services/schedule/pickup_auto_excusal_test.go:TestAutoExcusal_NoWeeklyBaselineMeansNoCoupling":                                                   "549ef33138e58f6b",
	"services/schedule/pickup_auto_excusal_test.go:TestAutoExcusal_PulledForwardPickupExcusesLaterBlocks":                                             "577be6673b45d2da",
	"services/schedule/pickup_auto_excusal_test.go:TestAutoExcusal_WeeklyBaselineAddedCouplesExistingException":                                       "64a8f195b2396253",
	"services/schedule/pickup_auto_excusal_test.go:TestAutoExcusal_WeeklyBaselineDeletedReleasesCoupling":                                             "130c5306b1a1d8af",
	"services/schedule/pickup_auto_excusal_test.go:TestAutoExcusal_WeeklyBaselineMovedEarlierReleasesCoupling":                                        "dc4ce410bdde658a",
	"services/schedule/shift_plan_sync_service_test.go:TestSickCascade_HalfDayRules":                                                                  "58ec55626a84ccce",
	"services/schedule/shift_plan_sync_service_test.go:TestSickCascade_PastShiftsRemainHistoricalDuringMarkAndReconcile":                              "5b7c5239e437c614",
	"services/schedule/template_source_class_filter_integration_test.go:TestTemplateSourceClassFilter_ClassChangeMovesTheChild":                       "64a125cca7e791b4",
	"services/schedule/template_source_class_filter_integration_test.go:TestTemplateSourceClassFilter_DeregistrationLimitsTheAssignment":              "bc86a31b8ed882c4",
	"services/schedule/template_source_class_filter_integration_test.go:TestTemplateSourceClassFilter_LaterApprovalJoinsTheTermin":                    "3fb9d95cd0fdb3ce",
	"services/schedule/template_source_class_filter_integration_test.go:TestTemplateSourceClassFilter_MatchesCaseInsensitively":                       "4efd98e711f33416",
	"services/schedule/template_source_class_filter_integration_test.go:TestTemplateSourceClassFilter_OfferingDayChangeReshapesTheRoster":             "2948229824bfb02b",
	"services/schedule/template_source_class_filter_integration_test.go:TestTemplateSourceClassFilter_SeedsOnlyTheFilteredClass":                      "8fba21fc7946d57a",
	"services/schedule/template_source_class_filter_integration_test.go:TestTemplateSourceClassFilter_UpdateSwitchesFromGradeToClass":                 "012c261f75152fef",
	"services/schedule/template_split_service_test.go:TestTemplateEndFromDate_CapsTemplateAndProtectsHistory":                                         "a3a10fdf4e5fbf33",
	"services/schedule/template_split_service_test.go:TestTemplateEnd_ConcurrentTemplateUpdatePreservesCommittedCap":                                  "40e2b97667953d8e",
	"services/schedule/template_split_service_test.go:TestTemplateMutations_RejectCareOfferingSeriesConflictsWithoutPersisting":                       "a7ad30ec4a9d8f21",
	"services/schedule/template_split_service_test.go:TestTemplateSplitAndEnd_RespectCurrentSegmentEnvelope":                                          "7bc77749fb861fb6",
	"services/schedule/template_split_service_test.go:TestTemplateSplit_ExplicitRosterAndWeekPattern":                                                 "8dc5a4dafa522772",
	"services/schedule/template_split_service_test.go:TestTemplateSplit_HappyPath_CarriesRosterAndProtectsHistory":                                    "7627aaa51878c1b2",
	"services/schedule/template_split_service_test.go:TestTemplateSplit_RejectsResplittingBoundedPredecessor":                                         "4cda6258dbc9af3c",
	"services/schedule/template_split_service_test.go:TestTemplateSplit_SingleEditThenSuccessorUpdateDoesNotDuplicate":                                "8dfa3a436358d671",
	"services/schedule/template_split_service_test.go:TestTemplateSplit_SuccessorValidFrom_NoPhantomBeforeEffective":                                  "65ef5b5f2c7540bd",
	"services/schedule/template_split_service_test.go:TestTemplateSplit_UpdateSegmentsPreservesBoundsDuringMaterialization":                           "a3ff9216a6488c34",
	"services/schedule/template_start_pull_forward_test.go:TestUpdateTemplate_StartDatePullForward_MovesEnvelopeRosterAndMaterializesGapOnly":         "4706067c3d2899ff",
	"services/schedule/template_start_pull_forward_test.go:TestUpdateTemplate_StartDatePullForward_MovesWeekdayScopedRoster":                          "fba57cc6d8a9bbaa",
	"services/schedule/template_start_pull_forward_test.go:TestUpdateTemplate_StartDatePullForward_RejectsPredecessorOverlap":                         "758c3929669dc9e8",
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
	"api/students/care_exit_handlers_test.go:TestStudentList_UsesBookingParticipationButKeepsAdministrationAndLivePresence":                           "5bd9f0840380bb5f",
	"api/timetable/deviation_log_test.go:TestApplyDeviations_ActiveInstance_EndsAndCreatesSupervisor":                                                 "bd4e4888311db598",
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
	"services/active/active_service_wrappers_internal_test.go:TestActiveServiceThinDelegates":                                                         "e9423b1d9c240c1d",
	"services/active/analytics_service_test.go:TestGetDashboardAnalytics":                                                                             "1dd38daafa9b76ff",
	"services/active/update_visit_mock_test.go:TestUpdateVisitLocksAttendanceBeforeClosingIt":                                                         "5e806ff8a0e12020",
	"services/active/visit_helpers_test.go:TestCreateVisit_ClearsParentStatusForToday":                                                                "e25bcc762ba39622",
	"services/active/visit_helpers_test.go:TestCreateVisit_ClearsPlannedStatusForToday":                                                               "b3c8f4b906be213f",
	"services/active/work_session_export_test.go:TestWSGetHistory_AuditCountError":                                                                    "cca262eafb7c6659",
	"services/active/work_session_export_test.go:TestWSGetHistory_BreaksError":                                                                        "7d4a50e445b9cd14",
	"services/active/work_session_service_test.go:TestWSGetHistory_ClosedSessionKeepsCachedBreaks":                                                    "2d8543848511fd97",
	"services/active/work_session_service_test.go:TestWSGetHistory_DeductsRunningBreakFromNetMinutes":                                                 "407fa9f165eae8e1",
	"services/active/work_session_service_test.go:TestWSGetHistory_RepoError":                                                                         "cf2ba0f72e565111",
	"services/active/work_session_service_test.go:TestWSGetHistory_RunningBreakIsCappedAtTheLiveLimit":                                                "bae33fd78f82e484",
	"services/active/work_session_service_test.go:TestWSGetHistory_SerializesRunningBreakInBreakMinutes":                                              "6d063e2660fec881",
	"services/calendar/service_integration_test.go:TestCalendarServiceIntegration_CleanupExpiredFeedTombstonesCascadesChildren":                       "df1e5073e16c5b91",
	"services/calendar/service_integration_test.go:TestCalendarServiceIntegration_StaffSubscriptionPublishesOccurrenceAndDeletionCancellations":       "f3b84ba79643f3ee",
	"services/calendar/service_integration_test.go:TestCalendarServiceIntegration_SubscriptionFeed":                                                   "5b0b7f19c479ba6a",
	"services/schedule/care_request_decision_snapshot_test.go:TestDecide_PickupChangeFreezesDiff":                                                     "60ce44a512161619",
	"services/schedule/instance_service_integration_test.go:TestInstance_ReplanWeek_RemovesFutureLegacyWeekendInstances":                              "099158bbf85f2740",
	"services/schedule/instance_service_integration_test.go:TestInstance_Start_ConflictWarning_Staff":                                                 "12410a9ab669ee8a",
	"services/schedule/instance_service_integration_test.go:TestInstance_Start_StaffBridgedOverrideDifferentRoom_Conflict":                            "e5493636852c19b8",
	"services/schedule/instance_service_integration_test.go:TestInstance_Start_StaffBridgedOverrideSameRoom_NoConflict":                               "2ab669912c01eea2",
	"services/schedule/instance_service_integration_test.go:TestInstance_Start_StaffBridgedWithoutRosterRow_Conflict":                                 "d1a0446880cffafa",
	"services/schedule/instance_service_integration_test.go:TestInstance_Start_StaffSameRoomIsNotAConflict":                                           "1c9e1b369ce5d462",
	"services/schedule/staff_schedule_overview_integration_test.go:TestShiftCoverageProjection_BatchesEffectiveSeriesReadsAndIsolatesTenant":          "12d05a91cd61cd1c",
	"services/schedule/staff_schedule_overview_integration_test.go:TestStaffScheduleOverview_WeeklySummariesIncludeShiftsOutsideViewport":             "2bba2c1d37f1187f",
	"services/schedule/staff_schedule_overview_integration_test.go:TestStaffScheduleOverview_WeeklySummariesResolveSollAndIsolateTenant":              "ab3017a3954ae1d7",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_MoveConsumesOriginalDateBeforeRematerialization":                   "8321cde0cc85c471",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_RepeatedMoveKeepsOriginalOccurrenceIdentity":                       "c2d0b89b8809031d",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_WeekPatternARespectsCycle":                                         "da2ec4a3ee977da6",
	"services/schedule/staff_shift_series_rule_edit_test.go:TestStaffShiftSeries_SplitRejectsExtensionWithoutRecurrenceOccurrence":                    "ebbe0e8cbcf987e4",
	"services/schedule/staff_shift_series_rule_edit_test.go:TestStaffShiftSeries_SplitRejectsWhenNextSegmentLeavesNoOccurrence":                       "ae632a8b2afdce0b",
	// Source patterns added to the ratchet still need an exact baseline for
	// tests that predate #2571. Keep this list shrink-only.
	"api/active/checkin_test.go:TestAttendance_Fields":                                                                                                                                                                "1a1ee2f363b5121d",
	"api/birthdays/api_test.go:TestOverviewListsTodaysChildren":                                                                                                                                                       "f8dda4e746b3eb76",
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
	"api/timetable/templates_series_test.go:TestUpdateTemplate_SeriesRosterFromReachesPredecessor":                                                                                                                    "df4add89ef2a5ad3",
	"api/timetable/templates_split_test.go:TestTemplateEndHandler_HappyPath":                                                                                                                                          "c24d97cbe0dc6a65",
	"api/timetable/templates_split_test.go:TestTemplateSplitHandler_UpdateSuccessorPreservesValidFrom":                                                                                                                "fa37be61fd5887db",
	"api/timetable/templates_split_test.go:TestTemplateUpdateHandler_RejectsInconsistentValidityEnvelopeWithoutMutation":                                                                                              "0b3402d526a85d6d",
	"api/timetable/templates_test.go:TestListTemplates_CapacityFields":                                                                                                                                                "ffd8da579ed417d4",
	"database/migrations/001015314_template_source_school_classes_test.go:TestTemplateSourceSchoolClassesDownPreservesSourcedEnrollmentHistory":                                                                       "78973faba2674a8c",
	"database/repositories/active/attendance_date_range_test.go:TestAttendanceRepository_FindByStudentAndDateRange":                                                                                                   "5b3252aad4ccb6ab",
	"database/repositories/active/bulk_readers_test.go:TestGroupSupervisorRepository_ListActiveSupervisedRooms":                                                                                                       "4a84319ecaaa2636",
	"database/repositories/active/group_supervisor_repository_test.go:TestGroupSupervisorRepository_EndAllActiveByStaffID":                                                                                            "5230b9d1baa861e6",
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
	"services/absence/excused_request_service_test.go:TestDecide_ApproveRefusedWhenPartialAbsenceExists":                                                                                                              "5f70b9e7f043ac36",
	"services/active/attendance_service_test.go:TestGetStudentAttendanceStatus_NotCheckedIn":                                                                                                                          "33ca603438f00e6d",
	"services/active/cleanup_service_test.go:TestCleanupStaleAttendance_CheckOutTimeIsBerlinEndOfDay":                                                                                                                 "867efc4787c5cf40",
	"services/active/cleanup_supervisors_test.go:TestCleanupStaleSupervisors_ClosesYesterdayRecords":                                                                                                                  "ccf9bf1c069dc79e",
	"services/active/staff_absence_service_test.go:TestAbsCreateAbsenceFor_RejectsCompTimeAgainstLaterLedgerCapacity":                                                                                                 "cb52ce0e3c797dc6",
	"services/active/staff_opening_balance_mock_test.go:TestStaffBalanceAdjustmentService_OpeningAllowsNegativeTarget":                                                                                                "c22b55376229cb66",
	"services/active/student_status_day_write_bulk_test.go:TestBulkCreateForDates_RejectsConflictWithoutPartialWrites":                                                                                                "ad043a1aae2b0898",
	"services/active/student_status_day_write_bulk_test.go:TestCreateForDates_RejectsConflictWithoutPartialWrites":                                                                                                    "81c21301241e828b",
	"services/active/work_session_autocheckout_mock_test.go:TestAutoCheckout_QueriesOpenSessionsIncludingToday":                                                                                                       "f6a260a5c501312a",
	"services/active/work_session_service_test.go:TestWSApplyCustomScheduleRows_StampsAnchorForFirstRotation":                                                                                                         "9d8bb796d1561ab1",
	"services/education/grade_transition_offering_resync_test.go:TestGradeTransitionService_ApplyAndRevert_ResyncOfferingSourcedRosters":                                                                              "87758be42fc5d4b6",
	"services/education/grade_transition_roster_reconcile_test.go:TestGradeTransitionService_Revert_FillsBackdatedInstance":                                                                                           "9bda6e2d66560233",
	"services/education/grade_transition_roster_reconcile_test.go:TestGradeTransitionService_Revert_FillsTodaysInstanceMaterializedWhileAlumnus":                                                                      "c230eb320cc571bc",
	"services/enrollment/care_offering_booking_stats_internal_test.go:TestBookingStatsWindow_DefaultsToTodayWithoutPhaseDates":                                                                                        "baed039a1339ab3a",
	"services/enrollment/care_offering_booking_stats_internal_test.go:TestListBookingStats_CountsInTheCapacityGatesWindow":                                                                                            "095f3718e8b38730",
	"services/enrollment/class_roster_care_end_test.go:TestClassRosterFiltersCareDate":                                                                                                                                "0f9af60cc1b16754",
	"services/enrollment/decision_service_test.go:TestDecisionService_Decide_ApprovedScheduledPastStartActivatesStudent":                                                                                              "46d93594ffa4652b",
	"services/enrollment/decision_service_test.go:TestDecisionService_ListChildOfferings_CarriesAttributesAndFutureBookings":                                                                                          "2f5d36f3a11f4fdc",
	"services/enrollment/offering_change_request_service_test.go:TestOfferingChangeRequestService_ListPending_ReportsDateClampedToThePhaseStart":                                                                      "3df4cc2b5b7ae61e",
	"services/enrollment/offering_change_request_service_test.go:TestOfferingChangeRequestService_ListPending_ReportsTheSelectableDateRange":                                                                          "028a96c441ea20eb",
	"services/enrollment/offering_source_service_test.go:TestDecide_MultiSourceFanOutSeedsFromPhaseStart":                                                                                                             "dd8b84b01b84a586",
	"services/enrollment/offering_source_service_test.go:TestListOfferingSourceOptions_CountsScopedToSelectedPeriod":                                                                                                  "3476f63625e40194",
	"services/enrollment/offering_source_service_test.go:TestUpdateChildOfferings_UndatedCorrectionKeepsPhaseStartOnMultiSource":                                                                                      "0c9bd3c3100b4b48",
	"services/enrollment/report_service_test.go:TestCareUsageEnrichesGuardiansAndSchedulePickup":                                                                                                                      "b2cde36a6c78a2a8",
	"services/enrollment/report_service_test.go:TestClassRosterUsesOfferingDateForPickupProjection":                                                                                                                   "7116b1aa5e568165",
	"services/feedback/feedback_service_test.go:TestFeedbackService_GetEntriesByDateRange":                                                                                                                            "4be832fee2bdf672",
	"services/feedback/feedback_service_test.go:TestFeedbackService_GetEntriesByStudentAndDateRange":                                                                                                                  "45ad2f40c29ca3a1",
	"services/parent/care_exception_service_test.go:TestListCareExceptions_FlagsAbsentArrivalRow":                                                                                                                     "6bb23c3df4d3d663",
	"services/parent/care_exception_service_test.go:TestListCareExceptions_FlagsAbsentPickupRow":                                                                                                                      "2b4f61b1f3615bc9",
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
	"services/parent/parent_write_service_test.go:TestSubmitSickNote_RefusesPartialAbsenceConflict":                                                                                                                   "601bfb157f07599b",
	"services/parent/sick_note_gate_pin_test.go:TestSickNoteStaysImmediateWhenApprovalDisabled":                                                                                                                       "9ecf71fcf9d35732",
	"services/schedule/partial_absence_pending_request_test.go:TestPartialAbsenceCreate_RefusesPendingFullDayRequest":                                                                                                 "f326bb021b3d560a",
	"services/schedule/shift_plan_sync_service_test.go:TestSickCascade_ClearSickForRange":                                                                                                                             "6f99b3306a7411c9",
	"services/schedule/shift_plan_sync_service_test.go:TestSickCascade_ConcurrentOverlappingReportsSerializeBeforeOverlapRead":                                                                                        "09e52a2bd60d6a30",
	"services/schedule/shift_plan_sync_service_test.go:TestSickCascade_UpdateRangeRollsBackWhenRemovedShiftCannotReactivate":                                                                                          "1f7b7891b0d383d9",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffScheduleOverview_SeriesFieldsRideExistingReads":                                                                                                "eab026e122973f6d",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_CapAllByStaffIDClampsFutureSeries":                                                                                                 "7c0fced349b1a4f0",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_CollisionSkipsAndReports":                                                                                                          "aabf0d7363658eba",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_CreateMaterializesFromTomorrow":                                                                                                    "bccda1261e99d895",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_CreateRejectsBadReferences":                                                                                                        "f902e6812c9fa03a",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_EditDetachesAndDeleteRecordsException":                                                                                             "f6017f7c1d0ca641",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_EndAtFirstOccurrence":                                                                                                              "563f334335601409",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_EndSeriesKeepsDetachedAndPast":                                                                                                     "29458afe688c7073",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_SplitAtFirstOccurrence":                                                                                                            "2f86e5c82ef25506",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_SplitOutsideSegmentRejected":                                                                                                       "ed850983d42e05b9",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_SplitPreservesDeviationsOnSuccessor":                                                                                               "83a0b5444ea2b017",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_SplitTodayUpdatesOccurrenceAndReplansTomorrow":                                                                                     "df276b38c3c57d50",
	"services/schedule/staff_shift_series_integration_test.go:TestStaffShiftSeries_WeekPatternRequiresCycle":                                                                                                          "fda3449efc7cf97a",
	"services/schedule/staff_shift_series_mock_test.go:TestEndSeriesUnit_ErrorBranches":                                                                                                                               "6d7654771d91914e",
	"services/schedule/staff_shift_series_mock_test.go:TestGetSeriesUnit":                                                                                                                                             "4e580640f0edfa79",
	"services/schedule/staff_shift_series_mock_test.go:TestSplitSeriesUnit_ErrorBranches":                                                                                                                             "43b7f15a816a989d",
	"services/schedule/staff_shift_series_rule_edit_test.go:TestStaffShiftSeries_SplitAppliesNewWeekdaysFromEffectiveDate":                                                                                            "42693683b4ad2705",
	"services/schedule/staff_shift_series_rule_edit_test.go:TestStaffShiftSeries_SplitBoundsEarlierSegmentAtNextSuccessor":                                                                                            "ef82b199b83d4c80",
	"services/schedule/staff_shift_series_rule_edit_test.go:TestStaffShiftSeries_SplitExtendsSeriesEndingToday":                                                                                                       "32f9c45a2a1b5edd",
	"services/schedule/staff_shift_series_rule_edit_test.go:TestStaffShiftSeries_SplitKeepsStoredValidityWhenUnset":                                                                                                   "985d3d41c595bb86",
	"services/schedule/staff_shift_series_rule_edit_test.go:TestStaffShiftSeries_SplitRejectsSupersededSegment":                                                                                                       "fde38b83b964ec79",
	"services/schedule/staff_shift_series_rule_edit_test.go:TestStaffShiftSeries_SplitRejectsValidityBeyondCalendarPeriod":                                                                                            "ccf5dd8d6d2c5c6f",
	"services/schedule/staff_shift_series_rule_edit_test.go:TestStaffShiftSeries_SplitRejectsWhenNoOccurrenceRemains":                                                                                                 "0f76c8b779cfbf78",
	"services/schedule/staff_shift_series_rule_edit_test.go:TestStaffShiftSeries_SplitShortensValidityAndDropsLaterShifts":                                                                                            "a5da5e42fa3f546c",
	"services/schedule/template_end_service_unit_test.go:TestTemplateEndFromDate_ReturnsSummaryAndDeletesOpenEndedWindow":                                                                                             "899c95a8695dd10f",
	"services/schedule/template_offering_source_unit_test.go:TestResyncUpdatedTemplateOfferingRoster":                                                                                                                 "d2a7291797217399",
	"services/schedule/template_series_roster_mock_test.go:TestReconcileSeriesPredecessorRoster_CreatesBoundedRows":                                                                                                   "f8eb0964ce415493",
	"services/users/care_booking_authority_integration_test.go:TestBookingMutationPlansFutureNaturalEndImmediately":                                                                                                   "31b10f4d9ea7b782",
	"services/users/care_booking_authority_integration_test.go:TestOverdueRebookingReplacesTheStaleCompletion":                                                                                                        "11b33fdd50f6f8a0",
	"services/users/care_lifecycle_service_test.go:TestCareLifecycle_CancelPutsThePlanBack":                                                                                                                           "e43d1b471e5cfb28",
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
	"services/users/staff_document_service_test.go:TestStaffDocumentService_CreateHydratesGeneratedTimestamps":                                                                                                        "127d3c104e32f4c1",
	"services/users/staff_document_service_test.go:TestStaffDocumentService_RetentionSchedule":                                                                                                                        "ba5f8052be3488bb",
	"simulate/feed_tombstone_test.go:TestRunFullDaySeedsStaffFeedTombstone":                                                                                                                                           "9de0fc1219436ec7",
	"database/repositories/users/care_withdrawal_completion_test.go:TestCareWithdrawalCompletionRepository_OnePendingTaskPerChild":                                                                                    "4a84dc3114fe62bb",
	"services/schedule/shift_plan_sync_service_test.go:TestSickCascade_ShiftOnlyChangesBroadcastTenantInvalidation":                                                                                                   "ee906e881bf24d90",
	"services/schedule/shift_plan_sync_service_test.go:TestSickCascade_ReconcileSickRangeAppliesOnlyDateDelta":                                                                                                        "f50d6fadb845938f",
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
