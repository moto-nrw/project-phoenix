package test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestModuleFunctionLengthRatchet caps function body length in the modules
// that migration #2580 already carved out of api/ and models/.
//
// Why a second length gate: every existing quality gate in this package stops
// at the api/ or models/ directory boundary. #2580 moved 228,306 of 638,717
// production LOC into backend/modules/ and backend/workflows/, so the code
// that grows fastest is exactly the code nothing measures. A 300-line service
// method is the shape .claude/rules/backend-conventions.md keeps legislating
// against from three directions — Rule 4 (thin handlers), Rule 8 (services
// encapsulate, not delegate) and Rule 12 (models hold data, not decisions) —
// without any of them counting lines. This ratchet counts lines, which is the
// crude proxy that catches the cases cognitive complexity misses: long, flat,
// branch-free orchestration that should be three named steps.
//
// The threshold is 60 body lines, measured from the opening brace line to the
// closing brace line inclusive, on named declarations only. Function literals
// are not counted: a long literal is a property of its enclosing declaration,
// which the ratchet already measures.
//
// Allowlist semantics (per function, not per file — four cases):
//
//   - A function NOT in the allowlist must have a body of ≤ 60 lines.
//   - An allowlisted function may never exceed its recorded length.
//   - When a split shortens a function, the test fails until the entry is
//     lowered — the ratchet only turns one way. Never raise a number.
//   - When a function drops to ≤ 60 lines (or is deleted or renamed) it
//     disappears from the scan and its entry must be removed.
//
// Keys are "relative/file.go:FuncName" with methods rendered the way the
// gocognit CLI renders them ("(*service).ListOptions"), so entries here line
// up with the neighbouring complexity ratchet's keys.
//
// Scope: every non-test .go file under modules/ and workflows/, including the
// legacy/ subtrees. Those are counted deliberately — the retained legacy code
// is the larger half of the problem, and exempting it would let a migration
// move a 200-line method into legacy/ to silence the gate.
//
// Seed: measured 2026-09-18 at merge commit ecf0003369 with a throwaway
// go/ast walk over modules/ and workflows/ (named FuncDecls with a body,
// non-test files), recording fn.Body.Rbrace.Line - fn.Body.Lbrace.Line + 1
// per function:
//
//	cd backend && ../scripts/run-go-toolchain.sh go run ./tmp/moduleFuncLen
//
// The first seed was taken at 19feca2822 and re-measured here after the merge
// of origin/development (a47f77b2c3): PR #3408 (issue #3350) moved seven
// over-threshold functions into modules/, which the ratchet cannot absorb
// without a fresh measurement.
//
// 299 of 14,623 functions were over the threshold at seed time (483 over 50
// lines, 83 over 100, 5 over 200). The seed freezes that state; it is not a
// statement that these 299 functions are acceptable.
const moduleFuncLenThreshold = 60

// Seeded 2026-09-18 from merge commit ecf0003369. Shrink-only: lower or
// delete entries, never add one and never raise a number.
var moduleFuncLenAllowlist = map[string]int{
	"modules/appointments/recurrence.go:boundedRecurrenceDates":                                          117,
	"modules/careplan/inbound/parent/api.go:(*Resource).RouterWithAuthRateLimiter":                       215,
	"modules/careplan/inbound/parent/child_write_handlers.go:(*Resource).submitSickNote":                 67,
	"modules/careplan/inbound/parent/child_write_handlers.go:renderParentWriteError":                     162,
	"modules/careplan/inbound/parent/enrollment_handlers.go:(*Resource).getEnrollmentBootstrap":          87,
	"modules/careplan/internal/adapters/postgres/withdrawal_completions.go:(*Store).ListWithdrawals":     78,
	"modules/careplan/internal/application/excused_requests.go:(*ExcusedRequests).Correct":               73,
	"modules/careplan/internal/application/excused_requests.go:(*ExcusedRequests).Decide":                154,
	"modules/careplan/internal/application/excused_requests.go:(*ExcusedRequests).ListHistory":           67,
	"modules/careplan/internal/application/excused_requests.go:(*ExcusedRequests).ListPending":           74,
	"modules/careplan/internal/application/offering_review_pending.go:(*OfferingReviews).pendingFacts":   92,
	"modules/careplan/internal/application/offering_review_pending.go:(*OfferingReviews).pendingReviews": 100,
	"modules/careplan/internal/application/schedule_reviews.go:(*ScheduleReviews).ListPending":           70,
	"modules/careplan/internal/application/schedule_reviews.go:(*ScheduleReviews).plan":                  66,
	// Moved in by PR #3408 (#3350) from services/users and
	// database/repositories/users; the bodies crossed the boundary at their
	// old length.
	"modules/careplan/legacy/carelifecycle/care_exit_cleanup.go:(*CareExitCleanupRepository).restoreRemovals":                             88,
	"modules/careplan/legacy/carelifecycle/care_lifecycle_service.go:(*careLifecycleService).ApplyDueEffects":                             92,
	"modules/careplan/legacy/carelifecycle/care_lifecycle_service.go:(*careLifecycleService).Cancel":                                      70,
	"modules/careplan/legacy/carelifecycle/care_lifecycle_service.go:(*careLifecycleService).Resume":                                      65,
	"modules/careplan/legacy/carelifecycle/care_lifecycle_service.go:(*careLifecycleService).buildPreview":                                108,
	"modules/careplan/legacy/carelifecycle/student_companion_service.go:(*companionService).checkCompanionRemovals":                       63,
	"modules/careplan/legacy/carelifecycle/student_companion_service.go:(*companionService).validateCompanionUpdate":                      69,
	"modules/careplan/legacy/careschedule/arrival_baseline_service.go:(*arrivalBaselineService).Project":                                  69,
	"modules/careplan/legacy/careschedule/arrival_service.go:(*arrivalScheduleService).BulkUpsertArrivalSchedules":                        229,
	"modules/careplan/legacy/careschedule/care_request_service.go:(*careScheduleRequestService).Correct":                                  90,
	"modules/careplan/legacy/careschedule/care_request_service.go:(*careScheduleRequestService).ListHistory":                              76,
	"modules/careplan/legacy/careschedule/care_request_service.go:(*careScheduleRequestService).applyCareScheduleRequest":                 69,
	"modules/careplan/legacy/careschedule/care_request_service.go:(*careScheduleRequestService).careScheduleDiffFrom":                     83,
	"modules/careplan/legacy/careschedule/care_request_service.go:buildCareScheduleChanges":                                               61,
	"modules/careplan/legacy/careschedule/effective_time_service.go:(*effectiveTimeCore).UpdateException":                                 76,
	"modules/careplan/legacy/careschedule/partial_absence_service.go:(*partialAbsenceService).Create":                                     77,
	"modules/careplan/legacy/careschedule/pickup_auto_excusal.go:(*PickupAutoExcusalSyncer).Sync":                                         66,
	"modules/careplan/legacy/careschedule/pickup_schedule_service.go:(*pickupScheduleService).BulkUpsertPickupSchedules":                  140,
	"modules/classday/internal/application/slotlists.go:(*service).ListOptions":                                                           296,
	"modules/classday/internal/application/slotlists.go:(*service).RenderList":                                                            71,
	"modules/classday/internal/application/slotlists.go:(*service).buildList":                                                             127,
	"modules/classday/internal/application/slotlists.go:(*service).classifyPlannedRow":                                                    128,
	"modules/classday/internal/application/slotlists.go:(*service).enrichEntries":                                                         171,
	"modules/classday/internal/application/slotlists.go:(*service).loadSlotPresence":                                                      74,
	"modules/classday/internal/application/slotlists.go:slotHeadingDisambiguation":                                                        72,
	"modules/communication/internal/adapters/staffinbox/projection.go:(*Projection).ListInbox":                                            61,
	"modules/communication/internal/parentmessages/events.go:(*EventEmitter).EmitChildEvent":                                              176,
	"modules/communication/internal/parentmessages/message_email.go:messageEmailCopy":                                                     84,
	"modules/communication/internal/parentmessages/service.go:(*Service).PostMessage":                                                     79,
	"modules/communication/internal/parentmessages/service.go:(*Service).notifyGuardianDevice":                                            61,
	"modules/communication/internal/staffannouncements/care_cancellation.go:(*service).PublishCareCancellation":                           83,
	"modules/communication/internal/staffannouncements/email.go:NewAnnouncementRenderer":                                                  61,
	"modules/communication/internal/staffannouncements/letter.go:(*service).ResendFailedEmails":                                           107,
	"modules/communication/internal/staffannouncements/letter.go:(*service).queueLetterMailsAs":                                           90,
	"modules/communication/internal/staffannouncements/poll.go:(*service).enqueueReminderEmails":                                          77,
	"modules/communication/internal/staffannouncements/poll.go:(*service).pushPollReminder":                                               64,
	"modules/communication/internal/staffannouncements/reminder.go:(*service).SendDueReminders":                                           72,
	"modules/communication/internal/staffannouncements/service.go:(*service).Publish":                                                     100,
	"modules/communication/internal/staffannouncements/service.go:(*service).Update":                                                      70,
	"modules/communication/internal/staffannouncements/service.go:(*service).enqueueAnnouncementEmailsAs":                                 131,
	"modules/communication/internal/staffannouncements/service.go:(*service).notifyAnnouncementGuardiansWith":                             91,
	"modules/communication/internal/staffannouncements/service.go:normalizeInput":                                                         78,
	"modules/communication/internal/staffmessages/notify.go:(*Service).notifyRecipients":                                                  84,
	"modules/communication/internal/staffmessages/service.go:(*Service).PostMessage":                                                      71,
	"modules/dataimport/fileformat/helpers.go:MapStudentRow":                                                                              168,
	"modules/dataimport/fileformat/template_writer.go:(Decoder).Template":                                                                 62,
	"modules/dataimport/fileformat/templates.go:writeHinweiseSheet":                                                                       145,
	"modules/dataimport/fileformat/xlsx_parser.go:(*XLSXParser).ParseStudents":                                                            76,
	"modules/dataimport/inbound/api.go:(*Resource).Router":                                                                                66,
	"modules/delivery/application/notifications/parent_copy.go:ParentAnnouncementCopy":                                                    95,
	"modules/delivery/application/notifications/preferences.go:(*preferenceService).GetForParent":                                         63,
	"modules/delivery/application/notifications/service.go:(*router).NotifyBatch":                                                         62,
	"modules/delivery/application/notifications/service.go:(*router).NotifySynchronously":                                                 62,
	"modules/delivery/application/notifications/service.go:validate":                                                                      69,
	"modules/delivery/application/notifications/types.go:init":                                                                            152,
	"modules/delivery/http/sse/school_api.go:(*Resource).schoolEventsHandler":                                                             71,
	"modules/devicescan/internal/application/systemspace.go:(*Service).systemActivity":                                                    68,
	"modules/enrollment/form_schema.go:(*FormField).validateQuestion":                                                                     83,
	"modules/enrollment/form_schema.go:(*FormLegalBlock).Validate":                                                                        73,
	"modules/enrollment/form_schema.go:(*FormSchema).Validate":                                                                            64,
	"modules/enrollment/internal/adapters/postgres/account_requests.go:(*Store).AccountRequests":                                          93,
	"modules/enrollment/internal/adapters/postgres/request_duplicates.go:(*Store).ActiveDuplicateChildren":                                68,
	"modules/enrollment/phase.go:(*Phase).Validate":                                                                                       143,
	"modules/enrollment/selection/materialize.go:MaterializeAdjustments":                                                                  65,
	"modules/enrollment/selection/materialize.go:materializeOfferingSelections":                                                           95,
	"modules/exporttransfer/internal/adapters/sftp/client.go:(*Client).Prepare":                                                           75,
	"modules/exporttransfer/internal/application/service.go:(*Service).Transfer":                                                          75,
	"modules/filestorage/internal/application/cleanup.go:(*Service).sweep":                                                                70,
	"modules/identityaccess/compose/new.go:New":                                                                                           97,
	"modules/identityaccess/internal/application/account_mfa_admin.go:(*AccountMFAFlows).OperatorSetGlobalMFAOverride":                    70,
	"modules/identityaccess/internal/application/account_mfa_admin.go:(*AccountMFAFlows).setTenantOverride":                               73,
	"modules/identityaccess/internal/application/account_mfa_flow.go:(*AccountMFAFlows).StartChallenge":                                   81,
	"modules/identityaccess/internal/application/account_mfa_flow.go:(*AccountMFAFlows).resolvePolicy":                                    62,
	"modules/identityaccess/internal/application/account_mfa_flow.go:(*AccountMFAFlows).verifyChallengeBound":                             75,
	"modules/identityaccess/internal/application/account_passkey_flow.go:(*AccountPasskeyFlows).verifyLogin":                              80,
	"modules/identityaccess/internal/application/account_provisioning.go:(*AccountProvisioning).LinkSchoolAccount":                        81,
	"modules/identityaccess/internal/application/account_provisioning.go:(*AccountProvisioning).RegisterSchoolAccount":                    73,
	"modules/identityaccess/internal/application/guardian_relative_access.go:(*AccountLifecycle).ApproveInvitation":                       65,
	"modules/identityaccess/internal/application/guardian_relative_access.go:(*AccountLifecycle).ListPendingApprovalsDetailed":            96,
	"modules/identityaccess/internal/application/guardian_relative_access.go:(*AccountLifecycle).createStudentInvitation":                 72,
	"modules/identityaccess/internal/application/guardian_relative_access.go:(*AccountLifecycle).revokeAccess":                            92,
	"modules/identityaccess/internal/application/operator_account_access.go:(*OperatorAccountAccess).GrantAccountTenantAccess":            95,
	"modules/identityaccess/internal/application/operator_account_access.go:(*OperatorAccountAccess).RevokeAccountTenantAccess":           72,
	"modules/identityaccess/internal/application/operator_account_access.go:(*OperatorAccountAccess).UpdateAccountTenantRole":             88,
	"modules/identityaccess/internal/application/operator_account_access.go:(*OperatorAccountAccess).loadTenantAccess":                    71,
	"modules/identityaccess/internal/application/operator_authentication.go:(*OperatorAuthentication).RefreshToken":                       96,
	"modules/identityaccess/internal/application/operator_mfa_flow.go:(*OperatorMFAFlows).StartChallenge":                                 74,
	"modules/identityaccess/internal/application/operator_passkey_flow.go:(*OperatorPasskeyFlows).verifyLogin":                            63,
	"modules/identityaccess/internal/application/operator_provisioning.go:(*OperatorProvisioning).InitiateOperatorEmailChange":            92,
	"modules/identityaccess/internal/application/role_administration.go:(*RoleAdministration).AssignRoleToAccount":                        102,
	"modules/identityaccess/internal/application/school_identity.go:(*AccountLifecycle).resolveIdentityPerson":                            61,
	"modules/identityaccess/internal/application/school_invitation.go:(*SchoolInvitation).AcceptInvitation":                               65,
	"modules/identityaccess/internal/application/school_login.go:(*AccountAuthentication).loginSchoolWithMFAGate":                         107,
	"modules/identityaccess/internal/application/session_refresh.go:(*AccountAuthentication).refreshSessionInTransaction":                 130,
	"modules/identityaccess/internal/application/session_revocation.go:(*AccountAuthentication).finishScheduledAccountWideWipe":           66,
	"modules/identityaccess/internal/application/staff_offboarding.go:(*AccountLifecycle).ExecuteStaffOffboarding":                        61,
	"modules/identityaccess/internal/application/staff_offboarding.go:(*AccountLifecycle).staffOffboardingSnapshot":                       77,
	"modules/identityaccess/internal/application/staff_preview.go:(*AccountLifecycle).StartStaffPreview":                                  133,
	"modules/identityaccess/legacy/usercontext/usercontext_service.go:(*userContextService).GetMyGroups":                                  62,
	"modules/mealplan/internal/adapters/postgres/store.go:(*Store).FindDailyParticipation":                                                83,
	"modules/mealplan/internal/adapters/postgres/store.go:(*Store).FindParticipation":                                                     66,
	"modules/organizationtenancy/inbound/operator/provisioning.go:ProvisioningErrorRenderer":                                              92,
	"modules/peopledirectory/enrollment_departure.go:normalizeEnrollmentDeparture":                                                        67,
	"modules/peopledirectory/internal/application/student_photo.go:(*StudentPhotoService).CommitPhoto":                                    62,
	"modules/peopledirectory/internal/application/student_write.go:(*StudentService).UpdateStudent":                                       66,
	"modules/planexport/betreuungsplan.go:(*betreuungsplanData).rows":                                                                     75,
	"modules/planexport/dienstplan.go:(*dienstplanData).rowsByArea":                                                                       91,
	"modules/planexport/legacy/legacy.go:(overviewAdapter).StaffScheduleOverview":                                                         64,
	"modules/requestreview/compose/corrections.go:(correctionLog).History":                                                                64,
	"modules/schoolcalendar/portal/internal/application/feed.go:(*service).ParentCalendarFeedByToken":                                     124,
	"modules/schoolcalendar/portal/internal/application/feed.go:(*service).projectStaffCalendarEvents":                                    140,
	"modules/schoolcalendar/portal/internal/application/ics.go:appointmentICSEvent":                                                       76,
	"modules/schoolcalendar/portal/internal/application/notifications.go:(*service).notifyGuardians":                                      83,
	"modules/schoolcalendar/portal/internal/application/service.go:(*service).RecipientOptions":                                           127,
	"modules/schoolcalendar/portal/internal/application/service.go:(*service).RespondToParentInvitation":                                  69,
	"modules/schoolcalendar/portal/internal/application/service.go:(*service).createStaffAppointment":                                     91,
	"modules/schoolcalendar/portal/internal/application/service.go:(*service).resolveTargets":                                             170,
	"modules/schoolcalendar/portal/internal/application/service.go:(*service).updateStaffAppointment":                                     102,
	"modules/schoolmembership/internal/application/offboarding.go:(*Service).retirementSnapshot":                                          64,
	"modules/schoolportal/api.go:(*Resource).RouterWithAuthRateLimiter":                                                                   90,
	"modules/schoolportal/auth_handlers.go:(*Resource).login":                                                                             64,
	"modules/schoolstructure/internal/adapters/postgres/transition_store.go:(*Store).ListTransitions":                                     64,
	"modules/schoolstructure/internal/application/transition.go:(*Service).UpdateTransition":                                              63,
	"modules/statistics/http/response.go:toReportResponse":                                                                                78,
	"modules/studentpresence/inbound/presence/analytics_handlers.go:(*Resource).getDashboardAnalytics":                                    69,
	"modules/studentpresence/inbound/presence/api.go:(*Resource).Router":                                                                  131,
	"modules/studentpresence/inbound/presence/checkin.go:(*Resource).parseAndValidateCheckinRequest":                                      62,
	"modules/studentpresence/inbound/presence/groups_handlers.go:(*Resource).buildVisitDisplayResponses":                                  68,
	"modules/studentpresence/internal/adapters/postgres/group_session.go:(*Store).EndGroupSession":                                        63,
	"modules/studentpresence/internal/adapters/postgres/room_utilization.go:(*Store).RoomUtilization":                                     88,
	"modules/studentpresence/legacy/services/active/active_service.go:(*service).createVisit":                                             104,
	"modules/studentpresence/legacy/services/active/analytics_service.go:(*service).GetDashboardAnalytics":                                73,
	"modules/studentpresence/legacy/services/active/attendance_service.go:(*service).performCheckIn":                                      103,
	"modules/studentpresence/legacy/services/active/attendance_service.go:(*service).performCheckOut":                                     73,
	"modules/studentpresence/legacy/services/active/cleanup_service.go:(*cleanupService).CleanupStaleAttendance":                          72,
	"modules/studentpresence/legacy/services/active/dashboard_helpers.go:(*service).fetchDashboardBaseData":                               95,
	"modules/studentpresence/legacy/services/active/school_checkin_batch.go:(*service).processSchoolCheckinBatch":                         130,
	"modules/studentpresence/legacy/services/active/school_checkin_batch.go:(*service).registerSchoolCheckinBatchBroadcast":               67,
	"modules/studentpresence/legacy/services/active/session_service.go:(*service).executeSessionStart":                                    69,
	"modules/studentpresence/legacy/services/active/student_status_day_write.go:(*StudentStatusDayService).BulkCreateForDates":            82,
	"modules/studentpresence/legacy/services/active/transit_service.go:(*service).assignTransitStudentsToActiveGroup":                     110,
	"modules/studentpresence/legacy/services/active/transit_service.go:(*service).moveStudentsToActiveGroupLocked":                        169,
	"modules/studentpresence/legacy/statistics/courses.go:(*service).courseSection":                                                       101,
	"modules/studentpresence/legacy/statistics/service.go:(*service).careDays":                                                            67,
	"modules/studentpresence/legacy/statistics/service.go:(*service).compute":                                                             91,
	"modules/timetable/compose/httpadapter/activity_handlers.go:(*Resource).createActivity":                                               70,
	"modules/timetable/compose/httpadapter/activity_handlers.go:(*Resource).listActivities":                                               67,
	"modules/timetable/compose/httpadapter/activity_handlers.go:(*Resource).quickCreateActivity":                                          77,
	"modules/timetable/compose/httpadapter/http.go:(*Resource).Router":                                                                    72,
	"modules/timetable/internal/adapters/postgres/calendar_period_references.go:(*Store).CountCalendarPeriodReferences":                   62,
	"modules/timetable/internal/adapters/postgres/groups.go:(*Store).ListCourseGroups":                                                    71,
	"modules/timetable/internal/adapters/postgres/pickup_extensions.go:(*Store).ListPickupExtensionWeekdayBlocks":                         70,
	"modules/timetable/legacy/timetableplanning/attendance_correction.go:(*TimetableDataService).CorrectInstanceStudentAttendance":        102,
	"modules/timetable/legacy/timetableplanning/attendance_sync_service.go:(*AttendanceSyncService).MirrorCheckInForVisit":                117,
	"modules/timetable/legacy/timetableplanning/attendance_sync_service.go:(*AttendanceSyncService).MirrorCheckOutForVisits":              61,
	"modules/timetable/legacy/timetableplanning/auto_start_service.go:(*autoStartService).RunForTenant":                                   101,
	"modules/timetable/legacy/timetableplanning/deviation_apply.go:(*instanceService).planDeviations":                                     82,
	"modules/timetable/legacy/timetableplanning/deviation_apply.go:validateDeviationStaff":                                                64,
	"modules/timetable/legacy/timetableplanning/deviation_service.go:(*instanceService).ApplySubstitute":                                  64,
	"modules/timetable/legacy/timetableplanning/edited_instance_detection.go:(*materializationService).DetectEditedInWindow":              172,
	"modules/timetable/legacy/timetableplanning/instance_conflict.go:DetectStartConflicts":                                                130,
	"modules/timetable/legacy/timetableplanning/instance_move_staff.go:(*instanceService).executeStaffMove":                               94,
	"modules/timetable/legacy/timetableplanning/instance_move_staff.go:(*instanceService).planStaffMove":                                  112,
	"modules/timetable/legacy/timetableplanning/instance_series_conversion.go:(*TimetableDataService).templateAssignmentsOn":              61,
	"modules/timetable/legacy/timetableplanning/instance_series_conversion.go:(*instanceSeriesConversionService).ConvertInstanceToSeries": 92,
	"modules/timetable/legacy/timetableplanning/instance_service.go:(*instanceService).Cancel":                                            108,
	"modules/timetable/legacy/timetableplanning/instance_service.go:(*instanceService).Complete":                                          178,
	"modules/timetable/legacy/timetableplanning/instance_service.go:(*instanceService).Reopen":                                            68,
	"modules/timetable/legacy/timetableplanning/instance_service.go:(*instanceService).ReplanWeek":                                        101,
	"modules/timetable/legacy/timetableplanning/instance_service.go:(*instanceService).Start":                                             153,
	"modules/timetable/legacy/timetableplanning/instance_service.go:(*instanceService).UpdatePlanned":                                     139,
	"modules/timetable/legacy/timetableplanning/instance_service.go:(*instanceService).absorbUnsupervisedOpenGroups":                      91,
	"modules/timetable/legacy/timetableplanning/instance_service.go:(*instanceService).broadcastInstanceEvent":                            65,
	"modules/timetable/legacy/timetableplanning/instance_service.go:(*instanceService).broadcastRestoredVisits":                           79,
	"modules/timetable/legacy/timetableplanning/instance_service.go:(*instanceService).reapplyDeviations":                                 145,
	"modules/timetable/legacy/timetableplanning/instance_service.go:(*instanceService).replaceInstanceAssignments":                        112,
	"modules/timetable/legacy/timetableplanning/instance_service.go:(*instanceService).snapshotDeviations":                                69,
	"modules/timetable/legacy/timetableplanning/instance_service.go:(*instanceService).validateInstanceReferences":                        89,
	"modules/timetable/legacy/timetableplanning/materialization_service.go:(*materializationService).materializeForTenantLocked":          90,
	"modules/timetable/legacy/timetableplanning/materialization_service.go:(*materializationService).materializeTemplate":                 184,
	"modules/timetable/legacy/timetableplanning/roster_reconciler.go:(*RosterReconciler).ReconcileSourcedTemplateRosters":                 110,
	"modules/timetable/legacy/timetableplanning/roster_reconciler.go:(*RosterReconciler).fillInstancesMaterializedDuringAlumnusWindow":    97,
	"modules/timetable/legacy/timetableplanning/schedule_service.go:(*service).FindAvailableSlots":                                        71,
	"modules/timetable/legacy/timetableplanning/template_create_service.go:(*TimetableDataService).createTemplateLocked":                  122,
	"modules/timetable/legacy/timetableplanning/template_create_service.go:validateOfferingSourceInput":                                   61,
	"modules/timetable/legacy/timetableplanning/template_series_roster.go:(*TimetableDataService).reconcilePredecessorEnrollmentRows":     68,
	"modules/timetable/legacy/timetableplanning/template_series_roster.go:(*TimetableDataService).reconcilePredecessorInstanceStaff":      108,
	"modules/timetable/legacy/timetableplanning/template_series_roster.go:(*TimetableDataService).reconcilePredecessorSegmentRoster":      86,
	"modules/timetable/legacy/timetableplanning/template_series_roster.go:(*TimetableDataService).reconcilePredecessorSupervisorRows":     88,
	"modules/timetable/legacy/timetableplanning/template_split_service.go:(*TemplateSplitService).createSuccessorGroup":                   91,
	"modules/timetable/legacy/timetableplanning/template_split_service.go:(*TemplateSplitService).endFromDateInTransaction":               97,
	"modules/timetable/legacy/timetableplanning/template_split_service.go:(*TemplateSplitService).splitInTransaction":                     113,
	"modules/timetable/legacy/timetableplanning/template_update_service.go:(*TimetableDataService).replaceTemplateRoster":                 70,
	"modules/timetable/legacy/timetableplanning/template_update_service.go:(*TimetableDataService).updateTemplateLocked":                  123,
	"modules/timetable/legacy/timetableplanning/timetable_bridge_service.go:(*TimetableBridgeService).notScheduledForEndedSessions":       90,
	"modules/timetable/legacy/timetableplanning/timetable_cleanup_service.go:(*timetableCleanupService).CleanupExpiredTimetableData":      71,
	"modules/timetable/legacy/timetableplanning/timetable_operations_service.go:(*timetableOperationsService).PlannedNow":                 129,
	"modules/timetable/legacy/timetableplanning/timetable_operations_service.go:(*timetableOperationsService).buildRosterWithCareDay":     146,
	"modules/timetable/legacy/timetableplanning/timetable_operations_service.go:(*timetableOperationsService).checkInStudent":             66,
	"modules/timetable/legacy/timetableplanning/timetable_operations_service.go:(*timetableOperationsService).enrichDayPlan":              120,
	"modules/timetable/legacy/timetableplanning/timetable_operations_service.go:mapPlannedInstance":                                       71,
	"modules/timetable/legacy/timetableplanning/timetable_read_exception_conflicts.go:conflictForStudent":                                 71,
	"modules/timetable/legacy/timetableplanning/timetable_read_student_week.go:(*TimetableDataService).PreloadStudentWeek":                92,
	"modules/timetableprojection/projection.go:CourseGroupsForOfferings":                                                                  62,
	"modules/workforce/inbound/timetracking/errors.go:classifyServiceError":                                                               63,
	"modules/workforce/inbound/timetracking/staff_admin_document_cleanup.go:(*StaffAdminResource).CleanupOrphanedStaffDocumentFiles":      70,
	"modules/workforce/inbound/timetracking/staff_admin_documents.go:(*StaffAdminResource).uploadStaffDocument":                           71,
	"modules/workforce/internal/adapters/postgres/offboarding.go:(*Store).PreviewStaffOffboarding":                                        61,
	"modules/workforce/internal/adapters/postgres/shift_store.go:applyStaffShiftFilter":                                                   63,
	"modules/workforce/internal/adapters/postgres/substitution_store.go:(*Store).ListGroupSubstitutions":                                  66,
	"modules/workforce/legacy/absence_repositories.go:applyStaffAbsenceCondition":                                                         61,
	"modules/workforce/legacy/shiftplanning/staff_assignment_service.go:(*staffAssignmentService).ListAssignmentsForStaff":                83,
	"modules/workforce/legacy/shiftplanning/staff_schedule_overview.go:(*staffScheduleOverviewService).loadOverviewData":                  68,
	"modules/workforce/legacy/shiftplanning/staff_schedule_overview.go:(*staffScheduleOverviewService).resolveWeeklyTargets":              83,
	"modules/workforce/legacy/shiftplanning/staff_shift_move.go:(*staffShiftService).MoveShift":                                           124,
	"modules/workforce/legacy/shiftplanning/staff_shift_series_service.go:(*staffShiftSeriesService).SplitSeries":                         157,
	"modules/workforce/legacy/shiftplanning/staff_shift_series_service.go:(*staffShiftSeriesService).materializeSeries":                   104,
	"modules/workforce/legacy/shiftplanning/staff_shift_service.go:(*staffShiftService).ApplyCancellation":                                223,
	"modules/workforce/legacy/shiftplanning/staff_shift_service.go:(*staffShiftService).updateShiftWithOptions":                           148,
	"modules/workforce/legacy/timetracking/staff_absence_rebooking.go:(*staffAbsenceService).RebookAbsences":                              76,
	"modules/workforce/legacy/timetracking/staff_absence_service.go:(*staffAbsenceService).CreateAbsenceFor":                              65,
	"modules/workforce/legacy/timetracking/staff_absence_service.go:(*staffAbsenceService).PreviewCompTimeBalance":                        74,
	"modules/workforce/legacy/timetracking/staff_absence_service.go:(*staffAbsenceService).RequestVacation":                               63,
	"modules/workforce/legacy/timetracking/staff_absence_service.go:(*staffAbsenceService).UpdateAbsence":                                 93,
	"modules/workforce/legacy/timetracking/staff_balance_adjustment_service.go:(*staffBalanceAdjustmentService).CreateAdjustment":         81,
	"modules/workforce/legacy/timetracking/staff_balance_adjustment_service.go:(*staffBalanceAdjustmentService).DeleteAdjustment":         75,
	"modules/workforce/legacy/timetracking/staff_balance_adjustment_service.go:(*staffBalanceAdjustmentService).ResetBalance":             97,
	"modules/workforce/legacy/timetracking/staff_overview_service.go:(*staffOverviewService).GetDashboardSummary":                         77,
	"modules/workforce/legacy/timetracking/staff_overview_service.go:(*staffOverviewService).GetTimeTrackingOverview":                     114,
	"modules/workforce/legacy/timetracking/staff_overview_service.go:(*staffOverviewService).buildPrefetch":                               97,
	"modules/workforce/legacy/timetracking/staff_vacation_opening.go:(*staffAbsenceService).SetVacationOpening":                           62,
	"modules/workforce/legacy/timetracking/work_session_service.go:(*workSessionService).AutoCheckoutDueSessions":                         161,
	"modules/workforce/legacy/timetracking/work_session_service.go:(*workSessionService).AutoEndExpiredBreaks":                            66,
	"modules/workforce/legacy/timetracking/work_session_service.go:(*workSessionService).CheckOutOn":                                      63,
	"modules/workforce/legacy/timetracking/work_session_service.go:(*workSessionService).CreateSessionAsAdmin":                            86,
	"modules/workforce/legacy/timetracking/work_session_service.go:(*workSessionService).ExportSessions":                                  76,
	"modules/workforce/legacy/timetracking/work_session_service.go:(*workSessionService).buildWeeklySummaries":                            65,
	"modules/workforce/legacy/timetracking/work_session_service.go:(*workSessionService).checkIn":                                         124,
	"modules/workforce/legacy/timetracking/work_session_service.go:(*workSessionService).detectPlannedDeviation":                          68,
	"modules/workforce/legacy/timetracking/work_session_service.go:(*workSessionService).historyResponse":                                 65,
	"modules/workforce/legacy/timetracking/work_time_month_service.go:(*workTimeMonthService).getMonthSummaryThrough":                     106,
	"modules/workforce/legacy/worksession_repositories.go:(staffBalanceAdjustmentRepository).List":                                        61,
	"modules/workforce/legacy/worksession_repositories.go:workSessionFilterFromOptions":                                                   61,
	"workflows/gradetransition/apply.go:(*Workflow).executeApply":                                                                         103,
	"workflows/gradetransition/class_list_entries.go:(*Workflow).revertClassListEntries":                                                  81,
	"workflows/gradetransition/class_teachers.go:(*Workflow).remapClassTeacherAssignments":                                                62,
	"workflows/gradetransition/class_teachers.go:(*Workflow).revertClassTeacherAssignments":                                               68,
	"workflows/gradetransition/compose/new.go:Assemble":                                                                                   85,
	"workflows/parentportal/care/parent_care_schedule_service.go:(*Service).buildCareScheduleView":                                        74,
	"workflows/parentportal/care/parent_today_status_service.go:(*Service).GetChildTodayStatus":                                           75,
	"workflows/parentportal/legacy/parent_consent_service.go:(*service).setPhotoConsent":                                                  106,
	"workflows/parentportal/legacy/parent_guardian_service.go:(*service).CreateGuardianContact":                                           111,
	"workflows/parentportal/legacy/parent_guardian_service.go:(*service).ListChildGuardians":                                              73,
	"workflows/parentportal/legacy/parent_guardian_service.go:(*service).UpdateGuardianContact":                                           189,
	"workflows/parentportal/legacy/parent_guardian_service.go:(*service).UpdateGuardianRelationship":                                      209,
	"workflows/parentportal/legacy/parent_guardian_service.go:projectChildGuardian":                                                       80,
	"workflows/parentportal/legacy/parent_master_data_request_service.go:(*service).SubmitMasterDataChangeRequest":                        148,
	"workflows/parentportal/legacy/parent_master_data_service.go:(*service).UpdateMasterDataField":                                        77,
	"workflows/parentportal/legacy/parent_master_data_service.go:(*service).applyGuardianPhoneEdit":                                       61,
	"workflows/parentportal/legacy/parent_master_data_service.go:(*service).applyGuardianProfileEdit":                                     66,
	"workflows/parentportal/legacy/parent_master_data_service.go:(*service).loadMasterData":                                               61,
	"workflows/parentportal/legacy/parent_related_accounts_service.go:(*service).InviteRelatedAccount":                                    62,
	"workflows/parentportal/legacy/parent_write_service.go:(*service).ChildFeatures":                                                      164,
	"workflows/parentportal/legacy/parent_write_service.go:(*service).DeleteCareException":                                                88,
	"workflows/parentportal/legacy/parent_write_service.go:(*service).SubmitPickupChangeRequest":                                          103,
	"workflows/parentportal/legacy/parent_write_service.go:(*service).SubmitSickNote":                                                     177,
	"workflows/parentportal/legacy/parent_write_service.go:(*service).submitCareException":                                                132,
	"workflows/parentportal/messaging/parent_announcement_service.go:(*Service).RespondToAnnouncement":                                    107,
	"workflows/parentportal/messaging/parent_announcement_service.go:(*Service).stampAnnouncement":                                        99,
	"workflows/parentportal/messaging/parent_messaging_service.go:(*Service).PostChildMessage":                                            104,
	"workflows/reminderdelivery/internal/application/batch.go:(*service).ComputeBatch":                                                    121,
	"workflows/reminderdelivery/internal/application/batch.go:(*service).loadBatchInputs":                                                 142,
	"workflows/reminderdelivery/internal/application/delivery.go:(*preparation).dispatchReminderPushes":                                   104,
	"workflows/reminderdelivery/internal/application/delivery.go:(*preparation).prepareReminderPushDispatch":                              101,
	"workflows/reminderdelivery/internal/application/service.go:(*service).Compute":                                                       92,
	"workflows/reminderdelivery/internal/application/service.go:(*service).pickupReminders":                                               64,
	"workflows/reminderdelivery/internal/application/service.go:buildActivityReminders":                                                   61,
	"workflows/sessionend/internal/application/command.go:(*command).EndSession":                                                          76,
	"workflows/staffoffboarding/compose/new.go:New":                                                                                       135,
	"workflows/staffoffboarding/offboarding.go:(*Workflow).snapshot":                                                                      71,
	"workflows/studentdeletion/compose/new.go:Assemble":                                                                                   133,
	"workflows/studentdeletion/deletion.go:(*Workflow).loadCounts":                                                                        69,
}

func TestModuleFunctionLengthRatchet(t *testing.T) {
	t.Parallel()

	backendRoot, err := findBackendRoot()
	if err != nil {
		t.Skipf("Could not find backend root: %v", err)
		return
	}

	lengths, err := moduleFuncLenScan(backendRoot)
	if err != nil {
		t.Fatalf("function-length scan failed: %v", err)
	}

	violations := moduleFuncLenViolations(lengths, moduleFuncLenAllowlist)
	if len(violations) > 0 {
		sort.Strings(violations)
		t.Errorf("Module function-length ratchet check failed (%d issue(s)):\n\n%s",
			len(violations), strings.Join(violations, "\n\n"))
	}
}

// moduleFuncLenViolations compares per-function body lengths over the
// threshold against the allowlist. Like the complexity ratchet it cannot reuse
// ratchetViolations: entries are capped lengths rather than hit counts, and an
// allowlisted function falling to or below the threshold leaves the scan
// entirely, which is the ratchet-down signal rather than a deleted file.
func moduleFuncLenViolations(lengths, allowlist map[string]int) []string {
	var violations []string
	for fn, got := range lengths {
		allowed, ok := allowlist[fn]
		switch {
		case !ok:
			violations = append(violations, fmt.Sprintf(
				"[func-length] %s has a %d-line body (limit %d), and is not in the allowlist.\n  Split the function into named steps — do not add new entries.",
				fn, got, moduleFuncLenThreshold))
		case got > allowed:
			violations = append(violations, fmt.Sprintf(
				"[func-length] %s has a %d-line body, allowed %d.\n  The function grew. Extract a step — never raise the allowlist.", fn, got, allowed))
		case got < allowed:
			violations = append(violations, fmt.Sprintf(
				"[func-length] %s has a %d-line body, allowlist says %d.\n  Nice — ratchet the entry down to %d so the improvement cannot regress.", fn, got, allowed, got))
		}
	}
	for fn, allowed := range allowlist {
		if _, ok := lengths[fn]; !ok {
			violations = append(violations, fmt.Sprintf(
				"[func-length] %s is allowlisted at %d lines but is now ≤ %d (or was deleted/renamed).\n  Remove the entry.",
				fn, allowed, moduleFuncLenThreshold))
		}
	}
	return violations
}

// moduleFuncLenScan returns the body length of every named function over the
// threshold in non-test .go files under modules/ and workflows/, keyed
// "relpath:FuncName" with methods rendered like the gocognit CLI. Declarations
// without a body (assembly stubs, cgo) are skipped; function literals are not
// counted.
func moduleFuncLenScan(backendRoot string) (map[string]int, error) {
	lengths := make(map[string]int)
	for _, sub := range []string{"modules", "workflows"} {
		root := filepath.Join(backendRoot, sub)
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if strings.HasPrefix(d.Name(), ".") && path != root {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			rel, relErr := filepath.Rel(backendRoot, path)
			if relErr != nil {
				return relErr
			}
			rel = filepath.ToSlash(rel)

			fset := token.NewFileSet()
			file, parseErr := parser.ParseFile(fset, path, nil, 0)
			if parseErr != nil {
				return fmt.Errorf("parse %s: %w", rel, parseErr)
			}
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				length := fset.Position(fn.Body.Rbrace).Line - fset.Position(fn.Body.Lbrace).Line + 1
				if length > moduleFuncLenThreshold {
					lengths[rel+":"+moduleFuncLenDisplayName(fn)] = length
				}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return lengths, nil
}

// moduleFuncLenDisplayName renders a function name the way the gocognit CLI
// does: plain functions as "name", methods as "(recv).name" / "(*recv).name".
func moduleFuncLenDisplayName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}
	return "(" + moduleFuncLenTypeExpr(fn.Recv.List[0].Type) + ")." + fn.Name.Name
}

func moduleFuncLenTypeExpr(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return "*" + moduleFuncLenTypeExpr(e.X)
	case *ast.IndexExpr: // generic receiver
		return moduleFuncLenTypeExpr(e.X)
	case *ast.IndexListExpr:
		return moduleFuncLenTypeExpr(e.X)
	default:
		return fmt.Sprintf("%T", expr)
	}
}
